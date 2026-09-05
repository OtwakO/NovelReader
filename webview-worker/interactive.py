"""Short-lived interactive browser contexts with bounded, fail-safe cleanup."""

from __future__ import annotations

import asyncio
import base64
import secrets
import time
from dataclasses import dataclass, field
from typing import Awaitable, Callable

from runtime import close_context


@dataclass
class InteractiveSession:
    context: object
    page: object
    created_at: float
    last_used_at: float
    lock: asyncio.Lock = field(default_factory=asyncio.Lock)


class InteractiveSessions:
    """Owns every live interactive context until idempotent close releases its slot."""

    def __init__(
        self,
        acquire: Callable[[dict], Awaitable[tuple[object, object, object]]],
        release: Callable[[object, bool], Awaitable[None]],
        idle_ttl_seconds: float,
        absolute_ttl_seconds: float,
        max_frame_bytes: int = 6 * 1024 * 1024,
        sweep_interval_seconds: float = 5,
    ):
        self._acquire = acquire
        self._release = release
        self._idle_ttl = idle_ttl_seconds
        self._absolute_ttl = absolute_ttl_seconds
        self._sweep_interval = sweep_interval_seconds
        self._max_frame_bytes = max_frame_bytes
        self._sessions: dict[str, InteractiveSession] = {}
        self._lock = asyncio.Lock()
        self._sweeper: asyncio.Task | None = None
        self._closing = False

    async def start(self) -> None:
        if self._sweeper is None:
            self._sweeper = asyncio.create_task(self._sweep(), name="webview-session-sweeper")

    async def create(self, request: dict) -> dict:
        if self._closing:
            raise ValueError("browser worker is shutting down")
        browser, context, page = await self._acquire(request)
        try:
            now = time.monotonic()
            session_id = secrets.token_urlsafe(24)
            async with self._lock:
                if self._closing:
                    raise ValueError("browser worker is shutting down")
                self._sessions[session_id] = InteractiveSession(context, page, now, now)
        except BaseException:
            browser_failed = not browser.is_connected()
            if not await close_context(context):
                browser_failed = True
            await self._release(browser, browser_failed)
            raise

        # Once registered, the session owns the context and its capacity slot.
        try:
            return await self.frame(session_id)
        except BaseException:
            await self.close(session_id)
            raise

    async def frame(self, session_id: str) -> dict:
        session = await self._get(session_id)
        async with session.lock:
            self._touch(session)
            image = await session.page.screenshot(type="jpeg", quality=95, scale="device")
            if len(image) > self._max_frame_bytes:
                for quality in (90, 85, 75):
                    image = await session.page.screenshot(type="jpeg", quality=quality, scale="device")
                    if len(image) <= self._max_frame_bytes:
                        break
                if len(image) > self._max_frame_bytes:
                    raise ValueError("interactive browser frame is too large")
            viewport = session.page.viewport_size or {"width": 390, "height": 720}
            return {
                "sessionId": session_id,
                "image": base64.b64encode(image).decode("ascii"),
                "mediaType": "image/jpeg",
                "width": viewport["width"],
                "height": viewport["height"],
                "url": session.page.url,
                "title": await session.page.title(),
            }

    async def input(self, session_id: str, event: dict) -> dict:
        session = await self._get(session_id)
        async with session.lock:
            self._touch(session)
            event_type = event.get("type")
            if event_type == "click":
                await session.page.mouse.click(float(event["x"]), float(event["y"]))
            elif event_type == "type":
                await session.page.keyboard.insert_text(str(event.get("text") or ""))
            elif event_type == "key":
                await session.page.keyboard.press(str(event["key"]))
            elif event_type == "scroll":
                delta_x = float(event.get("x") or 0)
                delta_y = float(event.get("y") or 0)
                before = await session.page.evaluate("() => ({ x: window.scrollX, y: window.scrollY })")
                after = await session.page.evaluate(
                    "([x, y]) => { window.scrollBy({ left: x, top: y, behavior: 'instant' }); return { x: window.scrollX, y: window.scrollY }; }",
                    [delta_x, delta_y],
                )
                if after == before:
                    await session.page.mouse.wheel(delta_x, delta_y)
            else:
                raise ValueError("unsupported browser input type")
        return await self.frame(session_id)

    async def close(self, session_id: str, save: bool = False, return_html: bool = False) -> dict:
        async with self._lock:
            session = self._sessions.pop(session_id, None)
        if session is None:
            return {"closed": True, "cookies": []}
        cookies = []
        html = ""
        browser = session.context.browser
        browser_failed = False
        try:
            async with session.lock:
                if save:
                    cookies = await session.context.cookies()
                if return_html:
                    html = await session.page.content()
                    if len(html.encode("utf-8")) > 512 * 1024:
                        raise ValueError("interactive browser HTML result is too large")
                final_url = session.page.url
        except Exception:
            browser_failed = not browser.is_connected()
            raise
        finally:
            if not await close_context(session.context):
                browser_failed = True
            await self._release(browser, browser_failed or not browser.is_connected())
        return {"closed": True, "cookies": cookies, "finalUrl": final_url, "html": html}

    async def close_all(self) -> None:
        self._closing = True
        if self._sweeper is not None:
            self._sweeper.cancel()
            await asyncio.gather(self._sweeper, return_exceptions=True)
            self._sweeper = None
        async with self._lock:
            session_ids = list(self._sessions)
        await asyncio.gather(*(self.close(session_id) for session_id in session_ids), return_exceptions=True)

    async def _get(self, session_id: str) -> InteractiveSession:
        async with self._lock:
            session = self._sessions.get(session_id)
        if session is None:
            raise KeyError("browser session not found")
        return session

    def _touch(self, session: InteractiveSession) -> None:
        session.last_used_at = time.monotonic()

    async def _sweep(self) -> None:
        while True:
            await asyncio.sleep(self._sweep_interval)
            now = time.monotonic()
            async with self._lock:
                expired = [
                    session_id
                    for session_id, session in self._sessions.items()
                    if now - session.last_used_at >= self._idle_ttl
                    or now - session.created_at >= self._absolute_ttl
                ]
            await asyncio.gather(*(self.close(session_id) for session_id in expired), return_exceptions=True)

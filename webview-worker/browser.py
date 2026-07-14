"""Bounded Patchright browser lifecycle and request execution."""

from __future__ import annotations

import asyncio
from urllib.parse import urlparse

from patchright.async_api import async_playwright

PROTOCOL_VERSION = 1
DEFAULT_TIMEOUT_MS = 30_000


class BrowserWorker:
    """Owns browser processes and bounds live contexts and queued requests."""

    def __init__(
        self,
        playwright,
        max_pages: int,
        max_pending: int,
        max_body_bytes: int,
        max_contexts: int,
    ):
        self.playwright = playwright
        self.max_body_bytes = max_body_bytes
        self.max_contexts = max_contexts
        self.queue: asyncio.Queue[tuple[dict, asyncio.Future]] = asyncio.Queue(maxsize=max_pending)
        self.browser = None
        self.state_lock = asyncio.Lock()
        self.active = 0
        self.completed = 0
        self.total_requests = 0
        self.total_completed = 0
        self.failed_requests = 0
        self.busy_rejections = 0
        self.recycled = 0
        self.closing = False
        self.consumers: list[asyncio.Task] = []
        self.max_pages = max_pages

    async def start(self) -> None:
        await self._launch_browser()
        self.consumers = [asyncio.create_task(self._consume()) for _ in range(self.max_pages)]

    async def submit(self, request: dict) -> dict:
        if self.closing:
            return self._error("browser worker is shutting down")
        future = asyncio.get_running_loop().create_future()
        try:
            self.queue.put_nowait((request, future))
        except asyncio.QueueFull:
            self.busy_rejections += 1
            return self._error("browser worker is busy")
        try:
            return await future
        except asyncio.CancelledError:
            future.cancel()
            raise

    async def close(self) -> None:
        self.closing = True
        while not self.queue.empty():
            _, future = self.queue.get_nowait()
            if not future.done():
                future.set_result(self._error("browser worker is shutting down"))
            self.queue.task_done()
        for consumer in self.consumers:
            consumer.cancel()
        await asyncio.gather(*self.consumers, return_exceptions=True)
        self.consumers.clear()
        async with self.state_lock:
            await self._close_browser()

    async def _consume(self) -> None:
        while True:
            request, future = await self.queue.get()
            try:
                if not future.cancelled():
                    future.set_result(await self._run(request))
            except asyncio.CancelledError:
                if not future.done():
                    future.set_result(self._error("browser worker is shutting down"))
                raise
            except Exception as error:
                if not future.done():
                    future.set_result(self._error(str(error)))
            finally:
                self.queue.task_done()

    async def _run(self, request: dict) -> dict:
        self.total_requests += 1
        timeout_ms = max(1, int(request.get("timeoutMs") or DEFAULT_TIMEOUT_MS))
        browser = await self._browser_for_request()
        async with self.state_lock:
            self.active += 1
        browser_failed = False
        try:
            async with asyncio.timeout(timeout_ms / 1000):
                return await self._execute(browser, request, timeout_ms)
        except TimeoutError:
            self.failed_requests += 1
            return self._error(f"browser request timed out after {timeout_ms}ms")
        except Exception as error:
            self.failed_requests += 1
            browser_failed = not browser.is_connected()
            return self._error(f"browser execution failed: {error}")
        finally:
            browser_failed = browser_failed or not browser.is_connected()
            await self._release_browser(browser, browser_failed)

    async def _browser_for_request(self):
        async with self.state_lock:
            if self.browser is None or not self.browser.is_connected():
                await self._close_browser()
                await self._launch_browser()
            return self.browser

    async def _release_browser(self, browser, browser_failed: bool) -> None:
        async with self.state_lock:
            self.active -= 1
            self.completed += 1
            self.total_completed += 1
            should_recycle = (
                self.active == 0
                and (browser_failed or self.completed >= self.max_contexts)
            )
            if should_recycle and self.browser is browser:
                self.completed = 0
                self.recycled += 1
                await self._close_browser()

    async def _launch_browser(self) -> None:
        self.browser = await self.playwright.chromium.launch(headless=True)

    async def _close_browser(self) -> None:
        if self.browser is not None:
            browser, self.browser = self.browser, None
            try:
                await browser.close()
            except Exception:
                pass

    async def _execute(self, browser, request: dict, timeout_ms: int) -> dict:
        target = request.get("url", "")
        if urlparse(target).scheme not in {"http", "https"}:
            return self._error("only HTTP(S) URLs are supported")
        if request.get("dnsIp"):
            return self._error("dnsIp is unsupported by browser transport")

        context = None
        try:
            context = await browser.new_context(
                extra_http_headers=request.get("headers") or {}
            )
            cookies = browser_cookies(request.get("cookies") or [], target)
            if cookies:
                try:
                    await context.add_cookies(cookies)
                except Exception as exc:
                    raise ValueError(f"cookie input rejected ({len(cookies)} cookies): {exc}") from exc
            page = await context.new_page()
            navigation_urls: list[str] = []
            page.on(
                "request",
                lambda event: navigation_urls.append(event.url)
                if event.is_navigation_request()
                else None,
            )
            method = (request.get("method") or "GET").upper()
            response = None
            if method == "GET":
                response = await page.goto(
                    target, wait_until="domcontentloaded", timeout=timeout_ms
                )
                await settle_page(page, timeout_ms)
            else:
                response = await context.request.fetch(
                    target,
                    method=method,
                    headers=request.get("headers") or {},
                    data=request.get("body") or None,
                    timeout=timeout_ms,
                    fail_on_status_code=False,
                )
                headers = response_headers(response)
                body = decode_response_body(await response.body(), request, headers)
                ensure_body_size(body, self.max_body_bytes)
                await page.set_content(
                    with_base_url(body, target),
                    wait_until="domcontentloaded",
                    timeout=timeout_ms,
                )

            delay_ms = max(0, int(request.get("delayMs") or 0))
            if delay_ms:
                await asyncio.sleep(delay_ms / 1000)
            script = request.get("webJs") or ""
            if script:
                await page.evaluate(
                    "async () => {" + script + "}", isolated_context=False
                )
                await settle_page(page, timeout_ms)

            body = await page.content()
            ensure_body_size(body, self.max_body_bytes)
            headers = await response_headers_async(response)
            final_url = page.url
            if not final_url or final_url == "about:blank":
                final_url = getattr(response, "url", None) or target
            return {
                "version": PROTOCOL_VERSION,
                "statusCode": response.status if response else 200,
                "headers": {key: [value] for key, value in headers.items()},
                "body": body,
                "finalUrl": final_url,
                "redirectChain": navigation_urls[1:],
                "cookies": protocol_cookies(await context.cookies()),
            }
        finally:
            if context is not None:
                try:
                    await asyncio.shield(asyncio.wait_for(context.close(), timeout=2))
                except BaseException:
                    # Cancellation must not strand a live browser context.
                    pass

    async def health(self) -> dict:
        async with self.state_lock:
            return {
                "version": PROTOCOL_VERSION,
                "ok": not self.closing and self.browser is not None and self.browser.is_connected(),
                "queueDepth": self.queue.qsize(),
                "active": self.active,
                "totalRequests": self.total_requests,
                "completedRequests": self.total_completed,
                "failedRequests": self.failed_requests,
                "busyRejections": self.busy_rejections,
                "browserRecycles": self.recycled,
            }

    @staticmethod
    def _error(message: str) -> dict:
        return {"version": PROTOCOL_VERSION, "error": message}


async def settle_page(page, timeout_ms: int) -> None:
    try:
        await page.wait_for_load_state("networkidle", timeout=min(timeout_ms, 5_000))
    except Exception:
        # Long-polling pages never become idle; their DOM is still usable.
        pass


def browser_cookies(cookies: list[dict], target: str) -> list[dict]:
    result = []
    for cookie in cookies:
        item = {
            "name": cookie["name"],
            "value": cookie["value"],
            "httpOnly": bool(cookie.get("httpOnly")),
            "secure": bool(cookie.get("secure")),
        }
        if cookie.get("domain"):
            item["domain"] = cookie["domain"]
            item["path"] = cookie.get("path") or "/"
        else:
            item["url"] = cookie.get("url") or target
        if cookie.get("expires"):
            item["expires"] = cookie["expires"]
        result.append(item)
    return result


def protocol_cookies(cookies: list[dict]) -> list[dict]:
    return [
        {
            "name": cookie["name"],
            "value": cookie["value"],
            "domain": cookie.get("domain", ""),
            "path": cookie.get("path") or "/",
            "expires": cookie.get("expires", -1),
            "httpOnly": cookie.get("httpOnly", False),
            "secure": cookie.get("secure", False),
        }
        for cookie in cookies
    ]


def with_base_url(body: str, target: str) -> str:
    base = f'<base href="{target}">'
    lower = body.lower()
    head = lower.find("<head")
    if head >= 0:
        end = body.find(">", head)
        if end >= 0:
            return body[: end + 1] + base + body[end + 1 :]
    return base + body


def ensure_body_size(body: str, max_bytes: int) -> None:
    if len(body.encode("utf-8")) > max_bytes:
        raise ValueError(f"response body exceeds {max_bytes} bytes")


def response_headers(response) -> dict[str, str]:
    """Read headers from Patchright APIResponse without requiring Playwright-only methods."""
    return dict(getattr(response, "headers", {}) or {})


async def response_headers_async(response) -> dict[str, str]:
    if response is None:
        return {}
    method = getattr(response, "all_headers", None)
    if callable(method):
        return dict(await method())
    return response_headers(response)


def decode_response_body(raw: bytes, request: dict, headers: dict[str, str]) -> str:
    """Decode fetched HTML using the source charset before loading it into the DOM."""
    charset = request.get("charset") or response_charset(headers) or "utf-8"
    try:
        return raw.decode(charset, errors="replace")
    except LookupError:
        return raw.decode("utf-8", errors="replace")


def response_charset(headers: dict[str, str]) -> str:
    content_type = headers.get("content-type", "")
    marker = "charset="
    lower = content_type.lower()
    if marker not in lower:
        return ""
    return content_type[lower.index(marker) + len(marker):].split(";", 1)[0].strip()

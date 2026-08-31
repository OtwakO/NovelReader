"""Bounded Patchright browser lifecycle and request execution."""

from __future__ import annotations

import asyncio
import base64
import binascii
import contextlib
import re
from urllib.parse import urlparse

from patchright.async_api import async_playwright

from interactive import InteractiveSessions

PROTOCOL_VERSION = 3
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
        interactive_idle_seconds: float = 120,
        interactive_absolute_seconds: float = 600,
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
        self.capacity = asyncio.Semaphore(max_pages)
        self.interactive = InteractiveSessions(
            self._open_interactive_context,
            self._release_browser,
            interactive_idle_seconds,
            interactive_absolute_seconds,
            max_frame_bytes=min(8 * 1024 * 1024, max(1024 * 1024, max_body_bytes * 3 // 5)),
        )

    async def start(self) -> None:
        await self._launch_browser()
        self.consumers = [asyncio.create_task(self._consume()) for _ in range(self.max_pages)]
        await self.interactive.start()

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
        await self.interactive.close_all()
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
        await self.capacity.acquire()
        browser = None
        acquired = False
        browser_failed = False
        try:
            browser = await self._browser_for_request()
            async with self.state_lock:
                self.active += 1
            acquired = True
            async with asyncio.timeout(timeout_ms / 1000):
                return await self._execute(browser, request, timeout_ms)
        except TimeoutError:
            self.failed_requests += 1
            return self._error(f"browser request timed out after {timeout_ms}ms")
        except Exception as error:
            self.failed_requests += 1
            browser_failed = browser is not None and not browser.is_connected()
            return self._error(f"browser execution failed: {error}")
        finally:
            if acquired:
                browser_failed = browser_failed or not browser.is_connected()
                await self._release_browser(browser, browser_failed)
            else:
                self.capacity.release()

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
        self.capacity.release()

    async def _open_interactive_context(self, request: dict):
        target = request.get("url", "")
        scheme = urlparse(target).scheme
        if scheme not in {"http", "https", "data"}:
            raise ValueError("only HTTP(S) and HTML data URLs are supported")
        document = None
        if scheme == "data":
            prefix = "data:text/html;base64,"
            if not target.startswith(prefix):
                raise ValueError("only base64 HTML data URLs are supported")
            try:
                document_bytes = base64.b64decode(target[len(prefix):], validate=True)
                document = document_bytes.decode("utf-8")
            except (binascii.Error, UnicodeDecodeError, ValueError) as error:
                raise ValueError("invalid base64 HTML data URL") from error
            if not document_bytes or len(document_bytes) > 512 * 1024:
                raise ValueError("HTML data document is empty or too large")
        await self.capacity.acquire()
        browser = None
        context = None
        acquired = False
        try:
            browser = await self._browser_for_request()
            async with self.state_lock:
                self.active += 1
            acquired = True
            viewport = request.get("viewport") or {}
            device_scale_factor = min(2.0, max(1.0, float(viewport.get("deviceScaleFactor") or 1)))
            context = await browser.new_context(
                extra_http_headers=request.get("headers") or {},
                device_scale_factor=device_scale_factor,
            )
            cookies = browser_cookies(request.get("cookies") or [], target)
            if cookies:
                await context.add_cookies(cookies)
            page = await context.new_page()
            await page.set_viewport_size({
                "width": min(1920, max(320, int(viewport.get("width") or 390))),
                "height": min(1440, max(320, int(viewport.get("height") or 720))),
            })
            timeout_ms = int(request.get("timeoutMs") or DEFAULT_TIMEOUT_MS)
            if document is not None:
                await page.set_content(document, wait_until="domcontentloaded", timeout=timeout_ms)
            else:
                await page.goto(target, wait_until="domcontentloaded", timeout=timeout_ms)
            return browser, context, page
        except BaseException:
            if context is not None:
                try:
                    await asyncio.shield(asyncio.wait_for(context.close(), timeout=2))
                except BaseException:
                    pass
            if acquired:
                await self._release_browser(browser, not browser.is_connected())
            else:
                self.capacity.release()
            raise

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
        if request.get("probe") is True:
            return await self._probe(browser, timeout_ms)
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
            source_pattern = compile_source_regex(request.get("sourceRegex"))
            source_match = asyncio.get_running_loop().create_future() if source_pattern else None
            page.on(
                "request",
                lambda event: navigation_urls.append(event.url)
                if event.is_navigation_request()
                else None,
            )
            if source_match is not None:
                page.on(
                    "request",
                    lambda event: capture_source_match(source_pattern, source_match, event.url),
                )
            method = (request.get("method") or "GET").upper()
            response = None
            if method == "GET":
                matched_url, response = await await_or_source_match(
                    page.goto(target, wait_until="domcontentloaded", timeout=timeout_ms),
                    source_match,
                )
                if matched_url:
                    return await sniffed_source_response(matched_url, target, context)
                matched_url, _ = await await_or_source_match(
                    settle_page(page, timeout_ms), source_match
                )
                if matched_url:
                    return await sniffed_source_response(matched_url, target, context)
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
                matched_url, _ = await await_or_source_match(
                    page.set_content(
                        with_base_url(body, target),
                        wait_until="domcontentloaded",
                        timeout=timeout_ms,
                    ),
                    source_match,
                )
                if matched_url:
                    return await sniffed_source_response(matched_url, target, context)

            if source_match is not None and source_match.done():
                return await sniffed_source_response(await source_match, target, context)

            delay_ms = max(0, int(request.get("delayMs") or 0))
            if delay_ms:
                matched_url, _ = await await_or_source_match(
                    asyncio.sleep(delay_ms / 1000), source_match
                )
                if matched_url:
                    return await sniffed_source_response(matched_url, target, context)
            script = request.get("webJs") or ""
            if script:
                matched_url, _ = await await_or_source_match(
                    page.evaluate("async () => {" + script + "}", isolated_context=False),
                    source_match,
                )
                if matched_url:
                    return await sniffed_source_response(matched_url, target, context)
                matched_url, _ = await await_or_source_match(
                    settle_page(page, timeout_ms), source_match
                )
                if matched_url:
                    return await sniffed_source_response(matched_url, target, context)

            if source_match is not None:
                if source_match.done():
                    return await sniffed_source_response(await source_match, target, context)
                try:
                    matched_url = await asyncio.wait_for(
                        asyncio.shield(source_match), timeout=max(0.001, timeout_ms / 1000)
                    )
                    return await sniffed_source_response(matched_url, target, context)
                except TimeoutError:
                    raise ValueError("sourceRegex did not match a loaded resource URL")

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

    async def _probe(self, browser, timeout_ms: int) -> dict:
        context = None
        try:
            context = await browser.new_context()
            page = await context.new_page()
            await page.set_content(
                "<main id='novelreader-webview-probe'>ready</main>",
                wait_until="domcontentloaded",
                timeout=timeout_ms,
            )
            marker = await page.locator("#novelreader-webview-probe").text_content()
            return {
                "version": PROTOCOL_VERSION,
                "statusCode": 200,
                "body": "novelreader-webview-ok" if marker == "ready" else "",
                "finalUrl": "about:blank",
            }
        finally:
            if context is not None:
                try:
                    await asyncio.shield(asyncio.wait_for(context.close(), timeout=2))
                except BaseException:
                    pass

    async def health(self) -> dict:
        async with self.state_lock:
            consumers_ready = len(self.consumers) == self.max_pages and all(
                not consumer.done() for consumer in self.consumers
            )
            return {
                "version": PROTOCOL_VERSION,
                "ok": not self.closing and consumers_ready,
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


async def await_or_source_match(awaitable, source_match) -> tuple[str | None, object]:
    operation = asyncio.create_task(awaitable)
    if source_match is None:
        return None, await operation
    done, _ = await asyncio.wait(
        {operation, source_match}, return_when=asyncio.FIRST_COMPLETED
    )
    if source_match in done:
        operation.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await operation
        return source_match.result(), None
    return None, await operation


def capture_source_match(pattern, match_future, resource_url: str) -> None:
    if not match_future.done() and pattern.fullmatch(resource_url):
        match_future.set_result(resource_url)


def compile_source_regex(value: object):
    text = str(value or "").strip()
    if not text:
        return None
    try:
        return re.compile(text)
    except re.error as exc:
        raise ValueError(f"invalid sourceRegex: {exc}") from exc


async def sniffed_source_response(matched_url: str, target: str, context) -> dict:
    return {
        "version": PROTOCOL_VERSION,
        "statusCode": 200,
        "headers": {},
        "body": matched_url,
        "finalUrl": target,
        "redirectChain": [],
        "cookies": protocol_cookies(await context.cookies()),
    }


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

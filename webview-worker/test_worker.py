"""Capacity configuration checks for the headless browser worker."""

import asyncio
import base64
import os
import unittest
from unittest.mock import AsyncMock, MagicMock, call, patch

from patchright.async_api import async_playwright

from interactive import InteractiveSessions

from browser import (
    BrowserWorker,
    await_or_source_match,
    capture_source_match,
    compile_source_regex,
    mediate_data_document_request,
    require_public_host,
    require_public_response,
)
from worker import capacity_settings


class BrowserWorkerHealthTest(unittest.IsolatedAsyncioTestCase):
    async def test_idle_recycled_browser_is_ready_when_consumers_are_alive(self) -> None:
        worker = BrowserWorker(None, max_pages=2, max_pending=8, max_body_bytes=1024, max_contexts=100)
        worker.browser = None
        worker.consumers = [MagicMock(done=lambda: False), MagicMock(done=lambda: False)]

        self.assertTrue((await worker.health())["ok"])
        worker.consumers[0].done = lambda: True
        self.assertFalse((await worker.health())["ok"])


class BrowserWorkerProbeTest(unittest.IsolatedAsyncioTestCase):
    async def test_probe_creates_context_and_evaluates_marker(self) -> None:
        page = MagicMock()
        page.set_content = AsyncMock()
        page.locator.return_value.text_content = AsyncMock(return_value="ready")
        context = MagicMock()
        context.new_page = AsyncMock(return_value=page)
        context.close = AsyncMock()
        browser = MagicMock()
        browser.new_context = AsyncMock(return_value=context)
        worker = BrowserWorker(None, max_pages=1, max_pending=1, max_body_bytes=1024, max_contexts=100)

        result = await worker._execute(browser, {"probe": True}, 1000)

        self.assertEqual(result["body"], "novelreader-webview-ok")
        browser.new_context.assert_awaited_once()
        context.close.assert_awaited_once()


class SourceRegexTest(unittest.TestCase):
    def test_uses_full_url_match(self) -> None:
        pattern = compile_source_regex(r"https://cdn\.test/.*\.(mp3|m4a)(\?.*)?")

        self.assertTrue(pattern.fullmatch("https://cdn.test/chapter.m4a?token=one"))
        self.assertFalse(pattern.fullmatch("prefix https://cdn.test/chapter.m4a?token=one"))

    def test_first_matching_resource_wins(self) -> None:
        async def exercise() -> str:
            pattern = compile_source_regex(r".*\.(mp3|m4a).*")
            match = asyncio.get_running_loop().create_future()
            capture_source_match(pattern, match, "https://cdn.test/first.mp3")
            capture_source_match(pattern, match, "https://cdn.test/second.m4a")
            return match.result()

        self.assertEqual(asyncio.run(exercise()), "https://cdn.test/first.mp3")

    def test_match_interrupts_pending_browser_operation(self) -> None:
        async def exercise() -> tuple[str | None, object]:
            match = asyncio.get_running_loop().create_future()

            async def pending() -> str:
                await asyncio.sleep(10)
                return "late"

            operation = asyncio.create_task(await_or_source_match(pending(), match))
            await asyncio.sleep(0)
            match.set_result("https://cdn.test/first.mp3")
            return await operation

        self.assertEqual(asyncio.run(exercise()), ("https://cdn.test/first.mp3", None))

    def test_invalid_regex_is_rejected(self) -> None:
        with self.assertRaisesRegex(ValueError, "invalid sourceRegex"):
            compile_source_regex("[")

    def test_blank_regex_disables_sniffing(self) -> None:
        self.assertIsNone(compile_source_regex("  "))


class CapacitySettingsTest(unittest.TestCase):
    def test_small_container_defaults(self) -> None:
        with patch.dict(os.environ, {}, clear=True):
            self.assertEqual(capacity_settings(), (2, 8, 10 * 1024 * 1024, 100, 120, 600))

    def test_overrides_and_clamps_admission_to_one(self) -> None:
        values = {
            "WEBVIEW_MAX_PAGES": "4",
            "WEBVIEW_MAX_PENDING": "0",
            "WEBVIEW_MAX_BODY_BYTES": "2048",
            "WEBVIEW_MAX_CONTEXTS_PER_BROWSER": "250",
        }
        with patch.dict(os.environ, values, clear=True):
            self.assertEqual(capacity_settings(), (4, 1, 2048, 250, 120, 600))


if __name__ == "__main__":
    unittest.main()


class InteractiveSessionsTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.browser = MagicMock()
        self.browser.is_connected.return_value = True
        self.context = MagicMock()
        self.context.browser = self.browser
        self.context.close = AsyncMock()
        self.context.cookies = AsyncMock(return_value=[{"name": "login", "value": "ready"}])
        self.page = MagicMock()
        self.page.url = "https://example.test/account"
        self.page.viewport_size = {"width": 390, "height": 720}
        self.page.screenshot = AsyncMock(return_value=b"frame")
        self.page.title = AsyncMock(return_value="Account")
        self.page.content = AsyncMock(return_value="<html><body>Account</body></html>")
        self.page.mouse.wheel = AsyncMock()
        self.acquire = AsyncMock(return_value=(self.browser, self.context, self.page))
        self.release = AsyncMock()
        self.sessions = InteractiveSessions(self.acquire, self.release, 60, 600, sweep_interval_seconds=0.01)
        await self.sessions.start()

    async def asyncTearDown(self) -> None:
        await self.sessions.close_all()

    async def test_frame_uses_high_quality_device_scale_jpeg(self) -> None:
        created = await self.sessions.create({"url": "https://example.test"})
        self.page.screenshot.assert_awaited_with(type="jpeg", quality=95, scale="device")
        self.assertEqual(created["mediaType"], "image/jpeg")
        await self.sessions.close(created["sessionId"])

    async def test_large_jpeg_retries_at_lower_quality(self) -> None:
        sessions = InteractiveSessions(self.acquire, self.release, 60, 600, max_frame_bytes=4)
        self.page.screenshot = AsyncMock(side_effect=[b"oversized", b"jpeg"])
        await sessions.start()
        try:
            created = await sessions.create({"url": "https://example.test"})
            self.assertEqual(created["mediaType"], "image/jpeg")
            self.page.screenshot.assert_has_awaits([call(type="jpeg", quality=95, scale="device"), call(type="jpeg", quality=90, scale="device")])
            await sessions.close(created["sessionId"])
        finally:
            await sessions.close_all()

    async def test_scroll_moves_document_instantly_before_falling_back_to_mouse_wheel(self) -> None:
        created = await self.sessions.create({"url": "https://example.test"})
        self.page.evaluate = AsyncMock(side_effect=[{"x": 0, "y": 0}, {"x": 0, "y": 900}])
        await self.sessions.input(created["sessionId"], {"type": "scroll", "y": 900})
        self.page.mouse.wheel.assert_not_awaited()
        scroll_call = self.page.evaluate.await_args_list[1]
        self.assertIn("behavior: 'instant'", scroll_call.args[0])
        await self.sessions.close(created["sessionId"])

    async def test_scroll_falls_back_for_nested_scrollers(self) -> None:
        created = await self.sessions.create({"url": "https://example.test"})
        self.page.evaluate = AsyncMock(side_effect=[{"x": 0, "y": 0}, {"x": 0, "y": 0}])
        await self.sessions.input(created["sessionId"], {"type": "scroll", "y": 900})
        self.page.mouse.wheel.assert_awaited_once_with(0, 900)
        await self.sessions.close(created["sessionId"])

    async def test_close_without_continuation_does_not_capture_html(self) -> None:
        created = await self.sessions.create({"url": "https://example.test"})
        await self.sessions.close(created["sessionId"], save=True, return_html=False)
        self.page.content.assert_not_awaited()

    async def test_close_rejects_oversized_html_and_releases(self) -> None:
        created = await self.sessions.create({"url": "https://example.test"})
        self.page.content = AsyncMock(return_value="x" * (512 * 1024 + 1))
        with self.assertRaisesRegex(ValueError, "HTML result is too large"):
            await self.sessions.close(created["sessionId"], save=True, return_html=True)
        self.context.close.assert_awaited_once()
        self.release.assert_awaited_once()

    async def test_close_is_idempotent_and_releases_once(self) -> None:
        created = await self.sessions.create({"url": "https://example.test"})

        result = await self.sessions.close(created["sessionId"], save=True, return_html=True)
        await self.sessions.close(created["sessionId"], save=True, return_html=True)

        self.assertEqual(result["cookies"][0]["name"], "login")
        self.context.close.assert_awaited_once()
        self.release.assert_awaited_once_with(self.browser, False)

    async def test_expired_session_closes_without_client_request(self) -> None:
        self.sessions._idle_ttl = 0
        created = await self.sessions.create({"url": "https://example.test"})

        await asyncio.sleep(0.03)

        with self.assertRaises(KeyError):
            await self.sessions.frame(created["sessionId"])
        self.context.close.assert_awaited_once()
        self.release.assert_awaited_once()

    async def test_shutdown_closes_every_registered_context(self) -> None:
        await self.sessions.create({"url": "https://example.test"})

        await self.sessions.close_all()

        self.context.close.assert_awaited_once()
        self.release.assert_awaited_once()


class WorkerCapacityFailureTest(unittest.IsolatedAsyncioTestCase):
    async def test_browser_launch_failure_releases_capacity(self) -> None:
        playwright = MagicMock()
        playwright.chromium.launch = AsyncMock(side_effect=RuntimeError("launch failed"))
        worker = BrowserWorker(playwright, 1, 1, 1024, 10)

        result = await worker._run({"url": "https://example.test"})

        self.assertIn("launch failed", result["error"])
        self.assertEqual(worker.capacity._value, 1)


class InteractiveConstructionFailureTest(unittest.IsolatedAsyncioTestCase):
    async def test_failed_navigation_closes_partial_context_and_releases_capacity(self) -> None:
        browser = MagicMock()
        browser.is_connected.return_value = True
        context = MagicMock()
        context.close = AsyncMock()
        context.add_cookies = AsyncMock()
        page = MagicMock()
        page.set_viewport_size = AsyncMock()
        page.goto = AsyncMock(side_effect=RuntimeError("navigation failed"))
        context.new_page = AsyncMock(return_value=page)
        browser.new_context = AsyncMock(return_value=context)
        playwright = MagicMock()
        playwright.chromium.launch = AsyncMock(return_value=browser)
        worker = BrowserWorker(playwright, 1, 1, 1024, 10)
        await worker.start()
        try:
            with self.assertRaisesRegex(RuntimeError, "navigation failed"):
                await worker._open_interactive_context({"url": "https://example.test"})
            context.close.assert_awaited_once()
            self.assertEqual(worker.capacity._value, 1)
        finally:
            await worker.close()


class DataDocumentRequestMediationTest(unittest.IsolatedAsyncioTestCase):
    async def test_fetch_request_is_fulfilled_without_browser_cors(self) -> None:
        response = MagicMock()
        response.status = 200
        response.headers = {"content-type": "text/plain", "access-control-allow-origin": "https://other.test"}
        response.server_addr = AsyncMock(return_value={"ipAddress": "8.8.8.8", "port": 443})
        response.body = AsyncMock(return_value=b"online")
        route = MagicMock()
        route.request.url = "https://route.example.test/status"
        route.request.resource_type = "fetch"
        route.fetch = AsyncMock(return_value=response)
        route.fulfill = AsyncMock()

        with patch("browser.require_public_host", new=AsyncMock()):
            await mediate_data_document_request(route, 1024, 3000)

        route.fetch.assert_awaited_once_with(timeout=3000, max_redirects=0)
        fulfilled = route.fulfill.await_args.kwargs
        self.assertEqual(fulfilled["status"], 200)
        self.assertEqual(fulfilled["body"], b"online")
        self.assertEqual(fulfilled["headers"]["access-control-allow-origin"], "null")

    async def test_non_http_and_non_fetch_requests_are_not_mediated(self) -> None:
        for url, resource_type, expected in [
            ("data:text/plain,local", "fetch", "abort"),
            ("https://route.example.test/script.js", "script", "continue_"),
        ]:
            route = MagicMock()
            route.request.url = url
            route.request.resource_type = resource_type
            route.abort = AsyncMock()
            route.continue_ = AsyncMock()
            with patch("browser.require_public_host", new=AsyncMock()):
                await mediate_data_document_request(route, 1024, 3000)
            getattr(route, expected).assert_awaited_once()
            route.fetch.assert_not_called()

    async def test_redirect_response_is_aborted(self) -> None:
        response = MagicMock()
        response.status = 302
        response.server_addr = AsyncMock(return_value={"ipAddress": "8.8.8.8", "port": 443})
        route = MagicMock()
        route.request.url = "https://route.example.test/status"
        route.request.resource_type = "fetch"
        route.fetch = AsyncMock(return_value=response)
        route.abort = AsyncMock()

        with patch("browser.require_public_host", new=AsyncMock()):
            await mediate_data_document_request(route, 1024, 3000)

        response.body.assert_not_called()
        route.abort.assert_awaited_once_with("failed")

    async def test_oversized_response_is_aborted(self) -> None:
        response = MagicMock()
        response.status = 200
        response.headers = {}
        response.body = AsyncMock(return_value=b"too large")
        route = MagicMock()
        route.request.url = "https://route.example.test/status"
        route.request.resource_type = "xhr"
        route.fetch = AsyncMock(return_value=response)
        route.abort = AsyncMock()

        with patch("browser.require_public_host", new=AsyncMock()):
            await mediate_data_document_request(route, 4, 3000)

        route.abort.assert_awaited_once_with("failed")
        route.fulfill.assert_not_called()


    async def test_private_network_target_is_aborted_before_fetch(self) -> None:
        route = MagicMock()
        route.request.url = "http://127.0.0.1/private"
        route.request.resource_type = "fetch"
        route.fetch = AsyncMock()
        route.abort = AsyncMock()

        await mediate_data_document_request(route, 1024, 3000)

        route.fetch.assert_not_awaited()
        route.abort.assert_awaited_once_with("failed")

    async def test_response_address_rejects_dns_rebinding(self) -> None:
        response = MagicMock()
        response.server_addr = AsyncMock(return_value={"ipAddress": "127.0.0.1", "port": 443})
        with self.assertRaisesRegex(ValueError, "non-public"):
            await require_public_response(response)

    async def test_public_host_check_rejects_any_non_public_resolution(self) -> None:
        loop = MagicMock()
        loop.getaddrinfo = AsyncMock(return_value=[
            (None, None, None, None, ("8.8.8.8", 443)),
            (None, None, None, None, ("127.0.0.1", 443)),
        ])
        with patch("browser.asyncio.get_running_loop", return_value=loop):
            with self.assertRaisesRegex(ValueError, "non-public"):
                await require_public_host("mixed.example", 443)


class DataDocumentChromiumRegressionTest(unittest.IsolatedAsyncioTestCase):
    async def test_opaque_document_fetch_succeeds_through_mediator(self) -> None:
        async def handler(reader, writer) -> None:
            await reader.readuntil(b"\r\n\r\n")
            body = b"online"
            writer.write(b"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 6\r\nConnection: close\r\n\r\n" + body)
            await writer.drain()
            writer.close()
            await writer.wait_closed()

        server = await asyncio.start_server(handler, "127.0.0.1", 0)
        port = server.sockets[0].getsockname()[1]
        document = f"<script>fetch('http://127.0.0.1:{port}/status').then(r=>r.text()).then(v=>document.body.textContent=v).catch(e=>document.body.textContent=e.name)</script>"
        target = "data:text/html;base64," + base64.b64encode(document.encode()).decode()
        async with async_playwright() as playwright:
            worker = BrowserWorker(playwright, 1, 1, 1024, 10)
            await worker.start()
            try:
                with patch("browser.require_public_host", new=AsyncMock()), patch("browser.require_public_response", new=AsyncMock()):
                    browser, context, page = await worker._open_interactive_context({"url": target, "timeoutMs": 5000})
                    await page.wait_for_function("document.body.textContent.length > 0", timeout=5000)
                    self.assertEqual(await page.text_content("body"), "online")
                await context.close()
                await worker._release_browser(browser, False)
            finally:
                await worker.close()
                server.close()
                await server.wait_closed()


class InteractiveDataURLTest(unittest.IsolatedAsyncioTestCase):
    async def test_html_data_document_is_opened(self) -> None:
        browser = MagicMock()
        browser.is_connected.return_value = True
        context = MagicMock()
        context.close = AsyncMock()
        context.add_cookies = AsyncMock()
        page = MagicMock()
        page.set_viewport_size = AsyncMock()
        page.goto = AsyncMock()
        page.route = AsyncMock()
        page.set_content = AsyncMock()
        context.new_page = AsyncMock(return_value=page)
        browser.new_context = AsyncMock(return_value=context)
        playwright = MagicMock()
        playwright.chromium.launch = AsyncMock(return_value=browser)
        worker = BrowserWorker(playwright, 1, 1, 1024, 10)
        await worker.start()
        try:
            target = "data:text/html;base64,PGgxPlNldHRpbmdzPC9oMT4="
            _, opened_context, _ = await worker._open_interactive_context({"url": target, "viewport": {"width": 1200, "height": 800, "deviceScaleFactor": 2}})
            browser.new_context.assert_awaited_once_with(extra_http_headers={}, device_scale_factor=2)
            page.route.assert_awaited_once()
            page.set_content.assert_awaited_once_with("<h1>Settings</h1>", wait_until="domcontentloaded", timeout=30000)
            page.goto.assert_not_awaited()
            await opened_context.close()
            await worker._release_browser(browser, False)
        finally:
            await worker.close()

    async def test_non_html_data_document_is_rejected(self) -> None:
        worker = BrowserWorker(MagicMock(), 1, 1, 1024, 10)
        with self.assertRaisesRegex(ValueError, "base64 HTML"):
            await worker._open_interactive_context({"url": "data:text/javascript;base64,YWxlcnQoMSk="})

    async def test_invalid_html_data_document_is_rejected_before_browser_launch(self) -> None:
        playwright = MagicMock()
        playwright.chromium.launch = AsyncMock()
        worker = BrowserWorker(playwright, 1, 1, 1024, 10)
        with self.assertRaisesRegex(ValueError, "invalid base64"):
            await worker._open_interactive_context({"url": "data:text/html;base64,not-base64"})
        playwright.chromium.launch.assert_not_awaited()

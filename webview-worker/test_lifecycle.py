"""Ownership and admission regressions; no live websites or user sessions."""

import asyncio
import os
import subprocess
import sys
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

from browser import BrowserWorker
from interactive import InteractiveSessions
from runtime import WorkerBusyError
from worker import serve_connection


class LifecycleTest(unittest.IsolatedAsyncioTestCase):
    async def test_stalled_response_has_a_deadline_and_does_not_write_twice(self):
        reader = asyncio.StreamReader()
        reader.feed_data(b"GET /healthz HTTP/1.1\r\n\r\n")
        service = MagicMock()
        service.closing = False
        service.health = AsyncMock(return_value={"ok": True})
        writer = MagicMock()

        async def stalled_writer():
            await asyncio.Event().wait()

        writer.drain = AsyncMock(side_effect=stalled_writer)
        writer.wait_closed = AsyncMock()
        with patch("worker.DEFAULT_TIMEOUT_MS", 10):
            with self.assertRaisesRegex(ConnectionError, "write timed out"):
                await serve_connection(reader, writer, service)
        writer.write.assert_called_once()
        writer.close.assert_called_once()

    async def test_uncertain_browser_teardown_retains_owner_and_stops_replacement(self):
        async def hang():
            await asyncio.Event().wait()

        async def ignore_cancellation():
            try:
                await asyncio.Event().wait()
            except asyncio.CancelledError:
                await finish.wait()

        for failure in (RuntimeError("close failed"), hang, ignore_cancellation):
            with self.subTest(failure=type(failure).__name__):
                finish = asyncio.Event()
                browser = MagicMock()
                browser.close = AsyncMock(side_effect=failure)
                browser.is_connected.return_value = True
                playwright = MagicMock()
                worker = BrowserWorker(playwright, 1, 2, 1024, 1)
                worker.browser = browser
                worker.active = 1
                await worker.capacity.acquire()
                with patch("browser.BROWSER_CLOSE_TIMEOUT_SECONDS", 0.01):
                    with self.assertRaisesRegex(RuntimeError, "teardown"):
                        await worker._release_browser(browser, False)
                self.assertIs(worker.browser, browser)
                self.assertTrue(worker.shutdown_requested.is_set())
                self.assertFalse((await worker.health())["ok"])
                self.assertEqual(worker.capacity._value, 1)
                with self.assertRaisesRegex(RuntimeError, "shutting down"):
                    await worker._browser_for_request()
                playwright.chromium.launch.assert_not_called()
                finish.set()
                await asyncio.gather(worker._browser_close_task, return_exceptions=True)

    async def test_interrupted_allocation_escalates_to_runtime_owner(self):
        for kind in ("browser", "context"):
            with self.subTest(kind=kind):
                entered = asyncio.Event()

                async def allocating(**kwargs):
                    entered.set()
                    await asyncio.Event().wait()

                playwright = MagicMock()
                playwright.chromium.launch = AsyncMock(side_effect=allocating)
                browser = MagicMock()
                browser.new_context = AsyncMock(side_effect=allocating)
                worker = BrowserWorker(playwright, 1, 2, 1024, 10)
                operation = worker._launch_browser() if kind == "browser" else worker._new_context(browser)
                task = asyncio.create_task(operation)
                await asyncio.wait_for(entered.wait(), 1)
                task.cancel()
                await asyncio.gather(task, return_exceptions=True)
                self.assertTrue(worker.shutdown_requested.is_set())

    async def test_unconfirmed_context_cleanup_stops_admission_and_retains_task(self):
        finish = asyncio.Event()

        async def stuck_close():
            try:
                await asyncio.Event().wait()
            except asyncio.CancelledError:
                await finish.wait()

        context = MagicMock()
        context.close = AsyncMock(side_effect=stuck_close)
        worker = BrowserWorker(None, 1, 2, 1024, 10)
        with patch("browser.CONTEXT_CLOSE_TIMEOUT_SECONDS", 0.01):
            self.assertFalse(await worker._close_context(context))
        self.assertTrue(worker.shutdown_requested.is_set())
        self.assertEqual(len(worker._context_closes), 1)
        self.assertIn("shutting down", (await worker.open_interactive({}))["error"])
        finish.set()
        await asyncio.gather(*list(worker._context_closes))

    async def test_cancellation_cannot_abandon_release_accounting(self):
        browser = MagicMock()
        browser.close = AsyncMock()
        worker = BrowserWorker(None, 1, 2, 1024, 10)
        worker.browser = browser
        worker.active = 1
        await worker.capacity.acquire()
        await worker.state_lock.acquire()
        try:
            caller = asyncio.create_task(worker._release_browser(browser, False))
            await asyncio.sleep(0)
            caller.cancel()
            await asyncio.gather(caller, return_exceptions=True)
            self.assertEqual(worker.active, 1)
            self.assertEqual(worker.capacity._value, 0)
        finally:
            worker.state_lock.release()
        await asyncio.wait_for(asyncio.gather(*list(worker._releases)), 1)
        self.assertEqual(worker.active, 0)
        self.assertEqual(worker.capacity._value, 1)
        await worker.close()

    async def test_shutdown_during_recycle_does_not_keep_consumer_alive(self):
        entered = asyncio.Event()
        finish = asyncio.Event()

        async def close_browser():
            entered.set()
            await finish.wait()

        browser = MagicMock()
        browser.is_connected.return_value = True
        browser.close = AsyncMock(side_effect=close_browser)
        playwright = MagicMock()
        playwright.chromium.launch = AsyncMock(return_value=browser)
        worker = BrowserWorker(playwright, 1, 2, 1024, 1)
        worker._execute = AsyncMock(return_value={"body": "done"})
        await worker.start()
        request = asyncio.create_task(worker.submit({}))
        await asyncio.wait_for(entered.wait(), 1)
        shutdown = asyncio.create_task(worker.close())
        await asyncio.sleep(0)
        finish.set()
        await asyncio.wait_for(shutdown, 1)
        await asyncio.wait_for(request, 1)
        self.assertFalse(worker.consumers)
        self.assertFalse(worker._releases)
        self.assertIsNone(worker.browser)
        browser.close.assert_awaited_once()

    async def test_interactive_opens_share_bounded_queue(self):
        browser = MagicMock()
        browser.close = AsyncMock()
        playwright = MagicMock()
        playwright.chromium.launch = AsyncMock(return_value=browser)
        worker = BrowserWorker(playwright, 1, 2, 1024, 10)
        entered = asyncio.Event()

        async def blocked_create(request):
            entered.set()
            await asyncio.Event().wait()

        worker.interactive.create = blocked_create
        tasks = []
        await worker.start()
        try:
            tasks.append(asyncio.create_task(worker.open_interactive({})))
            await asyncio.wait_for(entered.wait(), 1)
            tasks.extend(asyncio.create_task(worker.open_interactive({})) for _ in range(2))
            done, _ = await asyncio.wait(tasks, timeout=0.01)
            self.assertFalse(done)
            self.assertEqual((await worker.health())["queueDepth"], 2)
            self.assertEqual((await worker.open_interactive({}))["error"], "browser worker is busy")
        finally:
            for task in tasks:
                task.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)
            await worker.close()
        self.assertTrue(worker.queue.empty())
        browser.close.assert_awaited_once()

    async def test_stuck_frame_and_cancelled_close_keep_one_cleanup_owner(self):
        browser = MagicMock()
        browser.is_connected.return_value = True
        context = MagicMock()
        context.browser = browser
        context.close = AsyncMock()
        page = MagicMock()
        page.screenshot = AsyncMock(return_value=b"image")
        page.title = AsyncMock(return_value="test")
        page.viewport_size = {"width": 390, "height": 720}
        page.url = "https://example.test"
        release = AsyncMock()
        async def close_context(context):
            await context.close()
            return True

        sessions = InteractiveSessions(AsyncMock(return_value=(browser, context, page)), release, close_context, 120, 600, max_operations=1)
        session_id = (await sessions.create({}))["sessionId"]
        entered = asyncio.Event()

        async def stuck_frame(**kwargs):
            entered.set()
            await asyncio.Event().wait()

        page.screenshot.side_effect = stuck_frame
        with patch("interactive.DEFAULT_TIMEOUT_MS", 50):
            frame = asyncio.create_task(sessions.frame(session_id))
            await asyncio.wait_for(entered.wait(), 1)
            with self.assertRaises(WorkerBusyError):
                await sessions.frame(session_id)
            close = asyncio.create_task(sessions.close(session_id))
            await asyncio.sleep(0)  # let close register its owned task before cancelling the caller
            self.assertIn(session_id, sessions._sessions)
            self.assertIsNotNone(sessions._sessions[session_id].close_task)
            close.cancel()
            await asyncio.gather(close, return_exceptions=True)
            results = await asyncio.wait_for(asyncio.gather(frame, return_exceptions=True), 1)
            self.assertIsInstance(results[0], TimeoutError)
            await asyncio.wait_for(sessions.close(session_id), 1)
        self.assertNotIn(session_id, sessions._sessions)
        self.assertFalse(sessions._operations.locked())
        context.close.assert_awaited_once()
        release.assert_awaited_once()
        await sessions.close_all()


class RuntimeShutdownTest(unittest.TestCase):
    def test_root_shutdown_is_bounded_even_when_driver_stop_hangs(self):
        code = '''
import asyncio, os
from types import SimpleNamespace
from unittest.mock import AsyncMock
import worker
async def hang():
    await asyncio.Event().wait()
async def exercise():
    stop = AsyncMock(side_effect=hang) if os.environ['HANG'] == '1' else AsyncMock()
    runtime = SimpleNamespace(stop=stop)
    worker.async_playwright = lambda: SimpleNamespace(start=AsyncMock(return_value=runtime))
    service = SimpleNamespace(start=AsyncMock(), close=AsyncMock(), request_shutdown=lambda: None, shutdown_requested=asyncio.Event(), failure=None)
    service.shutdown_requested.set()
    worker.BrowserWorker = lambda *args: service
    worker.runtime_metadata = lambda: {}
    worker.SHUTDOWN_GRACE_SECONDS = 0.05
    await worker.main()
asyncio.run(exercise())
'''
        for hangs in (False, True):
            with self.subTest(hangs=hangs):
                env = {**os.environ, "HANG": str(int(hangs)), "WEBVIEW_WORKER_HOST": "127.0.0.1", "WEBVIEW_WORKER_PORT": "0", "WEBVIEW_BROWSER_MODE": "headless"}
                result = subprocess.run([sys.executable, "-c", code], env=env, capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode, 1 if hangs else 0, result.stderr)
                self.assertEqual("shutdown exceeded" in result.stderr, hangs)

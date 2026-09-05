"""Runtime configuration, display ownership, and browser cleanup contracts."""

import os
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

from patchright.async_api import async_playwright

from browser import BrowserWorker
from runtime import VirtualDisplay, browser_mode


class RuntimeTest(unittest.IsolatedAsyncioTestCase):
    async def test_mode_validation_and_external_display_ownership(self):
        with patch.dict(os.environ, {}, clear=True):
            self.assertEqual(browser_mode(), "headless")
            os.environ["WEBVIEW_BROWSER_MODE"] = "invalid"
            with self.assertRaises(ValueError):
                browser_mode()
        with patch.dict(os.environ, {"DISPLAY": ":42"}):
            display = VirtualDisplay("headful")
            await display.start()
            await display.close()
            self.assertEqual(os.environ["DISPLAY"], ":42")

    async def test_failed_context_close_recycles_after_other_work_drains(self):
        worker = BrowserWorker(None, 2, 1, 1024, 100)
        browser = MagicMock()
        browser.close = AsyncMock()
        worker.browser = browser
        worker.active = 2
        await worker.capacity.acquire()
        await worker.capacity.acquire()
        context = MagicMock()
        context.close = AsyncMock(side_effect=RuntimeError("closed transport"))
        context.new_page = AsyncMock(side_effect=RuntimeError("page failed"))
        browser.new_context = AsyncMock(return_value=context)
        with self.assertRaisesRegex(RuntimeError, "page failed"):
            await worker._probe(browser, 1000)
        await worker._release_browser(browser, False)
        browser.close.assert_not_awaited()
        await worker._release_browser(browser, False)
        browser.close.assert_awaited_once()
        self.assertIsNone(worker.browser)
        self.assertEqual(worker.recycled, 1)

    async def test_real_browser_recycles_without_retaining_contexts(self):
        mode = browser_mode()
        display = VirtualDisplay(mode)
        try:
            await display.start()
            async with async_playwright() as playwright:
                worker = BrowserWorker(playwright, 1, 1, 1024, 2, browser_mode=mode)
                try:
                    await worker.start()
                    first_browser = worker.browser
                    result = await worker.submit({"probe": True})
                    self.assertEqual(result.get("body"), "novelreader-webview-ok")
                    self.assertEqual(first_browser.contexts, [])
                    await worker.submit({"probe": True})
                    self.assertFalse(first_browser.is_connected())
                    self.assertEqual(worker.recycled, 1)
                    result = await worker.submit({"probe": True})
                    self.assertEqual(result.get("body"), "novelreader-webview-ok")
                    self.assertEqual(worker.browser.contexts, [])
                finally:
                    await worker.close()
        finally:
            await display.close()

    async def test_optional_benchmark_uses_installed_chrome_without_network(self):
        from test_live_cloudflare import launch_browser

        async with launch_browser("patchright") as browser:
            self.assertTrue(browser.is_connected())
        self.assertFalse(browser.is_connected())

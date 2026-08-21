"""Capacity configuration checks for the headless browser worker."""

import asyncio
import os
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

from browser import (
    BrowserWorker,
    await_or_source_match,
    capture_source_match,
    compile_source_regex,
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
            self.assertEqual(capacity_settings(), (2, 8, 10 * 1024 * 1024, 100))

    def test_overrides_and_clamps_admission_to_one(self) -> None:
        values = {
            "WEBVIEW_MAX_PAGES": "4",
            "WEBVIEW_MAX_PENDING": "0",
            "WEBVIEW_MAX_BODY_BYTES": "2048",
            "WEBVIEW_MAX_CONTEXTS_PER_BROWSER": "250",
        }
        with patch.dict(os.environ, values, clear=True):
            self.assertEqual(capacity_settings(), (4, 1, 2048, 250))


if __name__ == "__main__":
    unittest.main()

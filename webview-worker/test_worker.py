"""Capacity configuration checks for the headless browser worker."""

import os
import unittest
from unittest.mock import MagicMock, patch

from browser import BrowserWorker
from worker import capacity_settings


class BrowserWorkerHealthTest(unittest.IsolatedAsyncioTestCase):
    async def test_idle_recycled_browser_is_ready_when_consumers_are_alive(self) -> None:
        worker = BrowserWorker(None, max_pages=2, max_pending=8, max_body_bytes=1024, max_contexts=100)
        worker.browser = None
        worker.consumers = [MagicMock(done=lambda: False), MagicMock(done=lambda: False)]

        self.assertTrue((await worker.health())["ok"])
        worker.consumers[0].done = lambda: True
        self.assertFalse((await worker.health())["ok"])


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

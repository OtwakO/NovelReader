"""Capacity configuration checks for the headless browser worker."""

import os
import unittest
from unittest.mock import patch

from worker import capacity_settings


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

"""Opt-in live Cloudflare compatibility check.

This test is intentionally excluded from the default quality gate. The target is a
mutable third-party service whose result depends on Cloudflare policy and IP
reputation. It never submits the form or uses real credentials.
"""

import os
import unittest
from contextlib import asynccontextmanager


LIVE_TEST_ENABLED = os.environ.get("WEBVIEW_LIVE_CLOUDFLARE") == "1"
PLANET_MINECRAFT_SIGN_IN = "https://www.planetminecraft.com/account/sign_in/"


@asynccontextmanager
async def launch_browser(engine: str):
    if engine == "patchright":
        from patchright.async_api import async_playwright

        from runtime import VirtualDisplay, browser_mode

        mode = browser_mode()
        display = VirtualDisplay(mode)
        try:
            await display.start()
            async with async_playwright() as playwright:
                browser = await playwright.chromium.launch(
                    channel="chrome", headless=mode == "headless",
                )
                try:
                    yield browser
                finally:
                    await browser.close()
        finally:
            await display.close()
        return

    if engine == "camoufox":
        try:
            from camoufox.async_api import AsyncCamoufox
        except ImportError as error:
            raise unittest.SkipTest("camoufox is not installed in this environment") from error

        mode = os.environ.get("WEBVIEW_LIVE_CAMOUFOX_MODE", "headless")
        if mode not in {"headless", "virtual"}:
            raise ValueError("WEBVIEW_LIVE_CAMOUFOX_MODE must be headless or virtual")
        async with AsyncCamoufox(
            headless=True if mode == "headless" else "virtual",
            humanize=False,
        ) as browser:
            yield browser
        return

    raise ValueError("WEBVIEW_LIVE_BROWSER must be patchright or camoufox")


@unittest.skipUnless(
    LIVE_TEST_ENABLED,
    "set WEBVIEW_LIVE_CLOUDFLARE=1 to run the external Cloudflare check",
)
class PlanetMinecraftCloudflareLiveTest(unittest.IsolatedAsyncioTestCase):
    async def test_sign_in_turnstile_completes_without_submission(self) -> None:
        engine = os.environ.get("WEBVIEW_LIVE_BROWSER", "patchright")
        timeout_seconds = int(os.environ.get("WEBVIEW_LIVE_CLOUDFLARE_SECONDS", "30"))

        async with launch_browser(engine) as browser:
            context = await browser.new_context()
            try:
                page = await context.new_page()
                response = await page.goto(
                    PLANET_MINECRAFT_SIGN_IN,
                    wait_until="domcontentloaded",
                    timeout=60_000,
                )
                status = response.status if response else None
                title = await page.title()
                body = (await page.locator("body").inner_text()).lower()
                email = page.locator("#email")

                self.assertEqual(
                    status,
                    200,
                    f"{engine} did not pass the outer Cloudflare gate: HTTP {status}, title={title!r}",
                )
                self.assertNotIn("sorry, you have been blocked", body)
                self.assertNotIn("just a moment", body)
                self.assertEqual(
                    await email.count(),
                    1,
                    f"{engine} reached HTTP 200 but the normal sign-in form was unavailable",
                )

                await email.fill("novelreader-live-check@example.invalid")
                await page.locator("#password").fill("not-a-real-password")
                token = page.locator(
                    '[name="cf-turnstile-response"], [name^="cf-turnstile-response-"]'
                )
                submit = page.locator('input[type="submit"][name="login"]')

                token_lengths: list[int] = []
                for _ in range(timeout_seconds * 2 + 1):
                    token_lengths = await token.evaluate_all(
                        "elements => elements.map(element => (element.value || '').length)"
                    )
                    if any(length > 0 for length in token_lengths):
                        break
                    await page.wait_for_timeout(500)

                self.assertTrue(
                    any(length > 0 for length in token_lengths),
                    f"{engine} reached the sign-in page but Cloudflare Turnstile did not issue a token "
                    f"within {timeout_seconds}s",
                )
                self.assertFalse(
                    await submit.is_disabled(),
                    f"{engine} received a Turnstile token but the login submit remained disabled",
                )
            finally:
                await context.close()


if __name__ == "__main__":
    unittest.main()

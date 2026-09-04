"""Validated browser mode and lifecycle for the Patchright worker runtime."""

from __future__ import annotations

import asyncio
import os
import subprocess
from importlib.metadata import version
from pathlib import Path

BROWSER_MODES = frozenset({"headless", "headful"})
XVFB_DISPLAY = ":99"
XVFB_SOCKET = Path("/tmp/.X11-unix/X99")


def runtime_metadata() -> dict[str, str]:
    browser = subprocess.run(
        ["google-chrome", "--version"],
        check=True,
        capture_output=True,
        text=True,
        timeout=5,
    ).stdout.strip()
    return {
        "automationDriver": "patchright",
        "patchrightVersion": version("patchright"),
        "browserVersion": browser,
    }


def browser_mode() -> str:
    mode = os.getenv("WEBVIEW_BROWSER_MODE", "headless").strip().lower()
    if mode not in BROWSER_MODES:
        expected = ", ".join(sorted(BROWSER_MODES))
        raise ValueError(f"WEBVIEW_BROWSER_MODE must be one of: {expected}")
    return mode


class VirtualDisplay:
    """Own an Xvfb process only when headful Chrome needs a display."""

    def __init__(self, mode: str):
        self.mode = mode
        self.process: asyncio.subprocess.Process | None = None
        self.previous_display: str | None = None
        self.changed_display = False

    async def start(self) -> None:
        if self.mode != "headful" or os.getenv("DISPLAY"):
            return
        self.previous_display = os.environ.get("DISPLAY")
        self.changed_display = True
        os.environ["DISPLAY"] = XVFB_DISPLAY
        self.process = await asyncio.create_subprocess_exec(
            "Xvfb",
            XVFB_DISPLAY,
            "-screen",
            "0",
            "1920x1080x24",
            "-nolisten",
            "tcp",
            stdout=asyncio.subprocess.DEVNULL,
        )
        for _ in range(100):
            if self.process.returncode is not None:
                raise RuntimeError(f"Xvfb exited during startup with code {self.process.returncode}")
            if XVFB_SOCKET.exists():
                return
            await asyncio.sleep(0.05)
        await self.close()
        raise RuntimeError("Xvfb did not become ready")

    async def close(self) -> None:
        process, self.process = self.process, None
        if process is not None and process.returncode is None:
            process.terminate()
            try:
                await asyncio.wait_for(process.wait(), timeout=3)
            except TimeoutError:
                process.kill()
                await process.wait()
        if self.changed_display:
            if self.previous_display is None:
                os.environ.pop("DISPLAY", None)
            else:
                os.environ["DISPLAY"] = self.previous_display
            self.changed_display = False


async def close_context(context) -> bool:
    """Bound cleanup and let the owner recycle the browser on failure."""
    try:
        await asyncio.shield(asyncio.wait_for(context.close(), timeout=2))
        return True
    except BaseException:
        return False

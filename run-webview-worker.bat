@echo off
setlocal
cd /d "%~dp0webview-worker"

echo [1/4] Checking uv...
where uv >nul 2>&1 || (
  echo ERROR: uv was not found. Install it from https://docs.astral.sh/uv/ and try again.
  goto :failed
)

echo [2/4] Synchronizing the locked Python environment...
uv python install || goto :failed
uv sync --frozen || goto :failed

echo [3/4] Installing Patchright Chromium...
uv run --frozen --no-sync patchright install chromium || goto :failed

if not defined WEBVIEW_WORKER_HOST set "WEBVIEW_WORKER_HOST=127.0.0.1"
if not defined WEBVIEW_WORKER_PORT set "WEBVIEW_WORKER_PORT=8787"
echo [4/4] Starting the WebView worker at http://%WEBVIEW_WORKER_HOST%:%WEBVIEW_WORKER_PORT%
uv run --frozen --no-sync python worker.py
set "EXIT_CODE=%ERRORLEVEL%"
exit /b %EXIT_CODE%

:failed
echo.
echo The WebView worker could not be started.
pause
exit /b 1

@echo off
setlocal
cd /d "%~dp0"

echo [1/4] Checking Python...
where python >nul 2>&1 || (
  echo ERROR: Python was not found. Install Python 3 and try again.
  goto :failed
)

echo [2/4] Preparing the virtual environment...
if not exist "webview-worker\.venv\Scripts\python.exe" (
  python -m venv "webview-worker\.venv" || goto :failed
)
call "webview-worker\.venv\Scripts\activate.bat" || goto :failed

echo [3/4] Installing dependencies and Chromium...
python -m pip install -r "webview-worker\requirements.txt" || goto :failed
patchright install chromium || goto :failed

if not defined WEBVIEW_WORKER_HOST set "WEBVIEW_WORKER_HOST=127.0.0.1"
if not defined WEBVIEW_WORKER_PORT set "WEBVIEW_WORKER_PORT=8787"
echo [4/4] Starting the WebView worker at http://%WEBVIEW_WORKER_HOST%:%WEBVIEW_WORKER_PORT%
python "webview-worker\worker.py"
set "EXIT_CODE=%ERRORLEVEL%"
exit /b %EXIT_CODE%

:failed
echo.
echo The WebView worker could not be started.
pause
exit /b 1

@echo off
setlocal
cd /d "%~dp0"

echo [1/3] Checking prerequisites...
where npm >nul 2>&1 || (
  echo ERROR: npm was not found. Install Node.js and try again.
  goto :failed
)
where go >nul 2>&1 || (
  echo ERROR: Go was not found. Install Go and try again.
  goto :failed
)

echo [2/3] Installing and building the frontend...
pushd frontend
call npm install || goto :failed_popd
call npm run build || goto :failed_popd
popd

if not defined PORT set "PORT=8888"
echo [3/3] Starting NovelReader at http://localhost:%PORT%
pushd backend
set "DEBUG=1"
set "DEVELOPMENT_MODE=1"
set "WEBVIEW_ENDPOINT=http://127.0.0.1:8787"
go run ./cmd/server/ %*
set "EXIT_CODE=%ERRORLEVEL%"
popd
exit /b %EXIT_CODE%

:failed_popd
popd
:failed
echo.
echo NovelReader could not be started.
pause
exit /b 1

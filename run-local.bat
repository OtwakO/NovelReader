@echo off
setlocal EnableExtensions
cd /d "%~dp0"

set "LOCAL_DATA_DIR=%CD%\backend\data"

echo [1/4] Checking prerequisites...
where npm >nul 2>&1 || (
  echo ERROR: npm was not found. Install Node.js and try again.
  goto :failed
)
where go >nul 2>&1 || (
  echo ERROR: Go was not found. Install Go and try again.
  goto :failed
)
where powershell >nul 2>&1 || (
  echo ERROR: PowerShell was not found. It is required to generate a temporary setup token.
  goto :failed
)

if not defined ADMIN_BOOTSTRAP_TOKEN (
  for /f "usebackq delims=" %%T in (`powershell -NoProfile -NonInteractive -Command "$bytes = New-Object byte[] 24; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes); [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_')"`) do set "ADMIN_BOOTSTRAP_TOKEN=%%T"
)
if not defined ADMIN_BOOTSTRAP_TOKEN (
  echo ERROR: Could not generate ADMIN_BOOTSTRAP_TOKEN.
  goto :failed
)
if not defined REGISTRATION_ENABLED set "REGISTRATION_ENABLED=true"
if not defined PORT set "PORT=8888"
set "DATA_DIR=%LOCAL_DATA_DIR%"
set "DEBUG=1"
set "WEBVIEW_ENDPOINT=http://127.0.0.1:8787"

echo [2/4] Installing locked frontend dependencies and building...
pushd frontend
call npm ci || goto :failed_popd
call npm run build || goto :failed_popd
popd

echo [3/4] Local account configuration:
echo   Data directory:       %DATA_DIR%
echo   Registration enabled: %REGISTRATION_ENABLED%
if defined ADMIN_BOOTSTRAP_TOKEN (
  echo   Setup token:          %ADMIN_BOOTSTRAP_TOKEN%
  echo.
  echo Use the setup token above only if the browser shows first-Administrator setup.
  echo The generated token exists only for this launcher/server run and is ignored after setup closes.
)
echo.
echo [4/4] Starting NovelReader at http://localhost:%PORT%
pushd backend
go run ./cmd/server/ %*
set "EXIT_CODE=%ERRORLEVEL%"
popd
pause
exit /b %EXIT_CODE%

:failed_popd
popd
:failed
echo.
echo NovelReader could not be started.
pause
exit /b 1

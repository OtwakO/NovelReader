# Patchright WebView worker

This worker provides the headless browser boundary used by NovelReader's Go backend.

```bash
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
patchright install chromium
WEBVIEW_WORKER_PORT=8787 python worker.py
```

The worker binds to `127.0.0.1` by default and exposes `POST /execute`. Configure the Go
backend with `WEBVIEW_ENDPOINT=http://127.0.0.1:8787`. `WEBVIEW_MAX_PAGES` limits concurrent
browser contexts; each request gets an isolated context and cookies are returned to the Go
source session.

The worker is intended to run beside the Go server in Docker or as a localhost sidecar. Do
not expose it publicly: it accepts arbitrary navigation URLs by design.

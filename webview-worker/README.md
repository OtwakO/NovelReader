# Patchright WebView worker

This private sidecar provides the browser boundary used by NovelReader's Go backend.

## Local process

```bash
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
patchright install chromium
WEBVIEW_WORKER_PORT=8787 python worker.py
```

Direct runs bind `127.0.0.1` by default. Configure the backend with
`WEBVIEW_ENDPOINT=http://127.0.0.1:8787`.

## Container

The public `linux/amd64` image is `ghcr.io/otwako/novelreader-webview`. The image binds
`0.0.0.0:8787` internally for container networking, runs as UID 10001, and reports readiness at
`GET /healthz`. Use the root Compose profile rather than publishing its port:

```bash
export WEBVIEW_ENDPOINT=http://webview-worker:8787
docker compose --profile webview up -d --no-build
```

`WEBVIEW_MAX_PAGES` limits concurrent browser contexts, `WEBVIEW_MAX_PENDING` bounds queued
requests, `WEBVIEW_MAX_CONTEXTS_PER_BROWSER` recycles Chromium after clean usage, and
`WEBVIEW_MAX_BODY_BYTES` caps returned content. Each request gets an isolated context and cookies
are returned to the Go source session. Browser protocol v2 also accepts optional `sourceRegex`;
the worker full-matches loaded resource URLs and returns the first match as the response body.
Backend and worker versions must be upgraded together when this protocol changes.

Do not expose this worker publicly: `POST /execute` accepts arbitrary navigation URLs by design.

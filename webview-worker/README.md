# Patchright WebView worker

This private sidecar provides the browser boundary used by NovelReader's Go backend. Python and
its virtual environment are managed with [uv](https://docs.astral.sh/uv/); `uv.lock` is the
reproducible dependency authority.

## Local process

```bash
uv python install
uv sync --frozen
uv run --frozen --no-sync patchright install chromium
WEBVIEW_WORKER_PORT=8787 uv run --frozen --no-sync python worker.py
```

`uv sync` installs the Python version selected by `.python-version` when needed and creates or updates `.venv` automatically. Direct runs bind `127.0.0.1` by default.
Configure the backend with `WEBVIEW_ENDPOINT=http://127.0.0.1:8787`.

## Container

The public `linux/amd64` image is `ghcr.io/otwako/novelreader-webview`. Its build copies a pinned
uv binary, installs the exact locked Patchright environment, and installs the matching Chromium.
The image binds `0.0.0.0:8787` internally for container networking, runs as UID 10001, and reports
readiness at `GET /healthz`. Use the root Compose deployment rather than publishing its port.

`WEBVIEW_MAX_PAGES` limits concurrent browser contexts, `WEBVIEW_MAX_PENDING` bounds queued
requests, `WEBVIEW_MAX_CONTEXTS_PER_BROWSER` recycles Chromium after clean usage, and
`WEBVIEW_MAX_BODY_BYTES` caps returned content. Each request gets an isolated context and cookies
are returned to the Go source session. Browser protocol v2 also accepts optional `sourceRegex`;
the worker full-matches loaded resource URLs and returns the first match as the response body.
Backend and worker versions must be upgraded together when this protocol changes.

The same private worker also supports short-lived interactive login contexts. `WEBVIEW_INTERACTIVE_IDLE_SECONDS` (default 120) and `WEBVIEW_INTERACTIVE_ABSOLUTE_SECONDS` (default 600) enforce cleanup even when a client disappears. Interactive contexts share `WEBVIEW_MAX_PAGES` capacity with one-shot requests, are never persisted, and are closed on expiry or worker shutdown.

Base64 HTML `data:` documents receive a narrow request mediator for their `fetch` and XHR probes. The mediator accepts only HTTP(S), rejects any hostname resolution containing non-public addresses, does not follow redirects, bounds timeout and response size, and authorizes only the opaque `null` document origin. Other resource types retain normal browser handling. This avoids globally disabling Chromium web security or exposing a general browser proxy.

Do not expose this worker publicly: `POST /execute` accepts arbitrary navigation URLs by design.

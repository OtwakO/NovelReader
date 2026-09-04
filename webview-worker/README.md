# Patchright WebView worker

This private sidecar provides the browser boundary used by NovelReader's Go backend. Python and
its virtual environment are managed with [uv](https://docs.astral.sh/uv/). `uv.lock` pins the local
baseline; release builds deliberately resolve the latest stable Patchright and branded Chrome,
then CI tests that exact image in both modes before publication. There are no runtime auto-updates.

## Local process

```bash
uv python install
uv sync --frozen
uv run --frozen --no-sync patchright install chrome --with-deps
WEBVIEW_WORKER_PORT=8787 uv run --frozen --no-sync python worker.py
```

`uv sync` installs the Python version selected by `.python-version` when needed and creates or updates `.venv` automatically. Direct runs bind `127.0.0.1` by default.
Configure the backend with `WEBVIEW_ENDPOINT=http://127.0.0.1:8787`.

## Container

The public `linux/amd64` image is `ghcr.io/otwako/novelreader-webview`. Its build copies a pinned
uv binary and installs the latest stable Patchright and Chrome. Release builds disable Docker
cache for this image so browser/dependency installation is not silently reused. For an equivalent
local build, use `docker build --no-cache -t novelreader-webview ./webview-worker` from the repo root.
Rebuilding the same source revision may resolve newer dependencies; roll back using a previously
verified image digest, not by rebuilding an old source commit.
The image binds `0.0.0.0:8787` internally for container networking, runs as UID 10001, and reports
readiness at `GET /healthz`. Use the root Compose deployment rather than publishing its port.

`WEBVIEW_BROWSER_MODE=headless|headful` selects the mode for **all WebView requests**, including
interactive sessions; normal backend HTTP requests are unaffected. The default is `headless`.
Headful mode starts an owned Xvfb display when `DISPLAY` is unset (local Linux runs need Xvfb
installed), and stops it on shutdown. An existing display remains caller-owned. Both modes use
the same Chrome binary and isolated, non-persistent contexts, with no additional stealth flags,
UA overrides, or engine fallback. Headful Chrome costs more memory; keep concurrency bounded.
`/healthz` includes the resolved Patchright/Chrome versions and selected mode.

`WEBVIEW_MAX_PAGES` limits concurrent browser contexts, `WEBVIEW_MAX_PENDING` bounds queued
requests, `WEBVIEW_MAX_CONTEXTS_PER_BROWSER` recycles Chrome after clean usage (failed context cleanup also
marks the browser for recycling when active work drains), and
`WEBVIEW_MAX_BODY_BYTES` caps returned content. Each request gets an isolated context and cookies
are returned to the Go source session. Browser protocol v2 also accepts optional `sourceRegex`;
the worker full-matches loaded resource URLs and returns the first match as the response body.
Backend and worker versions must be upgraded together when this protocol changes.

The same private worker also supports short-lived interactive login contexts. `WEBVIEW_INTERACTIVE_IDLE_SECONDS` (default 120) and `WEBVIEW_INTERACTIVE_ABSOLUTE_SECONDS` (default 600) enforce cleanup even when a client disappears. Interactive contexts share `WEBVIEW_MAX_PAGES` capacity with one-shot requests, are never persisted, and are closed on expiry or worker shutdown.

Base64 HTML `data:` documents receive a narrow request mediator for their `fetch` and XHR probes. The mediator accepts only HTTP(S), rejects any hostname resolution containing non-public addresses, verifies the connected server address before exposing the response, does not follow redirects, bounds timeout and response size, and authorizes only the opaque `null` document origin. Other resource types retain normal browser handling. This avoids globally disabling Chromium web security or exposing a general browser proxy.

## Optional live Cloudflare check

`test_live_cloudflare.py` checks Planet Minecraft's public sign-in page without using real credentials or submitting the form. It requires both the normal login UI and a completed embedded Turnstile token. The test is skipped by default because Cloudflare policy, IP reputation, and the third-party page can change independently of NovelReader.

Run it explicitly against the production Patchright environment:

```bash
WEBVIEW_LIVE_CLOUDFLARE=1 uv run --frozen --no-sync \
  python -m unittest test_live_cloudflare.py
```

For an isolated Camoufox comparison, install Camoufox and its browser outside the project environment, then set `WEBVIEW_LIVE_BROWSER=camoufox`; `WEBVIEW_LIVE_CAMOUFOX_MODE` accepts `headless` or `virtual`. Do not add Camoufox to the locked production dependencies merely to run this optional benchmark.

Do not expose this worker publicly: `POST /execute` accepts arbitrary navigation URLs by design.

# NovelReader

NovelReader is a web-first novel reader with a Legado-compatible booksource engine. It imports raw Legado source JSON, executes regular and JavaScript sources through a shared request/session pipeline, and exposes a browser transport seam for future WebView support.

## Local setup

Requirements: Go, Node.js, and npm.

```bash
cd frontend && npm install && npm run build
cd .. && ./dev.sh run
```

The server listens on port `8888` by default. Set `PORT`, `DATABASE_PATH`, or `DATA_DIR` to override local settings.

Capacity defaults target a 2-vCPU/4-GB container: 16 source requests per search, 32 process-wide, 4 JavaScript runtimes, 1,024 workflow sessions with a 30-minute idle TTL, and 2 browser pages with 8 queued requests; the browser recycles after 100 contexts. Override with `SEARCH_CONCURRENCY`, `GLOBAL_SEARCH_CONCURRENCY`, `JS_POOL_SIZE`, `MAX_WORKFLOW_SESSIONS`, `SESSION_TTL_MINUTES`, `WEBVIEW_MAX_PAGES`, `WEBVIEW_MAX_PENDING`, or `WEBVIEW_MAX_CONTEXTS_PER_BROWSER`. A 4-vCPU/8-GB starting profile is `32`, `64`, `8`, `2048`, `60`, `4`, `16`, and `250` respectively; measure before increasing further.

WebView sources are optional and run through the headless Patchright worker. Start it from `webview-worker/` and set `WEBVIEW_ENDPOINT=http://127.0.0.1:8787`; without this setting, WebView requests fail explicitly while normal HTTP sources continue to work.

## Tests

```bash
cd backend
GOMODCACHE=/tmp/go-mod GOPATH=/tmp/go go test ./...
cd ../frontend && npm run build
```

Tests are colocated with the analyzer, source executor, transport, book, and conformance code.

## Raw-source conformance runner

Use raw JSON index identity rather than source names. The command records the expanded request, redacted headers, response status/final URL/body sample, extracted search results, and classification:

```bash
cd backend
GOMODCACHE=/tmp/go-mod GOPATH=/tmp/go go run ./cmd/conformance \
  -sources ../test_booksource4.json \
  -indices 1,84,89 \
  -query '凡人修仙传' \
  -health-url http://localhost:8888/
```

`-indices` is optional; omitting it runs every source. `-health-url` is optional but aborts the run if the target server stops responding. Add `-webview-endpoint http://127.0.0.1:8787` to execute `webView:true` requests through the Patchright worker. Add `-indices N -book-url URL` to run detail → TOC → first-chapter content for one source. The CLI uses the production fingerprint transport. Site DNS, WAF, timeout, WebView, and stale-rule failures are reported separately rather than silently treated as parser failures.

Deterministic response fixtures live in `testdata/booksource/`; their manifest test executes the declared rules offline.

## Headless WebView worker

```bash
cd webview-worker
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
patchright install chromium
WEBVIEW_WORKER_PORT=8787 python worker.py
```

For Docker, build `webview-worker/Dockerfile`. Keep the worker on a private network; it accepts arbitrary navigation URLs. Apply resource limits at runtime, for example `docker run --cpus=2 --memory=4g ...`; Dockerfiles cannot enforce deployment resource limits.

## Deployment

Build the frontend, then build and run the backend server:

```bash
cd frontend && npm run build
cd ../backend && go build -o novelreader ./cmd/server
./novelreader
```

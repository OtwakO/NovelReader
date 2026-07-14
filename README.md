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

Search runs in user-sized source batches. The default checks 50 sources with Balanced concurrency 8; the UI persists batch size and Gentle/Balanced/Fast or advanced concurrency preferences. User concurrency is only a request: `SEARCH_CONCURRENCY` remains the authoritative per-search ceiling, while global, JavaScript, and browser limits remain independent. “Search more” continues through a versioned cursor; changing the eligible source list requires restarting that search.

The 2-vCPU/4-GB HTTP-search gate sustained 48,000 deterministic source requests across 16 concurrent users with zero post-fix failures: median batch latency about 2.59s, p95 2.82s or lower, 27.32 MiB observed peak backend memory, and no retained established source connections. The separately constrained Patchright gate completed 200 requests from 10 clients with 2 pages and 8 queued: zero failures/rejections, 3.27s median, 3.33s p95, 153.6 MiB observed peak memory, and a healthy recycle after 100 contexts. Keep the current capacity ceilings until real upstream measurements justify changing them.

The batched SSE API is `GET /api/search/stream?q=...&batchSize=50&concurrency=8&cursor=...`. It emits `start`, per-source `results` or `source_error`, and `done` events. `done.nextCursor` continues a completed batch; `retryCursor` repeats an interrupted batch; `stale` means the eligible source revision changed. Omitting `batchSize` preserves the legacy all-source stream contract.

WebView sources are optional and run through the headless Patchright worker. Start it from `webview-worker/` and set `WEBVIEW_ENDPOINT=http://127.0.0.1:8787`; without this setting, WebView requests fail explicitly while normal HTTP sources continue to work.

## Tests

```bash
cd backend
GOMODCACHE=/tmp/go-mod GOPATH=/tmp/go go test ./...
cd ../frontend && npm test && npm run build
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

For Docker, build `webview-worker/Dockerfile`; the image binds `0.0.0.0:8787` internally so other containers can reach it. Publish it only on a private network because it accepts arbitrary navigation URLs. Apply resource limits at runtime, for example `docker run --cpus=2 --memory=4g ...`; Dockerfiles cannot enforce deployment resource limits.

## Deployment

Build the frontend, then build and run the backend server:

```bash
cd frontend && npm run build
cd ../backend && go build -o novelreader ./cmd/server
./novelreader
```

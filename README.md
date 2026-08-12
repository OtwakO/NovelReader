# NovelReader

NovelReader is a web-first novel reader with a Legado-compatible booksource engine. It imports raw Legado source JSON, supports source-native Explore catalogs alongside cross-source search, saves chapter and in-chapter reading progress with annotated bookmarks, safely switches between alternate sources, retains bounded offline chapter copies, executes regular and JavaScript sources through a shared request/session pipeline, and routes browser-backed sources through an optional WebView worker.

## Local setup

Requirements: Go, Node.js, and npm.

```bash
cd frontend && npm ci && npm run build
cd .. && ./dev.sh run
```

`frontend/dist/` is generated build output and is intentionally not tracked. Build it before starting the Go server from a fresh checkout. `frontend/package-lock.json` remains tracked and authoritative; use `npm ci` for routine setup/builds, and update the lockfile only when dependencies change intentionally.

On Windows, double-click `run-local.bat` (or run it from Command Prompt) to install the exact locked dependencies with `npm ci`, build the frontend, and start the server. The launcher uses `backend\data` as the current per-reader data root, generates and prints a temporary first-Administrator setup token on each run (the server ignores it after setup closes), and enables ordinary-reader registration for local testing unless `REGISTRATION_ENABLED` was set explicitly.

The server listens on port `8888` by default. Set `PORT` or `DATA_DIR` to override local settings. Run only one NovelReader server process against a writable `DATA_DIR`; startup now enforces this with an OS-level data-root lock and refuses a second process. `PUBLIC_URL` is optional: normally NovelReader accepts the same host and effective port the browser used, including Tailscale HTTPS in front of a plain-HTTP container upstream. This dynamic mode is intended for trusted localhost, LAN, and tailnet access. Set `PUBLIC_URL` to pin one canonical HTTP(S) browser origin when a reverse proxy rewrites `Host` or the app is reachable from an untrusted browser network. If startup rejects an old development data directory, follow [`docs/DEVELOPMENT_DATA_RESET.md`](docs/DEVELOPMENT_DATA_RESET.md); old global test state is intentionally reset rather than migrated. Set `ADMIN_BOOTSTRAP_TOKEN` to enable one-time first-Administrator setup. `ADMIN_RECOVERY_TOKEN` conditionally mounts disaster recovery only while configured. Treat both as temporary secrets and remove them after use; recovery disappears when its variable is removed. Public reader registration is disabled by default. Set `REGISTRATION_ENABLED=true` to show and enable account creation; optionally set `REGISTRATION_INVITE_CODE` to require a deployment admission code. The invite code is never stored and should be treated as a secret. Administrators can list ordinary readers, disable or re-enable them, issue one-time password-reset tokens, and permanently delete them from the Account page. Disabling immediately revokes every browser session while preserving the password and Reader Data. Reset tokens are shown once for secure delivery, expire after 30 minutes, supersede earlier unused tokens, and let the reader choose a replacement password from the sign-in page; successful completion revokes every reader session while preserving active/disabled status. Deletion requires typing the exact current username, immediately makes the account non-authenticating, drains in-flight Reader Data work, removes the immutable-ID reader home, then removes the account; failures retain a durable retryable deletion job and never restore partially removed data. Administrator accounts cannot be targeted.

Capacity defaults target a 2-vCPU/4-GB container: 16 source requests per search, 32 process-wide, 4 JavaScript runtimes, 1,024 workflow sessions with a 30-minute idle TTL, and 2 browser pages with 8 queued requests; the browser recycles after 100 contexts. Explore page fetches have a 30-second per-source deadline. Override capacity and workflow settings with `SEARCH_CONCURRENCY`, `GLOBAL_SEARCH_CONCURRENCY`, `JS_POOL_SIZE`, `MAX_WORKFLOW_SESSIONS`, `SESSION_TTL_MINUTES`, `EXPLORE_SOURCE_TIMEOUT_SECONDS` (a positive integer), `WEBVIEW_MAX_PAGES`, `WEBVIEW_MAX_PENDING`, or `WEBVIEW_MAX_CONTEXTS_PER_BROWSER`. A 4-vCPU/8-GB starting profile is `32`, `64`, `8`, `2048`, `60`, `4`, `16`, and `250` respectively; measure before increasing further.

Search runs in user-sized source batches. The default checks 50 sources with Balanced concurrency 8; the UI persists batch size and Gentle/Balanced/Fast or advanced concurrency preferences. User concurrency is only a request: `SEARCH_CONCURRENCY` remains the authoritative per-search ceiling, while global, JavaScript, and browser limits remain independent. “Search more” continues through a versioned cursor; changing the eligible source list requires restarting that search.

The 2-vCPU/4-GB HTTP-search gate sustained 48,000 deterministic source requests across 16 concurrent users with zero post-fix failures: median batch latency about 2.59s, p95 2.82s or lower, 27.32 MiB observed peak backend memory, and no retained established source connections. The separately constrained Patchright gate completed 200 requests from 10 clients with 2 pages and 8 queued: zero failures/rejections, 3.27s median, 3.33s p95, 153.6 MiB observed peak memory, and a healthy recycle after 100 contexts. Keep the current capacity ceilings until real upstream measurements justify changing them.

The batched SSE API is `GET /api/search/stream?q=...&batchSize=50&concurrency=8&cursor=...`. It emits `start`, per-source `results` or `source_error`, and `done` events. `done.nextCursor` continues a completed batch; `retryCursor` repeats an interrupted batch; `stale` means the eligible source revision changed. Omitting `batchSize` preserves the legacy all-source stream contract. TOC/content/book-info upstream failures return HTTP 502 with a stable `code` and crawl diagnostics; missing books, chapters, and sources return 404, while storage failures return 500. Stored bookshelf/detail covers use `GET /api/books/{id}/cover`, which derives the remote URL from stored book/source state and applies source-specific cover decoding when configured; it is not a general image proxy. Chapter content keeps `paragraphs` and may add ordered text/image `blocks`; reader images use `GET /api/books/{id}/chapters/{idx}/images/{imageIdx}`, which derives the remote resource from the stored chapter cache and applies portable `imageDecode` byte scripts. Android bitmap-transform decoders return an explicit unsupported response rather than undecoded image data.

Per-source Explore uses `GET /api/explore/sources` plus `POST /api/explore/catalog`, `/api/explore/control`, and `/api/explore/page`. Catalog and entry IDs are opaque and session-scoped; clients must replace the catalog after every control response. API responses expose typed entries, results, and safe diagnostics but never imported source URLs, rules, or actions.

WebView sources are optional and run through the headless Patchright worker. Start it from `webview-worker/` and set `WEBVIEW_ENDPOINT=http://127.0.0.1:8787`; without this setting, WebView requests fail explicitly while normal HTTP sources continue to work.

## Tests

```bash
cd backend
GOMODCACHE=/tmp/go-mod GOPATH=/tmp/go go test ./...
cd ../frontend && npm test && npm run build
```

Tests are colocated with the analyzer, source executor, transport, book, and conformance code. The deterministic workflow matrix runs detail → TOC → first/middle/last content for normal HTML, JSONPath, XPath/Regex, GBK POST, and multi-page TOC/content source shapes. With Docker Compose 2.20.2 or newer, `./docker-e2e.sh` additionally builds both production images and verifies frontend delivery, readiness, private WebView routing, rendered search, graceful shutdown, and named-volume persistence without live sites.

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

`-indices` is optional; omitting it runs every source. `-health-url` is optional but aborts the run if the target server stops responding. Add `-webview-endpoint http://127.0.0.1:8787` to execute `webView:true` requests through the Patchright worker. Add `-indices N -detail-result '{...}'` with one SearchResult JSON object to fetch Book Info only, or `-indices N -book-url URL` to run detail → TOC → first/middle/last non-volume chapter content. The CLI uses the production fingerprint transport. Site DNS, WAF, timeout, WebView, and stale-rule failures are reported separately rather than silently treated as parser failures.

Deterministic response fixtures live under `testdata/booksource/conformance/`; dated live evidence is separated by operation under `testdata/booksource/audits/`. The core manifest test executes declared fixture rules offline.

## Headless WebView worker

On Windows, start the worker in one Command Prompt, then start the app in another. `run-local.bat` automatically uses the local worker endpoint:

```bat
run-webview-worker.bat

run-local.bat
```

```bash
cd webview-worker
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
patchright install chromium
WEBVIEW_WORKER_PORT=8787 python worker.py
```

For Docker, build `webview-worker/Dockerfile`; the image binds `0.0.0.0:8787` internally so other containers can reach it. Publish it only on a private network because it accepts arbitrary navigation URLs. Apply resource limits at runtime, for example `docker run --cpus=2 --memory=4g ...`; Dockerfiles cannot enforce deployment resource limits.

## Container deployment

Requirements: Docker Engine and Docker Compose 2.20.2 or newer. The default stack pulls the public `linux/amd64` app image, binds the UI only to `127.0.0.1:8888`, and persists SQLite and downloaded data in the `novelreader-data` named volume:

```bash
docker compose pull
docker compose up -d --no-build
curl http://127.0.0.1:8888/api/healthz
```

Enable browser-backed sources with the private WebView profile:

```bash
export WEBVIEW_ENDPOINT=http://webview-worker:8787
docker compose --profile webview pull
docker compose --profile webview up -d --no-build
```

The worker has no host port. Do not add one: `POST /execute` can navigate arbitrary URLs. NovelReader normally derives origin policy from the browser `Origin` and preserved request `Host`; this is ergonomic for trusted LAN/tailnet deployments but inherently trusts the requested hostname. Set optional `PUBLIC_URL` to the exact browser-facing HTTP(S) origin if a reverse proxy rewrites `Host`, you want one access URL, or the service is exposed to untrusted browser networks. Override the host app port with `APP_PORT`, image references with `NOVELREADER_IMAGE` and `NOVELREADER_WEBVIEW_IMAGE`, and measured resource ceilings with the variables in `compose.yaml`.

To build locally instead of pulling GHCR:

```bash
export NOVELREADER_IMAGE=novelreader:local
export NOVELREADER_WEBVIEW_IMAGE=novelreader-webview:local
docker compose --profile webview build
docker compose --profile webview up -d --no-build
```

Both containers run as UID/GID 10001. Named volumes work without host preparation; a bind-mounted data directory must be writable by `10001:10001`. Stop the app before copying or archiving `/data` so the SQLite database and sidecar files form a consistent backup.

## GHCR publishing

`.github/workflows/publish.yml` verifies all tests and the Compose E2E gate before publishing:

- `ghcr.io/otwako/novelreader`
- `ghcr.io/otwako/novelreader-webview`

A `main` push receives `edge` and `sha-*`; a valid `v*` tag receives its semantic version and `sha-*`; manual dispatch receives `manual` and `sha-*`. No `latest` tag is produced. Base images intentionally follow the latest official Node, Go, Alpine, and Python tags; application lockfiles and Patchright remain pinned.

After the first workflow publication, the owner must mark both packages **Public** in GitHub package settings. Until then, authenticate with `docker login ghcr.io`. The repository must have a `main` branch and Actions permission to write packages before the workflow can publish.

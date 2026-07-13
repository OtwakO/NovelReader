# NovelReader

NovelReader is a web-first novel reader with a Legado-compatible booksource engine. It imports raw Legado source JSON, executes regular and JavaScript sources through a shared request/session pipeline, and exposes a browser transport seam for future WebView support.

## Local setup

Requirements: Go, Node.js, and npm.

```bash
cd frontend && npm install && npm run build
cd .. && ./dev.sh run
```

The server listens on port `8888` by default. Set `PORT`, `DATABASE_PATH`, or `DATA_DIR` to override local settings.

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

`-indices` is optional; omitting it runs every source. `-health-url` is optional but aborts the run if the target server stops responding. The CLI uses the production fingerprint transport. Site DNS, WAF, timeout, WebView, and stale-rule failures are reported separately rather than silently treated as parser failures.

Deterministic response fixtures live in `testdata/booksource/`; their manifest test executes the declared rules offline.

## Deployment

Build the frontend, then build and run the backend server:

```bash
cd frontend && npm run build
cd ../backend && go build -o novelreader ./cmd/server
./novelreader
```

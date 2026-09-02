# NovelReader

NovelReader is a self-hosted web app for finding, saving, and reading web novels with Legado-compatible BookSources.

Your books, sources, reading progress, bookmarks, settings, and cached chapters stay in your own NovelReader data folder.

## Features

- **BookSource support** — Import Legado-compatible BookSource JSON files without changing their stored definitions.
- **Search** — Search across enabled sources in manageable batches.
- **Explore** — Browse the native catalog offered by one source.
- **Source Collections** — Install sources from a JSON file or public URL, update them together, and temporarily hide a whole collection from Search and Explore.
- **Personal shelf** — Save books and keep their selected source binding.
- **Web reader** — Read prose and inline images with adjustable typography, reading width, Chinese conversion, image visibility, keyboard controls, and wake lock.
- **Progress and bookmarks** — Save chapter position and annotated bookmarks.
- **Offline chapter fallback** — Keep bounded cached copies of previously loaded chapters.
- **Source recovery** — Switch a saved book to another matching source when needed.
- **Reader accounts** — Give each reader an isolated library and settings area.
- **Backup and restore** — Export and restore portable Reader Data archives.
- **Optional browser sources** — A private WebView worker supports sources that need browser execution.
- **Installable web app** — Install NovelReader as a PWA on supported desktop and mobile browsers.

NovelReader aims for practical Legado compatibility, but it cannot support every Android- or JVM-specific source behavior. Captchas and automatic WAF bypass are not supported.

## Deploy with Docker Compose

### Requirements

- Docker Engine
- Docker Compose 2.20.2 or newer

### 1. Download the deployment file

Download [`docker-compose.yml`](docker-compose.yml) into an empty folder.

### 2. Set the first-admin token

Open `docker-compose.yml` and replace:

```yaml
ADMIN_BOOTSTRAP_TOKEN: "change-this-before-first-start"
```

with a strong temporary value.

You may also change:

- `8888:8888` to use another host port;
- `TZ` for your time zone;
- `PUID` and `PGID` for Linux file ownership;
- `PUBLIC_URL` when using a reverse proxy with one fixed public address.

### 3. Start NovelReader

```bash
docker compose pull
docker compose up -d
```

Open:

```text
http://localhost:8888
```

Use the temporary token to create the first Administrator account.

### 4. Remove the setup token

After the first Administrator is created, clear `ADMIN_BOOTSTRAP_TOKEN` in `docker-compose.yml`, then recreate the app container:

```bash
docker compose up -d --force-recreate app
```

NovelReader stores its data in the `data` folder beside `docker-compose.yml`. The WebView worker runs only on the private Compose network and does not expose a browser port to the host.

## Update

```bash
docker compose pull
docker compose up -d
```

## Stop

```bash
docker compose down
```

This keeps the `data` folder. Do not delete that folder unless you intend to remove all NovelReader data.

## Back up the deployment

Stop the app before copying its data folder:

```bash
docker compose stop app
tar -czf novelreader-data-$(date +%F).tar.gz data/
docker compose start app
```

NovelReader also provides per-reader backup and restore from the web interface.

## Registration and recovery

Public reader registration is disabled by default. To enable it, set:

```yaml
REGISTRATION_ENABLED: "true"
```

You can also set `REGISTRATION_INVITE_CODE` to require an invite code.

`ADMIN_RECOVERY_TOKEN` enables emergency Administrator recovery while the value is configured. Treat setup, recovery, and invite values as secrets and remove them when no longer needed.

## Local development

Requirements: Go, Node.js, and npm.

```bash
cd frontend
npm ci
npm run build
cd ..
./dev.sh run
```

On Windows, run `run-local.bat`.

The server uses port `8888` by default. The optional WebView worker additionally requires `uv`; see [`webview-worker/`](webview-worker/) for its locked environment.

## Tests

```bash
cd backend
go test ./...

cd ../frontend
npm test
npm run build
```

Required tests use deterministic synthetic fixtures and must work without private BookSources or live websites. Complete real BookSources stay in the ignored local `test-booksources/` directory and are used only for optional local compatibility checks and audits. See [`testdata/booksource/README.md`](testdata/booksource/README.md) for the fixture policy.

For container verification:

```bash
./docker-e2e.sh
```

## Project documentation

- [`PRODUCT.md`](PRODUCT.md) — product purpose and interaction principles
- [`PLAN.md`](PLAN.md) — current development state and priorities
- [`docs/architecture/`](docs/architecture/) — current architecture
- [`docs/roadmaps/`](docs/roadmaps/) — future compatibility and UX direction

## Container images

The deployment uses:

- `ghcr.io/otwako/novelreader:latest`
- `ghcr.io/otwako/novelreader-webview:latest`

If the packages are private, sign in before pulling:

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

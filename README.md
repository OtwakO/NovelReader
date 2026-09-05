# NovelReader

NovelReader is a self-hosted web app for finding, saving, and reading web novels with Legado-compatible BookSources.

Your books, sources, reading progress, bookmarks, source settings, and server-cached chapters stay in
your own NovelReader data folder. Reader appearance and prefetch preferences are stored in the browser.

## Features

- **BookSource support** — Import Legado-compatible BookSource JSON files without changing their stored definitions.
- **Search** — Search across enabled sources in manageable batches.
- **Explore** — Browse the native catalog offered by one source.
- **Source Collections** — Install sources from a JSON file or public URL, update them together, and temporarily hide a whole collection from Search and Explore.
- **Personal shelf** — Save books and keep their selected source binding.
- **Web reader** — Read prose and inline images with adjustable typography, reading width, Chinese conversion, image visibility, keyboard controls, and wake lock.
- **Chapter navigation** — Reuse recent chapters and preload the next readable chapter (enabled by default; disable under Typography). The reader's top-right three-dot menu offers Bookmarks and Refresh. Source switching clears session reuse; refetch still shows an explicit offline copy if the source is unavailable.
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

### Docker Compose from the checkout

Run from the repository root with Docker Desktop using Linux containers (or Docker Engine + Compose):

```bash
docker compose -f docker-compose.local.yml up --build -d
docker compose -f docker-compose.local.yml logs -f
docker compose -f docker-compose.local.yml down
```

This builds both services from the current checkout, including uncommitted code, rather than pulling
published NovelReader images. Open `http://localhost:8888`. Base images and build dependencies still
require downloads; code changes require another `up --build`. The WebView worker stays private.

Data is bind-mounted from `./data` by default. Edit `volumes`, `ports`, and `environment` directly in
`docker-compose.local.yml`, following the same style as the deployment Compose file. To use another
Windows directory, for example, change the bind mount to `"D:/NovelReader/data:/data"`. Check that the
selected directory contains your existing data before starting: Docker creates missing bind-mount
directories, which would result in a fresh installation. Never commit locally entered credentials.

**Before reusing data:** stop any native server or other deployment using that directory, and back
up the complete data root, including credential keys. Data from the retired Windows app launcher lives under
`./backend/data`; either change the bind mount to `./backend/data:/data` or use your intended `./data`
directory. Nothing is moved automatically. Do not open the same databases in two instances. Experimental branches can
have incompatible storage schemas; restore the backup before returning to an incompatible version.
On Linux, use `PUID`/`PGID` matching the directory's owner (default 1000); the entrypoint adjusts the
mount root's ownership. `down` removes containers, not the bind-mounted data.

Existing accounts remain usable. A fresh installation additionally needs a non-empty
`ADMIN_BOOTSTRAP_TOKEN` for first-administrator setup. Registration defaults on for this localhost-only
configuration. Both Compose files expose `WEBVIEW_BROWSER_MODE: "headless"`; change it to
`"headful"` for Chrome under Xvfb. This applies to all WebView requests with no automatic mode fallback.
Local builds reuse dependency layers; to explicitly refresh Patchright and Chrome, run
`docker compose -f docker-compose.local.yml build --no-cache webview-worker` before `up --build`.

### Native tools

Requirements: Go, Node.js, and npm.

```bash
cd frontend
npm ci
npm run build
cd ..
./dev.sh run
```

For Windows testing, use [local Docker Compose](#docker-compose-from-the-checkout); the native Windows
batch launchers have been retired.

The server uses port `8888` by default. Native Linux WebView setup additionally requires `uv` and
branded Chrome; see the [worker setup and platform limitations](webview-worker/README.md#local-process).
For Windows/macOS WebView testing, use the local Compose setup above.

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

The deployment uses a jointly verified app/worker pair:

- `ghcr.io/otwako/novelreader:latest`
- `ghcr.io/otwako/novelreader-webview:latest`

CI builds the app and worker concurrently using `docker-bake.hcl`, tests the worker in headless and
headful modes, and runs Compose against the exact images before publishing their digests without rebuilding. Main-branch releases update `latest` and
`edge`; each image also gets an immutable `sha-<full commit SHA>` reference. Alias updates are sequential
(worker first, app last), not a registry-wide atomic operation. To pin or roll back, keep both verified
image digests together. Rebuilding an old commit may resolve newer WebView dependencies.

If the packages are private, sign in before pulling:

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

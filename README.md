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
- **Chapter navigation** — Reuse recent chapters and prepare the next two readable chapters, including display conversion (enabled by default; disable under Typography). Expired forward copies renew while the reader is visible. Navigation shows the requested chapter immediately, with destination-specific loading and retry. The reader's top-right three-dot menu offers Bookmarks and Refresh. Source switching clears session reuse. Refresh bypasses client and backend chapter caches; on failure, already-displayed prose stays visible with an error.
- **Progress and bookmarks** — Save chapter position and annotated bookmarks.
- **Chapter caching** — BookSource reads reuse qualified backend copies for 24 hours from retrieval, without contacting the upstream site. Missing/expired copies fetch upstream; expired copies are never served as outage fallback. TXT/EPUB validity follows the imported book's content revision. Browser memory and IndexedDB retain up to three recently read books and five chapters per book; each retains its catalog too. Reopening makes a small fresh book-state validation request before reusing matching catalog, chapters and in-memory converted display. Full reload preserves catalog/chapter copies but may repeat display conversion; source controls load their metadata only when opened. Browser storage failure falls back to memory/network, not a reading failure. This is not offline/PWA chapter downloading.
- **Source recovery** — Switch a saved book to another matching source when needed.
- **Reader accounts** — Give each reader an isolated library and settings area.
- **Backup and restore** — Export and restore portable Reader Data archives.
- **Optional browser sources** — A private WebView worker supports sources that need browser execution.
- **Installable web app** — Install NovelReader as a PWA on supported desktop and mobile browsers.

Source documents are limited to 50 MiB. Custom font files are limited to 20 MiB (21 MiB for the complete multipart request); oversized source replacements and font uploads return HTTP 413.

NovelReader aims for practical Legado compatibility, but it cannot support every Android- or JVM-specific source behavior. Captchas and automatic WAF bypass are not supported.

## Deploy with Docker Compose

### Requirements

- Docker Engine on a 64-bit AMD64 or ARM64 Linux container host
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

### Chapter-resource cache capacity

The backend owns a disposable image-recipe cache at `DATA_DIR/cache/chapter-resources.sqlite`, outside portable Reader Data backups. BookSource chapters now publish immutable image bundles to this store. Recipes remain available through the chapter's advertised 24-hour freshness interval, independent of chapter-text eviction; source edits or ownership changes invalidate affected references. Upstream access is still required to retrieve remote image bytes. The broader cache/prefetch work remains in the [active implementation plan](docs/plans/2026-09-22-reader-cache-and-prefetch.md).

Set these environment variables on the app process, or uncomment the entries under `app.environment` in `docker-compose.yml` and recreate the app container:

| Variable | Default | Limit |
|---|---:|---|
| `CHAPTER_RESOURCE_CACHE_MAX_MIB` | 1024 | Installation-wide encoded bundle bytes, in MiB |
| `CHAPTER_RESOURCE_CACHE_READER_MAX_MIB` | 256 | Per-reader encoded bundle bytes, in MiB |
| `CHAPTER_RESOURCE_CACHE_MAX_BUNDLES` | 10000 | Installation-wide bundle count |
| `CHAPTER_RESOURCE_CACHE_READER_MAX_BUNDLES` | 2000 | Per-reader bundle count |

These are ceilings, not preallocated space; both scopes apply. They do not limit books, bookmarks, progress, imported TXT/EPUB data, chapter-text caches or browser caches. Bundles contain private image recipes and any inline image payloads, not downloaded remote images. Invalid or nonpositive values use defaults. Leave additional disk headroom for SQLite metadata/journals and durable application data; these settings are not filesystem quotas. Size limits for active usage, especially inline images—not simply registered-user count.

If a bundle cannot be admitted, useful prose remains readable with an unavailable-image notice; image-only chapters report a loading failure. Use the reader's existing chapter refresh action to retry. Expiry does not clear displayed prose or already-loaded images, but a later image request may require refresh.

The store reclaims expired bundles rather than evicting unexpired promises. Lowering limits below existing usage prevents new allocation until usage falls; it does not purge existing bundles. Storage-open failures are logged and leave existing cache files untouched; unrelated reading stays available. Fix the storage problem and restart to reopen the store. Reader deletion remains retryable if its cache cleanup fails.

## Update

Reload open reader tabs after upgrading so their Refresh action uses the current cache-bypass protocol.

This revision requires reader schema epoch 16 (EPUB server-inbox claims and portable cleanup ownership).
EPUB browser upload/review HTTP endpoints now use shared file admission and storage-owned acceptance;
accepted books read through the shared reader. The browser import/review UI supports TXT and EPUB;
server-inbox acquisition also supports both formats. Existing epoch-15 (or older) homes and portable archives are rejected, not migrated. Before upgrading an existing deployment,
stop it and preserve a complete `DATA_DIR` copy; follow the
[compatibility and reset runbook](docs/runbooks/development-data-reset.md) rather than
deleting data or editing schema markers to bypass the check.

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

When restoring a complete data-folder copy, follow the [cold-copy procedure](docs/runbooks/development-data-reset.md#complete-deployment-cold-copy), including renewal of reader-home generations before restart. Ordinary restarts do not require this.

NovelReader also provides per-reader backup and restore from the web interface. Confirming restore
retires this tab's old reader work and unsent uploads before replacement begins. If the response is
lost, use **Check restore status**; do not resend the commit. Navigation/reload returns this tab to
recovery until resolved. A prepared-but-unstarted request must be canceled before continuing. If the
server's process-local record is gone, explicitly continue with fresh state and inspect the library;
an unknown result does not mean success or failure.

Portable reader backups exclude fetched BookSource chapter caches on both export and restore;
those chapters require upstream access again after restoration. Saved catalogs, progress,
bookmarks, and imported TXT/EPUB originals and prepared reading data are preserved. Export
does not clear the live reader cache. Portable snapshots also omit home generation: manual
restore must copy their manifest along with the data. The complete deployment copy above
remains unfiltered. Other open tabs detect a replaced home on a subsequent reader request
and ask for a reload; stale writes are not automatically retried.

Portable reader backups have the same limits on export and restore: 2 GiB compressed,
8 GiB of unpacked entry payloads, and 100,000 entries (including directories and backup
metadata). An over-limit export fails; discard any partial download. For larger homes,
use the complete deployment backup above.

A successful replacement may report recovery or old-file cleanup warnings: the new data is already
active, while retained records/files need attention; check server logs. Corrupt or incompatible
archives are still rejected before replacement.

## Local TXT and EPUB imports

Admitted TXT and EPUB books use the existing reader, progress, bookmarks, and removal
controls. TXT is literal prose; EPUB uses sanitized structured prose, not publisher HTML/CSS.
Removing a local book also deletes its managed original and prepared files. If file cleanup is incomplete, Book Detail keeps a warning and a
**Retry file cleanup** action visible; the book is already removed from the library and the cleanup
record remains recoverable. On the shelf, choose **Local import** (**本地匯入**) to open the dedicated
page, then **Choose files**. **Review before adding to shelf** is enabled by default; uploaded books
wait for **Check book → Add book**. This device-local setting is shared with Settings. Turn it off to
automatically add newly selected books without content warnings; changing it does not approve files already queued.
Content warnings and failures require attention. Inspect the text and, for TXT, optionally open **Adjust chapters**
before adding; choose **Read** once the book is on the shelf. **Earlier imports** and **Import from server folder** keep recovery and server tools in
collapsible sections. The shelf itself has no upload or review workspace.
Failed analysis shows guidance specific to encoding, input format, section limits or storage problems;
older unclassified failures keep a safe generic message. The same guidance appears during re-analysis.
Transfers continue one file at a time while navigating within the app; closing/reloading the tab loses
unsent selections, not acquired files. After reloading, recover unfinished books under **Earlier
imports** and explicitly add them there; automatic approval is not applied to old receipts. An outage
pauses remaining uploads without silently repeating an uncertain upload or addition. Imports also
provides pending-discard and retained cleanup-retry controls.

EPUB imports keep original images by default. **Optimize EPUB images** is one saved device-local
choice shared by browser and server-folder intake; it applies to newly queued EPUBs, not files already
queued or acquired. Optimization uses server-prepared WebP quality 92, with proportional resizing to
a maximum 2048-pixel longest edge. Both modes retain the unchanged EPUB. If native encoding is
unavailable, an explicit portable-encoder performance notice appears; this alone does not force review.

TXT and EPUB **Book preview** share a visible contents list beside the reading area (above it on
mobile), with left/right arrow navigation. Title and author fields stay visible above the preview.
TXT shows the whole selected chapter, with bounded pages of chapter headings. EPUB uses saved authored
contents (or a reading-section list), with text and illustrations including an opening cover; internal
chapter links remain inactive. All displayed images, including authored inline images, are centered in
both preview and the reader. Browsing a preview does not add the book or save reading progress.
Existing ready imports need no reimport. EPUB content-loss notes remain visible above the preview. Retry uses the observed failed preparation; wait for running
preparation to finish before discarding it. No EPUB reparse/image-mode change is offered after
acquisition. The server-folder section uses a **Format** selector for TXT or EPUB.

Earlier imports defaults to **All** formats; choose **TXT** or **EPUB** to narrow it. It shows persisted
records, including this tab’s transfers, newest imports first. Preparation and publication do not move
records to the top. The shared **Status** filter offers Processing, Ready, Needs review, Failed, Added
and Removing. **Needs review** means TXT interpretation warnings or EPUB content/navigation warnings;
it is independent of the device’s review-before-adding setting. Ready excludes these warning cases.
Format/status changes return to the first page and clear bulk selection.

This list is not a second copy of book content: published originals/indexes remain needed for reading. Pending/failed imports retain their files until discarded; there is no automatic
expiry. Removing a book or discarding a pending import deletes its record after file cleanup succeeds.
**Clear finished** only clears this tab's progress list, not books or persisted imports.

Finish copying inbox files before opening the server-folder section or scanning (automated producers should use a
temporary name, then rename). Uncertain leftovers require explicit review: confirmation removes only
a verified duplicate; release keeps the file for a later import. After interruption, inspect its
receipt and inbox claim rather than blindly importing again. The authenticated
[browser-upload](docs/architecture/authentication-and-reader-storage.md#txt-browser-upload-http) and
[inbox APIs](docs/architecture/authentication-and-reader-storage.md#txt-inbox-http) remain available
for direct clients. Epoch 12 separates TXT file lifecycle from active/candidate interpretations;
custom patterns are available both during import and when re-analyzing published books.
There is no migration layer.

For custom chapter detection, select **Custom pattern** in the import or re-analysis review. Enter a Go/RE2
expression, for example `(?i)part [0-9]+.*`, then prepare and review the saved headings before adding or applying.
Patterns are limited to 2 KiB of UTF-8 and match whole trimmed lines (up to 512 bytes); captures do
not replace titles. Lookarounds, backreferences and empty-text matches are unsupported. Invalid
patterns leave the previous interpretation intact. If no headings match, generated divisions still
require review. The exact requested pattern is retained even with that fallback.

To change an existing TXT book, open **Book details → Re-analyze TXT**. Prepare an interpretation,
inspect its contents, full selected chapters and reading-state impact, then explicitly confirm **Apply reviewed
interpretation**. The current book remains readable while preparation runs. Only proven section
matches preserve positions; otherwise choose a resume section or **Start at the beginning**.
Unresolved bookmarks keep their notes and old locations but cannot navigate into the new interpretation.
Applying replaces the old index; there is no index history/undo. Export a backup first if you need one.

**Discard prepared interpretation** keeps the current book and its original file. If a review becomes
stale or a response is lost, refresh status before another decision; unsent option edits are preserved.
An old reader tab that detects the changed revision stops writes and offers **Reopen current saved
location** rather than silently reusing its former section or position.

## Registration and recovery

Public reader registration is disabled by default. To enable it, set:

```yaml
REGISTRATION_ENABLED: "true"
```

You can also set `REGISTRATION_INVITE_CODE` to require an invite code.

`ADMIN_RECOVERY_TOKEN` enables emergency Administrator recovery while the value is configured. Treat setup, recovery, and invite values as secrets and remove them when no longer needed.

## Local development

The current Reader Data schema includes font-cleanup metadata, an ordered chapter index, and generation-qualified TXT interpretations. Older reader schemas are rejected rather than migrated during internal development, so schema changes require fresh or matching-version Reader Data. No data is reset automatically. Use the [development reset runbook](docs/runbooks/development-data-reset.md) if needed; preserve a cold copy before resetting anything you want to keep.

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

The internal image optimizer can use portable Go encoding without native codecs.
Native encoding is substantially faster and needs loadable `libwebp` and `libwebpdemux`
(`libwebp.so` / `libwebpdemux.so` on Linux). The application container provisions these
and checks actual native encoding during its build. Local Import offers optional EPUB image
optimization for browser and server-folder intake; see [Local TXT and EPUB imports](#local-txt-and-epub-imports).

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

Image-processing checks: `cd backend && go test ./internal/imageproc`; add
`CGO_ENABLED=0 go test -tags nodynamic ./internal/imageproc` to verify the portable path.
The Docker build runs these module tests in the final runtime with
`NOVELREADER_TEST_REQUIRE_NATIVE_WEBP=1`, failing if native encoding is unavailable.

For container verification:

```bash
# Publication gate regressions (requires bash, Python 3 and jq; no registry access):
python3 -m unittest discover -s build -p 'test_*.py' -q
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

The release workflow targets `linux/amd64` and `linux/arm64` using native runners. Docker selects the
matching architecture from the shared image tag; Compose needs no platform override. Both use Patchright
with branded Google Chrome, installed from Google's official architecture-specific package.

CI builds the app and worker concurrently on each architecture using `docker-bake.hcl`, tests the worker
in headless and headful modes, and runs Compose against the exact images. Only after both architectures
pass does CI assemble and verify multi-platform indexes from those tested digests, without rebuilding.
Main-branch releases update `latest` and
`edge`; each image also gets an immutable `sha-<full commit SHA>` reference. Alias updates are sequential
(worker first, app last), not a registry-wide atomic operation. To pin or roll back, keep both verified
image digests together. Rebuilding an old commit may resolve newer WebView dependencies.

Manual workflow runs default to verification only: they stage `ci-*` images/indexes but do not change
release aliases or immutable SHA tags. Select the `publish` input to also promote the verified pair to
`manual` (or the version alias when run against a release tag).

If the packages are private, sign in before pulling:

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

---
status: completed
updated: 2026-09-26
---

# Unified original-cover caching

## Goal

Give BookSource and imported EPUB covers consistent browser-cache reuse without resizing,
re-encoding, duplicating, or persistently caching images on the server. Repeated visits should
reuse a fresh original cover; known source/publication changes and reader-home replacement
must select a different cache identity.

**Branch:** `feat/unified-cover-cache`, created from `main` at `2bc5a70`.
**Decision status:** implementation authorized. The user confirmed this is internal development:
old cover URLs need not remain compatible; manual testing can clear browser data.

Done means both providers use the shared cover presentation boundary, fresh covers can be
reused without a server request, invalidation/isolation tests pass, and actual browser-cache
reuse is verified separately from HTTP-header unit tests. Do not promise a Lighthouse score.

## Scope

Included:
- Original-cover delivery for stored BookSource and EPUB books using existing authenticated APIs.
- Shared private seven-day HTTP caching and provider-qualified display URLs.
- Source/binding/configuration, publication, reader, and restore-generation invalidation.
- Preservation of candidate/Search/Explore BookSource cover behavior and source-native decoding.
- Focused regression tests, browser verification, and relevant current documentation updates.

Excluded:
- Thumbnails, compression/re-encoding of images, extra image files, server-side image caches,
  new dependencies for image processing, or use of `files/covers/` as a cache.
- A frontend IndexedDB/Cache Storage/service-worker cover cache or cache-management UI.
- Changes to EPUB internal-resource caching, chapter content caching, retention, or prefetch.
- TOC virtualization, Chinese-conversion rendering, contrast, text compression, SEO, or other
  Lighthouse findings. These are separate workstreams, not implicit additions to this plan.
- New refresh controls, background upstream polling, data migrations, deployment, or a push.

## Baseline evidence and implementation

The latest local Lighthouse reports, captured on 2026-09-26 after the `2bc5a70` release, showed
an approximately 790 KiB original EPUB cover downloaded during both navigation and the later
interaction journey; a BookSource cover was reused from disk during the journey. This is the
motivation for repeat-download improvement, not evidence of a controlled before/after result.
The first download remains full-sized by explicit user choice.

Reports are disposable, untracked evidence: `lighthouse-analysis-nagivation-latest.html`
(filename spelling intentional) and `lighthouse-analysis-timespan-latest.html`, plus the earlier
captures. Do not commit reports, extracted raw report JSON, private book identifiers, or URLs.
The disposable reports were removed after this work, as requested. Tests do not depend on them.

Pre-change code entry points (paths relative to repository root; final behavior is recorded under
Current State and in `docs/architecture/discovery-and-reading.md`):

| Owner | Current behavior / implementation reference |
|---|---|
| Display projection | `backend/internal/api/library_reads.go`: BookSource emits a versioned `/api/books/{id}/cover` URL; EPUB emits its general resource URL via `epubResourceHref`. Shelf and detail share this projection. |
| Cover identity and response | `backend/internal/api/cover_display.go`: `coverRevision`, `storedCoverDisplayURL`, signed candidate references, and `writeCoverBytes`; success uses `private, max-age=604800`, `Vary: Cookie`, original content type and `nosniff`. |
| Stored cover retrieval | `backend/internal/api/reader_handlers.go::handleGetBookCover` is currently BookSource-only, using the native book/source stores and `Searcher.GetBookCover`. It does not validate the `v` query against current state. |
| Candidate covers | `handleGetCoverDisplay` in `cover_display.go` serves signed BookSource references for unshelved results. Preserve signature validation and source-specific retrieval. |
| EPUB resources | `backend/internal/api/epub_resources.go`: `ReadResource` checks publication/revision; responses deliberately use `private, no-store`. Covers currently inherit this general-resource policy. |
| Reader lifecycle | `backend/internal/api/reader_api.go`: authenticated runtime/home lease, default `no-store`, and optional `readerGeneration` query validation. `coverCacheScope` currently derives from reader ID, not home generation. |
| Routes | `backend/internal/api/reader_routes.go`: existing stored-cover, candidate-cover and EPUB-resource routes. |
| Browser presentation | `frontend/src/features/books/BookCover.vue`: renders the supplied URL, keeps aspect-ratio/backdrop behavior, and resets failed-image state when the URL changes. |
| Empty covers directory | `backend/internal/readerstore/home.go`: `files/covers/` is created and required by home validation, but has no production cover writer. Leave it alone. |

**Baseline gap addressed by this work:** BookSource cover identity did not include the persistent
home generation. Cover-only generation qualification now closes that gap. EPUB resource URLs
already included reader/generation qualification; their general policy remains unchanged.

## Accepted approach

### Shared ownership, not a generic media framework

Unify cover presentation at the existing API boundary. Reuse the stored-book cover route for
both providers where practical; keep the signed candidate route for books not yet in the library.
“Shared delivery” means one cache/identity policy and response writer, not forcing every cover
into one URL shape or introducing interfaces with one implementation.

The intended flow is:

1. Shelf/detail projections emit a same-origin, qualified `coverDisplayUrl` from metadata.
2. Browser reuses an eligible fresh HTTP-cache entry, or requests that URL.
3. On a server request, existing authentication/home ownership applies; resolve the stored
   library item and validate cover qualification before returning a cacheable success.
4. BookSource retrieves bytes through its existing source-aware execution path. EPUB reads
   the publication's designated cover through existing EPUB storage and resource validation.
5. Both return bytes through the shared cover response policy.

Do not fetch artwork or parse the EPUB merely to build shelf metadata. Preserve existing bulk
source-revision enrichment rather than adding per-book network or expensive storage reads.
Do not accept an arbitrary caller-supplied image URL or allow the cover route to expose any
resource from a publication instead of its designated cover.

### Original means no new transformation

EPUB cover responses preserve the original resource bytes and media type. BookSource responses
preserve existing source-required headers, cookies, URL rules and cover decoding; “original”
does not mean bypassing decoding required to obtain the usable image. Add no resizing or encoding.

### Cache policy

- Successful qualified cover responses use `Cache-Control: private, max-age=604800` and
  `Vary: Cookie`, retaining media type and `X-Content-Type-Options: nosniff` behavior.
- Failures, rejected qualifications and authorization failures must not receive seven-day caching.
- No `public` caching or assumption that cached bytes are erased on logout/deletion.
- Seven days bounds freshness, not browser disk residence. Browsers may evict sooner or retain
  stale bytes longer; expired entries must contact the server before ordinary reuse.
- No new ETag/304 requirement. Conditional revalidation can be considered separately if measured
  expiry traffic warrants it; it is not necessary for fresh-cache hits.
- Known invalidation changes the displayed URL after fresh metadata is obtained. It neither
  purges old browser entries nor pushes metadata changes into already-rendered pages.
- Silent upstream replacement at an unchanged URL can remain cosmetically stale until expiry.
  This is accepted for covers. No new explicit cover-refresh feature is in scope.

### Identity and lifecycle invariants

| Event | Required result |
|---|---|
| Same reader/home and unchanged cover inputs | Stable generated URL; ordinary navigation/progress updates must not bust the cache. |
| BookSource binding, source definition, cover URL, or relevant source profile changes | New cover identity. Preserve existing settings/authentication revisions and native variable inputs; inspect current coverage before adding anything. |
| EPUB designated cover or publication revision changes | New cover identity using publication metadata, not BookSource state. |
| Reader changes | Different cover namespace; a network request remains authenticated against the active reader. |
| Portable restore/home replacement with repeated book IDs/revisions | New cover identity from persistent home generation. An old generation-qualified URL must not select replacement-home artwork. |
| Ordinary restart without data replacement | Do not invent a rotating timestamp/cache nonce for stored covers; preserve stable identity where the existing contract allows it. |
| Deleted/missing book, missing cover, unavailable source, stale publication | Explicit existing-style failure on network access; frontend fallback still works. Cached bytes cannot be remotely revoked. |

Identity must qualify the provider inputs actually used to retrieve bytes. Do not serve newly
selected cover bytes as a cacheable response under a recognized stale qualifier. Retain existing
reader-home leases and source execution ownership rather than adding disconnected locks.
Never serialize credentials/cookies into new cache-key parameters; use existing opaque/version
mechanisms and safe diagnostic conventions.

## Resolved implementation contract

- Stored covers require the current `v` qualifier; absent/stale qualifiers return an uncached
  not-found response. No legacy fallback or new version-negotiation layer: the user explicitly
  waived old-URL compatibility. Imported books themselves need no migration.
- Keep the existing reader-ID scope used by general EPUB resources unchanged. A cover-only
  scope adds home generation to stored and signed candidate identities. Candidate references
  must match their current scope and source/profile revision, not merely have a valid signature.
- Reuse EPUB `ReadResource` for the designated cover only; preserve its bounded reads and
  publication checks. No new image preparation or storage path.
- Qualify BookSource inputs before retrieval and recheck current cover identity after retrieval,
  before making the response cacheable. Existing source workflow and home leases remain owners;
  optimistic revision checks reject concurrent changes without a second lock or storage layer.
- All projections reuse the same provider-specific identity helpers; shelf enrichment remains
  batched. The frontend continues consuming `coverDisplayUrl` unchanged.

### Keep explicit provider handling; accept minor duplicate reads

The API owns cover qualification/HTTP policy; existing provider stores and retrieval paths own
image acquisition. Keep the two explicit provider branches rather than adding a generic provider
interface or repository layer solely to eliminate duplicate reads.

`loadStoredCover` reads the library item, and BookSource `GetBook` reads it again as part of its
coherent native projection. Repeating this before/after acquisition adds two redundant indexed local
reads per server-handled BookSource cover. Fresh browser hits bypass this work entirely. No measured
bottleneck justifies more indirection; preserve the before/after identity checks for correctness.

Revisit only if profiling shows material cost or additional real callers need the same snapshot.
Then prefer one coherent metadata read returning the required cover inputs, not a generic cover
framework. The user accepted retaining the current implementation after the scoped sanity review.

## Milestones and progress

- [x] **M0 — Plan and branch:** accepted scope, inspected baseline, boundaries and handoff recorded.
- [x] **M1 — Finalize contract:** resolve the three gates; add deterministic tests for the intended
  qualification and cache policy, reproducing the current EPUB no-store/restore identity gaps.
- [x] **M2 — Shared delivery:** extend stored-cover handling through library/provider ownership;
  reuse original-image acquisition and the common success writer; keep general EPUB resources
  unchanged. Integrate all cover URL projections and identity invalidation in the same working step.
- [x] **M3 — Correctness verification:** run focused API/provider regressions, lifecycle/isolation
  tests and relevant frontend tests; review the actual diff for orphaned helpers and scope creep.
- [x] **M4 — Browser verification and completion:** demonstrate repeat cache hits and changed-key
  misses, update current architecture documentation if needed, record limits and complete the plan.

Commit complete working steps, not half-wired HTTP changes. Update Current State, Next Action and
Verification at meaningful milestones and before a substantial commit or unfinished handoff.
Do not merge or push without authorization.

## Verification

Passed:
- `go test -race ./internal/api -run 'Cover|EPUBPublication|LibraryReads' -timeout 120s`.
- Complete affected packages: `go test ./internal/api ./internal/epubstore -timeout 180s`.
- `npm test -- src/features/books/BookCover.test.ts` (unchanged frontend presentation contract).
- Scoped diff review and whitespace validation. AFT lacked authoritative Go/Markdown diagnostics;
  the actual Go compilation/tests above are the verification authority.

Real-browser check: local Linux Headless Chrome 148, fresh cache, no extensions or request
interception, synthetic original PNG covers through the real authenticated server. An ephemeral
fixture reused the existing ownership/EPUB test setup and was removed afterward. For **each** of
BookSource and EPUB, cold loading transferred 396 browser-reported bytes (96-byte image plus
Resource Timing overhead); leaving/re-entering and normal reload each transferred **0 bytes** and
made **no additional cover request at the server**. Advancing source/publication revisions produced
new URLs and one fresh request per cover. Both images decoded at their original 3×2 dimensions.

Limits: this is cover-delivery verification, not a new live Lighthouse benchmark or full Vue journey.
Real seven-day expiry/browser-restart disk persistence was not waited out. Reader-switch/restore
rejection was verified in authenticated API tests, not browser automation. No new image/file writes
were introduced (code inspection); no server disk-growth benchmark is claimed.

### Existing reusable tests

- `backend/internal/api/cover_test.go`: source headers/decoder, original-byte fallback, cache
  headers, source/profile/reader identity changes, candidate tampering and arbitrary-target rejection.
- `backend/internal/api/epub_reading_test.go`: synthetic EPUB and authenticated reading/resource
  routes. Extend/reuse the fixture; do not add real books or source definitions.
- `backend/internal/api/library_reads_test.go`: shared shelf/detail projection and provider behavior.
- `frontend/src/features/books/BookCover.test.ts`: backend URL rendering, image fallback and retry
  on URL change. Run related shelf/detail tests if their integration changes.

### Required behavioral coverage

1. EPUB cover delivery returns exact original bytes/media type and the same success cache policy
   as BookSource; general EPUB-resource responses remain `no-store`.
2. Unchanged inputs produce stable URLs; each relevant invalidation family changes its key. Avoid
   duplicating existing source/profile tests when an extension covers the same invariant.
3. Real authenticated reader contexts cover account isolation and restore with identical stored
   IDs/revisions but a new generation. URL inequality alone is not sufficient server-access coverage.
4. Missing/stale/tampered/unauthorized requests fail without long-lived success caching. Check the
   agreed legacy behavior explicitly and preserve source retrieval/decoder semantics.
5. Shelf and single-book metadata select the same cover identity without fetching the cover itself;
   unsupported/missing-cover providers retain the current fallback rather than failing the shelf.

Start with affected tests, e.g. from `backend/`:
`go test ./internal/api -run 'Cover|EPUBPublication|LibraryReads' -timeout 120s`.
Then run the API package and changed provider packages. Use focused race-enabled lifecycle tests
where ownership changes; expand only for actual coupling. From `frontend/`, run the BookCover and
any affected projection/shelf/detail tests; run typecheck/lint/build if frontend code changes.
Commands listed here are planned checks, not claims of passing runs.

### Browser and resource acceptance

Use an authenticated synthetic/local book with a fresh browser profile, extensions disabled and
DevTools **Disable cache unchecked**. Request interception can disable the HTTP cache; use a real
server and browser network/cache evidence rather than mocked-route tests for this check.

- Cold visit: original cover downloads and displays correctly.
- Warm leave/re-enter and normal reload: same URL reuses memory/disk cache when available;
  distinguish a browser-visible cached request from an actual server transfer.
- Known cover/source/publication change: refreshed metadata supplies a new URL and updated image.
- Reader switch/restore: different identity; old qualified network requests cannot resolve the new
  reader/home cover. No claim of revoking already-cached pixels.
- Verify seven-day freshness via headers; use controlled browser time/expiry testing if available,
  otherwise explicitly report that real seven-day expiry was not waited out.
- Server produces no extra cover files, no schema change, and no new portable-backup cache payload.

The expected gain is fewer repeated original-image transfers, not a smaller cold payload or a
promised Lighthouse score. Report measured requests/bytes and browser conditions separately.

## Compatibility and rollback

Keep `coverDisplayUrl` and existing routes; use backend-only wiring. Old stored/candidate cover
qualifiers are intentionally unsupported (internal-development decision); reload metadata. Old EPUB resource URLs
remain valid under their original no-store policy. No migration, new storage directory, or backup
format change is planned. Inspect authorization/cache behavior carefully: browser identity and
restore boundaries make implementation security-sensitive despite the narrow user-visible change.

Rollback is a code revert to the previous metadata projection/delivery policy, not deletion of
reader files. Already-fresh browser responses may survive a rollback until expiry/eviction; changing
server headers cannot retroactively revoke them. Use a new qualified URL namespace if a later
correctness fix must avoid an existing cached response. Do not promise immediate cache revocation.

## Current State

Implementation and scoped verification complete on `feat/unified-cover-cache`. Stored delivery
lives in `cover_stored.go`; projection and signed candidate delivery share replacement-aware identity
and the success response policy. No frontend, general EPUB-resource, image processing, schema or
filesystem-layout changes. Current architecture is updated in `docs/architecture/discovery-and-reading.md`.
Merge into `main` and remote push are authorized. Container publication/deployment has not been
verified at this documentation checkpoint; no public performance gain is claimed.

## Next Action

No implementation work remains. Verify the publication workflow after integration; reload book
metadata (and clear old browser cache if desired) during manual testing. Keep unrelated Lighthouse/TOC
work separate.

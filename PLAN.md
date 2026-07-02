# NovelReader — Plan

## Objective

A web-first, mobile-friendly novel reading application with a Go backend and Svelte 5 frontend. At its core is a legado-compatible **booksource system** that can import, manage, search, and crawl books from arbitrary websites using the legado BookSource JSON format. Designed for Docker deployment, single-user first but architecturally multi-user ready.

## Architecture

### Directory Layout

```
novelreader/
├── backend/
│   ├── cmd/server/            # Entry point (port 8888)
│   ├── internal/
│   │   ├── api/               # HTTP handlers, SSE, routes
│   │   ├── booksource/        # BookSource entity, CRUD, import/export
│   │   ├── fetcher/           # HTTP client (stateless + cookie jar variants)
│   │   ├── analyzer/          # Rule engine: CSS, XPath, JSONPath, Regex, JS
│   │   ├── book/              # Search stream, TOC parser, content, store
│   │   ├── processor/         # Content sanitization, paragraph formatting
│   │   ├── config/            # ENV-based config
│   │   └── database/          # SQLite (WAL, read pool)
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── lib/               # Bookshelf, SearchPage, Reader, Settings, SourceList, BookDetail
│   │   └── api/               # Typed API client with SSE streaming
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── dev.sh                     # Build/run script
├── docker-compose.yml
├── test_booksource.json       # Test sources (26 search sources)
├── test_booksource2.json      # Test sources (230 full sources)
├── reference/legado/          # Legado source repo reference
├── PLAN.md
└── README.md
```

### Key Modules

| Module | Responsibility |
|--------|---------------|
| `analyzer` | Rule parsing engine. Dispatches CSS (goquery), XPath (antchfx), JSONPath, Regex, JS (goja pool×4). Handles `##` replacement, `||` OR/chain, `@js:` blocks. |
| `booksource` | BookSource entity with custom `UnmarshalJSON` (accepts nested rule objects). SQLite CRUD. |
| `fetcher` | `Client` with context support. Two variants: `New()` with cookie jar (content fetching), `NewStateless()` without (search — no cross-user leak). |
| `book` | `Searcher` with `SearchStream()` (per-source callback), `ChapterListParser` (legado ToC with pagination/volumes), chapter/content fetching. |
| `processor` | `ContentProcessor`: strips HTML tags (XSS prevention), converts `<br>`/`</p>` to newlines, dedup title, paragraph formatting. No Chinese conversion yet. |
| `api` | REST handlers: sources, books, search, chapters, content, fonts, progress. SSE endpoint at `GET /api/search/stream`. |

### Data Flow — Search (SSE streaming)

```
User types "凡人修仙传" + Enter
  → Frontend opens EventSource: GET /api/search/stream?q=凡人修仙传
  → Backend fans out across 256 sources (50 concurrent, 8s each, 30s total)
  → Per-source callback fires when each source completes:
      SSE event: {type:"results", source:"365小说网", data:[...]}
  → Frontend appends results reactively (Svelte $state)
  → On error: {type:"error", source:"五二书库", message:"timeout"}
  → On all done: {type:"done", total:42, sourcesDone:18}
  → User can cancel mid-search (es.close() → r.Context() cancels all in-flight)
  → Per-source cap: max 20 results per source, enforced in parser
```

### Data Flow — Chapter Content

```
User taps chapter in reader
  → GET /api/books/:id/chapters/:idx/content
  → Backend fetches chapter URL from source via ruleContent
  → Processor strips HTML tags, converts <br> to newlines, formats paragraphs
  → Returns { title, paragraphs: [...] }
  → Frontend renders as plain text paragraphs (no {@html} — XSS safe)
```

### Concurrency Design

```
Request-scoped (per user, per search):
  ├── 50 goroutines (semaphore), each with 8s per-source deadline
  ├── ctx derived from r.Context() → cancelled on client disconnect
  ├── buffered channel (len=candidates) — no goroutine leak on early exit
  └── JSVM pool (4 runtimes) — Eval borrows one, resets bindings, returns

Shared across users (safe):
  ├── searchFetcher: *http.Client with Jar: nil (stateless, no-cookie)
  ├── fetcher: *http.Client with shared cookie jar (correct: per-host cookies)
  ├── CacheManager: LRU with 4096 max entries (container/list)
  ├── SQLite: WAL mode, SetMaxOpenConns(4) for concurrent reads
  └── sourceStore/bookStore: database access (reads concurrent, writes serialized)
```

## Phases

### Phase 1 — Booksource Engine Core (current)
- [x] Project scaffolding: Go module, Svelte 5, Go 1.22 ServeMux, dev.sh
- [x] Database: book_sources, books, chapters, fonts — SQLite WAL, pure Go
- [x] BookSource CRUD: JSON import (nested rules as RawMessage), list, delete, enable/disable
- [x] HTTP fetcher: context-aware, stateless search variant, cookie jar variant, 10MB cap
- [x] Analyzer: CSS, XPath, JSONPath, Regex, JSVM pool×4, `##` replacement, `||` OR/chaining
- [x] Search: SSE streaming, 50 concurrent, 30s timeout, 8s per-source, 20-result cap
- [x] TOC parser: legado-compatible, pagination (nextTocUrl), volume detection, reverse ordering
- [x] Content: fetch via ruleContent, strip HTML, sanitize, paragraph format
- [x] Content processor: HTML→text (XSS safe), title dedup, paragraph reflow
- [x] Reader: font size/weight/line-height/family, bg/text color, preset themes
- [x] Font upload and selection
- [x] Bookshelf: cover, author, last chapter, progress bar, broken-cover fallback
- [x] Book enrichment: fetch full info (cover, intro, author) from source on add
- [ ] Docker build setup (dev.sh exists, multi-stage Dockerfile pending)
- [ ] E2E test with a known-working book source

### Phase 2 — Reading Experience
- Reading progress sync
- Bookmarks
- Page turn animations
- Dark mode themes
- Offline caching
- Explore/discover page

### Phase 3 — Polish & Extras
- Replace rules editor
- Book source debug tool
- Multi-user auth
- Chinese conversion (opencc)
- PWA support

## Out of Scope (Phase 1)
- Multi-user auth (design supports it, implementation deferred)
- Chinese conversion (partial map removed — garbled output)
- Audio books, TTS, RSS, EPUB export
- WebSocket (SSE is sufficient — unidirectional search)
- Virtual scrolling (YAGNI at 20-200 results)

## Deferred Work

Items intentionally skipped or deferred, documented so they don't get forgotten.

| Deferred Item | Affected Sources | Why Deferred | Revisit When |
|---------------|-----------------|-------------|--------------|
| `loginUrl`/`loginUI`/`loginCheckJS` — source auth portal | ~179 sources (19% have loginUrl) | Login flows need UI interaction (credential input, captcha). Not feasible server-side-only. | If user demand for locked sources. Likely need WebView bridge. |
| `coverDecodeJs` — cover image decoding | ~1 source (0.1%, likely Pixiv) | Pixiv covers need JS-based URL transform. Single source. | If Pixiv source becomes critical. Add `evalCoverDecode` in enrichment path. |
| `customButton`/`eventListener` — legado reader UI actions | ~285 sources (30%) | These are legado-app UI extensions (custom buttons, event hooks). Not fetch/crawl logic. Permanently out of scope for backend-only. | Never — belongs in a mobile/desktop client, not this API. |
| `concurrentRate` — per-source request throttle | ~11 sources (1%) | Per-source rate limiting adds complexity (per-source semaphore). 50-way fan-out already limited by semaphore. | If a specific source consistently 503s under concurrent load. |
| `enabledCookieJar` per-source control on content path | All 939 sources (100% set false) | Content fetcher uses shared cookie jar. Stateless content fetcher per source needs more refactoring. | If sources start relying on per-source cookie isolation for login sessions. |
| `enabledReview` — review/book-rating rules | ~9 sources (1%) | Needs a review-parsing engine and UI for displaying reviews. Separate feature. | Phase 3 — polish features. |
| Explore/Discovery (`exploreUrl` + `ruleExplore`) | ~723 sources (76%) | Needs new endpoint, crawl logic, and UI screens. Phase 2 feature. | Phase 2. Design already extensible: exploreUrl → BuildURL → parseRuleExplore. |
| `phonehttp` — mobile UA toggle | ~1 source (0.1%) | Flips User-Agent to mobile. Trivial to add header override. | If the one source is critical. |
| WebView rendering (headless browser) | ~19 sources (2%) | Sources with `webView:true` need a JS-capable browser. Requires chromedp/playwright integration. | When WebView-exclusive sources become critical. Plug-and-go: add a headless-browser fetcher, swap in search path. |

## Current State

**Phase**: Phase 1 — core engine operational, search streaming, bookshelf, reader all functional.

**Last completed**: BookSource field audit + fixes. Missing headers on TOC path (BLOCKER), JSLib never loaded (MAJOR), stateless search fetcher, bookSourceType filter, bookUrlPattern validation, enabledCookieJar defaults.

**Working**: Import 939 sources → SSE search (results stream in, text-only sources only) → enrich book (fetch cover/intro) → bookshelf → chapter list (pagination, volumes) → reader

**Known limitations**: Many sources behind Cloudflare (server-side HTTP client can't bypass). Book source rules go stale over time — users find fresh sources. `customButton` (30%) and `eventListener` (30%) are legado-reader UI fields intentionally not mapped. `concurrentRate` stored but not enforced yet.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Router | Go 1.22 `http.ServeMux` | No external dep, path params built in |
| SQLite | `modernc.org/sqlite` | Pure Go, no CGO, simpler Docker |
| Database pool | `SetMaxOpenConns(4)` | WAL mode permits concurrent readers |
| Search transport | SSE (text/event-stream) | stdlib only, native EventSource, r.Context()→cancel free |
| Search fetcher | Stateless (no cookie jar) | Prevents cross-user cookie contamination |
| Cache | LRU with 4096 max (container/list) | Bounded memory, stdlib only |
| JSVM | Pool of 4, no per-eval re-init | init loaded once at startup; bindings reset per-eval |
| Per-source cap | 20 results, enforced in parser | Prevents source flooding, before dedup |
| Cross-source dedup | None | Showing same book from 2 sources is useful (user picks) |
| Virtual scroll | YAGNI | 20-200 results, `{#each}` is fine |
| Chinese conversion | Deferred | Partial map was garbled; needs opencc |
| Port | 8888 | 8080 commonly taken |

## Scaling Notes (Multi-User)

| Concern | Current | Future |
|---------|---------|--------|
| Cookie isolation | Search: stateless client. Content: shared jar (correct — per-host cookies) | Per-user clients if source logins added |
| JSVM | Pool of 4, no per-eval re-init | Increase pool or use per-user pools if bottleneck measured |
| SQLite writes | Serialized (single writer) | Fine for 3-10 users. Migrate to Postgres for >50 concurrent users |
| Cache | LRU 4096 entries | Increase or use Redis if distributed |
| Goroutine count | 50 per search, fine at 3-10 users | Add global cap if >20 concurrent users |
| SSE per connection | One stream per active search | HTTP/2 multiplexing handles this |

## Issues & Fixes

### [2026-07-03] Search fetcher had shared cookie jar — cross-user leak
- **Problem**: `searchFetcher` used `NewWithTimeout` which creates a cookie jar. Cookies accumulated from one user's search could leak to another.
- **Fix**: Created `NewStateless(timeout)` constructor with `Jar: nil`. Search uses stateless client. Content/intro/TOC fetching still uses jar (correct — per-host cookies benefit both users).
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/book/search.go`
- **Watch out**: Content fetching (GetChapterContent, GetBookInfo) still shares cookies. Correct for now but revisit if per-user source logins are added.

### [2026-07-03] JSVM per-eval re-init wasted CPU + blocked pool
- **Problem**: `Eval` re-ran `initCode` on every call via `RunString`. With 4 runtimes and 50 goroutines, each eval held its runtime longer than needed, inflating contention.
- **Fix**: Removed per-eval `RunString`. `initCode` is loaded once by `LoadLib` into each runtime when it enters the pool. Per-eval only sets `result`/`baseUrl`/`java` bindings.
- **Affected**: `backend/internal/analyzer/js.go`
- **Watch out**: `LoadLib` must be called before any eval. Currently called once at startup.

### [2026-07-03] CacheManager was unbounded
- **Problem**: `CacheManager` used a plain `map[string]string` with no size limit. Memory grows without bound under multi-user load.
- **Fix**: Rewrote with `container/list` LRU eviction, `maxEntries = 4096`.
- **Affected**: `backend/internal/analyzer/cache.go`
- **Watch out**: Eviction is LRU by access order. Tunable via `maxEntries` const.

### [2026-07-03] EventSource auto-reconnect restarted search on transient errors
- **Problem**: EventSource's default behavior on any network error is to reconnect to the same URL, restarting the entire 256-source fan-out.
- **Fix**: Track `finished` flag. On `onerror`, close EventSource if not finished. Trade auto-reconnect for clean stop.
- **Affected**: `frontend/src/api/client.ts`
- **Watch out**: User must manually re-submit search after error. Acceptable for a one-shot search.

### [2026-07-03] Live search counter only counted errors
- **Problem**: `sourcesDone` was only incremented in `onError` callback, not in `onResult`. Status always showed "from 0 sources" during search.
- **Fix**: Increment `sourcesDone` in both callbacks.
- **Affected**: `frontend/src/lib/SearchPage.svelte`

### [2026-07-03] EventSource not closed on route-away
- **Problem**: Navigating away mid-search left EventSource open. Backend kept searching 30s doing wasted work.
- **Fix**: Added `$effect` cleanup that closes EventSource on component unmount.
- **Affected**: `frontend/src/lib/SearchPage.svelte`

### [2026-07-02] XSS via `{@html}` rendering unsanitized source HTML
- **Problem**: Reader used `{@html p}` to render paragraphs. Source HTML (from `@html` attr rules) was injected directly into DOM — `<img src=x onerror=…>` or `<script>` could execute.
- **Fix**: Processor strips all HTML tags, converts `<br>`/`</p>` to newlines, unescapes entities. Reader uses `{p}` (plain text).
- **Affected**: `backend/internal/processor/processor.go`, `frontend/src/lib/Reader.svelte`

### [2026-07-02] Chapter URLs resolved against wrong base
- **Problem**: Chapter URLs were resolved against source root URL instead of the page they were found on. A relative href `456.html` on `https://site.com/book/123/` would resolve to `https://site.com/456.html` instead of `https://site.com/book/123/456.html`.
- **Fix**: `parsePage` now resolves chapter URLs against the TOC page URL (where the selector found them). Added `resolveURL` helper.
- **Affected**: `backend/internal/book/chapterlist.go`

### [2026-07-02] `total_chapter_num` never populated → progress bar dead
- **Problem**: After fetching chapters, `books.total_chapter_num` was never updated. Progress bar always showed "Ch.1 / ?".
- **Fix**: Added `UpdateTotalChapters()` called after TOC fetch. Bookshelf progress bar now renders.
- **Affected**: `backend/internal/book/store.go`, `backend/internal/api/server.go`

### [2026-07-02] Volume entries became broken chapters
- **Problem**: Chapters with empty URL were stored as regular chapters. Opening one resolved the empty URL against source base → garbage URL → 500.
- **Fix**: Empty URL now infers `isVolume=true`. Volume chapters are stored but won't be fetched for content.
- **Affected**: `backend/internal/book/chapterlist.go`

### [2026-07-02] Source routes broken — URLs in path segments
- **Problem**: DELETE/PUT routes used path segments `{url}`. URLs contain slashes — impossible to match.
- **Fix**: Changed to query params. `DELETE /api/sources?url=...`, `PUT /api/sources?url=...`.
- **Affected**: `backend/internal/api/server.go`, `frontend/src/api/client.ts`

### [2026-07-02] Partial Chinese conversion removed
- **Problem**: 50-character s2t/t2s map produced mixed-script garbled text.
- **Fix**: Removed maps, `convertChinese` is a no-op. Awaiting opencc.
- **Affected**: `backend/internal/processor/processor.go`

### [2026-07-03] TOC fetch ignored source headers (BLOCKER)
- **Problem**: `GetChapterList` passed `nil` for headers. Any source needing `Cookie`, `Referer`, or custom `User-Agent` to serve the TOC page got empty results.
- **Fix**: Now passes `parseHeaderJSON(src.Header)` instead of `nil`.
- **Affected**: `backend/internal/book/search.go` (line ~376)
- **Watch out**: Same fix applied to pagination fetches within ChapterListParser (both go through the same closure).

### [2026-07-03] JSLib never loaded into JSVM (MAJOR)
- **Problem**: `src.JSLib` was stored in DB but never evaluated. Sources relying on JSLib-defined helper functions got undefined-reference errors on every rule eval.
- **Fix**: Analyzer now prepends `src.JSLib` to every `jsEval`/`jsEvalList`/`jsEvalElements` call via `SetJSLib()`.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/book/search.go`, `backend/internal/book/chapterlist.go`
- **Watch out**: URL template eval (`@js:` in BuildURL) doesn't get JSLib prepended — only rule eval paths through Analyzer do. Fix if @js: URL sources also use JSLib.

### [2026-07-03] Search used shared cookie jar — cross-source contamination
- **Problem**: `searchFetcher` used `NewInsecure()` which creates a cookie jar. With 50 concurrent source searches, cookies set by one source's redirect domain leaked into another's request.
- **Fix**: Added `NewInsecureStateless()` constructor with `Jar: nil`. Search uses it.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/book/search.go`
- **Watch out**: Content fetcher (`s.fetcher`) still has a cookie jar. Correct for per-host cookie reuse during chapter reading.

### [2026-07-03] Search included audio/image sources (MAJOR)
- **Problem**: `searchCandidates` only checked `SearchURL != "" && RuleSearch != ""`, letting `BookSourceType=1` (audio) and `2` (image) sources through. These produce garbled results when searched with text parsing.
- **Fix**: Added `src.BookSourceType == 0` filter.
- **Affected**: `backend/internal/book/search.go`
- **Watch out**: 30 removed sources from search. None were producing valid text results anyway.

### [2026-07-03] bookUrlPattern stored but never validated
- **Problem**: `src.BookURLPattern` was persisted but never used. Search results with over-matching `bookList` selectors (ad/promo links) were accepted as valid books.
- **Fix**: Pre-compile pattern regex before parsing loop; skip results where `bookUrl` doesn't match.
- **Affected**: `backend/internal/book/search.go`
- **Watch out**: Invalid regex patterns are silently ignored (not all sources have valid patterns).

### [2026-07-03] Divergent defaults between NewFromJSON and ImportSources
- **Problem**: `NewFromJSON` set `CreatedAt`/`UpdatedAt`/`LastUpdateTime` defaults that were already handled by `UnmarshalJSON` and the store's `Upsert`/`ImportBatch`. Redundant addition+removal for single-source imports.
- **Fix**: Removed redundant defaults from `NewFromJSON`. All path go through `UnmarshalJSON` for field defaults, then `Upsert`/`ImportBatch` for timestamp initialization.
- **Affected**: `backend/internal/booksource/entity.go`

### [2026-07-03] Intentionally unmapped legado fields undocumented
- **Problem**: `customButton` (30%), `eventListener` (30%), `enabledReview` (1%), `phonehttp` (0.1%), `userid` (0%) were silently dropped on import. No documentation explaining why.
- **Fix**: Added ponytail comment on `BookSource` struct listing intentionally-omitted fields and why.
- **Affected**: `backend/internal/booksource/entity.go`
- **Watch out**: These fields are LOST on re-export. Acceptable — they're legado-reader UI features, not fetch logic.

### [2026-07-03] concurrentRate not enforced — documented
- **Problem**: `src.ConcurrentRate` exists in DB but no code enforces per-source rate limiting.
- **Fix**: Added ponytail comment on `searchCandidates` describing the ceiling and upgrade path.
- **Affected**: `backend/internal/book/search.go`

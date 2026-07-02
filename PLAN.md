# NovelReader — Plan

## Objective

A web-first, mobile-friendly novel reading application with a Go backend and Svelte 5 frontend. At its core is a legado-compatible **booksource system** that can import, manage, search, and crawl books from arbitrary websites using the legado BookSource JSON format. Designed for Docker deployment, single-user first but architecturally ready for multi-user.

## Architecture

### Directory Layout

```
novelreader/
├── backend/
│   ├── cmd/server/            # Entry point
│   ├── internal/
│   │   ├── api/               # HTTP handlers, routes, middleware
│   │   ├── booksource/        # BookSource entity, CRUD, import/export
│   │   ├── fetcher/           # HTTP client with cookie/header management
│   │   ├── analyzer/          # Rule parsing engine (CSS, XPath, JSONPath, Regex, JS)
│   │   ├── book/              # Book, chapter, content management
│   │   ├── processor/         # Content processing (replace rules, C2C, paragraph)
│   │   └── config/            # Server configuration
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── lib/               # Reusable components (reader, settings, etc.)
│   │   ├── routes/            # Page components
│   │   ├── stores/            # Svelte stores for state
│   │   └── api/               # API client module
│   ├── static/
│   ├── package.json
│   └── vite.config.js
├── docker-compose.yml
├── .gitignore
├── README.md
└── PLAN.md
```

### Key Modules

| Module | Responsibility |
|--------|---------------|
| `backend/internal/analyzer` | Port of legado's AnalyzeRule + AnalyzeUrl. Parses rule strings, dispatches to CSS/XPath/JSONPath/Regex/JS engines. |
| `backend/internal/booksource` | BookSource entity struct, SQLite persistence, JSON import/export, validation. |
| `backend/internal/fetcher` | HTTP client with cookie jar, header map, login support, concurrent rate limiting. |
| `backend/internal/book` | Book/chapter persistence, search orchestration, TOC fetching, content caching. |
| `backend/internal/processor` | ContentProcessor: replace rules, Chinese conversion, paragraph reflow, duplicate title removal. |
| `backend/internal/api` | REST handlers using `net/http` with a lightweight router. |
| `frontend/` | Svelte 5 SPA. Vite for build. Tailwind CSS for styling. |


### Data Flow — Search

```
User types keyword → API /api/search?q=...
  → backend selects enabled book sources
  → For each source:
      → analyzer.BuildURL(searchUrl, key, page) → constructs fetch URL with JS interpolation
      → fetcher.Get(url, headers) → raw HTTP response (HTML/JSON)
      → analyzer.ParseSearchResult(body, ruleSearch) → extracts book list
  → Aggregate deduped results → return JSON
```

### Data Flow — Chapter Content

```
User taps chapter → API GET /api/books/:id/chapters/:idx/content
  → book.GetChapter(bookId, idx) → chapter URL
  → fetcher.Get(chapterUrl, headers) → raw HTML
  → analyzer.ParseContent(html, ruleContent) → extracted text
  → processor.Process(text, book settings) → cleaned paragraphs
  → return JSON { title, paragraphs: [...] }
```

### Data Flow — BookSource Import

```
User uploads JSON (array of sources or single) → API POST /api/sources
  → Validate JSON structure
  → For each source:
      → Upsert by bookSourceUrl
      → Store rule fields as JSON strings in SQLite
  → Return imported count
```

### BookSource JSON Format (Legado Compatible)

```json
{
  "bookSourceUrl": "https://example.com",
  "bookSourceName": "Example",
  "bookSourceGroup": "Group",
  "bookSourceType": 0,
  "enabled": true,
  "enabledExplore": true,
  "searchUrl": "https://example.com/search?q={{key}}",
  "ruleSearch": {
    "bookList": ".result-list .book-item",
    "name": "h3 a@text",
    "author": ".author@text",
    "coverUrl": "img@src",
    "intro": ".desc@text",
    "kind": ".tag@text",
    "lastChapter": ".latest@text",
    "bookUrl": "h3 a@href"
  },
  "ruleBookInfo": { "name": "…", "author": "…", "coverUrl": "…" },
  "ruleToc": { "chapterList": "…", "chapterName": "…", "chapterUrl": "…" },
  "ruleContent": { "content": "…" },
  "ruleExplore": { "bookList": "…" },
  "header": null,
  "jsLib": null,
  "loginUrl": null
}
```

### Frontend Routes

| Route | Component | Purpose |
|-------|-----------|---------|
| `/` | SourceList | Manage book sources (import, enable/disable, delete) |
| `/search` | SearchPage | Search across sources |
| `/book/:id` | BookDetail | Book info, chapter list |
| `/read/:bookId/:chapterIdx` | Reader | Chapter reader with typography controls |
| `/settings` | SettingsPage | Typography, fonts, display preferences |

## Phases

### Phase 1 — Booksource Engine Core (current)
- [x] Project scaffolding: Go backend module, Svelte 5 frontend, Go 1.22 stdlib router
- [x] Database schema: book_sources, books, chapters, fonts tables with SQLite
- [x] BookSource CRUD: import JSON (array/single), list, delete (query-param), enable/disable
- [x] HTTP fetcher with cookie jar, timeout, redirect handling, 10MB limit
- [x] Analyzer engine: CSS (goquery), XPath (htmlquery), JSONPath, Regex, JS (goja)
- [x] Rule parser: `||` chaining, `<js>`/`@js:` blocks, `##` suffix replacement, auto-mode detection
- [x] Search: concurrent across sources, dedup, flat-map rule parsing
- [x] Book info: parse via ruleBookInfo (not wired to API yet)
- [x] TOC: fetch and parse chapter list via ruleToc
- [x] Content: fetch and parse chapter content via ruleContent
- [x] Content processor: duplicate title, re-segment, replace rules, paragraph formatting
- [x] Minimal reader UI: font size, font weight, line height, font family, bg/text color
- [x] Font upload (multipart) and selection in reader
- [ ] Wire book info fetch into add-flow (currently adds without fetching full info)
- [~] Docker build setup (dev.sh created, multi-stage Dockerfile pending)
- [~] E2E test: tested with 230 real sources; search/concurrency/context working; some sources behind Cloudflare

### Phase 2 — Reading Experience
- [ ] Reading progress sync
- [ ] Bookmarks
- [ ] Page turn animations
- [ ] Dark mode themes
- [ ] CSS custom properties for theming
- [ ] Offline caching of read chapters
- [ ] Explore/discover page

### Phase 3 — Polish & Extras
- [ ] Replace rules editor in UI
- [ ] Book source debug tool
- [ ] Multi-user support (user table, auth, scoped data)
- [ ] OPDS/Calibre integration
- [ ] Progress sync across devices
- [ ] PWA support

## Out of Scope (Phase 1)
- Audio books
- RSS sources
- TTS (text-to-speech)
- WebSocket debug tools
- EPub export
- Multi-user auth

## Current State

- **Phase**: Phase 1 — scaffolding complete, core engine built
- **Last completed**: Go backend + Svelte 5 frontend scaffolded, both compile clean
- **Completed items**:
  - Go module with `modernc.org/sqlite`, goquery, htmlquery, jsonpath, goja dependencies
  - Database schema: book_sources, books, chapters, fonts tables
  - BookSource entity with JSON import (array/single), CRUD via query-param API
  - HTTP fetcher with cookie jar, 30s timeout, 10MB limit, redirect handling
  - Analyzer engine: CSS (goquery), XPath (htmlquery), JSONPath (PaesslerAG), Regex, JS (goja)
  - Rule parser handles: `||` chain, `<js>`/`@js:` blocks, `##` suffix replacement, auto-mode detection
  - Search: concurrent across sources, dedup by bookURL, structured rule parsing
  - Content processor: duplicate title removal, re-segment, paragraph formatting, replace rules
  - REST API: sources, search, books, chapters, content, fonts, progress
  - Svelte 5 SPA with hash routing: SourceList, SearchPage, BookDetail, Reader, Settings
  - Reader: adjustable font size/weight/line-height/family, background/text color with presets
  - Font upload (multipart) and selection in reader
  - Frontend builds to 58KB JS + 8KB CSS
- **Default port**: 8888 (configurable via PORT env)

## Open Questions (resolved)

- Use Chi router or stdlib `net/http` → Go 1.22 `http.ServeMux` with path params — no external router needed.
- Go SQLite driver → `modernc.org/sqlite` (pure Go, no CGO).

## Decisions made

- Source CRUD uses query params for URL PKs (bookSourceUrl contains slashes, can't go in path segments)
- Chinese conversion deferred until a proper library (opencc) is integrated
- CacheManager wired but unused — will be used for chapter/content caching in Phase 2

## Issues & Fixes

### [2026-07-02] Source delete/update routes broken by URL-in-path issue
- **Problem**: Source URLs contain slashes (`https://...`), making them impossible to pass as Go 1.22 `ServeMux` path segments. DELETE and PUT routes were silently no-ops.
- **Fix**: Changed to query-param-based: `DELETE /api/sources?url=...` and `PUT /api/sources?url=...`. Same for book delete.
- **Affected**: `backend/internal/api/server.go` (routes + handlers), `frontend/src/api/client.ts`
- **Watch out**: All source CRUD in frontend must use query params, not path segments.

### [2026-07-02] `##pattern##replacement` suffix not stripped from CSS/XPath/JSON rules
- **Problem**: Legado sources append `##广告##` etc. to field rules. The old code never extracted this, passing the full string including `##` junk to parsers.
- **Fix**: Added `extractReplaceSuffix` in `ruleparser.go` — strips `##regex##replacement###` from any rule expression before mode detection and populates `Rule.ReplaceRegex`/`Replacement`/`ReplaceFirst`.
- **Affected**: `backend/internal/analyzer/ruleparser.go` (new file), `analyzer.go` (removed old parser)
- **Watch out**: The `applyRuleString`/`applyRuleStringList` methods in `analyzer.go` already had the post-extraction replace branch — it's now fed correctly.

### [2026-07-02] `@js:` rules split on `||` broke JS logical-OR
- **Problem**: Old parser split `@js:...` on `||` thinking it's a chain separator. JS uses `||` as logical OR, common in legado rules.
- **Fix**: `ruleparser.go` handles `<js>...</js>` and `@js:...` as terminal segments — no `||` splitting inside JS blocks.
- **Affected**: `backend/internal/analyzer/ruleparser.go`
- **Watch out**: JS chain via `||` must be done with multiple `<js>` blocks.

### [2026-07-02] Partial Chinese conversion map removed
- **Problem**: 50-character s2t/t2s map produced mixed-script garbled text. Worse than no conversion.
- **Fix**: Removed maps and made `convertChinese` a no-op. Awaiting opencc integration.
- **Affected**: `backend/internal/processor/processor.go`
- **Watch out**: Add opencc or `golang.org/x/text` for proper conversion in Phase 2.

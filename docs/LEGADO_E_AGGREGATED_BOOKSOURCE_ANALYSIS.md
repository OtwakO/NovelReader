# Legado-E Aggregated BookSource Compatibility Analysis

## Scope

This document analyzes `test-booksources/test_光遇聚合_aggregated_booksource.json` against:

- the successor reference implementation in `reference/legado-E/`;
- NovelReader's current BookSource import, JavaScript, request-execution, workflow-session, book-context, Search, Explore, Book Info, TOC, and Content modules.

This is an architecture and compatibility assessment, not an implementation plan or a promise of full support for every feature embedded in the fixture.

The fixture identifies itself as `🔅光遇聚合(26.8.7)`. It is one BookSource definition containing approximately:

- 155 KiB of `jsLib` code after JSON decoding;
- 7 KiB of `loginUi`;
- 8 KiB of `loginUrl` logic;
- a large JavaScript-generated Explore catalog;
- Search, Book Info, TOC, and Content rules that proxy several upstream providers through a configurable gateway.

## Conclusion

**NovelReader can support the core aggregated text-source architecture cleanly, but it does not support this fixture today.**

The correct reusable abstraction is not a special `光遇` adapter and not an “aggregate source” domain type. Legado-E treats this definition as an ordinary BookSource built from existing generic primitives:

1. source JavaScript creates an in-memory `data:` URL;
2. a non-empty URL-option `type` requests byte-mode handling;
3. the data payload is Base64-decoded and presented to rules as a hexadecimal response body;
4. rule JavaScript calls the gateway and returns ordinary JSON;
5. subsequent stages create new synthetic `data:` URLs carrying opaque workflow state.

The labels `gysearch`, `gydetail`, `gycatalog`, and `gycontent` are opaque metadata. Legado-E does not dispatch custom code by these names.

That design fits NovelReader's existing shared `sourceexec` seam well. Supporting typed in-memory data requests once would benefit every compatible source using the same Legado mechanism and would keep source-specific logic inside imported rules.

However, complete support for this exact definition also reaches several already-known structural areas:

- durable per-reader/per-source variables;
- per-book variable mutation and persistence;
- source-defined settings/login UI;
- login information and credentials;
- browser handoff and interactive verification;
- paragraph comments and generated interactive images;
- dynamic audio/image/video modes;
- external bookshelf synchronization.

Those must remain separate capability slices rather than being bundled into “aggregated source support.”

## What this BookSource actually is

The source is a programmable gateway client rather than a simple crawler that combines several HTML sites directly.

### Search

`searchUrl` builds a payload containing:

- query key;
- selected media tab;
- selected upstream source group;
- page number;
- whether disabled upstream definitions should be forced.

It returns:

```text
data:;base64,<encoded payload>,{"type":"gysearch"}
```

`ruleSearch.bookList` then:

1. decodes the hexadecimal result back into the JSON payload;
2. calls the configured gateway's `/search` endpoint;
3. passes the returned JSON into `$.data`.

Each result's `bookUrl` is another typed data URL containing the gateway book ID, selected upstream provider, media tab, and an optional direct upstream URL.

### Book Info

The synthetic book URL carries a `gydetail` payload. `ruleBookInfo.init` decodes it and requests `/detail`. The normal metadata fields then read the returned JSON.

The generated `tocUrl` is a `gycatalog` data URL containing canonical gateway identity and metadata for the selected upstream binding.

### TOC

`ruleToc.chapterList` decodes the synthetic TOC payload, calls `/catalog`, and returns the gateway's chapter array. It stores `book_id` through `java.put` so per-chapter URL generation can retrieve it later through `java.get`.

Each chapter becomes a `gycontent` data URL carrying:

- gateway book ID;
- item/chapter ID;
- title;
- selected upstream provider;
- media tab;
- optional direct upstream URL.

### Content

`ruleContent.content` decodes the synthetic chapter payload, requests `/content`, applies optional comment/image/video behavior, and finally returns `$.content`.

In the default **server** network mode, the gateway performs upstream aggregation and extraction. An optional **local** mode instead asks the Legado host to crawl an upstream URL and submit the HTML to the gateway.

## Legado-E execution semantics

### Typed data URLs

Legado-E's `AnalyzeUrl` recognizes data URIs and decodes their Base64 bytes. When the URL option contains a non-null `type`, `getStrResponseAwait()` skips HTTP and returns the data bytes hex-encoded as the response body.

Primary evidence:

- `reference/legado-E/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt:417-426`
- `reference/legado-E/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt:634-655`

The source's rule scripts then call `java.hexDecodeToString(result)` to recover the original JSON.

### URL options

Legado-E parses a trailing URL-option object and supports fields including:

- `method`;
- `headers`;
- `body`;
- `type`;
- `charset`;
- `retry`;
- `webView`;
- `webJs`;
- `bodyJs`;
- `dnsIp`;
- `js`;
- `webViewDelayTime`.

For this fixture's core text workflow, the important subset is:

- `type` for typed in-memory responses;
- `js` for conditional URL rewriting;
- `method`, `headers`, and `body` for gateway requests made by `java.ajax`.

### Shared `jsLib` scope

Functions such as `getVariable`, `setVariable`, `BaseUrl`, `request`, `getToken`, and `checkEnv` are not special host methods. They are declared by the source's `jsLib` and execute with dynamic bindings for `java`, `source`, `book`, `chapter`, `result`, `cookie`, and `cache`.

Primary evidence:

- `reference/legado-E/app/src/main/java/io/legado/app/model/SharedJsScope.kt`
- `reference/legado-E/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt:364-388`
- `reference/legado-E/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeRule.kt:828-859`

### Mutable state namespaces

The fixture uses several distinct kinds of state which should not be collapsed into one generic map.

#### Source variable

`source.getVariable()` and `source.setVariable(string)` store one source-owned string. This source interprets it as a JSON settings object containing gateway hosts, selected line, timeouts, enabled features, and source choices.

Legado-E stores it under the source identity. Missing state returns an empty string.

Primary evidence:

- `reference/legado-E/app/src/main/java/io/legado/app/data/entities/BaseSource.kt:242-269`

#### Per-book and per-chapter rule variables

`java.put` and `java.get` are context-sensitive. During TOC parsing, `java.put('book_id', ...)` attaches the value to the book. During chapter URL generation, `java.get('book_id')` can fall through from chapter to book.

The fixture also reads `book.getVariable('custom')`.

Primary evidence:

- `reference/legado-E/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeRule.kt:794-823`
- `reference/legado-E/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt:390-412`
- `reference/legado-E/app/src/main/java/io/legado/app/data/entities/BaseBook.kt:8-40`
- `reference/legado-E/app/src/main/java/io/legado/app/data/entities/BookChapter.kt:92-118`

#### Login information

`source.getLoginInfo`, `getLoginInfoMap`, and `putLoginInfo` use a separate credential/login namespace. This is not the same as the source settings variable or the request cookie jar.

Primary evidence:

- `reference/legado-E/app/src/main/java/io/legado/app/data/entities/BaseSource.kt:169-231`
- `reference/legado-E/app/src/main/java/io/legado/app/ui/login/SourceLoginDialog.kt`

## Current NovelReader compatibility

### Import and persistence: compatible

NovelReader imports the definition successfully:

- `jsLib`, `loginUi`, `loginUrl`, and all rule objects are retained;
- unknown fields such as `customButton` and `eventListener` remain in preserved source JSON;
- raw import/export remains lossless.

Relevant modules:

- `backend/internal/booksource/entity.go`
- `backend/internal/booksource/json.go`

A focused probe successfully imported the source and built its Search URL as a `data:` URL with `type="gysearch"`.

### URL construction: mostly compatible

NovelReader already supports:

- `<js>...</js>` and `@js:` URL construction;
- trailing URL-option parsing;
- URL-option `type` storage;
- URL-option `js` evaluation;
- POST body/header options;
- `jsLib` loading;
- source/session bindings.

Relevant module:

- `backend/internal/analyzer/urlbuilder.go`

### Request execution: missing the decisive primitive

NovelReader carries the option's `Type` into `sourceexec.RequestSpec`, but `RoutingTransport` sends every non-WebView request to `HTTPTransport`. `HTTPTransport` assumes an HTTP/HTTPS URL and has no typed data-response path.

Relevant modules:

- `backend/internal/sourceexec/request.go`
- `backend/internal/sourceexec/router.go`
- `backend/internal/sourceexec/http_transport.go`

A focused probe showed that the fixture's Search URL builds correctly as a typed `data:` request, but the current execution seam has no behavior that decodes the payload and returns the Legado-compatible hexadecimal body.

**This is the first root cause and the cleanest implementation seam.**

### JavaScript bridge: useful subset, incomplete for this source

Already present:

- `java.ajax`;
- Base64 encode/decode;
- `java.androidId`;
- `java.put`;
- `java.log` and no-op toast behavior;
- source variable methods;
- cookie object methods;
- cache memory methods;
- source-level and workflow cookies/headers;
- request cancellation and bounded execution.

Missing or mismatched methods used by the fixture include:

- `java.hexDecodeToString` — required for every core stage;
- documented `java.getCookie` — the cookie object exists, but the alias used by parts of the fixture is missing;
- exact contextual `java.get`/`java.put` persistence semantics;
- `book.getVariable` as a callable object method;
- `book.setUseReplaceRule`;
- `source.getLoginInfo`, `getLoginInfoMap`, and `putLoginInfo`;
- `java.getWebViewUA`;
- `java.startBrowserAwait` and `java.startBrowser`;
- platform/fork probes such as `deviceID`, `getAppVariant`, `qread`, and `reLoginView`.

The first four items affect the core workflow. Login, browser, and platform methods can be staged separately.

Relevant module:

- `backend/internal/analyzer/js.go`

### Source state: workflow continuity exists, durable settings do not

NovelReader's `SourceSession` cleanly isolates cookies, variables, headers, login headers, last URL, and transient memory. Workflow/book sessions preserve state through many Search → Detail → TOC → Content paths.

However, current source variables are bounded in-memory session state. They can expire, be evicted, or disappear after restart. They do not currently represent a durable reader-owned source settings record.

Relevant modules:

- `backend/internal/sourceexec/session.go`
- `backend/internal/sourceexec/session_registry.go`

This fixture uses the source variable for functional configuration, including gateway hosts and line switching. A durable source-variable module is therefore needed for reliable support, although safe defaults may permit a narrow first runtime test.

### Book and chapter context: fields exist, method behavior is incomplete

NovelReader supplies rich `book` and `chapter` context maps and preserves a serialized `Book.VariableMap`. It also has contextual analysis hooks.

However, JavaScript currently receives plain maps rather than Legado-compatible objects with methods such as `book.getVariable`. Current `java.put` writes to process-level JavaScript cache data rather than the active book/chapter variable map, and `java.get` is not exported as its contextual counterpart.

Relevant modules:

- `backend/internal/book/context.go`
- `backend/internal/analyzer/js.go`

This is the second important core seam after typed data requests.

### Search, Book Info, TOC, and Content orchestration: structurally compatible

NovelReader already routes all four stages through a common executor and preserves book-session continuity. That is a strong fit for the source's chain of synthetic URLs.

Relevant modules:

- `backend/internal/book/search.go`
- `backend/internal/book/chapterlist.go`
- `backend/internal/book/context.go`
- `backend/internal/sourceexec/executor.go`

No separate aggregate workflow should be added above these modules.

## Core support matrix

The following matrix defines a practical first compatibility target: text novels, server network mode, anonymous gateway use, no source-defined login UI, no direct local upstream crawling, no comments, and no media modes.

| Capability | Core requirement | Current status | Clean seam |
|---|---:|---|---|
| Import large `jsLib` and rules | Yes | Supported | `booksource` import |
| Evaluate URL JavaScript with `jsLib` | Yes | Supported/verify limits | `analyzer.JSVM` / URL builder |
| Preserve URL option `type` | Yes | Supported as metadata | URL builder / `RequestSpec` |
| Execute typed `data:` response | Yes | Missing | `sourceexec` in-memory transport/policy |
| Return data bytes as lowercase/compatible hex body | Yes | Missing | same shared transport seam |
| `java.hexDecodeToString` | Yes | Missing | Java bridge |
| `java.ajax` with JSON POST options | Yes | Supported | Java bridge/fetcher |
| `source.getVariable/setVariable` | Yes | In-memory only | durable reader source settings + live session hydration |
| `java.get/put` contextual book variables | Yes | Incomplete/mismatched | Analyzer/JS context object |
| `book.getVariable('custom')` | Yes | Missing method surface | book JS object |
| Cookies returning empty state | Yes | Supported through `cookie.*` | SourceSession |
| `java.getCookie` alias | Conditional core | Missing | Java bridge alias |
| Harmless toast/log calls | Yes | Supported enough | Java bridge |
| Search pagination | Yes | Supported | Search URL builder |
| Synthetic Detail/TOC/Content URLs | Yes | Structurally compatible | existing workflows |
| Dynamic source settings UI | No for first slice | Missing | later source-settings design |
| Login credentials/UI | No for first slice | Missing | later encrypted login design |
| Direct local upstream crawling | No for first slice | Partial HTTP; browser methods missing | later capability slice |
| Paragraph comments/reviews | No | Unsupported | separate product/domain design |
| Audio/image/video tabs | No | Unsupported/partial by type | separate media domains |
| External gateway bookshelf sync | No | Should remain disabled | do not couple to NovelReader shelf |

## Recommended architecture

### 1. Add a generic typed-data request path inside `sourceexec`

Do not special-case `gysearch` or inspect the type label.

The module interface should remain the existing request/response contract:

```go
Transport.Do(context.Context, RequestSpec) (Response, error)
```

The implementation should recognize the exact Legado condition:

- URL is a valid supported `data:` URI;
- `RequestSpec.Type` is non-empty.

It should:

1. parse and Base64-decode the data payload under strict size limits;
2. hex-encode the resulting bytes in the same format expected by `java.hexDecodeToString`;
3. return a successful in-memory response with a distinct transport diagnostic such as `data`;
4. perform no network request;
5. preserve cancellation and clear malformed/oversized errors.

This can be an internal adapter routed before HTTP/WebView, or an explicit execution policy within `RoutingTransport`. The external interface should not grow unless a second actual caller requires it.

Security limits are required because URL scripts are untrusted and can synthesize arbitrary payload sizes. The fixture itself is small; a conservative bounded payload is sufficient.

### 2. Add focused bridge methods through existing JS interfaces

Implement only exact methods demonstrated by real sources, beginning with:

- `java.hexDecodeToString(string)`;
- `java.getCookie(url)` and the documented key overload if upstream semantics require it;
- contextual `java.get(key)` and `java.put(key, value)`;
- a Legado-compatible `book.getVariable(key)` method.

Each method family should have public bridge tests using the real fixture's call shape.

Do not add unrestricted JVM or Android class emulation.

### 3. Correct contextual variable ownership

The runtime needs a small variable-owner interface behind the JavaScript context, not special aggregate state.

The desired precedence is:

- current chapter variables;
- current book variables;
- transient rule data;
- source key/value fallback where applicable.

Writes must mutate the active owner and synchronize durable Book/Chapter state where the workflow owns a persisted entity.

This should deepen the existing Analyzer/JS context module rather than adding maps in Search, TOC, and Content callers.

### 4. Add durable reader-owned source settings separately

Source settings are Reader Data, not credentials. They should be stored per immutable reader and BookSource identity in the reader database, with a small string-value interface matching `source.getVariable/setVariable`.

Live `SourceSession` instances can be hydrated from durable state and write through when the source changes the value.

Do not store source credentials in the same table. Login tokens, credentials, and reversible secrets remain subject to the encrypted credential-store design.

### 5. Expose source settings deliberately, not by blindly rendering `loginUi`

The fixture places ordinary settings and powerful actions inside `loginUi`. Executing it unchanged would combine:

- harmless selectors and toggles;
- arbitrary source JavaScript actions;
- cloud configuration updates;
- browser navigation;
- login and logout;
- cookie manipulation;
- source self-update behavior.

A later source-settings capability should classify supported control types and actions. The first core runtime slice can use source defaults and an administrator-facing raw source-variable editor only if explicitly approved; it should not pretend full Login UI compatibility.

### 6. Keep optional capabilities separate

Do not bundle these into the initial aggregate-source slice:

- local direct crawling;
- interactive `startBrowserAwait` verification;
- source-defined authentication;
- paragraph comments and click handlers;
- generated image options for comments;
- video/browser playback;
- audio/image/video BookSource modes;
- synchronization to the gateway's own bookshelf.

This keeps the first change reversible and prevents one complex fixture from redefining NovelReader's domain model.

## Proposed implementation phases

### Phase A — deterministic compatibility fixture

Create a minimized, local, deterministic aggregate-style fixture that proves:

1. Search URL JavaScript returns typed `data:` content;
2. rules hex-decode it and call a local fake gateway;
3. Search result creates a synthetic Detail URL;
4. Book Info creates a synthetic TOC URL;
5. TOC stores a contextual book variable and creates synthetic chapter URLs;
6. Content retrieves the variable and produces readable prose.

Use the real fixture as design evidence, but do not make live external gateway availability the primary regression gate.

### Phase B — typed data transport and hex bridge

Implement the generic `sourceexec` data-response behavior and `java.hexDecodeToString`. Verify malformed, non-Base64, oversized, empty-type, timeout/cancellation, and ordinary HTTP non-regression cases.

### Phase C — contextual variable methods

Implement exact `java.get/put` and `book.getVariable` behavior through a variable-owner interface. Verify book-to-chapter fallback and persistence across TOC → Content.

### Phase D — core server-mode end-to-end test

Run the unmodified fixture in text/server mode if its external gateway is available. Classify external failure separately from runtime incompatibility.

### Phase E — durable source settings

Design and implement per-reader source-variable storage, hydrate live sessions, and expose a safe settings surface. This is a standard data-model feature and should update `PLAN.md` before implementation.

### Phase F — optional capability slices

Discuss and approve login UI, browser verification, comments, and media workflows separately. Each should have its own threat model, interface, and acceptance criteria.

## Risks and constraints

### Untrusted JavaScript and remote configuration

The source downloads cloud configuration and contains self-update/settings actions. NovelReader must continue enforcing:

- execution timeouts and cancellation;
- bounded JS runtime pools;
- bounded response and data payload sizes;
- transport allowlists/policies already applied to source requests;
- no filesystem, process, or arbitrary host access;
- no automatic source definition replacement without explicit user confirmation.

### SSRF and gateway selection

The source settings can choose gateway hosts. This is normal BookSource behavior but still untrusted network input. Existing request safety and deployment network policy remain relevant; aggregate support should not introduce a privileged bypass.

### State isolation

Source settings, cookies, variables, and credentials must remain scoped by reader identity and source identity. A process-global source variable would leak gateway selections or tokens between reader accounts.

### External synchronization

The fixture can synchronize progress to the gateway's own bookshelf. NovelReader should not enable that by default or conflate it with canonical shelf persistence. It is an external side effect requiring explicit user opt-in and credential handling.

### Media polymorphism

Although the imported definition declares `bookSourceType: 0`, its scripts dynamically assign audio, image, and video types. NovelReader should support the text subset first and report unsupported selected modes explicitly rather than attempting to render them as prose.

## Decision

**Recommended:** Support the generic typed-data and contextual-variable semantics through existing deep modules, beginning with a deterministic text/server-mode fixture.

**Do not:**

- add a `GuangYuSource` implementation;
- add `if type == "gysearch"` or other label dispatch;
- model an aggregate source as several imported BookSources;
- move gateway logic into the frontend;
- make source-defined login/settings JavaScript a prerequisite for the core request chain;
- enable external bookshelf synchronization as part of ordinary NovelReader progress saving;
- claim full fixture support until optional browser/login/comment/media branches are classified.

Under this approach, the architecture remains clean: frontend features continue consuming stable domain APIs, while BookSource URL construction, JavaScript, aggregation, fetching, parsing, state, and compatibility remain backend-owned.

## Verification performed for this analysis

- Parsed and inventoried the unmodified fixture.
- Verified NovelReader imports it and preserves its large `jsLib`, login UI, and rule fields.
- Verified NovelReader builds the Search URL as a typed `data:` request with `type="gysearch"`.
- Traced the current request router and confirmed it has no typed data-response execution path.
- Traced JS bridge and context ownership against every core stage.
- Compared behavior with Legado-E primary source code.
- Ran focused existing Go tests:

```text
go test ./internal/booksource ./internal/analyzer ./internal/sourceexec ./internal/book
```

All focused packages passed. No production source code was changed during this analysis.

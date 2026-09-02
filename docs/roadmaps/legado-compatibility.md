# Legado Compatibility Roadmap

**Status:** Current future direction, evidence-driven

## Compatibility standard

NovelReader targets practical compatibility with documented and observed Legado BookSource behavior for web-hosted text reading. Work fixes shared analyzer, JavaScript, request, session, source-interaction, or workflow seams before considering source-specific behavior.

Compatibility does not mean solving captchas, bypassing WAF/site policy, emulating every Android/JVM API, or fabricating success when upstream services fail.

The detailed 2026-08 audit queue and completed slices are preserved in [`docs/archive/audits/legado-compatibility-tracker-2026-08.md`](../archive/audits/legado-compatibility-tracker-2026-08.md). Use current tests/code and fresh audit evidence rather than unchecked historical statuses.

## Current baseline

Operational shared capabilities include:

- lossless BookSource import/export with unknown-field preservation;
- Default/JSoup, CSS, XPath, JSONPath, Regex, templates, replacements, indices, and mutable rule context;
- source JavaScript libraries, Java bridge helpers, mutable source/book/chapter state, cookies, and request options;
- shared HTTP, fingerprint, typed-data, and WebView transports;
- Search, Explore, Book Info, TOC pagination/formatting, content pagination/replacement, images, and source sessions;
- reader-owned source profiles, encrypted login state, normalized source actions, controlled browser sessions, and bounded `startBrowserAwait` replay;
- programmable aggregate BookSources through generic typed data and shared state rather than aggregate-specific adapters;
- deterministic conformance fixtures plus operation-specific live-audit workflows.

## Priority families

Select the next slice from reproducible evidence and practical impact.

### 1. Rule and typed-value semantics

Close remaining differences in Default/JSoup/CSS/XPath/JSONPath/Regex behavior, connector ordering, null/empty handling, nested typed values, and mutable entity exposure. Prefer one small rule family or observed failure at a time.

### 2. JavaScript bridge and runtime isolation

Complete helpers used by real text BookSources, with bounded execution, recursion/time limits, explicit side effects, and no runtime/session leakage. Avoid broad JVM emulation without a demonstrated caller.

### 3. Request, cookie, and login parity

Continue shared request-option, redirect, charset, cookie, source-variable, login-check, and browser-handoff behavior where normal source workflows require it. Source state remains reader/source owned and must clean up when the Source ID disappears.

### 4. Mutable workflow context

Expand source/book/chapter field behavior only when represented by a current domain contract or justified by a reusable compatibility seam. Do not expose arbitrary frontend mutation of source execution state.

### 5. Diagnostics and regression closure

Improve redacted request/rule evidence only for demonstrated debugging needs. Preserve explicit classifications for DNS, upstream HTTP, WAF, timeout, WebView, stale rules, parser failures, storage, and unsupported behavior.

## Design-required capabilities

These need focused product/security design before implementation:

- source-defined custom interaction controls beyond the normalized current model;
- captchas or verification flows that cannot fit the controlled browser seam;
- media/file-source download and import workflows;
- audio/TTS, comic/image-sequence, video, and other non-text models;
- external bookshelf synchronization;
- Android-only UI extensions or filesystem/process APIs.

## Explicit non-goals

- source-specific adapters for one fixture when shared Legado behavior explains it;
- automatic captcha/WAF bypass;
- continuous broad source-health monitoring;
- unrestricted source JavaScript filesystem/process/network access;
- claiming a live-source failure is an engine defect without transport/upstream evidence.

## Slice workflow

1. Identify current upstream/reference behavior.
2. Reproduce through the nearest production interface with deterministic evidence.
3. Fix one shared seam with the smallest complete change.
4. Run focused tests; broaden only for real coupling risk.
5. Record durable conclusions in the relevant plan, roadmap, research note, or immutable audit evidence—not a chronological root log.

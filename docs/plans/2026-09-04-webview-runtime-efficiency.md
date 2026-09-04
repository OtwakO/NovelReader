# WebView Runtime Efficiency

**Status:** Active

**Branch:** `feat/webview-runtime-efficiency`

## Goal

Improve successful WebView source execution per unit of deployment and runtime cost while preserving NovelReader's reader isolation and bounded browser security seam. The user's normal browser score of 7/100 on fingerprint-scan.com is a reference target, not a promise that a Linux container on another network can reproduce the same score.

## Scope

- Reduce WebView image and runtime waste that does not contribute to successful execution.
- Measure Patchright configurations under the same host, network, concurrency, and scanner workload.
- Preserve one shared browser process with a fresh non-persistent `BrowserContext` per request/session.
- Preserve bounded capacity, destination-scoped cookies, opaque-data request mediation, redirect/body/timeout limits, and connected-address validation.
- Consider another browser engine only after a same-workload proof demonstrates a better success/resource tradeoff and its distribution terms are acceptable.

Out of scope:

- scanner-specific JavaScript patches, forged User-Agent/client hints, CAPTCHA solving, source-specific bypasses, or automatic WAF evasion;
- cross-reader persistent browser profiles;
- bundling multiple production browser engines in one image;
- claiming universal stealth from one detector.

## Accepted Approach

1. Establish repeatable baselines for image size, idle/one/two-context memory, latency, fingerprint signals, and protected-page outcomes.
2. Remove proven packaging waste without changing browser behavior.
3. Evaluate the smallest coherent Patchright runtime modes. Reject modes that do not materially improve successful execution or that require inconsistent fingerprint injection.
4. Keep browser-engine choice inside the worker implementation; the backend-owned protocol remains unchanged.
5. Evaluate CloakBrowser only as a separate pinned proof of concept if Patchright cannot reach an acceptable success/resource frontier. Adoption requires licensing clarity and all security-critical worker regressions to pass against the real binary.

## Decisions

- Optimize **successful useful work per resource**, not RAM, image size, or one bot score independently.
- Context isolation is non-negotiable: no shared cookies, local storage, or profile state across reader requests.
- Keep the current direct `BrowserWorker` interface until a second production engine is accepted. A browser adapter abstraction now would be speculative.
- Do not hide `HeadlessChrome` by independently overriding headers or JavaScript properties. A coherent runtime change is preferable to fingerprint lies that detectors can cross-check.
- Patchright's full-Chromium new-headless path is not current work: it crashes before navigation in the tested Linux environment, and successful Google Chrome headless runs still scored 100/100.

## Current State

Measured on 2026-09-04:

- Current Patchright bundled headless mode: fingerprint-scan.com bot risk **100/100**.
- Patchright with Google Chrome headless, including a persistent profile: **100/100**.
- Patchright with Google Chrome headful under Xvfb: **51.5/100** on the same host/IP.
- Baseline release worker container: about **613 MB** content-addressed image size, **153 MiB idle**, and **414 MiB observed peak** for two concurrent scanner requests.
- The optimized compact image uses a multi-stage build, installs only the headless shell, and excludes build-only uv. Its content-addressed size is **343 MB** (44% smaller), with `/ms-playwright` reduced from ~656 MB to ~267 MB unpacked.
- Optimized compact runtime behavior is unchanged: **153 MiB idle**, two concurrent scanner requests complete, and fingerprint-scan remains **100/100**. Short Docker-stats samples observed 414–451 MiB peak, so no RAM change is claimed from a disk-only packaging correction.
- Patchright maps `chromium` and `chromium-headless-shell` dependency installation to the same Linux package set; the Dockerfile uses the precise artifact name but claims no package-size saving from it.
- CloakBrowser research is recorded at [CloakBrowser fit for NovelReader's WebView worker](../research/cloakbrowser-webview-fit.md). It is not yet proven better for NovelReader and has distribution/licensing constraints.

## Next Action

Evaluate a bounded headful Patchright image/runtime as a separate build/runtime candidate, preserving the same shared-browser/per-context isolation model. Measure score, image size, idle/two-context RAM, and latency before deciding whether it is an acceptable production mode.

Do not add an automatic per-request fallback: retrying a blocked request through a second engine doubles work and complicates state. If the headful frontier remains unacceptable, prepare the separate pinned CloakBrowser benchmark described in the research note.

## Verification

For packaging changes:

- worker unit and Chromium regression tests: **32 passed**;
- Python bytecode/import checks: **passed**;
- built-image import and health checks: **passed**;
- exact Compose WebView E2E path with the current atomic app/worker pair: **passed**;
- content-addressed and unpacked image measurements: **613 MB → 343 MB**, browser payload **656 MB → 267 MB**;
- idle and two-concurrent-context memory measurements: idle unchanged at ~153 MiB; short-sample peaks 414–451 MiB.

For runtime/engine changes:

- all packaging gates above;
- repeated fingerprint-scan runs under the same host/IP;
- representative protected public pages and optional ignored private BookSource compatibility checks;
- route mediation, destination-scoped cookie, redirect, body-size, private-network, and connected-address regressions;
- no credential or private BookSource material in code, logs, fixtures, or captured evidence.

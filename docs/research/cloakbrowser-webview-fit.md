# CloakBrowser fit for NovelReader's WebView worker

**Conclusion:** do **not** replace Patchright outright yet. CloakBrowser is API-adjacent and may improve headless fingerprinting, but the advantage is not independently established for NovelReader's workload, while it introduces a closed proprietary browser binary, licensing/session-service dependencies, a Patchright-only API incompatibility, and no demonstrated image/RSS saving. The sensible next step is a pinned, licensed proof-of-concept benchmark—not a migration commitment.

## Fit summary

| Area | Finding | Confidence |
|---|---|---|
| Linux amd64 sidecar | Official Linux x86-64 binary and Docker image exist. The current official `latest` image manifest is linux/amd64. | Proven |
| Async migration | `launch_async()` returns the standard `playwright.async_api` `Browser`; one process can create multiple `browser.new_context()` contexts. Most worker calls therefore remain Playwright calls. | Proven |
| Required APIs | Standard Playwright supplies `goto`, routing and `route.fetch/fulfill/abort`, URL-filtered cookies, `response.server_addr`, screenshots, evaluation, response bodies/events, and non-persistent isolated contexts. CloakBrowser claims all Playwright methods work because it launches its binary through ordinary Playwright. | Proven at wrapper/API level; binary compatibility still needs an integration test |
| Known incompatibility | NovelReader uses Patchright's `page.evaluate(..., isolated_context=False)`. Upstream Playwright's `page.evaluate()` has only `expression` and `arg`; CloakBrowser imports upstream `playwright.async_api`, so this keyword must be removed. Its exact semantic effect must be regression-tested for BookSource scripts. | Proven API difference; behavioral impact unknown |
| Stealth | CloakHQ claims source-level Chromium patches and strong headless results. Its own README nevertheless recommends `headless=False` for some protected sites. Discussion #154 contains a user benchmark where plain Patchright beat CloakBrowser for KillBot, while the maintainer says Patchright added no measurable benefit across 50+ systems. NovelReader's same-host trial of the freely downloadable v146 binary scored 100/100 across three headless and three headful official-image launches; current v151 remains untested. | v146 rejected; v151 unknown for NovelReader |
| Footprint | CloakHQ says the binary archive is about 200 MB. The official Dockerfile also installs Node, Xvfb, Openbox, fonts, both Python and JS wrappers, and examples. The official `latest` linux/amd64 image is **625,271,759 bytes** content-addressed (~596 MiB), versus NovelReader's optimized Patchright image at **342,704,799 bytes**. Short v146 official-image samples observed ~347 MiB peak headless and ~494 MiB headful for three sequential scans; these are not equivalent to NovelReader's two-concurrent-request workload. | Image size proven; limited RSS sample |
| Context isolation | Standard non-persistent Playwright contexts do not write browsing data to disk and provide independent sessions. This matches NovelReader's no-cross-reader-storage model if it continues creating/closing one context per request. Cloak's fingerprint seed is generated per **browser launch**, however, so concurrent contexts in the shared process likely share the same device fingerprint; per-context fingerprint isolation is not documented. | Storage isolation proven; fingerprint scope inferred/unknown |
| Security/trust | The wrapper is MIT, but CloakHQ's Chromium patches and distributed binary are closed and proprietary. Maintainers state there has not yet been an independent C++ security audit. Signed manifests, checksums, attestations, and VirusTotal establish origin/integrity or scanner results, not source auditability. | Proven |
| Releases/pinning | Wrapper and binary versions are separate. Exact binary pinning is supported with `browser_version`/`CLOAKBROWSER_VERSION`; signed manifests and rollback instructions are published. The Python dependency is only `playwright>=1.40`, so NovelReader should pin both wrapper, Playwright, and binary explicitly rather than accept floating compatibility. | Proven |
| Maintenance/maturity | The repository has extensive tests and frequent browser releases, but it is a young 2026 project and the core patch set is maintained solely by its vendor. Current/newest builds and concurrency are license-controlled; the older v146 line remains publicly downloadable. | Proven facts; long-term continuity unknown |

## Migration effort and gaps

A minimal conversion would replace Patchright startup with `from cloakbrowser import launch_async` and retain the existing shared `Browser` plus per-request `BrowserContext` design. The max-two-context semaphore remains NovelReader code; CloakBrowser does not need to manage it. Interactive base64 HTML is decoded and loaded with `page.set_content()` while a route mediator handles its HTTP(S) fetch/XHR requests, so a real-binary trial must preserve that opaque-page compatibility seam rather than merely prove ordinary navigation.

Expected changes are small but not literally a one-line import swap:

1. Replace `async_playwright().start()` / `playwright.chromium.launch()` lifecycle with `launch_async()` and adjust shutdown ownership; CloakBrowser patches `browser.close()` to stop its internally-created Playwright instance.
2. Remove Patchright's `isolated_context=False` argument and verify that BookSource `webJs` still executes in the intended page/main world.
3. Change the container build to install and prefetch an exact CloakBrowser binary. Disable automatic update checks and pin the binary so deployments are reproducible.
4. Decide whether to keep headless mode. Headful would retain Xvfb and probably preserve or increase resource use; headless is the only configuration likely to simplify the image.
5. Run NovelReader's route/DNS-rebinding tests against the real binary, especially `route.fetch(max_redirects=0)` plus `APIResponse.server_addr()`. The API exists upstream, but CloakBrowser has no repository test specifically demonstrating this security-critical combination.

No required standard Playwright API was found to be absent. The only confirmed source incompatibility is `isolated_context`. The important unknown is not method presence but whether the custom binary preserves NovelReader's routing, response-address, cookie, and script-world behavior exactly.

## Isolation, licensing, and operational risk

Non-persistent contexts are the correct CloakBrowser mode; persistent profiles would conflict with NovelReader's storage requirement. A crash or exploit in the shared browser process can still affect both concurrent contexts, just as today; CloakBrowser does not provide a stronger process/container isolation boundary.

The binary license permits internal Docker use, but requires a separate OEM/SaaS license when the binary is embedded/exposed through an API or used for customer-controlled browser workflows. Because NovelReader exposes short-lived interactive WebView sessions and users can influence targets/scripts through BookSources, applicability is **not safely inferable** from the text. CloakHQ should confirm in writing whether NovelReader distribution/hosting requires OEM/SaaS terms before adoption. The current license also allows pricing, feature, and usage-limit changes with notice, and license validation/concurrent-session monitoring are explicit network communications. A free current-build key is limited to one concurrent session, which does not meet NovelReader's maximum of two; an appropriate paid tier would be required for the intended benchmark.

## Benchmark relevance

NovelReader's existing fingerprint-scan results (Patchright bundled Chromium: 100 headless; Chrome+Xvfb: 51.5–56.5 headful under the same IP) are more directly relevant than CloakHQ's cross-site marketing table because they measure the actual worker environment and target scanner. The freely downloadable Cloak v146 official image scored 100/100 in three headless and three headful launches under the same network, so that older build is not a viable replacement. They also show that “headful” is not automatically stealthier. Discussion #154 reinforces that browser choice can reverse by detector: the maintainer reports no Patchright benefit across its suite, while a user reports Patchright winning strongly on KillBot.

A fair trial must therefore use the same host/IP, container limits, URLs, request mix, and repeated runs for:

- current Patchright headless;
- pinned CloakBrowser headless with default settings;
- optionally pinned CloakBrowser headed+Xvfb only if a real source fails headless.

Record fingerprint-scan score and variance, real BookSource success rate, CAPTCHA/block rate, cold start, navigation latency, peak and steady RSS for zero/one/two contexts, compressed image size, unpacked image size, browser cache size, and DNS-rebinding/routing regression results. CloakHQ's screenshots and live-site tables are vendor evidence, not substitutes for this workload-specific benchmark.

## Recommendation

Keep the optimized Patchright image as the production default. Do not continue with Cloak v146. Consider current CloakBrowser v151 only behind an experimental image/configuration after:

1. written license confirmation for NovelReader's interactive/API use and a two-session entitlement;
2. exact wrapper + Playwright + binary pinning with auto-update disabled;
3. passing the full WebView worker tests against the real Cloak binary, including `server_addr` validation;
4. a repeated same-IP benchmark showing a material real-source improvement without unacceptable RSS/image growth.

Absent that evidence, CloakBrowser is a **stealth experiment**, not a clearly better architectural fit. It offers plausible headless anti-detection gains, but no proven footprint benefit and a materially worse trust/licensing profile than the current open Patchright/Chromium stack.

## Primary sources

- CloakBrowser repository README and API/platform/Docker/fingerprint documentation: <https://github.com/CloakHQ/CloakBrowser> (retrieved 2026-09-04)
- Python wrapper implementation (`launch_async` uses `playwright.async_api`, custom executable, and returns a Playwright `Browser`): <https://github.com/CloakHQ/CloakBrowser/blob/main/cloakbrowser/browser.py>
- Python package manifest (`playwright>=1.40`, wrapper MIT metadata): <https://github.com/CloakHQ/CloakBrowser/blob/main/pyproject.toml>
- Official Dockerfile: <https://github.com/CloakHQ/CloakBrowser/blob/main/Dockerfile>
- Wrapper MIT license: <https://github.com/CloakHQ/CloakBrowser/blob/main/LICENSE>
- Proprietary binary license, including internal/OEM/SaaS, subscription, restrictions, and license communications: <https://github.com/CloakHQ/CloakBrowser/blob/main/BINARY-LICENSE.md>
- Releases, platform artifacts, signed checksums, binary pinning and rollback: <https://github.com/CloakHQ/CloakBrowser/releases>
- Patchright comparison and contrary user benchmark: <https://github.com/CloakHQ/CloakBrowser/discussions/154>
- Maintainer rationale for dropping Patchright support: <https://github.com/CloakHQ/CloakBrowser/issues/121>
- Closed-patch trust/security discussion and no completed third-party audit: <https://github.com/CloakHQ/CloakBrowser/issues/105>
- Playwright BrowserContext isolation and cookie/routing APIs: <https://playwright.dev/python/docs/api/class-browsercontext>
- Playwright route fetch/fulfill/abort API: <https://playwright.dev/python/docs/api/class-route>
- Playwright response body and `server_addr` API: <https://playwright.dev/python/docs/api/class-response>
- Playwright page navigation/evaluate/screenshot/response events: <https://playwright.dev/python/docs/api/class-page>
- Official Docker Hub image/registry: <https://hub.docker.com/r/cloakhq/cloakbrowser> (linux/amd64 `latest` manifest inspected for compressed layer total)

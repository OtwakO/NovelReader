# Camoufox fit for NovelReader's WebView worker

**Date:** 2026-09-04

## Question

Can [Camoufox](https://github.com/daijro/camoufox) provide a materially lower automation score and better protected-page access than NovelReader's optimized Patchright worker without weakening the worker's reader isolation, bounded networking, or deployment efficiency?

## Conclusion

Camoufox is meaningfully stealthier on the tested public targets, but it is **not currently a safe drop-in production replacement**.

- Two completed native-headless fingerprint-scan runs scored **22/100** and **7/100**, versus Patchright headless at **100/100** on the same host/network.
- Camoufox reached Planet Minecraft's normal login page with HTTP 200 in native-headless and virtual-display modes; the current Patchright worker received Cloudflare HTTP 403 and no login form.
- Planet Minecraft's embedded Cloudflare Turnstile did **not** complete under Camoufox: its response token remained empty and the login submit stayed disabled after 30 seconds. Removing Camoufox's default uBlock Origin addon did not change this result.
- Two simultaneous Camoufox contexts both reached the normal login page, so the worker's normal concurrency shape is possible.
- A security-critical API is missing: the response returned by Firefox `route.fetch()` is an `APIResponse` without `server_addr()`. NovelReader uses the actual connected address after `route.fetch()` to reject DNS rebinding. Camoufox cannot replace Patchright until an equally strong, testable connected-address validation seam exists.
- The installed browser directory measured **1,284,403,360 bytes** and the isolated Python environment **241,755,339 bytes**. A short host process-tree sample measured about **414 MiB after launch** and **856 MiB with one active Planet Minecraft page**. These measurements are not a container-image or cgroup-equivalent benchmark, but they do not support an efficiency win.

Keep Patchright as production default. Camoufox remains a promising stealth reference, not an accepted engine.

## Tested versions and environment

- Python wrapper: `camoufox 0.5.5`
- Browser selected by `official/stable`: `152.0.4-beta.29`
- Playwright: `1.60.0`
- BrowserForge: `1.2.4`
- Host/network: same development host and residential Taiwan connection used for the Patchright and CloakBrowser comparisons
- Configuration: Camoufox-generated default fingerprints; no manual User-Agent, platform, WebGL, scanner-specific, or challenge-specific overrides
- Modes: native `headless=True` and Camoufox-recommended `headless="virtual"`

Camoufox's browser source is MPL-2.0. Its Python wrapper is distributed separately through PyPI. Unlike the tested CloakBrowser release path, no vendor session-license gate was required.

## Fingerprint-scan result

| Mode | Completed scores | Other outcomes | Interpretation |
| --- | ---: | --- | --- |
| Camoufox native headless | **22**, **7** | one browser closed during the combined run; later scanner collections stopped completing | Material improvement when collection completed, but stability/repeatability is not yet sufficient |
| Camoufox virtual display | none | scanner remained uncompleted; the visible initial `0` was not a result | Inconclusive; do not report as 0/100 |
| Optimized Patchright headless | **100** | repeated completion | Stable but readily classified as automated |

The completed Camoufox runs exposed coherent generated desktop signals such as `navigator.webdriver=false`, five plugins, platform-matched Firefox 152 user agents, and generated Windows WebGL identities. The two valid scores are evidence of lower detection on this scanner, not a universal stealth guarantee.

Subsequent Camoufox scanner runs loaded the page but left `window.FINGERPRINT_SCAN.status` unset. Because the collection never completed, those runs are excluded rather than treated as zero or failed scores. The public scanner may throttle repeated tests or its collection script may fail for a generated identity; the evidence does not distinguish those causes.

## Planet Minecraft Cloudflare test

Target: <https://www.planetminecraft.com/account/sign_in/>

This is a manual live benchmark, not a default CI test. The third-party challenge, IP reputation, and policy can change independently of NovelReader.

### Pass criteria

The test separates two different Cloudflare boundaries:

1. **Outer page gate passed:** HTTP 200 normal `Login` page, visible email/password controls, and no persistent `Attention Required`, `Just a moment`, or `verify you are human` takeover.
2. **Embedded Turnstile completed:** after filling synthetic non-submitted values, the hidden `cf-turnstile-response` receives a non-empty token and the login submit is enabled.

No real credentials were used and no login form was submitted.

### Result

| Engine/mode | Outer page gate | Embedded Turnstile | Evidence |
| --- | --- | --- | --- |
| Camoufox native headless | **Passed** | **Not completed** | HTTP 200, title `Login`, visible email/password; token length remained 0 and submit stayed disabled for 30 seconds |
| Camoufox virtual display | **Passed** | **Not completed** | Same normal page and empty-token outcome |
| Camoufox virtual without default uBlock Origin | **Passed** | **Not completed** | Excluding the addon did not change token or button state |
| Optimized Patchright headless | **Blocked** | Not reached | HTTP 403, title `Attention Required! | Cloudflare`, `Sorry, you have been blocked`, no email control |

Two Camoufox contexts launched concurrently and both reached the normal login page with HTTP 200 in about seven seconds. This proves a useful improvement over Patchright for the outer Cloudflare decision on this host/IP. It does **not** prove that Camoufox can complete Planet Minecraft login, because the site's Turnstile remained unresolved.

## NovelReader API compatibility

### Compatible surfaces observed

- fresh Playwright `BrowserContext` creation;
- exact-URL cookie retrieval through `context.cookies([url])`;
- page routing and `route.fetch()` / `route.fulfill()` / `route.abort()` methods;
- top-level navigation `Response.server_addr()`;
- at least two simultaneous isolated contexts.

### Blocking incompatibility

NovelReader's `mediate_data_document_request` performs this order:

1. validate all DNS answers for the destination host;
2. execute `route.fetch(max_redirects=0)`;
3. reject redirects;
4. inspect the **actual connected server address** through `response.server_addr()`;
5. reject non-public addresses before reading and fulfilling the response.

With Camoufox's Firefox Playwright transport, `route.fetch()` returns `APIResponse`, and that object has no `server_addr()` method. Top-level navigation responses exposing `server_addr()` do not solve this routed-fetch boundary. Removing the post-connect check would reopen the DNS-rebinding window and is not acceptable merely to adopt another engine.

No adapter or alternate transport should be introduced until it can prove equivalent connected-address, redirect, timeout, body-size, and cookie-scope behavior with focused regressions.

## Footprint and stability

Measured isolated installation:

- Camoufox browser directory: **1,284,403,360 bytes** (~1.20 GiB)
- Python environment: **241,755,339 bytes** (~231 MiB)
- Browser directory fonts: about **932 MiB** by `du`
- Patchright optimized production image: **342,704,799 bytes** content-addressed

These are not identical measures: Camoufox was not packaged into a purpose-built production image. A fair image comparison would need a minimal multi-stage Camoufox Dockerfile and cgroup measurement. The current unpacked result nevertheless shows that adopting the default distribution would not automatically reduce deployment storage.

Observed instability:

- one combined native-headless trial closed its browser/page unexpectedly;
- later fingerprint collections stopped completing despite successful navigation;
- Planet Minecraft navigation itself remained repeatable, including two concurrent contexts.

## Decision and reconsideration gate

Do not migrate the production worker to Camoufox now. Reconsider only if all of the following become true:

1. a bounded implementation can prove the actual connected destination address after a routed request without weakening the current DNS-rebinding protection;
2. worker routing, destination-scoped cookies, redirect rejection, timeout, response-size, and private-network regressions pass against the real Camoufox runtime;
3. a minimal container is measured under the same idle/one/two-context workload;
4. repeated scanner and representative protected-page runs remain stable;
5. the relevant source workload benefits from reaching the normal page even when an embedded challenge may still require user interaction.

Do not add automatic engine fallback or CAPTCHA solving. A blocked request retried through a second full browser would double work and complicate reader/session state.

## Primary sources

- Camoufox repository and maintenance warning: <https://github.com/daijro/camoufox>
- Python usage and launch modes: <https://camoufox.com/python/usage/>
- Installation/version management: <https://camoufox.com/python/installation/>
- Virtual-display recommendation: <https://camoufox.com/python/virtual-display/>
- Browser source license: <https://github.com/daijro/camoufox/blob/main/LICENSE>
- Python package metadata: <https://pypi.org/project/camoufox/>

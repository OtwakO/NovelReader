---
status: completed
updated: 2026-09-26
---

# Native ARM64 container releases

## Goal

Publish app and WebView images for `linux/amd64` and `linux/arm64` under the existing shared tags. Docker selects the host architecture. Both architectures retain exact-image verification before publication.

## Scope

Docker/Bake, release CI, architecture-neutral Compose verification and deployment documentation. No application/data migrations, browser-engine substitutions, automatic browser fallback or production deployment.

## Accepted Approach

- Keep Patchright and branded stable Chrome on both architectures. Install Google's official architecture-specific Debian package directly instead of Patchright's Chrome installer, which currently rejects Linux ARM64.
- Build app/worker concurrently on each native GitHub runner (`ubuntu-24.04` and `ubuntu-24.04-arm`). Keep native OpenCC/WebP checks, both browser-mode tests and Compose E2E on each architecture.
- Stage only verified image digests with architecture-qualified tags. Collect the two architecture results, construct multi-platform indexes, then promote immutable SHA references and existing aliases only after both pairs pass.
- Preserve publication serialization, worker-first/app-last alias order and fresh build-time worker dependency resolution. Do not rebuild after testing.

## Decisions and compatibility

Patchright remains common to both platforms; replacing it with Playwright only on ARM would introduce unnecessary behavior differences. Google now supplies an official ARM64 Chrome package: the downloaded package reports `google-chrome-stable`, `154.0.8037.57-1`, `arm64`. This establishes package availability, not runtime compatibility.

Shared tags become multi-platform indexes rather than single-platform manifests. Existing AMD64 consumers still select AMD64 automatically. No forced platform is needed in Compose. Failed architecture builds must leave public aliases unchanged.

Rollback: revert the build/workflow changes and use previously verified per-image digests for deployment; do not overwrite existing SHA tags or rebuild old sources expecting identical fresh browser dependencies. No stored reader data is changed. Registry promotion of two repositories is not atomic, as in the existing workflow.

## Current State

Implemented on `feat/arm64-containers`: the worker installs Google's native Chrome package while retaining Patchright's dependency/font installation; Bake and Compose follow the native architecture; CI builds/tests each architecture and passes verified digests to `build/publish-images.sh` for index validation and promotion. Five synthetic registry tests exercise the publication gates. Manual dispatch defaults to verification-only and stages `ci-*` references without moving release aliases or immutable SHA tags.

Local AMD64 images and runtime checks pass. The app reused existing build layers; the changed worker was rebuilt. Local Docker is AMD64-only, so ARM verification ran on GitHub's native ARM runner. Hosted run 36176447907 passed every prerequisite, both native architecture jobs and real-registry index verification. Existing unrelated untracked files are left untouched. The branch is pushed and verified, but not merged or released to public aliases.

## Next Action

Await authorization to merge `feat/arm64-containers` into `main` and publish. Implementation commit `2cb78ee` passed verification-only run [36176447907](https://github.com/OtwakO/NovelReader/actions/runs/36176447907) with `publish=false`; no implementation work remains scheduled. A merge/push to `main` triggers a new fully gated production release, not reuse of this run's mutable staging tags.

## Verification

Verified before implementation:
- Patchright 1.63.0 includes ARM64 wheels; its Chrome shell installer explicitly rejects `aarch64`.
- Google's official ARM64 `.deb` downloaded successfully and `dpkg-deb -f` confirmed native architecture.
- Patchright's dry-run lists an ARM64 managed browser, but this alternative is not selected.

Verified locally after implementation:
- Actionlint 1.7.12, Bake expansion (including explicit ARM64 targets/cache scope), Compose configuration and shell syntax pass.
- `python3 -m unittest discover -s build -p 'test_*.py' -q`: 5 publication tests pass (requires jq); registry create/inspect failures, missing architecture, wrong manifest platform, immutable conflicts and verification-only mode do not promote public aliases.
- AMD64 app/worker Bake build succeeded. After retaining the original browser dependencies/fonts, worker rebuild and both exact-image browser-mode suites passed: 46 passed / 1 optional live test skipped per mode.
- Exact-image `E2E_SKIP_BUILD=1 bash docker-e2e.sh` passed frontend, readiness, private WebView, synthetic rendered search, graceful stop and persistence checks.
- Local Docker commands needed an isolated anonymous `DOCKER_CONFIG` and writable temporary `BUILDX_CONFIG` because the sandbox cannot use Docker Desktop's credential helper. No host configuration changed.

Verified on GitHub in run 36176447907 (commit `2cb78ee`):
- Backend, frontend and locked WebView prerequisite jobs all passed.
- Native `ubuntu-24.04` / AMD64 and `ubuntu-24.04-arm` / ARM64 jobs built the exact release pair, checked image architecture, passed both browser-mode suites and passed Compose E2E before staging.
- Real GHCR indexes contained exactly the verified AMD64 and ARM64 child digests. Publication-gate regressions passed again on the publication runner.
- The final step explicitly reported `Verification only; public tags are unchanged.` No release aliases or immutable SHA tags were created by this run.
- Staged index references: app `ghcr.io/otwako/novelreader@sha256:08ebdcb28fe2c0b1f7df90d6730945220b58a5696d957a0597564fcf2d4d5c56`; worker `ghcr.io/otwako/novelreader-webview@sha256:e46ea24a25901439b6f7bfddb290f1501095d4ff8dd4d89dc2977e4b5e218ba0`.

Limits: no live-source compatibility audit, production deployment, public-tag promotion or controlled cross-architecture size/build-time comparison was performed. Staging references are verification evidence, not a promised retention policy.

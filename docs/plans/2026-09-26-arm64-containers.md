---
status: active
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

Local AMD64 images and runtime checks pass. The app reused existing build layers; the changed worker was rebuilt. Local Docker is AMD64-only with no ARM emulation advertised, so ARM runtime validation remains a hosted gate. Existing unrelated untracked files are left untouched. The user authorized pushing this feature branch and running verification-only hosted CI, not merging or releasing it.

## Next Action

Push the implementation commit and dispatch `publish.yml` on `feat/arm64-containers` with `publish=false`. Inspect both native architecture jobs and the staged manifest indexes; record the result here before recommending a merge. No `main` merge or public-tag promotion is authorized.

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

Pending:
- Hosted native ARM64 image builds and runtime gates, plus hosted AMD64 verification.
- Real registry index platform/digest verification. No ARM64 image size or build-time measurement is claimed.

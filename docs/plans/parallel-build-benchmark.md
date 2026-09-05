# Parallel image-build benchmark

**Status:** Active

## Goal

Measure whether concurrent app/worker builds on one GitHub runner beat sequential builds enough
to justify changing the release workflow. This is an experiment, not an accepted production redesign.

## Accepted approach

- Branch-only, non-publishing workflow on `perf/parallel-build-benchmark`.
- One sequential and one parallel case, each on a fresh `ubuntu-latest` runner of the same class.
- Docker Bake uses the existing production Dockerfiles. Sequential invokes the targets separately;
  parallel invokes both in one build graph. Explicit `source: .` uses the checkout rather than Bake's
  default Git snapshot. Image export/loading is included in the measured steps.
- Import the production app GHA cache without exporting updates. Both cases introduce the same
  harmless backend COPY input marker to model a source edit rather than a fully cached app.
- Worker builds remain uncached and resolve fresh Patchright/Chrome. Record versions and runner
  capacity; version differences would make the comparison less controlled.
- Preserve native Go tests within the app build, both worker-mode checks, and exact-image Compose E2E.
- No GHCR login, image upload, alias changes, production cache export, or reader-data access.

## Current state

Benchmark workflow and Bake targets passed local validation and were pushed at `a5f366c`.
Initial run [33957263031](https://github.com/OtwakO/NovelReader/actions/runs/33957263031) passed,
but the default Git context ignored the injected cache marker: app was fully cached (8s).
Build totals of 95s sequential / 89s parallel are a warm-app observation only, not the intended
source-change comparison. Both runners had 4 CPUs/~16 GB RAM and Patchright 1.62.3 / Chrome 152.0.7977.82.
The benchmark now explicitly selects the checkout directory; a corrected run is pending.
Production `.github/workflows/publish.yml` and Dockerfiles are unchanged.

## Next action

Push the context correction and inspect the resulting run. Verify that the native app build actually
executes rather than hitting the complete cache. Compare
build-step totals and total job time, inspect cache hits and concurrent overlap, and report evidence
before proposing a production change. Keep baseline and candidate results separate from prior runs.

## Verification and interpretation

`actionlint` 1.7.12 passed. `docker buildx bake --print` parsed both targets; assertions confirmed
Docker-only output, no cache exports, and an uncached worker. Hosted verification remains pending.

One pair is directional evidence, not a stable performance guarantee: separate runners,
shared-runner resource contention, network throughput, and cache availability can affect results.
A faster build that fails either integration check is not a viable result. Do not remove release
verification or change browser freshness to improve a benchmark score.

## Rollback and cleanup

No deployment rollback is needed: this workflow only triggers on the experiment branch and builds
local runner images. Do not merge benchmark scaffolding into the production workflow without a
separate decision. Retire the branch-only workflow after recording the outcome; Git preserves it.

# Parallel image-build benchmark

**Status:** Active

## Goal

Measure whether concurrent app/worker builds on one GitHub runner beat sequential builds enough
to justify changing the release workflow. This is an experiment, not an accepted production redesign.

## Accepted approach

- Branch-only, non-publishing workflow on `perf/parallel-build-benchmark`.
- One sequential and one parallel case, each on a fresh `ubuntu-latest` runner of the same class.
- Docker Bake uses the existing production Dockerfiles. Sequential invokes the targets separately;
  parallel invokes both in one build graph. Image export/loading is included in the measured steps.
- Import the production app GHA cache without exporting updates. Both cases introduce the same
  harmless backend COPY input marker to model a source edit rather than a fully cached app.
- Worker builds remain uncached and resolve fresh Patchright/Chrome. Record versions and runner
  capacity; version differences would make the comparison less controlled.
- Preserve native Go tests within the app build, both worker-mode checks, and exact-image Compose E2E.
- No GHCR login, image upload, alias changes, production cache export, or reader-data access.

## Current state

Benchmark workflow and Bake targets passed local validation; branch push and hosted run pending.
Production `.github/workflows/publish.yml` and Dockerfiles are unchanged.

## Next action

Validate the benchmark, push the approved branch, and inspect the resulting Actions run. Compare
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

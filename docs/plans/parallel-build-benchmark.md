# Parallel image-build benchmark

**Status:** Completed

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
The benchmark now explicitly selects the checkout directory. Corrected run
[33957509840](https://github.com/OtwakO/NovelReader/actions/runs/33957509840) at `584f289` passed both cases.
Production workflow changes have not been made.
Production `.github/workflows/publish.yml` and Dockerfiles are unchanged.

## Next action

Seek approval before adapting the release workflow to one concurrent Bake invocation. Preserve its
revision labels, cache export, exact-image verification, and publication gates. Do not merge the
branch-only benchmark scaffolding as a production implementation; measure the real release afterward.

## Result

| Measurement | Sequential | Parallel |
| --- | ---: | ---: |
| App build | 70s | overlaps worker |
| Worker build | 109s | overlaps app |
| Combined build steps (including Docker export/load) | 179s | 117s |
| Native Go test/build layer | 47.6s | 70.4s |
| Full benchmark job | 250s | 181s |
| Worker-mode tests | 10s | 11s |
| Compose E2E | 34s | 38s |

Both corrected cases used 4 CPUs/~16 GB RAM, Patchright 1.62.3, and Chrome 152.0.7977.82.
Native Go compilation executed in both cases, confirming the intended source-change scenario.
Both integration gates passed. Builds saved 62s (~35%); the total job saved 69s, including runner
setup variation. Parallel contention slowed the Go layer, but overlap still improved total time.

Recommendation: a production parallel-build trial is justified. This does not promise an identical
release speedup: the benchmark omits cache export and registry staging, and retains one sample per
case. The warm-app first run showed only a 6s build improvement, so benefits depend on changed inputs.

## Verification and interpretation

`actionlint` 1.7.12 passed. `docker buildx bake --print` parsed both targets; assertions confirmed
Docker-only output, no cache exports, and an uncached worker. Corrected hosted verification passed
native tests, both worker modes, and Compose E2E for both scheduling modes.

One pair is directional evidence, not a stable performance guarantee: separate runners,
shared-runner resource contention, network throughput, and cache availability can affect results.
A faster build that fails either integration check is not a viable result. Do not remove release
verification or change browser freshness to improve a benchmark score.

## Rollback and cleanup

No deployment rollback is needed: this workflow only triggers on the experiment branch and builds
local runner images. Do not merge benchmark scaffolding into the production workflow without a
separate decision. Retire the branch-only workflow after recording the outcome; Git preserves it.

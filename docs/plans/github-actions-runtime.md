# GitHub Actions runtime

**Status:** Completed

## Goal and scope

Reduce avoidable workflow work without removing tests, changing image freshness policy, or
restructuring jobs. Accepted scope: exclude Markdown-only changes within watched source trees,
disable npm install audit/funding requests, and stage the two verified images concurrently.
The accepted build investigation covers matching native Go test/build `-trimpath` flags and checking
whether worker `.venv` permission setup duplicates image data. Implement only demonstrated improvements.

## Accepted approach

- Keep all existing positive path filters, followed by Markdown exclusions for backend, frontend,
  and webview-worker. Mixed documentation/code pushes still run.
- Use `npm ci --no-audit --no-fund` in frontend CI. Tests and production build stay unchanged.
- Run both image staging operations concurrently with separate temporary digest files. Wait for
  both processes; any failure prevents publishing digest outputs and reaching alias promotion.
- Preserve exact-image Compose verification before staging, immutable SHA references, and sequential
  worker-first/app-last alias promotion. No stale-image reuse, job matrix, new action, or caching layer.

- Keep the Go native tests and release build in the same existing stage, with matching compilation
  flags so shared dependencies can reuse the within-build cache.
- Leave the worker Dockerfile unchanged: local image history showed its permission/user-setup layer
  is only 77.8 kB versus a 147 MB `.venv` copy. The duplicate-environment-layer hypothesis was falsified;
  moving permissions would add churn without a meaningful measured benefit. No new stages or dependencies.

## Evidence and limits

Recent successful runs 33664845113, 33799849902, and 33854867655 took 254, 410, and 637 seconds.
Combined staging took 68–72 seconds. The slowest run spent five minutes in `npm ci`; logs did not
establish which network request caused the stall. Flags remove optional requests but are not a
proven fix for that outlier. Parallel uploads share runner bandwidth, so savings are unmeasured.

Latest baseline run 33952229227: app build 114s (native Go tests/build layer 82.5s), worker build
111s (Chrome installation 44.7s; image export/load 56.2s). Build improvements are hypotheses until
measured; no claim that these changes eliminate those complete durations.

Local before/after builds used the same native tests and release build with separate initially empty
Go compilation caches. The native layer fell from 36.2s to 24.8s; release-build compiler invocations
following tests fell from 386 to 82 (`go build -x` diagnostic output). Test time was comparable.
This demonstrates dependency reuse and a 31% local layer improvement, not a hosted workflow speedup.
No generated logs, private input, or benchmarking instrumentation is part of the production change.

## Hosted result

Run [33953863150](https://github.com/OtwakO/NovelReader/actions/runs/33953863150) at `d6f5894`
passed all verification and publication steps. Compared with baseline run 33952229227:

| Measurement | Before | After |
| --- | ---: | ---: |
| Creation to final job completion | 462s | 395s |
| App image build | 114s | 66s |
| Native Go test/build layer | 82.5s | 43.7s |
| Worker image build | 111s | 96s |
| Verified-image staging | 54s | 43s |
| Backend verification job | 96s | 107s |

Overall improvement was 67s (14.5%) in this single comparison. Native compilation savings agree
with the controlled local result. Worker/network/runner variation also contributes; do not attribute
every timing change to these edits or promise a fixed duration. Worker build/export and backend
verification remain substantial, but no further optimization is accepted in this workstream.

## Current state

The three workflow changes are committed and locally verified. Native test/build flags now match;
the controlled cold-compilation-cache comparison passed and demonstrated less duplicate compilation.
Worker permission changes were rejected based on layer evidence. The full production app image builds,
and its native libraries resolve. Test coverage, jobs, browser freshness settings, Compose gates,
and alias-promotion ordering are unchanged. Hosted verification and publication passed; the workstream
is complete.

## Next action

No implementation or verification remains pending. Retain these measurements as a baseline if
future runs justify another focused optimization.

## Verification

- `actionlint` 1.7.12 passed, using temporary tool caches; no repository dependency was added.
- Executed the actual staging shell with a mock Docker executable: success, app push failure, and
  worker push failure passed. A rendezvous proved both uploads start concurrently; both complete
  before the step returns, and either failure suppresses all digest outputs for promotion.
- Nine representative path-filter cases passed using the existing frontend minimatch dependency:
  documentation-only paths are excluded, while mixed/code/workflow/production-Compose changes run.
- The workflow-only slice did not rerun application tests. The build follow-up ran both native
  `internal/chineseconv` and `internal/api` package tests in the baseline and candidate image stages;
  both passed. The actual production Dockerfile also built successfully, including its native tests;
  the resulting binary resolves OpenCC and its other dynamic libraries. No reader data was mounted.
- Hosted run 33953863150 passed exact-image Compose verification, both WebView modes, concurrent
  GHCR staging, and immutable-reference/alias promotion. Timing evidence is recorded above.

## Compatibility and rollback

No image tags, document/API contracts, data mounts, or dependency versions change. Revert the
workflow edit to restore prior triggers, npm options, and sequential staging. Revert the Dockerfile
follow-up independently to restore prior Go flags. Worker permission-layer placement is unchanged.
Concurrent staging may leave one staging tag if its peer fails, just as sequential staging could; public aliases must
not move in that case. Application data is not involved.

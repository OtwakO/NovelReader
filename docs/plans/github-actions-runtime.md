# GitHub Actions runtime

**Status:** Active

## Goal and scope

Reduce avoidable workflow work without removing tests, changing image freshness policy, or
restructuring jobs. Accepted scope: exclude Markdown-only changes within watched source trees,
disable npm install audit/funding requests, and stage the two verified images concurrently.

## Accepted approach

- Keep all existing positive path filters, followed by Markdown exclusions for backend, frontend,
  and webview-worker. Mixed documentation/code pushes still run.
- Use `npm ci --no-audit --no-fund` in frontend CI. Tests and production build stay unchanged.
- Run both image staging operations concurrently with separate temporary digest files. Wait for
  both processes; any failure prevents publishing digest outputs and reaching alias promotion.
- Preserve exact-image Compose verification before staging, immutable SHA references, and sequential
  worker-first/app-last alias promotion. No stale-image reuse, job matrix, new action, or caching layer.

## Evidence and limits

Recent successful runs 33664845113, 33799849902, and 33854867655 took 254, 410, and 637 seconds.
Combined staging took 68–72 seconds. The slowest run spent five minutes in `npm ci`; logs did not
establish which network request caused the stall. Flags remove optional requests but are not a
proven fix for that outlier. Parallel uploads share runner bandwidth, so savings are unmeasured.

## Current state

All three scoped changes are implemented and locally verified. No tests, jobs, browser freshness
settings, Compose gates, or alias-promotion ordering were removed or changed.

## Next action

Await merge/push approval, then compare a subsequent hosted run against the baseline. Check both
normal publication and timing before claiming a speedup. No hosted run has exercised these edits yet.

## Verification

- `actionlint` 1.7.12 passed, using temporary tool caches; no repository dependency was added.
- Executed the actual staging shell with a mock Docker executable: success, app push failure, and
  worker push failure passed. A rendezvous proved both uploads start concurrently; both complete
  before the step returns, and either failure suppresses all digest outputs for promotion.
- Nine representative path-filter cases passed using the existing frontend minimatch dependency:
  documentation-only paths are excluded, while mixed/code/workflow/production-Compose changes run.
- No application tests were rerun for this workflow-only change. Real GHCR upload behavior and timing
  remain unverified. Do not launch or publish a benchmark release without approval.

## Compatibility and rollback

No image tags, document/API contracts, data mounts, or dependency versions change. Revert the
workflow edit to restore prior triggers, npm options, and sequential staging. Concurrent staging
may leave one staging tag if its peer fails, just as sequential staging could; public aliases must
not move in that case. Application data is not involved.

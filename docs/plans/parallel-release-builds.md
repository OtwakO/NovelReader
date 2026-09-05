# Parallel release builds

**Status:** Active

## Goal and accepted approach

Adopt same-runner concurrent app/worker builds without changing release verification or publication.
Use one pinned Docker Bake action and two explicit targets in `docker-bake.hcl`, not separate build
jobs or image-transfer artifacts. Keep the build description small; no shared target abstraction.

## Evidence

Non-publishing benchmark [33957509840](https://github.com/OtwakO/NovelReader/actions/runs/33957509840)
measured 179s sequential versus 117s concurrent builds, including image loading, on matched
4-CPU/~16-GB runners. Both worker modes and Compose passed. Native Go compilation slowed under
contention, but total builds improved. One pair is directional evidence, not a guarantee.

The first benchmark used Bake's default Git snapshot and ignored a working-directory cache marker;
production explicitly uses `source: .`, matching the prior build steps' checkout contexts.
Benchmark-only workflow, cache marker, and HCL scaffolding stay off main.

## Preserved contracts

- `linux/amd64`; local tags `novelreader:e2e` and `novelreader-webview:e2e`.
- OCI revision labels set from the actual workflow SHA for both targets.
- App GHA cache import/export, scope `novelreader`, export mode `max`.
- Worker `no-cache` for fresh build-time Patchright/Chrome; no worker cache export.
- Docker output only during build, disabled provenance/SBOM as before; no push during Bake.
- Backend/frontend/locked-worker prerequisites; both image worker modes; exact-image Compose E2E.
- Concurrent verified-image staging; immutable SHA references; worker-first/app-last alias promotion.
- Global publication concurrency lock. No data/schema/deployment changes.

## Current state

Production Bake targets and workflow wiring implemented and locally verified. No benchmark workflow,
cache marker, or benchmark target file is included. Main-branch publication verification is pending.

## Next action

Validate resolved targets against the old build-step contract, lint the workflow, review the diff,
commit and fast-forward merge to main, then push as approved. Inspect the resulting release before
claiming completion or a timing improvement.

## Verification

- `actionlint` 1.7.12 passed; Bake parsed and expanded both targets successfully.
- Resolved targets were compared with the prior build-step inputs: contexts, platforms, tags, revision
  labels, cache import/export, worker no-cache, Docker-only outputs, and disabled attestations match.
- Parsed YAML comparison confirmed every other container-job step, prerequisite job, publication lock,
  and permission remains unchanged. The new HCL file is included in release path triggers.
- Existing staging-shell success/app-failure/worker-failure checks and path-filter cases passed.
- Unchanged application suites were not rerun locally. Actual images, integration, cache export, and
  registry publication will be verified by the approved main-branch release run.

## Rollback

Restore the two build-push action steps from the prior main revision and remove the Bake target file
and its path filter. No image tags or database state need migration. Do not bypass a failing release
gate; previously published images remain the deployment fallback.

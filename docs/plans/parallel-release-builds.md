# Parallel release builds

**Status:** Completed

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

Implemented as `505cbd7`, merged and pushed to main. Production run
[33958362843](https://github.com/OtwakO/NovelReader/actions/runs/33958362843) passed all gates and publication.
No benchmark workflow, cache marker, or benchmark target file was included.

## Next action

No implementation or verification remains pending. Keep this result as a baseline; further
optimization requires measured evidence rather than removing release gates.

## Verification

- `actionlint` 1.7.12 passed; Bake parsed and expanded both targets successfully.
- Resolved targets were compared with the prior build-step inputs: contexts, platforms, tags, revision
  labels, cache import/export, worker no-cache, Docker-only outputs, and disabled attestations match.
- Parsed YAML comparison confirmed every other container-job step, prerequisite job, publication lock,
  and permission remains unchanged. The new HCL file is included in release path triggers.
- Existing staging-shell success/app-failure/worker-failure checks and path-filter cases passed.
- Unchanged application suites were not rerun locally. Hosted run 33958362843 passed backend/frontend/
  worker prerequisites, both image browser modes, exact-image Compose verification, concurrent staging,
  and immutable-reference/alias promotion. Build logs confirmed GHA cache export.

## Hosted timing

The first production run finished in 321s (5m21s): backend verification 107s; concurrent build step
94s; image worker-mode checks 10s; Compose 39s; staging 46s; promotion 5s.
The app native layer was cached, so this is not an apples-to-apples comparison with the prior 395s
run that compiled it. Do not attribute the entire 74s difference to scheduling. The separate
source-change benchmark above remains the controlled evidence for build overlap.

## Rollback

Restore the two build-push action steps from the prior main revision and remove the Bake target file
and its path filter. No image tags or database state need migration. Do not bypass a failing release
gate; previously published images remain the deployment fallback.

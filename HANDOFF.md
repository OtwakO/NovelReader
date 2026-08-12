# NovelReader Development Handoff

This is the canonical starting point for a new development session. Read this file first, then open only the linked document for the task being continued.

## Current checkpoint

- **Branch:** `main`
- **Last implementation commit:** `911d879` — `fix: support charset-aware URL encoding`
- **Remote state after this handoff commit:** local `main` is 190 commits ahead of `origin/main`; nothing has been pushed.
- **Audit branch:** `audit/search-bookinfo-v1` was fast-forwarded into `main` at `911d879`; the handoff documentation exists only on `main` unless the branch is moved later.
- **Completed milestone:** the Explore and Search → Book Info live-audit sessions are wrapped up. The latest Search → Book Info v4 audit found no recoverable shared-engine gap. Its final charset-aware `java.encodeURI(value, charset)` omission was fixed and verified without changing raw 72's legitimate-empty classification.
- **Current product-development track:** the fail-closed per-reader storage/authentication ownership cutover is implemented. Account registration, password change, administration, reset, and durable deletion are complete.

## Next recommended action

Complete the remaining account-shell verification gate before resuming compatibility work:

1. Start from a **clean disposable data root**.
2. Run one real browser workflow:
   - first-Administrator setup;
   - logout/login;
   - import real BookSources;
   - search for a real book;
   - add it to the shelf;
   - open detail, TOC, and chapter content;
   - logout;
   - prove private Reader Data is unavailable afterward.
3. Record exact Playwright evidence and any environment limitation.
4. Complete the remaining Phase 2 legacy-removal checks in
   [`docs/USER_STORAGE_IMPLEMENTATION_TASKS.md`](docs/USER_STORAGE_IMPLEMENTATION_TASKS.md).
5. Update this handoff and `PLAN.md` in the same completed change.

After that gate, resume the paused compatibility queue at focused explicit CSS/Jsoup compatibility. Do not start LC-016 durable source login until the ownership boundary is fully verified.

## Source-of-truth documents

| Need | Read |
|---|---|
| Current architecture, decisions, phase state, compatibility history | [`PLAN.md`](PLAN.md) |
| Authentication/storage execution checklist and exact next gate | [`docs/USER_STORAGE_IMPLEMENTATION_TASKS.md`](docs/USER_STORAGE_IMPLEMENTATION_TASKS.md) |
| Accepted authentication and storage contract | [`docs/AUTHENTICATION_DESIGN.md`](docs/AUTHENTICATION_DESIGN.md) |
| Domain terminology | [`CONTEXT.md`](CONTEXT.md) |
| Non-obvious implementation history and verification limitations | [`DEVELOPMENT.md`](DEVELOPMENT.md) |
| Setup, tests, conformance runner, WebView, deployment | [`README.md`](README.md) |
| BookSource audit discipline | [`.agents/skills/booksource-audit-workflow/SKILL.md`](.agents/skills/booksource-audit-workflow/SKILL.md) |

`PLAN.md` is intentionally comprehensive; do not read all of `DEVELOPMENT.md` or every historical issue before starting. Use this file to choose the relevant section.

## Verification baseline

The following passed immediately before the audit branch was fast-forwarded into `main`:

- Full backend `go test ./...`
- Full backend `go vet ./...`
- Race tests for analyzer, book, source execution, and conformance
- Search → Book Info v1–v4 audit verifiers and post-fix verifiers
- Frontend tests: **39/39**
- Isolated frontend production build
- Raw-72 exact frozen-source replay

Useful focused audit checks:

```bash
node scripts/search-bookinfo-audit/v4/verify.mjs
node scripts/search-bookinfo-audit/v4/verify-fixes.mjs
```

Normal project checks:

```bash
cd backend
GOMODCACHE=/tmp/novelreader-gomod GOCACHE=/tmp/novelreader-gocache go test ./...
GOMODCACHE=/tmp/novelreader-gomod GOCACHE=/tmp/novelreader-gocache go vet ./...

cd ../frontend
npm test
npm run build
```

On this host, the frontend production build may require the Rollup optional binary matching the installed Rollup version. It was previously installed with `--no-save --package-lock=false`; do not modify package metadata merely to satisfy the host-specific optional package.

## Working-tree warning

The repository is **not clean**, but the remaining changes predate the completed audit work and were deliberately not committed:

```text
D  frontend/dist/assets/index-C7IlCsJ9.css
D  frontend/dist/assets/index-CU1kHOB_.js
M  frontend/dist/index.html
M  frontend/package-lock.json
?? frontend/dist/assets/index-BzpYi2eO.css
?? frontend/dist/assets/index-CdTvtmU7.js
```

The `dist` changes are a generated asset replacement. `package-lock.json` contains npm-generated optional-package/peer metadata changes. Ownership and desired disposition have not been decided.

**Do not reset, stash, discard, regenerate, or commit these files as part of another task without first reviewing them with the user.** Build to an isolated `/tmp` output when possible so verification does not overwrite them.

## Development constraints to preserve

- Local development data is disposable until the project has public users or irreplaceable state; reset incompatible development roots rather than adding migrations.
- Every Reader Data route must authenticate before selecting the reader-owned home. Do not restore global Reader Data stores or compatibility fallbacks.
- BookSource compatibility fixes belong at shared executor/analyzer/session seams—never per-site branches.
- Live BookSource audits are evidence-only. Report and obtain approval before implementing fixes.
- Exact audit definitions must be imported; importing an entire compilation can replace sampled duplicate-URL identities.
- Use Playwright when browser behavior resolves a real ambiguity or verifies a changed user workflow, not as a default substitute for direct HTTP evidence.
- Default to no subagents. Use a focused subagent only when independent judgment, real parallelism, specialized capability, or context containment materially improves the task.

## Session wrap-up checklist

Before ending the next substantial session:

1. Update the checkpoint and next action in this file.
2. Update `PLAN.md` if architecture, phase, current state, or decisions changed.
3. Append to `DEVELOPMENT.md` only for non-obvious history worth rediscovering.
4. Run verification appropriate to the changed boundary and report its exact scope.
5. Commit one logical, working change; do not include the unrelated frontend artifacts unless the user explicitly decides their disposition.

# Documentation Reorganization

**Status:** Completed

## Goal

Turn the legacy documentation collection into a progressively loadable context structure where current architecture, future direction, completed work, operational procedures, and historical evidence are clearly distinguishable.

## Scope

- Rewrite `PLAN.md` as the concise current-state router.
- Classify and relocate substantial root/docs documents by canonical role.
- Rewrite stale current contracts from implemented behavior.
- Extract still-live Reader UX direction into a current roadmap.
- Preserve superseded designs, checklists, audits, and chronological notes as explicitly historical material.
- Update internal links and verify the fresh-agent retrieval path.

Out of scope:

- Changing production code or behavior.
- Rewriting historical documents to use current terminology.
- Creating documentation categories without a demonstrated owner.
- Treating Git history as a substitute for the historical material the user asked to preserve.

## Accepted Approach

Use these canonical homes:

- `README.md`, `PRODUCT.md`, `PLAN.md` — current entry points.
- `docs/architecture/` — current system and workflow architecture.
- `docs/decisions/` — durable cross-cutting rationale.
- `docs/plans/` — substantial workstream handoffs; completed plans remain frozen at stable paths.
- `docs/reference/` — stable domain and compatibility reference.
- `docs/roadmaps/` — unresolved future direction.
- `docs/research/` — durable investigative findings that remain useful.
- `docs/runbooks/` — operator/developer procedures.
- `docs/verification/` — dated evidence.
- `docs/archive/` — explicitly non-authoritative audits, superseded research, chronological logs, and snapshots.

Current code, tests, recent completed plans, and Git history are authoritative when legacy documents disagree. Moves preserve provenance; archived documents receive a clear historical status header.

## Decisions

- Preserve superseded material under `docs/archive/` rather than deleting it.
- Extract still-relevant Reader UX ideas into a concise roadmap and archive the old frontend-era assessment.
- Rewrite the candidate/catalog journey to metadata-first admission and asynchronous catalog synchronization.
- Archive the storage implementation checklist instead of trying to maintain completed and superseded phases as a live tracker.
- Keep completed dated implementation plans in `docs/plans/` at stable paths and mark lifecycle status explicitly.

## Outcome

The documentation tree now separates current architecture, durable decisions, completed plans, future roadmaps, stable reference, runbooks, verification, and historical material. `PLAN.md` is the current-state router; stale candidate/catalog and authentication/storage contracts were replaced with current documents; still-relevant Reader UX and Legado compatibility direction moved into roadmaps; superseded designs, audits, checklists, research, and chronological logs are explicitly archived.

No product code or behavior changed.

## Verification

- Every substantial existing document has one current or historical role.
- `PLAN.md` routes to current architecture, roadmaps, constraints, and relevant completed plans.
- All nine archived Markdown documents carry an explicit historical/non-authoritative notice.
- A repository-wide relative-link check covered 34 Markdown files with zero broken targets.
- Current documents contain no references to the superseded root/docs paths moved by this workstream.
- `git diff --check` passes.
- The retrieval path is direct: `AGENTS.md` → `PLAN.md` → one focused architecture, roadmap, reference, plan, runbook, or verification document; archive access is optional historical depth.

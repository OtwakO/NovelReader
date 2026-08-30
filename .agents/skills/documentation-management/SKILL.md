---
name: documentation-management
description: Manage persistent repository documentation and context recovery. Use when creating or restructuring project docs, plans, roadmaps, architecture or decisions; when substantial accepted work needs durable multi-session handoff state; when organizing a large or legacy repository; or when the user asks to orient or catch up on current repository state.
---

# Documentation management

Treat repository documentation as a **context index**: durable enough for a fresh agent to recover intent and state, small enough that agents load only what the task needs.

Follow applicable repository instructions first. This skill supplies procedure and templates; repository `AGENTS.md` remains policy.

## Choose the branch

- **Maintain** — create or update documentation for current work. Follow this file.
- **Orient / Catch up** — recover the current repository/workstream state with minimal context. Read [`references/catchup.md`](references/catchup.md). This branch is read-only unless the user also asks to repair/update documentation or continue implementation.
- **Bootstrap** — organize/document an existing large or legacy repository. Read [`references/legacy-repo.md`](references/legacy-repo.md) before changing the documentation structure.
- **Create a new persistent document** — read [`assets/document-templates.md`](assets/document-templates.md) for the smallest matching template.

Do not create documentation merely to make the tree look complete. A document earns its place when future retrieval value exceeds its maintenance cost.

## Load context progressively

Start with the smallest route that can answer the task:

1. Read applicable `AGENTS.md` instructions.
2. Read `PLAN.md` for project state and active-work routing when it exists.
3. Follow only the plan or scoped document relevant to the work.
4. Read local `AGENTS.md` / subsystem `README.md`, then directly relevant code and tests.
5. Reach architecture, decisions, roadmaps, notes, runbooks, reference docs, or Git history only when the task needs them.

**Completion criterion:** before editing persistent docs, you can name the canonical document that owns each piece of information you intend to change. If ownership is unclear and consequential, resolve it before writing.

## Canonical roles

Use only the document types the repository needs:

| Need | Canonical home |
|---|---|
| User/operator setup, usage, testing, configuration, deployment | `README.md` |
| Current project state and routing | `PLAN.md` |
| Accepted substantial implementation work and handoff state | `docs/plans/<topic>.md` |
| Project-wide future direction | optional `ROADMAP.md` |
| Scoped/feature future direction | optional `docs/roadmaps/<topic>.md` |
| Current architecture | optional `docs/architecture/` |
| Durable cross-cutting rationale | optional `docs/decisions/` |
| Rare non-obvious discovery that fits nowhere better | optional `docs/notes/` |
| Operational procedure | optional `docs/runbooks/` |
| Stable manually maintained specification/reference | optional `docs/reference/` |
| Exact change history | Git |
| Actual behavior | code and tests |

One fact has one canonical home. Link to it elsewhere instead of restating it.

## Creation gate

Before creating a new document, check all of these:

1. The information is durable enough that a later session is likely to need it.
2. It does not already have a canonical home.
3. Adding it to an existing focused document would make that document less coherent or harder to retrieve from.
4. The document has a distinct lifecycle or audience that justifies its own file.

If any condition fails, update the existing canonical source instead.

Small/localized work normally needs no new planning document.

## Implementation plans

Create or update an implementation plan **after the approach is accepted** when the work is substantial enough that preserving decisions or progress across meaningful steps/sessions has real value. Common cases: multi-session features, migrations, structural/high-risk changes, or work with several meaningful milestones.

Keep the path stable and semantic. Prefer `docs/plans/YYYY-MM-DD-topic.md` when the repository has no established naming convention. Change lifecycle with metadata, not directory moves.

An active plan makes these sections easy to find:

- `Goal`
- `Scope`
- `Accepted Approach`
- `Decisions` when non-obvious choices exist
- `Current State`
- `Next Action`
- `Verification`

Add `Progress` and `Open Questions` only when useful.

`Current State`, `Next Action`, and `Verification` are mutable current truth. Update them in place; do not append session diaries. Git already preserves prior versions.

Keep substantial active work handoff-ready. Update those three sections at meaningful milestones whenever the recorded state would otherwise become materially stale. Before a significant commit or an intentional session wrap-up with unfinished work, make them accurate enough that a fresh agent can resume without the previous conversation. Do not update after every edit, and do not wait until session end to record all progress.

**Completion criterion:** a fresh agent reading `PLAN.md`, this plan, and the directly referenced code/tests can identify what is done, what is unresolved, the exact next meaningful action, and what has actually been verified.

## Decisions

**Preserve decisions, not deliberation.** Record a decision when its reason would not be obvious from the resulting code or when the choice constrains future work.

Capture only what carries future value:

- the decision;
- why it was chosen;
- meaningful alternatives that a future maintainer might otherwise retry;
- consequences or constraints when material;
- `Revisit when` conditions when the choice depends on assumptions that may change.

Routine implementation choices stay in code/Git.

Keep feature-specific decisions in the implementation plan. Promote one to `docs/decisions/` only when its rationale remains authoritative across workstreams after the originating plan is complete.

When a durable decision is reversed, create the new decision and cross-link/supersede the old one. Do not rewrite historical rationale to make it look current.

## Roadmaps

A roadmap is future direction, not an implementation checklist or history log.

- Use `ROADMAP.md` only when project-wide direction is useful.
- Use `docs/roadmaps/<topic>.md` when independent areas have independent horizons.
- Multiple concurrent scoped roadmaps are normal.
- Link accepted current work to implementation plans instead of embedding detailed steps.
- Remove or compress completed direction when it no longer helps future navigation; implementation history already lives in plans and Git.

## Architecture and reference

Architecture docs describe **current truth** and stay accurate as architecture changes. Split by subsystem when independent areas evolve independently; keep an index/overview small and link downward.

Prefer executable/generated sources for schemas, CLI options, API fields, and other facts already cheaply discoverable from the repository. Manually document semantics, rationale, constraints, examples, and gotchas that the environment cannot explain.

## Notes and historical material

Avoid monolithic chronological development diaries. If an existing `DEVELOPMENT.md` is large, treat it as valid historical material rather than migrating it solely for consistency.

Create a topic note only for a durable finding a future maintainer would otherwise have to rediscover and that does not belong in an active plan, architecture doc, decision, runbook, test, or code comment.

Completed plans and superseded decisions are historical context. Mark their status and normally stop maintaining them; do not keep old plans synchronized with later file moves or architecture changes.

## Concurrent workstreams

One plan belongs to a **workstream**, not an agent. Agents are temporary; workstream context persists.

Use branches/worktrees or the repository's established mechanism for actual isolation. Plan metadata may name the affected scope/files when that makes overlap discoverable, but do not invent per-agent ownership or locking files unless the repository has demonstrated a need.

Before starting structural work in a large repository, check whether another active plan overlaps the same subsystem. If overlap creates a real integration decision, surface it rather than silently creating competing canonical state.

## Update by event

Update only the source whose owned truth changed:

- usage/setup/operation → `README.md`;
- project state/routing → `PLAN.md`;
- implementation/handoff → active plan;
- future direction → relevant roadmap;
- current design → architecture;
- durable rationale → plan/decision;
- rare durable discovery → note;
- operational procedure → runbook.

If none changed, no documentation edit is required.

Before a commit, check for stale canonical documentation; do not touch docs merely because a commit happened.

## Conflict rule

Documentation is orientation, not proof. When current documentation and implementation disagree materially, determine whether the document is stale or the implementation violates an intended contract. Resolve the inconsistency; do not silently pick a winner.

## Finish

Before declaring documentation work complete, verify every applicable condition:

- a fresh agent has an obvious entry point;
- active work is reachable from `PLAN.md` or the repository's equivalent router;
- each durable fact has one canonical home;
- current-state documents describe current truth;
- historical documents are not being maintained as current truth;
- active plans contain usable handoff state;
- meaningful decisions and their reasons are preserved without transcript-like deliberation;
- links/pointers you changed resolve;
- no new document exists only for symmetry or ceremony.

Report what you created, moved, froze, superseded, or deliberately left alone, plus any unresolved documentation ambiguity.

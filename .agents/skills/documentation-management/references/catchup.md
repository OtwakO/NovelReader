# Orient / catch up on a repository

Use this branch when the user asks to orient, catch up, resume context, or explain the current state of a repository/workstream. The goal is **minimum sufficient context**, not a repository audit.

This branch is read-only by default. Do not edit documentation merely because catch-up discovers something stale. Report material discrepancies. Transition to the normal maintain/bootstrap workflow only when the user also asked to update/reorganize docs or continue implementation in a way that requires edits.

## 1. Establish the route

Load context progressively:

1. Read applicable `AGENTS.md` instructions.
2. Read `PLAN.md` or the repository's equivalent current-state/router document when it exists.
3. Inspect Git state cheaply: current branch/worktree, `git status`, and recent relevant commits when they help identify the active workstream or uncommitted work.
4. If the user named a workstream, follow its linked implementation plan. Otherwise infer the most relevant active workstream from `PLAN.md`, branch/worktree, current changes, and the user's prompt.
5. Read only the relevant plan sections first: `Goal`, `Current State`, `Next Action`, and `Verification`. Read `Accepted Approach` / `Decisions` only when needed to understand why.
6. Inspect directly referenced code/tests only enough to verify material current-state claims or answer the user's catch-up request.
7. Reach architecture, decisions, roadmaps, notes, runbooks, reference docs, issues, or deeper Git history only when a concrete unresolved question requires them.

Do not read every active plan by default. If several workstreams are active and none is singled out, summarize their names/status from the router and inspect deeply only the one most relevant to the prompt or current branch. If there is no clear primary workstream, give a project-level orientation and identify the active choices without manufacturing certainty.

## 2. Reconcile current state

Treat documentation as orientation, not proof.

Compare the handoff state against cheap repository evidence:

- uncommitted changes;
- current branch/worktree;
- recent relevant commits;
- directly referenced code/tests when material.

If they agree, proceed.

If they materially disagree, report the discrepancy explicitly, for example:

- plan says a milestone is incomplete but the code/commit appears to contain it;
- plan says verification passed but the referenced test no longer exists;
- working tree has substantial changes newer than the recorded `Current State`;
- `PLAN.md` points to a completed/superseded plan as active.

Do not silently rewrite the docs during catch-up. If the user asked to continue work, first use the best supported current state and note the stale handoff; update the canonical docs as part of the subsequent implementation workflow when appropriate.

## 3. Return a compact orientation

Prefer a short state brief with only useful sections:

- **Project / area** — what this repository or selected workstream is doing.
- **Current work** — active workstream and meaningful progress.
- **Working tree** — only notable uncommitted/branch state, if any.
- **Next action** — the exact next meaningful step when one is recorded or clearly supported.
- **Verification** — what has actually been verified and what remains.
- **Blockers / decisions** — only unresolved items that affect what happens next.
- **Pointers** — the one or two canonical docs/files a continuing agent should open next.

Do not narrate the repository inventory, dump commit history, restate every completed milestone, or summarize unrelated roadmaps.

If the user invoked catch-up only, stop after orientation. If they asked to "catch up and continue", continue from the supported next action after the brief; do not ask them to restate context already recoverable from the repository.

## Completion criterion

Catch-up is complete when a fresh agent can answer, with evidence and without reading unrelated history:

1. What project/workstream am I in?
2. What is the meaningful current state?
3. Is there notable uncommitted or branch-specific work?
4. What is the next meaningful action?
5. What has and has not been verified?
6. Is any documentation materially stale or contradictory?
7. Which canonical source should I read next if deeper context is needed?

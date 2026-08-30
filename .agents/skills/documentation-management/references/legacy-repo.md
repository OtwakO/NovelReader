# Bootstrap a large or legacy repository

Use this branch when the user asks to organize, document, migrate, or retrofit an existing repository into the persistent-context structure.

The goal is **recovery, not cleanup theater**: give future agents a reliable context route while preserving important historical knowledge and avoiding speculative rewrites.

## 1. Inventory before restructuring

Inspect enough of the repository to identify:

- applicable `AGENTS.md` / agent instructions;
- root `README`, plans, roadmaps, architecture/design docs, development logs, ADRs/decisions, runbooks, references, and substantial subsystem READMEs;
- major code/package boundaries and obvious public entry points;
- current active work visible in docs, branches/issues if available, and recent relevant Git history when needed;
- duplicated, conflicting, obviously historical, or apparently stale documentation.

Do not read every file by default. Use filenames, indexes, manifests, search, and links to narrow the inspection, then open only the material needed to classify it.

Build an inventory with, at minimum:

| Existing item | Apparent role | Current/historical/unclear | Overlap/conflict | Proposed action |
|---|---|---|---|---|

Proposed actions should be one of: `keep`, `rewrite in place`, `split`, `link`, `freeze as history`, `supersede`, `move only if path stability is safe`, or `needs user decision`.

**Completion criterion:** every substantial existing documentation source has an evidence-based proposed role or is explicitly marked unclear. Do not reorganize while major sources are still unclassified.

## 2. User decision checkpoint

Before consequential structural edits, **always give the user one compact decision checkpoint**.

Ask whether there are important historical decisions, constraints, intended future directions, or unwritten context that must be preserved and may not be recoverable from the repository. Then list only the consequential ambiguities you actually found.

Typical decision points:

- two documents disagree about intended/current architecture;
- it is unclear whether an old roadmap is still intended or merely historical;
- several documents could plausibly be the canonical source for the same subject;
- a large `DEVELOPMENT.md` contains decisions whose continuing authority is unclear;
- subsystem boundaries or roadmap scopes require product/ownership judgment;
- a document appears obsolete but may encode rationale not present in code/Git;
- an external tracker may be canonical for work state;
- converting or freezing a document would change how future agents interpret it.

Do not ask the user to decide facts that are cheaply inferable from the repository. Batch the real choices into one message and include your recommended default with the reason.

If no consequential ambiguity is found, still tell the user the structure you infer and ask whether there is any important decision/context not represented in the repo that should be preserved before you reorganize it.

Do not perform ambiguous/destructive documentation restructuring until this checkpoint is resolved, unless the user explicitly delegated those choices to you. When delegated, prefer reversible choices and preserve the original material/history.

## 3. Propose the target map

After the checkpoint, present a minimal target structure based on actual needs, not the full taxonomy by default.

Typical starting point:

```text
AGENTS.md
README.md
PLAN.md
docs/
└── plans/
```

Add only demonstrated categories:

```text
docs/
├── plans/
├── roadmaps/       # independent long-term directions exist
├── architecture/   # current design no longer fits a concise PLAN
├── decisions/      # durable cross-cutting rationale exists
├── notes/          # rare durable discoveries need a home
├── runbooks/       # operational procedures exist
└── reference/      # stable manual reference is justified
```

For each existing document, state its disposition and canonical destination. Prefer stable paths; avoid mass moves whose only benefit is cosmetic consistency.

**Completion criterion:** the proposed map has one canonical home for each live information class and identifies which existing documents remain historical rather than current.

## 4. Migrate incrementally

Apply the approved map in small coherent steps:

1. Establish or tighten `PLAN.md` as the current-state/router document.
2. Link active substantial work to implementation plans; create plans only where durable handoff value exists.
3. Separate scoped future direction into roadmaps only where independent horizons exist.
4. Extract current architecture only when `PLAN.md` cannot remain concise without it.
5. Preserve feature decisions in plans; promote only genuinely cross-cutting durable rationale.
6. Freeze historical chronological logs instead of rewriting/migrating every entry.
7. Add topic notes/runbooks/reference only for real uncovered needs.
8. Replace duplicated live prose with links to the canonical source.
9. Update documentation pointers in `AGENTS.md` only when needed and without rewriting unrelated agent policy.

Do not rewrite historical documents to conform to present-day terminology unless the user explicitly wants historical cleanup. Preserve provenance when splitting or superseding material.

## 5. Fresh-agent test

Test the result as if you had no conversation history.

Starting from repository root, confirm that a fresh agent can:

1. discover applicable agent instructions;
2. read `PLAN.md` and understand the project's current state and active areas;
3. reach the relevant active plan for a chosen workstream;
4. identify its `Current State`, `Next Action`, and `Verification` without reading a session log;
5. follow links to current architecture or durable decisions only when needed;
6. distinguish current truth from historical plans/notes;
7. locate the relevant code/tests without reading unrelated documentation.

Also check links/pointers changed by the migration and search for obvious duplicate canonical statements left behind.

**Completion criterion:** the fresh-agent route works for every active workstream you reorganized, and no unresolved user-important decision was silently discarded.

## 6. Report the migration

Summarize:

- the new routing structure;
- documents kept, split, frozen, superseded, or deliberately not moved;
- important decisions/rationale preserved;
- unresolved ambiguities;
- any documentation debt deliberately left because fixing it would cost more than its retrieval value.

Do not claim the repository is fully documented merely because its documentation tree is organized. State the scope actually inspected and verified.

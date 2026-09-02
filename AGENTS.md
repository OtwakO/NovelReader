My GitHub Handle: [OtwakO](https://github.com/OtwakO)

You are running inside a sandboxed environment, /tmp directory is ephemeral and resets when session is closed. For files that would be used in future session and warrants persistency, properly organize, name, and store them under the project directory.

# Codebase Architecture & Engineering Guidelines

## Core Principles

Write practical, maintainable code. Match effort to the task — do not apply maximum process to
every change.

- Solve the requested problem with the smallest complete change.
- Keep the codebase easy to understand in parts, not just as a whole: a change should be
  understandable without reading the entire repository.
- Testing and review should build enough confidence to move forward, not eliminate every
  theoretical risk.
- Do not add features, abstractions, dependencies, refactors, or tooling because they might be
  useful later.
- If an optional improvement is worth doing, describe its benefit and cost first, and get
  confirmation before implementing it.
- Only expand scope beyond what was asked when it's required for correctness, security, data
  integrity, or to keep the project runnable.
- When unsure, prefer the option that's easier to reverse later.
- If a simpler approach meets the request, say so before implementing something more complex —
  even if the request described the complex version.

## Design Approach

Pick the plainest approach that solves the actual problem. Between two working solutions, prefer
the one a future reader would recognize immediately over the one that's clever or impressive.

### Picking a design
Start with the simplest tool available: a function, a conditional, a small data change. Only reach
for a named design pattern when the problem already has the shape that pattern exists to solve.

A few common patterns as examples of the rule, not the full list:
- Several interchangeable behaviors chosen at runtime → an interface with a few implementations
  (what "strategy" is for).
- Object construction with several genuinely different valid configurations → a factory or
  builder.
- Several independent things need to react to one event → an observer or event system.
- Two incompatible interfaces need to talk to each other → an adapter.

The same test applies to any other pattern too — decorator, command, chain of responsibility,
repository, dependency injection, memoization, and so on: use it when the problem already has that
pattern's shape, skip it when it doesn't. Different parts of the same codebase can and should end
up using different patterns, because they have different problem shapes. Match the pattern to the
problem actually in front of you — don't pick one from memory because it seems like good practice.

If the problem doesn't match any pattern's shape, use the plain version instead. These are signs a
pattern was added for its own sake rather than because the problem needed it — remove it and use
the direct version:
- An interface, abstract class, or config option with exactly one real implementation or value,
  and no second one actually planned.
- A layer whose only job is to call the layer beneath it unchanged.
- Something built to be generic or configurable for a case nobody has asked for yet.

Build for the case in front of you. Generalize once a second real case actually shows up, not
before.

Do not add defensive branches, error handling, configurability, or extension points for scenarios
the actual contract makes impossible. Validate and handle failure at real boundaries — untrusted
input, network, disk, external services — rather than inventing hypothetical states deep inside
trusted code.

Treat code size as a smell, not a target. If a straightforward version would be dramatically
smaller than the current implementation, simplify before landing it. Before finishing, ask whether
the solution is more complicated than the problem requires; if it is, reduce it.

### Fix the cause, not the symptom
When something is broken, find out why before deciding how to fix it. A fix that doesn't explain
the root cause is a patch, not a fix.

Signs a fix is a patch instead of a real fix:
- It's a special case for one bad input or state, and another bad case that would need its own
  special case is easy to imagine.
- It catches, ignores, or silences an error without knowing why the error happened.
- It's applied where the symptom showed up, not where the bad data or bad state was actually
  produced.
- It's landing right next to another recent fix in the same area.

If any of these are true, trace back to where the problem actually starts and fix it there
instead.

Sometimes the real fix genuinely isn't possible right now — a third-party dependency, a hard
deadline, code you don't own. When that happens: say so explicitly and mark the workaround clearly
in the code (e.g. `# workaround: <reason>, real fix would be <what>`). If the limitation is durable
and non-obvious, record it in the active implementation plan or the smallest relevant durable note
so a future agent doesn't mistake the workaround for the real fix.

Never stack a new patch on an existing patch without first checking whether the original patch
should become the real fix instead.

## Matching Effort to Risk

Classify the current change into one of three categories before starting. This decides how much
planning, testing, and documentation the change needs.

### Small or Localized
Examples: a contained bug fix, a small script, a text or config change, a simple UI tweak.
- Read the file being changed and its direct callers or dependencies. Nothing more.
- Make the smallest safe change.
- Run the smallest test that covers it.
- No new architecture, no repo-wide review, no new abstractions.
- Update docs only if something durable actually changed.

### Standard
Examples: a normal feature, a behavior change, a change touching a few related files.
- Write one or two sentences on what "done" looks like before starting.
- Add or update tests for the new or changed behavior.
- Run the tests for the affected area first; only run more if something breaks or looks coupled.
- If the work is substantial, decision-heavy, or likely to outlive the current session, use or
  create a dedicated implementation plan after the approach is accepted.
- Update `PLAN.md` only if project-level current state or priorities actually changed.

### High-Risk or Structural
Examples: migrations, auth/authorization, public API changes, shared data models, deployments,
destructive operations (delete, drop, overwrite), or anything flagged as sensitive.
- Write the plan down before touching code; substantial High-Risk work normally warrants a durable
  implementation plan.
- Confirm any decision that would be expensive to undo.
- Document rollback and compatibility concerns in the relevant plan or architecture/decision doc.
- Use broader tests (integration or end-to-end) where the risk justifies it.
- Keep only the authoritative documentation affected by the work current as meaningful state
  changes; do not update unrelated docs for ceremony.

A change stays High-Risk because of what it touches, not how many lines it is — a one-line change
to a payments function is still High-Risk. Don't classify a change as High-Risk just because more
testing is *possible*; only because the risk is real.

## Definition of Done

A task is done when:
- The requested behavior works.
- The project (or the part you touched) still runs.
- The tests appropriate to the change's category above pass.
- No new warnings, dead code, debug prints, or unexplained TODOs.
- No secrets or hardcoded environment-specific values.
- Any authoritative documentation made stale by the change was updated; unrelated docs were not
  touched just because they exist.
- `PLAN.md` reflects project-level state changes when relevant, and any active implementation plan
  accurately records meaningful unfinished state or verification limits.
- Durable non-obvious rationale or discoveries were recorded in the smallest canonical place when
  future agents would otherwise have to rediscover them.
- Nothing unrelated was changed without asking first.
- A bug fix addresses the root cause — or, if it's a workaround, that's stated explicitly and
  logged (see Design Approach).

Not every task needs new tests, a full test run, a README edit, or a documentation update — only
add what the category above and the actual change justify.

**Report what you actually did.** If you only ran the tests for one file, say "tests for X pass" —
not "tests pass." Never describe a partial test run as a full one.

## Handling Ambiguity

Do not hide assumptions or confusion inside the implementation. Surface any assumption or tradeoff
that materially affects behavior. If the uncertainty is harmless, reversible, and inferable from
the existing code or conventions, make the call and state the assumption briefly when useful.

Stop and ask when the uncertainty would affect architecture, data shape, public interfaces, auth,
third-party service choice, a destructive operation, or anything expensive to undo.

- Batch your questions into one message if you have more than one.
- Don't ask about things you can infer from the existing code or conventions — just make the call.
- If multiple interpretations would lead to meaningfully different results, present the plausible
  interpretations and ask which one is meant rather than picking one silently.
- If there are multiple viable approaches with meaningful tradeoffs, state those tradeoffs briefly
  instead of silently choosing based on hidden assumptions.
- If a simpler approach fully meets the request, say so before proceeding with a more complex one
  (see Core Principles).
- If you're about to guess on something consequential, stop and ask instead.

## Planning Before Coding

For a Small/Localized change, do not create planning ceremony — make the smallest safe change.

Before starting a new project or meaningful Standard/High-Risk work, establish what you're building,
what "done" means, the important boundaries/data flow, the main steps, and known risks or open
questions. Keep this brief unless the work itself is substantial.

If a substantial proposal has been accepted and the work is likely to span multiple meaningful
steps, decisions, or sessions, create or update a durable implementation plan under `docs/plans/`.
The plan is the persistent handoff artifact for that workstream; do not create one for routine local
changes just for consistency.

## Documentation & Persistent Context

Documentation is a **progressively-loadable persistent context layer**, not a second copy of the
codebase or a diary. Its purpose is to make long-lived projects easy to maintain and cheap for a
fresh agent to resume.

Core rules:
- **One fact, one canonical home.** Link instead of duplicating information across docs.
- **Update by relevance, not ceremony.** Update only documents whose owned truth changed; many
  commits legitimately need no documentation change.
- **Current truth and history are different.** Current-state docs stay accurate; completed plans and
  superseded decisions normally remain frozen historical context.
- **Preserve decisions, not deliberation.** Record important choices, rationale, meaningful rejected
  alternatives, and reconsideration conditions when they would not be obvious from code. Skip
  routine implementation choices.
- **Every document must earn its maintenance cost.** Do not create one merely because a document
  type exists.

### Canonical roles
Use only the types the repository actually needs:
- `README.md` — user/operator setup, usage, testing, configuration, and deployment.
- `PLAN.md` — concise project dashboard and routing layer: objective, system map, current state,
  active-work links, immediate priorities, and project-level questions/constraints.
- `docs/plans/<topic>.md` — substantial accepted implementation work and its durable handoff state.
  One plan belongs to a workstream, not an agent.
- `ROADMAP.md` / `docs/roadmaps/<topic>.md` — optional project or scoped long-term direction; multiple
  independent roadmaps are fine.
- `docs/architecture/` — current architecture when detail no longer fits concisely in `PLAN.md`.
- `docs/decisions/` — optional durable cross-cutting decisions whose rationale outlives one plan.
- `docs/notes/`, `docs/runbooks/`, `docs/reference/` — create only for a real need: rare durable
  discoveries, operational procedures, or stable reference material respectively.

The legacy chronological development log is archived at
`docs/archive/history/development-log-through-2026-08-30.md`. Do not append to it. Put new knowledge
in the smallest canonical current document and use Git history for chronology.

### Plans, decisions, and handoff
Create or update an implementation plan after the approach is accepted when the work is substantial
enough that its decisions or progress are likely to matter across meaningful steps or sessions —
commonly multi-session, structural, migration, or High-Risk work. Small/Localized work normally does
not warrant a plan.

Keep plan paths stable and semantic. An active plan should make **Goal**, **Scope**, **Accepted
Approach**, important **Decisions**, **Current State**, **Next Action**, and **Verification** easy to
find; add progress/open questions only when useful. Update current state in place rather than
appending session diaries — Git already preserves old versions.

Keep substantial active work **handoff-ready** rather than waiting until the end of a session.
Update the active plan at meaningful milestones whenever its `Current State`, `Next Action`, or
`Verification` would otherwise become materially stale. Before a significant commit or an intentional
session wrap-up with unfinished work, ensure those sections accurately describe the stopping point so
a fresh session can resume without the prior conversation. Do not update the plan after every edit,
and do not defer all state recording until session end.

When completed, record the outcome and mark the plan completed; it then becomes historical context
and normally stops receiving maintenance. If a durable decision is later reversed, record the new
decision and link/supersede the old one rather than rewriting history.

For concurrent development, keep one plan per persistent workstream and use branches/worktrees for
actual isolation. Agents are temporary; workstream context persists. Do not create per-agent plan or
locking files unless the project has a demonstrated need for them.

### Fresh-session retrieval
Load context progressively rather than reading everything:
1. Applicable `AGENTS.md` instructions.
2. `PLAN.md` for project orientation and active work.
3. The relevant implementation plan, if one exists.
4. Relevant local `AGENTS.md` / subsystem `README.md`, then directly affected code and tests.
5. Linked architecture, decisions, roadmaps, notes, runbooks, reference docs, or Git history only
   when the task actually needs them.

Documentation is orientation, not proof. If current docs and code materially disagree, determine
which is stale or whether the implementation violates an intended contract; do not silently assume
either side wins.

Before committing, check whether the change made any canonical documentation stale and update only
what changed: usage → README; project state → PLAN; implementation/handoff → active plan; future
direction → roadmap; current design → architecture; durable rationale → plan/decision; rare durable
discovery → note. If none changed, do not edit docs.

### Documentation skill
If a `documentation-management` skill (or repository-equivalent documentation/planning skill) is
available, consult it when creating or restructuring persistent docs, organizing/documenting an
existing large or legacy repository, starting substantial accepted multi-session work, managing
roadmaps or durable decisions, preparing a non-trivial handoff, or when the user explicitly asks for
a repository orientation/catch-up. The skill supplies detailed templates, lifecycle procedure, and a
read-only catch-up workflow; this `AGENTS.md` supplies the policy.

Do **not** load the skill for routine Small/Localized work unless documentation is actually part of
the task. If no such skill is available, follow the rules above directly — repository work must not
depend on the skill being installed.

## Modularity, Coupling, and Cohesion
- Each file or module owns one clear thing, completely.
- Group code by feature, not by type (not all "models" in one folder and all "controllers" in
  another).
- Prefer a few well-designed functions or interfaces over many thin wrappers.
- Modules talk to each other through their public interface only — never reach into another
  module's internals.
- Dependencies flow in one direction. No circular dependencies.
- Avoid catch-all files: no `utils.py` / `helpers.js` that becomes a dumping ground, no single
  file that ends up knowing about everything.
- Define a shared type or data shape once. Don't redefine it in multiple places.
- If a normal change keeps touching many unrelated files, that's a sign the boundaries are wrong —
  fix the boundary, don't just keep editing around it.

### File Size
Aim for under roughly 250 lines per file as a guide, not a hard rule.

Split a file when:
- It's doing more than one job.
- You have to read unrelated code to understand the part you need.
- Unrelated changes keep colliding in it.
- A clear, self-contained piece of it could become its own module.

Don't split a file into tiny pieces just to hit a line count — that makes things harder to follow,
not easier.

### Shared Code
If something is used in more than one place — logging, config, validation, auth helpers, error
formatting — build it once behind a simple interface instead of copying it.
- Keep the setup and config details inside the shared module. Callers shouldn't need to know how
  it works internally.
- If you're not sure something will be reused, write it inline for now. Pull it out into a shared
  module once it's actually duplicated.
- Don't build a shared abstraction for something that might be reused someday — only for something
  that already is.

## Making Changes: Stay in Scope
- Only touch what the task needs.
- Only clean up things your own change introduced or broke — not pre-existing issues you happened
  to notice.
- If your change makes an import, variable, function, file, or other code unused, remove that new
  orphan as part of the same change. Do not remove unrelated pre-existing dead code unless asked.
- If you notice unrelated dead code, bugs, or cleanup opportunities, mention them separately if
  useful; do not silently fold them into the current task.
- Don't rename, reformat, or "modernize" code that isn't part of the task.
- Match the existing style around your change, even if you would write it differently from scratch.
- Don't change existing public behavior unless that's the point of the task.
- Every changed line should be traceable to the requested behavior or to work required for its
  correctness, verification, documentation, or keeping the project runnable.

Before writing code, look at:
- The file you're changing
- Its public interface (what other code calls)
- Its direct callers
- Its existing tests
- Any shared types it uses

Only look at more of the repo if something is unclear or a test fails unexpectedly. For an
existing project, use `PLAN.md` and any relevant active implementation plan for orientation instead
of re-reading the whole codebase.

## Review
- For a Small/Localized change: review your own diff once — the changed lines, what calls them,
  and how they could fail. That's enough.
- Don't re-review the whole repo for a small change.
- Save a deeper, security-focused, or wider review for High-Risk/Structural work.
- Stop reviewing once you've caught the real risks — repeating the same check again doesn't add
  confidence.

## Interface Design
- Make inputs, outputs, and side effects explicit — use types if the language has them, otherwise
  document them clearly.
- A function should do one cohesive thing. Split it if it's doing two unrelated things — not just
  because its name has "and" in it.
- Avoid hidden global state and surprising side effects.
- If other code, or another team or service, depends on an interface, treat it as a contract:
  don't break it without a migration plan, and deprecate before removing.
- If an interface is internal and every caller can be updated in the same change, it's fine to
  change it directly.

## Error Handling
- Match the convention already used in that part of the codebase, layer by layer — e.g. exceptions
  inside business logic, typed results at a boundary, HTTP error responses in the API layer, exit
  codes in a CLI. Different layers can reasonably use different mechanisms; just be consistent
  within each one.
- Every error should carry enough information to know what failed.
- Know the difference between an error you can recover from (handle it) and one you can't (fail
  loudly, don't try to continue).
- Any external call — network, disk, another service — needs error handling and a timeout. Never
  assume it will succeed.
- Never swallow an error silently, and never report a partial success as if it fully succeeded.

## Testing

Tests should give enough confidence that the change works — not maximum possible coverage.

### What to run, in order
1. Run the smallest existing test that already covers what you changed.
2. Write or update a test for: the new behavior, the bug you fixed, or a real failure mode.
3. Run the tests for that file, module, or boundary.
4. Only go wider than that if: the targeted tests failed unexpectedly, you changed something
   shared, there's a real coupling risk, or the change is High-Risk.

Don't run the entire test suite, every linter, and every scanner for a small change by default.

### How many tests
Aim for the fewest tests that cover the real risk:
- One normal or expected case
- One real edge case or failure mode
- One regression test if you're fixing a bug
- One boundary test, if you changed how two parts talk to each other

Skip: near-duplicate tests, every possible input combination with no real risk behind them, tests
for implementation details that didn't change, and end-to-end tests where a smaller test already
proves it.

- Test isolated logic at the unit level.
- Test real boundaries (API, database, etc.) at the integration level.
- Reserve end-to-end tests for critical paths that can't be checked more cheaply.
- Reuse existing test setup and fixtures instead of building new ones for one test.
- Tests shouldn't depend on running in a specific order.

### Write the test first, or after?
Write the test first when the logic is non-trivial, you're reproducing a bug, or you're
protecting a public interface. For simple wiring, UI layout, config, or a Small/Localized change,
it's fine to write the code first and test right after.

### Keep test output short
- Run tests through the project's normal test command in quiet mode, not verbose mode.
- A passing run should look like one line, e.g. `5/5 passed`. Don't print a line per test.
- A failing run should show which test failed and why — the real error, not just "failed":
  ```
  4/5 passed, 1 failed
  Failed: test_name — expected X, got Y
  ```
- Only include a full stack trace or logs if the short message isn't enough to explain the
  failure.

### When to stop
Stop once the change is covered at a reasonable level and the tests pass. Adding another similar
test after that repeats the same confidence — it doesn't add new confidence.

### [Important] The general rules are:
- Design tests that are effective at covering the **real practical risks of the interested scope**.
- Tests should be deterministic, and **token efficient to write, run, and maintain**.
- Tests should be **repeatable** and **easy to run** in a repeatable way.
- Do not overfit tests to a specific test dataset, test runner, or test framework.
- Duplicated tests, repo-wide tests when the scope is small and added no real value, are a waste of token and time without offering any real confidence, that is just over-testing for no real gain.
- Tests should maximize confidence per test and per fixture token, not maximize test count.

### BookSource fixtures and CI
- Do not commit or push new complete or real-world BookSource definitions, corpus extracts, raw source objects, or audit output embedding source rules/scripts/headers/cookies/credentials/private endpoints. This applies even when the remote is private.
- Keep real BookSources under the ignored repository-root `test-booksources/` directory and inspect generated evidence before committing it; a neutral filename does not make embedded source content safe.
- Default tests and GitHub Actions must pass from a clean checkout without `test-booksources/`, live websites, credentials, cookies, or developer-local state. Repository fixtures must be minimal, synthetic, and deterministic.
- A test that genuinely requires a complete real source is an optional local compatibility check: it should skip clearly when the ignored fixture is absent, while malformed or unreadable installed fixtures still fail.
- Do not create a synthetic duplicate for every private compatibility test. Add or extend a deterministic regression only when a real source reveals a reusable NovelReader defect not already covered.
- Live-source audits remain explicit local workflows and must not become dependencies of `go test ./...`, GitHub Actions, or container publication.
- Follow `testdata/booksource/README.md` for detailed placement and sanitization rules. Existing historical tracked source material is not precedent for adding more.

## Refactoring
Only refactor when:
- You can't safely make the requested change without it.
- The existing structure is actively blocking the change.
- A file mixes clearly unrelated responsibilities.
- Real, existing duplication is causing bugs or maintenance pain.
- You're about to add a fix on top of a previous fix in the same area (see Design Approach).
- The user asked for it.

Keep it as small as it needs to be, and say why you're doing it. If a bigger refactor seems worth
it, describe the benefit and cost and ask before doing it — don't fold it into a feature or bug
fix commit.

Refactoring should not change what the code does. If tests need to change because of a
"refactor," it wasn't just a refactor.

## Git
- Start every new project with `git init` and a `.gitignore` (secrets, build output, dependency
  folders, editor files).
- Commit at each complete, working step — not every single edit, and not one giant commit at the
  end.
- Every commit should leave the project (or the part you touched) running, with its tests passing.
- Before committing, check whether the change made any authoritative documentation stale. Update
  only the affected canonical docs and include those updates in the same logical commit when
  appropriate.
- One logical change per commit — don't mix a feature, a fix, and a cleanup in the same commit.
- Commit message format: `type: short description` — e.g. `feat: add auth module`,
  `fix: handle null response`, `refactor: split router`.
- Tag a milestone (e.g. a release) only if the tag will actually be useful for rollback later.
- Don't push unless asked to, or unless that's the established workflow for this repo.

### Before anything destructive
These need explicit confirmation first:
- `git push --force` — use `--force-with-lease` instead, and only on a branch that's actually
  yours.
- `git reset --hard` on a shared or remote-tracked branch.
- `git rebase` on a branch that's already been pushed and shared.
- `git clean -fd` — always run `git clean -nfd` first to see what would be deleted.
- `git stash drop` or `git stash clear` — confirm nothing in there is still needed.

Never commit directly to `main`/`master` on a multi-person project — use a branch. Check
`git status` and recent history before any destructive command. Never discard changes that aren't
yours.

## Naming and Code Quality
- Names should explain themselves. Avoid cryptic abbreviations and single-letter names, except
  obvious cases like a loop index.
- Follow the naming style already used in the codebase.
- No dead code, commented-out blocks, debug prints, or unexplained TODOs.
- Keep secrets and environment-specific values out of the code — use config or env vars.
- Don't duplicate a constant that already exists elsewhere — reuse it.

## Idempotency
Anything that might run more than once by accident — setup scripts, migrations, retried API
calls, file generation — should be safe to run twice with the same result.

For things that genuinely can't be idempotent by nature (sending a payment, sending a
notification, incrementing a counter), use a dedup key or a transaction so a retry doesn't double
it up.

Don't build this kind of safety into a script that only ever runs once, by hand, one time.

## Subagents

**This section is about you — the model doing this task — deciding whether to hand part of it to
another agent.**

Subagents are inherently costly. They start with fresh context, often have to re-read files or
reconstruct project state, consume additional tokens and time, and create another result that must
be checked and integrated. Treat delegation as an optimization with a real cost, not as a default
way to appear thorough.

Default: zero subagents. Prefer doing the work yourself, in this same session, whenever you can do
so safely and effectively. Only delegate when the task would **genuinely benefit significantly**
from it and that benefit clearly outweighs the added context, token, coordination, and verification
cost.

Exception: if the user explicitly requests a specific subagent, delegation pattern, or use of
subagents for the task, follow that request. In that case, the user's instruction overrides the
default cost-avoidance heuristic, unless it conflicts with safety requirements or the available
tooling/capabilities. Do not silently replace the requested subagent workflow with a single-agent
approach just because it would be cheaper.

Good reasons to delegate include:
- The work splits into substantial, genuinely independent pieces that can be done in parallel, and
  the time saved is meaningful enough to justify the duplicate context cost.
- The work is High-Risk, and a second, independently-formed opinion would materially improve
  confidence or catch something you might rationalize past — e.g. reviewing an auth change. Not
  for routine work.
- The files or history involved won't reasonably fit in your own context, so delegation avoids a
  real context limitation rather than creating one.

If the benefit is marginal, uncertain, or merely "could be useful," do not delegate. A simple or
moderate task that one agent can safely complete does not need separate planning, implementation,
review, or verification agents.

If you do delegate: give it the exact question, the smallest set of files it needs, and what you
expect back. Don't ask it to re-read the whole repo. Treat what it gives you as a claim to verify,
not a fact to trust automatically.

Don't delegate to look thorough. Don't split one task into "implementer" + "reviewer" +
"verifier" agents when one agent could safely do all of it. Stop delegating once you have enough
confidence to move on.

## Keeping Context Small
This is about repo-wide habits, not any one task:
- Name files, workstreams, and documents semantically so their purpose is obvious without opening
  them.
- Keep public interfaces small.
- Keep a helper near the feature it supports, unless it's genuinely shared by several features.
- Load context progressively: `PLAN.md` → relevant active plan → relevant local docs → code/tests →
  deeper historical/reference material only when needed.
- Don't read unrelated roadmaps, completed plans, notes, or Git history merely to appear thorough.
- Don't re-read a file whose content you already have and that hasn't changed.
- Run the targeted test before reaching for the whole suite.

None of this is permission to skip correctness, security, or a real interface concern — it's
about not doing more work than the task needs.

## Dependencies
- Every dependency is something you now have to maintain — justify adding one.
- Prefer small, actively maintained libraries over large or abandoned ones.
- If this project is an application you deploy, lock exact versions so it's reproducible.
- If this project is a library others import, declare a compatible version range for consumers,
  but still lock exact versions in this repo's own CI and dev setup.
- Don't upgrade or swap a dependency as a side effect of an unrelated change.

## Logging
- Set up one structured logging system for the project and use it everywhere — don't let each
  file invent its own format.
- A small one-off script can just print — it doesn't need the full logging setup.
- Use log levels consistently: DEBUG for detail, INFO for normal operation, WARN for something
  degraded but working, ERROR for something that needs attention.
- A production error log should have enough detail to know what happened without a debugger
  attached.
- Never log passwords, tokens, secrets, or personal data.

## Security

Apply real, specific protections — not a generic "sanitize everything" pass:
- Treat input as untrusted at the point it enters your system. Check its shape and constraints.
- Use the protection that matches where the data is going: parameterized queries for a database,
  escaping or encoding for HTML output, safe deserialization for untrusted data, path
  normalization for file paths.
- Check auth at a clear boundary (e.g. middleware, gateway) — not scattered inside business logic.
- Check authorization wherever the information needed to decide access actually lives.
- Never put a secret in source control, logs, or an error message.
- Know the common failure modes for your stack: injection, insecure deserialization, path
  traversal, broken access control.

**Match the effort to real exposure.** A local tool used only by you or a few trusted people
doesn't need rate limiting, CSRF protection, or fuzz testing — there's no realistic attacker in
that picture. A public-facing app handling real user data does need the standard protections
above, applied properly. Reserve deep threat modeling and dedicated security testing for the
actual high-risk cases: auth, payments, and anything touching regulated or personal data.

If you find a real, existing vulnerability while doing unrelated work, it's fine to fix or flag
it — but don't turn a small task into a full security audit on your own initiative.

## Scaling
- Design for roughly the next order of magnitude of load — not for a scale you don't have yet.
- Don't optimize before you've measured that something is actually slow.
- Keep business logic separate from the transport layer (HTTP, CLI, queue) so either can change
  independently.
- Define a shared data shape once, not once per module.
- Propose big changes — caching, concurrency, sharding, a new queue — and their tradeoffs before
  implementing them.

## Wrapping Up a Task
Keep your final summary short:
- What changed
- What you tested, and at what level (see "Report what you actually did" above)
- Anything unresolved or limited
- Whether any authoritative documentation or unfinished-work handoff state needed updating, and
  whether you already updated it
- At most one or two optional follow-up ideas, clearly marked as optional

Don't narrate every step you took or list every passing test.

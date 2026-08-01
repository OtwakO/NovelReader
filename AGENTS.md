My GitHub Handle: [OtwakO](https://github.com/OtwakO)

# Codebase Architecture & Engineering Guidelines

## Core Principles

Write practical, maintainable code with effort proportional to the task.

- Solve the requested problem with the smallest complete change.
- Preserve strong architecture and locality: a feature should be understandable without loading the
  entire repository or a monolithic file.
- Verification should establish sufficient confidence, not exhaust every theoretical possibility.
- Do not add features, abstractions, dependencies, refactors, tooling, or infrastructure merely
  because they might be useful later.
- When an optional improvement is worthwhile, briefly propose it and explain its benefit and cost
  before implementing it.
- Expand scope without approval only when required for correctness, security, data integrity, or
  keeping the affected project path runnable.
- Prefer reversible decisions when uncertainty remains.

## Proportionality

Classify work by actual impact rather than applying maximum ceremony to every task.

### Small or Localized Work

Examples: contained bug fixes, simple scripts, text changes, minor configuration, and straightforward
UI adjustments.

- Read the target area and its direct dependencies.
- Make the smallest safe change.
- Run the smallest meaningful test or check.
- Do not create formal architecture, broad reviews, new abstractions, or repository-wide cleanup.
- Update project documentation only when the durable state or useful history changed.

### Standard Work

Examples: a contained feature, meaningful behavior change, or change spanning a few related modules.

- State concise success criteria and a brief plan.
- Add or update focused tests for changed behavior and meaningful risks.
- Run targeted tests first; expand only when coupling or failures justify it.
- Update `PLAN.md` when scope, architecture, phase, current state, or next steps change.

### High-Risk or Structural Work

Examples: migrations, authentication, authorization, public API changes, shared data models,
deployment changes, destructive operations, or broad architecture changes.

- Record a clear plan before implementation.
- Confirm expensive-to-reverse decisions.
- Document compatibility, migration, rollback, and data-safety concerns.
- Use broader integration or end-to-end verification where risk justifies it.
- Keep handoff documentation current throughout the work.

Do not escalate a task merely because more engineering work is possible.

## Definition of Done

A task is complete when all applicable items are true:

- The requested behavior is implemented.
- The affected project path remains runnable.
- Relevant focused tests or checks pass.
- No new warnings, errors, dead code, debug statements, or unexplained TODOs were introduced.
- No secrets, environment-specific values, or duplicated policy values were improperly hardcoded.
- Public documentation was updated when setup, usage, configuration, or interfaces changed.
- `PLAN.md` reflects material changes to durable project state.
- `DEVELOPMENT.md` records non-obvious history worth preserving.
- No unrelated changes or unapproved optional improvements were introduced.
- Required work or verification was not skipped silently.

Not every task requires new tests, a full-suite run, README changes, a development-log entry, or a
commit. Apply each item proportionally.

Report verification exactly. Say “targeted tests pass” when only targeted tests were run.

## Handling Ambiguity

Ask when uncertainty materially affects architecture, data shape, public interfaces, authentication,
authorization, third-party selection, destructive operations, or another expensive-to-reverse
decision.

- Ask early and consolidate genuine blockers into one exchange where practical.
- Do not stop for low-risk details that can be inferred from existing conventions.
- Choose the simplest reversible option for minor implementation details.
- State an assumption only when it materially affects the result.
- Present multiple interpretations when the request is genuinely ambiguous.
- Push back before implementing a substantially more complex approach when a simpler one satisfies
  the requirement.
- Stop rather than proceeding on a consequential guess.

## Planning Before Coding

Before substantial work on a new project or significant feature, define:

- Objective and success criteria
- Directory structure and module ownership
- Data flow and interface contracts
- Main implementation phases
- Important risks and open questions

Group by feature or domain rather than merely by file type. Keep related models, logic, interfaces,
and tests close together. A canonical entry point should be obvious from the repository root.

For a small localized change, do not create planning ceremony. Identify the smallest safe change
and proceed.

## PLAN.md

Every maintained project should have a concise `PLAN.md` at the repository root. It is the
authoritative handoff for the project's durable present and intended future—not a chronological
activity log.

Include applicable sections only:

- **Objective**
- **Architecture overview**
- **Implementation phases and completion criteria**
- **Current state, exact stopping point, and next step**
- **Open questions and tradeoffs**
- **Out of scope**
- **Environment and operational constraints**
- **Key durable decisions**

Create it before substantial implementation in a new project. For an existing project without one,
create it when beginning meaningful multi-step work; do not reconstruct every historical detail.

Update `PLAN.md` when objectives, scope, architecture, interfaces, phase, current state, next step,
constraints, open questions, or durable decisions materially change.

Before every commit, check whether the pending change makes `PLAN.md` stale. Update it before the
commit when needed and include the update in the same commit. Do not edit it for every command,
test run, or minor implementation detail.

At handoff, `PLAN.md` should answer:

1. What is being built?
2. How is it organized?
3. What is complete?
4. What is in progress?
5. What happens next?
6. What decisions or risks remain?

## DEVELOPMENT.md

`DEVELOPMENT.md` preserves meaningful chronological context that is not obvious from the code, diff,
commit message, tests, or current plan.

Record only useful history such as:

- Difficult bugs and debugging discoveries
- Non-obvious implementation or architecture decisions
- Failed approaches worth avoiding
- Environment quirks and temporary workarounds
- Unusual verification limitations
- Exact stopping points for unfinished work

Do not log every command, routine edit, successful implementation step, ordinary test invocation,
or a prose restatement of a commit.

Use compact entries:

```markdown
### [YYYY-MM-DD] Short title
- **Context**: task or phase
- **Change**: what changed
- **Reason**: why
- **Verified**: focused checks
- **Affected**: files or modules
- **Watch out**: risks, limitations, or follow-up
```

Omit fields that add no value.

Before every commit, check whether the pending work contains non-obvious history worth preserving.
Update `DEVELOPMENT.md` before the commit when needed and include it in the same commit. Do not
create an entry merely because a commit is being made.

`PLAN.md` describes durable state. `DEVELOPMENT.md` preserves useful history. Update both when a
historical event also changes the durable plan.

## Modularity, Coupling, and Cohesion

Architecture should maximize locality of understanding and locality of change.

- Each module or file owns one clear responsibility and owns it completely.
- Group code by feature or domain.
- Prefer deep modules with simple, stable public interfaces.
- Prefer a few meaningful functions over many shallow wrappers.
- Keep coupling low and cohesion high.
- Communicate through explicit interfaces rather than reaching into another module's internals.
- Dependencies should flow in a clear direction. Circular dependencies are forbidden.
- Avoid god objects, catch-all utility modules, central coordinator files, and monolith files that
  accumulate unrelated behavior.
- Keep transport, persistence, and business logic separate where this improves locality,
  testability, or reuse.
- Define shared core types and data shapes once rather than duplicating them.
- A normal feature change should remain within one coherent area. If changes repeatedly ripple
  across many unrelated files, inspect the boundaries.

### File Size and Locality

Target roughly fewer than 250 lines per file as a useful default, not a mechanical limit.

Split a file when:

- It owns multiple responsibilities.
- Understanding one feature requires reading large unrelated sections.
- Unrelated changes frequently collide in it.
- It has become difficult for a human or AI to load and reason about safely.
- A coherent submodule can be extracted behind a simple interface.

Do not create tiny fragments merely to satisfy a line count. Excessive fragmentation also harms
locality. The goal is cohesive, bounded modules.

### Reusable Shared Capabilities

When functionality is repeatedly needed or inherently cross-cutting, design it once behind a small,
stable, easy-to-use interface.

Good candidates include logging, configuration access, validation policy, serialization, metrics,
authentication primitives, error mapping, and external client construction.

- Keep setup, formatting, policy, and lifecycle management inside the shared module.
- Callers should not need to understand its internal configuration.
- Reuse an existing shared capability rather than creating local variants.
- Centralize known repeated behavior before copies spread.
- When reuse is uncertain, implement directly and extract only after duplication becomes real.
- Separate reusable mechanisms from feature-specific policy.
- Give shared capabilities clear ownership; do not turn `utils` into a dumping ground.

For example, configure one project logging system and expose a straightforward logger interface.
Feature modules should not independently configure handlers, formats, destinations, or context
fields.

Reuse does not justify speculative abstraction. Centralize established common behavior, not
hypothetical future variation.

## Surgical Changes and Reading Scope

- Touch only what the task requires.
- Clean up only problems introduced by the current change.
- Do not perform drive-by formatting, renaming, modernization, or unrelated refactoring.
- Match the surrounding style unless it is materially harmful.
- Preserve public behavior unless the task explicitly changes it.

Before writing, begin with:

- The target module
- Its public exports
- Direct callers or entry points
- Nearby tests
- Shared types and utilities directly involved

Expand repository exploration only when dependencies, failures, or unclear behavior require it.
Use `PLAN.md` for orientation and consult relevant `DEVELOPMENT.md` entries before broadly rereading
the codebase.

## Review Strategy

- For localized work, perform one focused review of the changed diff, affected interfaces, and
  meaningful failure paths.
- Do not perform repeated full-repository reviews for a small change.
- Use broader, independent, security-focused, or repository-wide review only for structural,
  cross-cutting, high-risk, or unusually difficult work.
- Stop when identified risks are addressed and another pass would repeat the same checks.

## Interface Design

- Make inputs, outputs, side effects, and ownership explicit.
- Use types where supported; document contracts clearly in dynamic languages.
- Functions and methods should have one cohesive responsibility. Split them when they combine
  independently changing concerns—not merely because their name contains “and.”
- Avoid hidden global state and surprising side effects.
- Stable interfaces used by external or independently deployed consumers are contracts.
- Use a migration or compatibility plan when breaking such contracts.
- Deprecate before removal when compatibility is required.
- Internal or explicitly unstable interfaces may change directly when all callers are updated
  atomically and the change remains localized.

## Error Handling

- Use consistent conventions within each architectural layer.
- Different layers may use different appropriate mechanisms: exceptions internally, typed results
  at a boundary, HTTP errors in transport code, or exit codes in a CLI.
- Errors should carry enough context to identify the failed operation.
- Distinguish recoverable from unrecoverable failures.
- Handle expected external I/O failures at the appropriate boundary.
- Use timeouts where supported and where waiting indefinitely is unsafe.
- Do not add redundant wrappers when an existing boundary handler already provides sufficient
  context.
- Never silently swallow a required failure or report partial success as complete success.

## Testing and Verification Strategy

Tests must be focused, concise, deterministic, and proportional to risk. The goal is enough evidence
that the changed behavior works and relevant nearby behavior remains intact.

### Selection Order

1. Run the smallest existing test covering the changed behavior.
2. Add or update focused tests for new behavior, bug regressions, meaningful failure modes, or
   changed contracts.
3. Run the nearest relevant test module, package, or integration boundary.
4. Expand only when targeted tests fail unexpectedly, shared infrastructure changed, known coupling
   creates risk, the full suite is cheap, the change is high-risk, or repository policy requires it.

Do not automatically run every suite, platform matrix, linter, type checker, formatter, benchmark,
and scanner for a localized change.

### Test Quantity and Level

Prefer the fewest tests covering distinct risks, usually some combination of:

- One representative success case
- One important edge or failure case
- One regression case for a bug fix
- One boundary test when integration behavior changed

Avoid duplicative variants, exhaustive combinations without demonstrated risk, tests for unchanged
implementation details, trivial snapshots, and end-to-end tests when a cheaper level is sufficient.

- Unit test isolated logic.
- Integration test meaningful boundaries.
- End-to-end test only critical paths that cannot be validated more cheaply.
- Reuse existing fixtures and mocks.
- Do not build a new test harness for one simple case.
- Tests must not depend on execution order.
- When randomized ordering or data is used to expose isolation problems, report the seed.
- Control time, external I/O, and shared state so failures are reproducible.
- Tests should be easy to locate from the feature they protect. Follow the repository's established
  colocated or mirrored test-directory structure.

### TDD

Use test-first development when it clarifies non-trivial logic, reproduces a bug, or protects a
public contract. Strict Red → Green → Refactor is useful, not mandatory ceremony.

Implementation-first followed by focused verification is acceptable for straightforward wiring,
UI layout, configuration, scripts, exploratory spikes, and very small localized changes.

### Programmatic and Concise Output

Execute and evaluate tests programmatically through the project's runner or a deterministic
verification script.

- Use quiet or concise runner output by default.
- Produce one aggregate summary.
- Do not print one line for every passing test unless diagnosing order or isolation.
- Show details only for failed, errored, required-but-skipped, or incomplete tests.
- For each failure, include the test identifier and shortest useful explanation.
- Include only relevant assertion details, traceback frames, captured logs, or error context.
- Preserve full raw output as an artifact when useful, but summarize it in the main response.
- Failures must produce a non-zero exit status.

Preferred output:

```text
Tests: 4/5 passed, 1 failed.
Failed: test_name — expected X, got Y.
```

When all pass:

```text
Tests: 5/5 passed.
```

Add traceback or logs only when the concise failure message is insufficient.

### Stop Rule

Stop when the changed behavior is covered at the appropriate level, relevant tests pass, meaningful
failure paths are checked, and further tests would duplicate confidence rather than address a
specific risk.

Never describe targeted verification as a full-suite pass.

## Refactoring

Refactor when:

- The requested change cannot be implemented safely without it.
- Existing structure directly blocks locality or correctness.
- A module contains genuinely unrelated responsibilities.
- Real duplication creates maintenance risk.
- The user requested it.

Keep necessary refactoring local and explain why it is required. Propose broader refactors before
implementation, including their concrete benefit, cost, and risk.

Do not mix unrelated cleanup into feature work. Refactoring must preserve observable behavior.
When existing patterns conflict, choose the more established, recent, or well-tested one rather than
inventing a third blended pattern. Record consequential decisions in `PLAN.md` and useful historical
context in `DEVELOPMENT.md`.

## Git Discipline

- Initialize Git at the start of a new maintained project and maintain an appropriate `.gitignore`.
- Commit frequently at meaningful, completed, atomic milestones.
- A milestone should represent one coherent change that is useful to inspect, debug, revert, or
  bisect independently.
- Do not commit every edit, command, exploratory attempt, temporary state, or half-finished
  intermediate step merely to increase commit frequency.
- Avoid `wip` commits in normal development. If unfinished work must be preserved for an explicit
  handoff, label it clearly and do not present it as a stable milestone.
- Every normal commit should leave the affected project path runnable with relevant checks passing.
- Before every commit, review `PLAN.md` and `DEVELOPMENT.md`. Update either when needed and include
  those updates in the same commit. Do not create artificial documentation edits.
- Keep commits atomic: one logical concern per commit. Separate features, fixes, refactors, and
  unrelated cleanup when they can be reviewed or reverted independently.
- Use `type: short description`, such as `feat: add auth module`,
  `fix: handle null payment response`, or `refactor: split router`.
- Tag major stable milestones only when tags provide real release or rollback value.
- Do not push or publish commits unless requested or established by the repository workflow.

### Git Safety

The following require explicit approval and understanding:

- `git push --force`; prefer `--force-with-lease` for an approved history rewrite
- `git reset --hard` on a shared or remote-tracked branch
- `git rebase` on a branch already pushed and shared
- `git clean -fd` without first previewing with `git clean -nfd`
- `git stash drop` or `git stash clear` without verifying the contents are disposable

Do not commit directly to `main` or `master` in a multi-person project. Before destructive
operations, inspect the current branch, working tree, and recent history. Never discard unrelated
user changes.

## Code Quality and Naming

- Prefer self-explanatory names for variables, functions, methods, classes, interfaces, modules, and
  files.
- Avoid cryptic abbreviations, magic words, and single-letter names except for obvious, tightly
  scoped conventions such as loop indices.
- Follow established naming conventions unless they materially harm comprehension. Do not introduce
  additional cryptic names merely to imitate poor local examples.
- Clear names should reduce definition tracing during exploration and debugging.
- Apply the repository's established formatting, linting, and typing conventions.
- Keep secrets and environment-specific values in appropriate configuration.
- Local constants with clear ownership are acceptable.
- Centralize duplicated policy values.
- Do not leave dead code, commented-out implementation blocks, debug statements, or unexplained
  TODOs.

## Idempotency

Operations likely to be retried, resumed, replayed, or run during initialization should be
idempotent where practical.

This especially applies to setup scripts, migrations, initialization, file generation, retried
database writes, and external API requests.

For inherently non-idempotent operations such as payments, notifications, event appends, or counter
increments, use deduplication keys, transactional boundaries, or explicit safeguards when duplicate
execution is a realistic risk.

Do not add elaborate idempotency infrastructure to local one-shot operations with no realistic
replay path.

## LLM and Subagent Usage

Use an LLM for judgment-heavy tasks such as classification, drafting, summarization, extraction, and
interpretation. Use deterministic code for routing, retries, calculations, parsing, and transforms
that ordinary code can perform reliably.

### Subagents and Fresh Context

Each fresh agent must reconstruct context, reread files, and reconcile findings.

**Default subagent count is zero.** Use one only when a concrete benefit exceeds context
reconstruction and integration cost.

A subagent may be justified for:

- Genuinely independent parallel work with little overlap
- Specialist review of a high-risk area
- A large task with cleanly separable domains
- Fresh-context review when the current context is bloated, polluted, or losing attention

Do not spawn subagents to create the appearance of rigor, duplicate work, or perform routine
planning and review that the current agent can handle safely.

Before delegating:

- Define the exact question, relevant files, expected output, and benefit.
- Provide the smallest sufficient context.
- Prefer one well-scoped subagent over several overlapping reviewers.
- Do not ask it to reread the entire repository by default.
- Ensure relevant `PLAN.md` and `DEVELOPMENT.md` context is current; update them only when durable
  state or useful history actually changed.

Avoid recursive subagent chains and separate planning, implementation, and review agents for a task
one agent can understand safely. Parallelize only genuinely independent work. Treat subagent output
as advisory and verify important claims before changing code.

Stop delegating once the task has sufficient coverage and confidence.

## Token Efficiency and AI Maintainability

- Organize the repository for bounded context and local reasoning.
- Make directory and file names reveal feature ownership and purpose.
- Keep public APIs small and explicit.
- Keep helpers near the feature they support unless genuinely shared.
- Avoid hidden conventions and implicit magic.
- Add file-level descriptions only when they improve orientation.
- Explain non-obvious reasons and constraints, not obvious code.
- Use `PLAN.md` as the first-pass project context.
- Consult only relevant `DEVELOPMENT.md` history.
- Start with the target feature, direct callers, nearby tests, and directly used shared types.
- Avoid redundant tool calls and rereading unchanged files.
- Run targeted checks before broad suites.
- Keep plans, logs, test output, and completion reports concise.
- Stop when success criteria and focused verification are satisfied.

Token efficiency comes from locality, scope control, and proportional verification—not from
silently skipping correctness, security, data integrity, or interface work.

## Dependency Management

- Every dependency is a long-term liability; justify it before adding.
- Prefer small, focused, actively maintained libraries.
- Applications and deployments must be reproducible through an exact lockfile or equivalent.
- Reusable libraries may declare tested compatible ranges while locking exact versions in CI and
  development environments.
- Do not upgrade or replace dependencies as an unrelated side effect.

## Observability

- Maintained applications and services should use centrally configured structured logging behind a
  small reusable interface.
- Feature modules should not independently invent handlers, formats, destinations, or contextual
  field conventions.
- Small scripts may use the simplest appropriate reporting mechanism unless project logging already
  exists.
- Use log levels consistently.
- Production failures should include enough context to diagnose the operation.
- Never log passwords, tokens, secrets, or unnecessary personal data.

## Security

- Treat external input as untrusted at relevant boundaries.
- Validate input against expected shape and constraints.
- Use destination-appropriate protections: parameterized database queries, contextual output
  encoding or escaping, safe deserialization, and path normalization where relevant.
- Do not rely on a generic “sanitize everything” step.
- Perform authentication at clear system boundaries.
- Perform authorization where the information needed to decide access is available. Resource-level
  authorization may belong in application or domain services.
- Never place secrets in source control, logs, or error messages.
- Consider vulnerability classes relevant to the stack, including injection, insecure
  deserialization, path traversal, and broken access control.
- Security may justify a narrow scope expansion to avoid introducing or preserving a clear
  vulnerability. Surface serious pre-existing issues without silently turning a focused task into
  a broad security rewrite.

## Scalability

- Design for the next realistic order of magnitude, not the next ten.
- Avoid premature optimization.
- Measure before broad performance changes.
- Keep business logic independent from transport where this improves locality and reuse.
- Keep I/O and computation separate where this improves clarity or testability.
- Define shared data shapes once.
- Propose major caching, concurrency, queueing, sharding, or distributed-system changes before
  implementing them.

## Documentation

README should cover:

- What the project does
- Local setup
- How to run it
- How to run relevant tests
- Deployment when deployment is part of the project

Public interfaces should have concise descriptions of purpose and non-obvious behavior.

Use:

- `README.md` for user and operator guidance
- `PLAN.md` for durable architecture, current state, and next steps
- `DEVELOPMENT.md` for useful implementation and troubleshooting history
- `ARCHITECTURE.md` only when architecture is too substantial to remain concise in `PLAN.md`

Do not duplicate information already clear from code or another authoritative document.

## Completion Report

Keep the final report concise:

- What changed
- What was tested or checked
- Any limitation or unresolved issue
- Any necessary `PLAN.md` or `DEVELOPMENT.md` update
- At most one or two optional improvements, clearly separated from completed work

Do not narrate routine implementation steps or list every passing test.

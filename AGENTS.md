# AI Engineering Harness

This document governs how AI operates on this codebase. It is written for an AI agent as the
primary reader. Every rule anticipates a specific way AI goes wrong and installs a stop before it.
Apply the spirit of every rule regardless of stack, language, or framework.

---

## How to Read This Document

Rules are written as constraints on AI behaviour, not aspirations. When a rule says "stop," stop.
When it says "ask," ask. When it says "do not proceed," the correct response is never to proceed
quietly and hope it works out. Compliance is not optional and does not require a reminder.

---

## Part 1: Before Writing Any Code

### 1.1 Orient First

At the start of every session, before touching any file:

1. Check if `PLAN.md` exists at the project root.
2. If it exists, read it in full. Do not rely on context from a previous session — context resets.
   PLAN.md is the source of truth.
3. If it does not exist, this is a new project. Create PLAN.md before anything else (see Part 2).
4. State out loud what phase the project is in, what was last completed, and what the current task is.
   If any of this is unclear from PLAN.md, stop and ask before proceeding.

This step is mandatory. Skipping it and proceeding from memory or inference is a known AI failure
mode — context from a previous session is gone. Do not act as if it isn't.

### 1.2 Create PLAN.md Before Any Code

PLAN.md is the first commit on every project. Implementation does not begin until it exists and
has been committed.

PLAN.md must cover:

- **Objective**: one paragraph on what is being built and why
- **Architecture**: directory structure, key modules, data flow, interface contracts
- **Phases**: ordered milestones, each with explicit completion criteria
- **Open questions**: unresolved decisions with options and tradeoffs; remove when resolved
- **Out of scope**: what is explicitly excluded from this cycle
- **Current state**: active phase, last completed step, next action (kept updated throughout)
- **Issues & Fixes**: append-only log of problems encountered and resolved (see 2.3)

The directory structure decided in PLAN.md is final. It does not evolve organically. Structural
mistakes in early commits compound into every file that follows.

### 1.3 Decide the Structure Before Writing Files

Group by feature/domain, not by file type. Keep model, logic, and tests for a domain together.
A single canonical entry point must be obvious from the project root.

Do not create files outside the planned structure without updating PLAN.md first and stating why.

---

## Part 2: PLAN.md — Maintenance Rules

### 2.1 PLAN.md Is Always Current

PLAN.md is updated at the moment a change occurs — not at the end of a session, not on request,
not in a batch. This requires no reminder. It is part of every task.

- **Phase transitions**: mark the phase done, note any deviation from the original plan
- **Scope changes**: record the change and a one-line rationale inline
- **Architecture divergence**: if implementation differs from what was planned, update the
  architecture section and note why

PLAN.md updates are committed in the same commit as the code change that prompted them. A commit
that changes behaviour without updating PLAN.md is incomplete.

### 2.2 Current State Section

The **Current State** section of PLAN.md must always answer:

- What phase is active and what comes next
- What was the last completed step
- What is in progress and exactly where it stopped
- Any environment quirks, external dependencies, or decisions not obvious from the code

Any developer or AI reading PLAN.md must be able to resume work without asking questions.
If that is not true, PLAN.md is out of date and must be fixed before proceeding.

### 2.3 Issues & Fixes Log

Every non-trivial bug, unexpected behaviour, architectural correction, or hard-won discovery is
logged in the **Issues & Fixes** section of PLAN.md immediately when it is resolved.

Format — one block, four fields, no prose:

```
### [YYYY-MM-DD] Short title
- **Problem**: what broke or was wrong
- **Fix**: what was done
- **Affected**: files or modules touched
- **Watch out**: follow-on risks or related areas
```

The log is append-only. Entries are never edited or deleted.

---

## Part 3: AI Behaviour Constraints

These rules exist because AI fails in predictable ways. Each rule names the failure it prevents.

### 3.1 Stop Before Irreversible Actions

Before taking any action that cannot be easily undone, stop and confirm with the user.
Irreversible actions include:

- Deleting files or directories
- Mass renaming or restructuring across many files
- Overwriting existing logic with a rewrite
- Running destructive database or migration operations
- Any `git reset --hard`, `git clean`, or force-push

State what you are about to do, why, and what the effect will be. Do not proceed until confirmed.

> **The failure this prevents**: AI agents proceed confidently through destructive operations
> because nothing in the task description said "stop." The stop must be built in here.

### 3.2 Do Not Fix What You Were Not Asked to Fix

Work on exactly the scope given. Do not refactor adjacent code, rename things that "look wrong,"
restructure files that weren't mentioned, or improve things opportunistically.

If something genuinely needs fixing outside the current scope, note it in PLAN.md under a
**Deferred Work** section and leave it alone until it is explicitly assigned.

> **The failure this prevents**: AI agents routinely expand scope silently. Each "small improvement"
> adds untested surface area and obscures what actually changed.

### 3.3 Assume Nothing Carried Over from the Last Session

Context resets between sessions. Do not assume:

- That a decision made previously is still valid
- That partially written code from a prior session is complete or correct
- That a dependency was installed, a file was created, or a step was completed

Read PLAN.md. Check the filesystem. Verify state before acting on it.

> **The failure this prevents**: AI agents hallucinate continuity. They act as if they remember
> something they cannot actually access.

### 3.4 State Assumptions Before Acting on Them

When a requirement is unclear and asking would be excessive, make the simplest reasonable
assumption — and state it explicitly before proceeding:

`"Assuming X because Y — proceed unless this should be different."`

Never silently assume. Never build on an unstated assumption. If the assumption affects
architecture, data models, or external contracts, it warrants asking rather than assuming.

### 3.5 Ask Early, Ask Once

Before starting any non-trivial task, identify all blockers and open questions and surface them
in a single exchange. Do not start and then surface blockers one at a time as they are encountered.

**What warrants asking**: stack choices, data model design, API contracts, auth approach,
third-party service selection, any decision that would be expensive to reverse.

**What does not warrant asking**: internal naming, file structure details, implementation approach
for well-defined behaviour. Make a sensible call and proceed.

### 3.6 When Tests Fail Mid-Task, Stop

If a test that was passing begins to fail during a task:

1. Do not continue implementing other parts of the task
2. Do not commit
3. Diagnose the failure and fix it, or revert the change that caused it
4. Only continue once all previously passing tests are passing again

> **The failure this prevents**: AI agents proceed past failing tests, accumulate broken state,
> and produce a codebase where it is unclear what is broken or why.

### 3.7 Do Not Simulate TDD Compliance

TDD means writing a failing test before writing the implementation. It does not mean writing tests
after the fact that happen to pass. The sequence is:

1. Write the test — it must fail
2. Run it — confirm it fails
3. Write the minimum implementation to make it pass
4. Run it — confirm it passes
5. Refactor — confirm it still passes
6. Commit

Writing implementation first and then writing tests that validate it is not TDD. Do not describe
it as TDD.

> **The failure this prevents**: AI agents produce test-shaped code that validates their own
> output rather than specifying behaviour independently.

### 3.8 Do Not Silently Recover from Errors

When something fails — a command, a build, a test, an API call — surface it immediately.
Do not attempt a quiet workaround and continue as if nothing happened. Do not swallow an error
to keep momentum.

Report: what failed, what the error was, what was tried, what the options are.

---

## Part 4: Writing Code

### 4.1 One Responsibility Per File

Each module owns one responsibility and does it completely. If a file handles more than one
concern, it needs to be split. Target under 250 lines per file — a file should be fully
understandable in a single read.

### 4.2 Deep Modules, Simple Interfaces

Rich internal logic behind minimal, stable public APIs. Prefer fewer well-designed functions
over many shallow wrappers. Every module exposes a clear public interface. Internal helpers are
private by convention or language feature.

### 4.3 Locality of Change

A feature or fix should touch as few files as possible — ideally one. If a change ripples across
five or more files, the module boundaries are wrong. Fix the boundaries, not the ripple.

### 4.4 Low Coupling

Modules communicate through explicit interfaces, never by reaching into each other's internals.
Depend downward (on utilities and primitives), never sideways or upward. Circular dependencies
are forbidden.

### 4.5 Explicit Inputs and Outputs

No implicit global state. No hidden side effects. Inputs and outputs are typed or clearly
documented. A function that does two things should be two functions.

### 4.6 Fail Loudly and Early

Never silently swallow errors. Errors must carry enough context to identify root cause without a
debugger. Distinguish recoverable errors (handle gracefully) from unrecoverable ones (crash fast,
log everything). All external I/O has explicit error handling and timeouts.

### 4.7 One Error Handling Pattern

One strategy for the entire project, applied consistently. Never mix patterns.

---

## Part 5: Code Legibility for AI Readers

This section governs how code is written so that a future AI session can read and understand it
without needing external context. Apply these rules as a matter of output quality, not ceremony.

- Directory layout and filenames make the project's purpose obvious without reading code
- One-line comment at the top of each non-trivial file stating its role
- For complex logic, comment the *why*, not the *what*
- No hidden conventions or implicit magic — behaviour is obvious from the call site
- No dead code, commented-out blocks, or unexplained TODOs
- All config and secrets in environment variables or config files — never hardcoded
- Consistent naming conventions, decided before writing and applied uniformly throughout

---

## Part 6: Testing

### 6.1 Default to TDD

Write a failing test before implementing any non-trivial logic. See rule 3.7 for the required
sequence. A feature is not done until it is tested and passing.

### 6.2 Test at the Right Level

- Unit tests for isolated logic
- Integration tests for system boundaries
- End-to-end tests for critical paths only — they are slow and brittle; do not over-invest

### 6.3 Tests Must Be Deterministic

No randomness, time-dependent logic, or live external services in tests. Mock or stub all I/O
at the boundary.

### 6.4 Test Names Are Documentation

A failing test must describe exactly what broke without requiring the reader to read the body.
Name tests as sentences: `"returns 404 when user is not found"`, not `"test_user"`.

### 6.5 Coverage Is a Floor

Coverage is a minimum bar, not a goal. Focus on critical paths, edge cases, and failure modes.
A well-tested critical path is worth more than 100% coverage of trivial code.

### 6.6 Tests Live with the Code They Test

Tests are not in a distant folder. They live alongside the module they test.

---

## Part 7: Git Discipline

### 7.1 Initialize at Project Start

Initialize git before writing any code. Every project is version-controlled from the first file.
The first commit is PLAN.md and `.gitignore`. No exceptions.

### 7.2 .gitignore Before First Commit

At minimum exclude: secrets and `.env` files, build artifacts, compiled output, dependency
directories (`node_modules/`, `venv/`, etc.), editor and OS metadata.

### 7.3 The Commit Contract

Every commit leaves the project runnable with all tests passing. No exceptions. A commit that
breaks the build is not a valid commit.

### 7.4 Commit Format

`type: short description`

Examples: `feat: add auth module`, `fix: null response from payment API`, `refactor: split router`

No `wip`, `stuff`, `update`, `changes`, or `misc`. Each commit message describes the specific
change and its reason.

### 7.5 Atomic Commits

One logical change per commit. PLAN.md update goes in the same commit as the code change that
prompted it. Mixed concerns make reverts catastrophic.

### 7.6 Forbidden Commands

Do not run these without explicit approval and full understanding of the consequences:

- `git push --force` — destroys remote history. Use `--force-with-lease` only if fully understood,
  never on a shared branch.
- `git reset --hard` on a shared or remote-tracked branch — discards commits others may depend on.
- `git rebase` on any branch that has been pushed and shared — rewrites public history.
- `git clean -fd` without first running `git clean -nfd` to preview what will be deleted.
- `git stash drop` or `git stash clear` without confirming contents are no longer needed.

### 7.7 Branch Safety

Never commit directly to `main`/`master` in a multi-person project. Before any destructive
operation, confirm current branch with `git status` and `git log --oneline -10`. When in doubt,
create a backup branch: `git branch backup/<name>`.

### 7.8 Tag Stable Milestones

Tag MVP, feature-complete, and pre-deploy states for unambiguous rollback points.

---

## Part 8: Engineering Standards

### 8.1 Refactoring Discipline

Refactor as a dedicated step, never mixed into a feature or bug fix commit. Triggers: duplication
appearing a second time, a file approaching 250 lines, a change requiring too many files to touch.
Refactoring must leave observable behaviour identical — if tests change, something is wrong.

### 8.2 Idempotency

Operations that create, modify, or delete state must be safe to run more than once. File creation,
DB writes, and external API calls check for existing state before acting, or are structured to
produce the same result on repeat. Applies especially to setup scripts, migrations, and
initialization code.

### 8.3 Dependency Management

Pin to exact versions. Every dependency is a long-term liability — justify before adding. Prefer
small, focused libraries. Avoid abandoned or unvetted packages.

### 8.4 Observability

Structured logging from day one in a consistent, parseable format. No plain concatenated strings.
Log levels: DEBUG for noise, INFO for normal operations, WARN for degraded states, ERROR for
failures requiring attention. Never log passwords, tokens, or personal data.

### 8.5 Security Defaults

All external input is untrusted until validated and sanitized. Auth checks at the boundary, not
buried in business logic. Secrets never in source control, logs, or error messages. Default
failure modes for the stack in use: injection, insecure deserialization, path traversal, broken
access control — treat these as expected, not edge cases.

### 8.6 Scalability Defaults

Design for the next order of magnitude, not ten orders. Business logic has no knowledge of the
transport layer (HTTP, CLI, queue). Keep I/O and compute separate. Core types defined once,
imported everywhere.

---

## Part 9: Documentation

- **README** must cover: what it does, local setup, how to run tests, how to run or deploy.
  Nothing more required, nothing less acceptable.
- **Public interfaces** have a one-line description of purpose and any non-obvious behaviour.
- **Non-obvious architecture decisions** are recorded in the README or `ARCHITECTURE.md`.
- **PLAN.md** is not optional documentation — it is operational infrastructure. See Part 2.

---

## Definition of Done

A task is complete only when every item below is true. A task that skips any item is in progress,
not done.

- [ ] Implementation matches the stated scope — nothing more, nothing less
- [ ] Tests written before implementation (TDD), passing, and committed
- [ ] No new warnings, errors, dead code, debug statements, or hardcoded values
- [ ] All previously passing tests still pass
- [ ] PLAN.md updated: current state, any issues logged, phase status current
- [ ] README or interface docs updated if public behaviour changed
- [ ] Project is runnable from a clean checkout
- [ ] Committed with a meaningful message following the format in 7.4

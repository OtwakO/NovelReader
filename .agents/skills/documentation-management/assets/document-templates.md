# Minimal documentation templates

Use the smallest template that fits. Delete optional sections that add no retrieval value. Follow an existing repository convention when it is already coherent.

## PLAN.md

```markdown
# Project State

## Objective
Current purpose of the project.

## System Map
Major areas, responsibility, and links/paths to detail.

## Current State
Only project-level current truth.

## Active Work
- [Workstream](docs/plans/YYYY-MM-DD-topic.md) — short current status

## Immediate Priorities
1. ...

## Open Questions
Only project-level unresolved questions.

## Constraints
Only durable project-level constraints.
```

Keep `PLAN.md` a router. Detailed implementation steps belong in plans; detailed architecture belongs in architecture docs when needed.

## Implementation plan

```markdown
---
status: active
updated: YYYY-MM-DD
---

# Topic

## Goal
What done means.

## Scope
Included and excluded boundaries.

## Accepted Approach
The approach accepted for implementation.

## Decisions
### Decision title
**Decision:** ...
**Why:** ...
**Alternatives:** ...
**Revisit when:** ...

## Progress
- [x] Meaningful milestone
- [ ] Meaningful milestone

## Current State
What is true now. Name concrete files/modules/tests when useful.

## Next Action
The next meaningful action a fresh agent can take.

## Verification
Verified:
- ...

Still needed:
- ...

## Open Questions
Only unresolved questions that affect this workstream.
```

When complete, set `status: completed`, record the final outcome/verification, and normally stop maintaining the file.

## Scoped roadmap

```markdown
# Topic Roadmap

## Goal
Long-term desired outcome for this area.

## Now
Current direction. Link accepted work to plans.

## Next
Likely subsequent direction.

## Later
Tentative possibilities worth preserving.

## Constraints
Only constraints specific to this roadmap.
```

A roadmap describes direction, not implementation steps or completed history.

## Durable decision

```markdown
---
status: accepted
---

# Decision title

## Context
The durable problem/constraint that made a decision necessary.

## Decision
What is authoritative.

## Rationale
Why this choice was made.

## Alternatives
Only meaningful alternatives future maintainers might otherwise retry.

## Consequences
Material constraints/tradeoffs created by the decision.

## Revisit When
Conditions that should reopen the decision.
```

When superseded, keep the old record and link it to the replacement.

## Architecture document

```markdown
# Subsystem

## Responsibility
What this subsystem owns and does not own.

## Boundaries
Public entry points and dependencies on other areas.

## Data / Control Flow
Only the flow needed to understand the design.

## Invariants
Rules current implementations must preserve.

## Related Decisions
Links to durable rationale when needed.
```

Describe current design. Do not embed implementation history.

## Durable note

```markdown
# Finding

## Finding
The non-obvious fact.

## Impact
Why future work needs to know it.

## Evidence
How it was established, when useful.

## Watch Out
The condition under which this matters or can be removed.
```

Use only when the knowledge has no better canonical home.

## Runbook

```markdown
# Procedure

## When to Use
Trigger/incident this procedure handles.

## Preconditions
State/access required before starting.

## Steps
1. Action — completion criterion.
2. Action — completion criterion.

## Verification
How to know the procedure succeeded.

## Rollback / Recovery
Only when the procedure can fail in a recoverable way.
```

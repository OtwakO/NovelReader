# Documentation Index

Start with [`PLAN.md`](../PLAN.md) for current project state and routing. This index explains document roles; it is not a second project plan.

## Current documentation

- [`architecture/`](architecture/) — current subsystem design and behavioral contracts.
- [`decisions/`](decisions/) — durable cross-cutting rationale.
- [`plans/`](plans/) — substantial workstream state; completed plans remain frozen historical handoffs.
- [`roadmaps/`](roadmaps/) — proposed future direction that is not yet committed work.
- [`reference/`](reference/) — stable terminology and manually maintained reference material.
- [`research/`](research/) — durable investigative findings.
- [`runbooks/`](runbooks/) — operator and developer procedures.
- [`verification/`](verification/) — dated evidence and machine-readable records.

## Historical documentation

[`archive/`](archive/) contains non-authoritative audits, superseded designs, chronological logs, implementation checklists, and research snapshots. Read it only when current documents or Git history do not explain why a decision exists.

Historical documents carry an archive notice and should not be maintained as current truth.

## External behavioral evidence

Deterministic and live BookSource evidence lives under [`../testdata/booksource/`](../testdata/booksource/), not in this documentation tree. Reference implementations used for compatibility analysis live under `reference/` at repository root where present.

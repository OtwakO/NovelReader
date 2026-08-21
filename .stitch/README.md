# NovelReader Stitch Design References

This directory preserves the disposable design work that guided the first Vue production frontend. These files are historical visual and interaction references, not production source code and not authoritative backend behavior.

## Contents

### `01-generated-mobile-reference/`

The five original Chinese-first, mobile-first Stitch screens generated with Gemini 3.1 Pro:

- Bookshelf
- Discover and Search
- Book Detail
- Reader
- Source Switching (originally named Source Recovery)

Each screen includes the generated HTML and its screenshot. `metadata.json` records the Stitch project, design system, screen identifiers, and original dimensions.

### `02-interactive-prototype/runtime/`

The standalone in-memory prototype used for early interaction review and responsive exploration. It does not call NovelReader's backend and must not be treated as a production implementation.

Open `index.html` through a local static server if the prototype needs to be reviewed again.

### `02-interactive-prototype/evidence/`

Screenshots are grouped by the decision they supported:

- `initial-direction/` — first desktop/mobile and discovery concepts
- `desktop-mobile-refinement/` — library-first layout, accent, and health-banner iterations
- `explore-search/` — strict single-BookSource Explore and Search studies
- `responsive-audit/` — viewport and Book Detail geometry checks
- `production-alignment/` — later Shelf, TOC, cover, typography, and section-rhythm comparisons

## Authority and reuse

Use these artifacts to understand visual intent, rejected directions, and interaction history. Production behavior is determined by the active Vue frontend, backend contracts, tests, and `PLAN.md`. Future implementations may borrow patterns from these references but should not copy unsupported prototype behavior into production without product and backend approval.

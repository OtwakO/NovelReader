---
status: active
---

# Local Import and frontend presentation consistency

## Goal and Scope
A dedicated **Local import / 本地匯入** page, reached by one shelf button, with simple default operation and all existing fine controls accessible. Elsewhere, unify presentation only: typography tiers/weights, disclosures, controls and action alignment. Preserve palette, backend contracts, reader prose preferences and other screens' workflows.

## Accepted Approach
- Take interaction cues from Legado-E's separate local-book screen and the web reference apps' compact task controls; do not copy their frameworks or backend behavior.
- Establish shared typography tokens and reuse Search's bordered disclosure with right-aligned chevron. Apply to equivalent existing disclosures rather than layering global CSS overrides over conflicting local rules.
- Rework the dedicated import page around choosing files, a compact progress list and contextual review. Keep automatic acceptance and queue lifetime; the shelf no longer renders the workspace.
- Two logical commits: cross-app presentation foundation, then Local Import rework. This document records a concise change/revert map, not a file-by-file diary.

## Change / Revert Map
### Cross-app presentation foundation
- `frontend/src/ui/styles/tokens.css`: six size roles and two weights; local declarations now reference those roles. Large page/featured headings are quieter. User-selected prose size/spacing stays separate.
- `AppDisclosure`: shared Search-style border, chevron, keyboard/focus behavior and body padding, used by Search, targeted source search, backup API help and import disclosures.
- `FeatureScaffold`: consistent page title/description and action slot; removed the repeated default NovelReader eyebrow, preserving explicitly supplied context.
- `controls.css`: one button/link appearance and reusable action-row layout; matching action rows migrated across nine feature views. Palette and workflows unchanged.
- Maintenance contract: `frontend/src/ui/README.md`. Revert the foundation commit as a unit because local styles reference its shared tokens/components.

### Dedicated Local Import
Pending; separate commit from the foundation.

## Current State
Shared presentation foundation implemented. Inspected Legado-E local-book layout/menu, Rust web shelf toolbar, and packaged web-legado navigation/settings controls. Local Import rework remains pending.

## Next Action
Complete dedicated Local Import and localized navigation, then inspect desktop/mobile including representative existing screens. Update usage/architecture docs at the navigation cutover.

## Verification
Foundation: frontend typecheck, production build and all 68 test files / 256 tests passed before the final CSS action-row extraction. AFT inspection repeatedly timed out; it did not establish a clean diagnostic result. Final lint and desktop/mobile inspection remain pending. No existing reader data or deployment changes.

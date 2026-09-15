---
status: completed
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
### Cross-app presentation foundation — `502511d`
- `frontend/src/ui/styles/tokens.css`: six size roles and two weights; local declarations now reference those roles. Large page/featured headings are quieter. User-selected prose size/spacing stays separate.
- `AppDisclosure`: shared Search-style border, chevron, keyboard/focus behavior and body padding, used by Search, targeted source search, backup API help and import disclosures.
- `FeatureScaffold`: consistent page title/description and action slot; removed the repeated default NovelReader eyebrow, preserving explicitly supplied context.
- `controls.css`: one button/link appearance and reusable action-row layout; matching action rows migrated across nine feature views. Palette and workflows unchanged.
- Ordinary field values no longer inherit small/bold label typography; they use body size and regular weight.
- Maintenance contract: `frontend/src/ui/README.md`. Local Import depends on this foundation: its commit can be reverted alone, but a full foundation rollback should revert the dependent import rework first. Typography-only rollback can instead restore token/local CSS choices without removing shared components.

### Dedicated Local Import
Commit: `feat: move local imports to a dedicated workspace`. Shelf has only a Local Import navigation action; dedicated page owns upload/progress/inline review, vertical optional disclosures, and a return-to-shelf action. Labels are Local import / 本地匯入 / 本地导入. Shared button/link styling replaces import-specific button skins; fields and actions align consistently. Automatic approval, queue lifetime, historical review and safe reparse contracts are unchanged.

## Current State
Complete at the recorded scope: shared presentation and dedicated import navigation are implemented and verified. No backend, schema, dependency or deployment changes; existing user data remains untouched. Reference inputs were Legado-E's local-book layout/menu, Rust web shelf toolbar and packaged web-legado navigation/settings controls—not full frontend ports.

## Next Action
Manual usability feedback. No implementation remains pending in this accepted scope.

## Verification
- Final frontend suite: 68 files / 258 tests pass; production build including typecheck, changed-file ESLint and whitespace checks pass.
- Real server, fresh isolated epoch-12 home, no API mocks: shelf navigation, two automatic TXT additions, one inline review/addition, keyboard disclosure, fine-control visibility and revision-qualified reading passed. Historical imports and server-folder controls open on demand.
- Desktop 1280×900 and mobile 390×844: inspected Imports/review plus Shelf, Search, Explore, Sources, Settings, Backups and Account. No horizontal overflow or page errors. Page headings computed at 32/24 px and weight 600; shared actions at 14 px / 600 with minimum 44 px height. One correction batch fixed field inheritance; final confirmation showed ordinary fields at 16 px / 400 on Settings, Account, Imports and Search.
- AFT inspection timed out and established no clean diagnostic result; compiler/tests were authoritative. Backend suites, hosted CI, deployment, every dialog/data variant and WebView/live-source behavior were not verified in this frontend-only pass.

---
status: completed
updated: 2026-09-16
---

# Reader-first presentation

## Goal
Unify the existing parchment interface with clearer typography, navigation and task hierarchy without changing reading or storage contracts.

## Scope and accepted approach
- Keep current/system fonts and palette. Refine shared size/weight/line-height roles; preserve reader prose preferences.
- Reuse AppIcon for meaningful navigation icons. Keep labels and native link semantics.
- Put everyday Settings before optional WebView diagnostics using AppDisclosure.
- Theme native controls, reduce destructive-action emphasis before confirmation, clarify reparse/settings language, and add useful search guidance from existing state.
- No new dependencies, services, fetching architecture, or reader data changes.

## Current State
Implemented and verified. Shared typography/control and navigation checkpoint: `6af32e9`. Reader-first Settings, clearer task wording/search guidance and quieter pre-confirmation removal are recorded in `feat: make settings and search reader focused`. No new packages or backend changes. Settings template formatting makes the reordered groups explicit.

## Next Action
No implementation pending in this scope. Manual usability feedback is next; retain current fonts and user-controlled reader prose.

## Verification
- Affected boundaries: 12 test files / 40 tests passed. Full frontend: 70 files / 264 tests passed after shared CSS changes.
- Final production build/vue-tsc, scoped ESLint and whitespace checks passed. AFT inspection timed out; no authoritative AFT result claimed.
- Synthetic production Chromium checks at 1280px and 390px: shelf/detail/reparse, Settings and search, labeled SVG navigation, no horizontal overflow or page errors; diagnostics make no request until opened and native keyboard range input works with themed accent.
- Mechanical design scan of shell/settings/search returned no findings. No backend suite, real-source audit, screen-reader or cross-browser certification; existing data untouched.

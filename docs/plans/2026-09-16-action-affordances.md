---
status: completed
---
# Frontend action affordances

## Goal
Make actions recognizable and aligned without changing the calm parchment identity or application workflows.

## Scope
Shared button appearance; book-detail/reparse action placement; preview resume selection and full-height desktop prose sample; action-style navigation on shelf/Explore; centered locale selection; desktop PWA brand asset. No backend/data changes, new dependencies, animation system or broad page redesign.

## Accepted Approach
- Filled accent for the main action, outlined parchment for secondary actions, a lighter but visible outline for quiet actions. Keep genuine contextual links, chapter links and menu items distinct from action buttons.
- Preserve RouterLink navigation semantics while using shared button classes for action destinations.
- Move TXT reparse into the chapter section header via a presentation slot; align preview titles/actions instead of applying button margins.
- Promote the existing reader SVG icon component to shared UI and add only reading/selection symbols. Icons accompany text only where they aid recognition; no emoji or icon-everywhere treatment.
- Reuse the existing PWA icon. Use CSS for centering and responsive alignment, not layout listeners.

## Current State
Implementation complete. Shared controls now retain visible outlines; action navigation reuses their appearance. The existing reader SVGs were promoted to AppIcon with reading/selection additions. Reparse is in the chapter header; preview actions are aligned rows with selected feedback; desktop prose grows into the pane rather than stopping at an independent height cap. Locale and PWA branding updated. Desktop/mobile confirmation passes. Resume buttons reserve selection-icon space, and the centered locale label keeps its native arrow in normal flow. No implementation remains pending.

## Next Action
Manual usability feedback. Revert/reference boundary: `fix: unify frontend action affordances and preview alignment`; shared appearance/icon ownership and the named feature-local placements are one presentation-only change.

## Verification
- Full frontend suite: 68 files / 261 tests passed, including new detail-header and resume-selection regressions. Final production build/typecheck, scoped ESLint and whitespace checks passed after the CSS correction.
- AFT inspections failed to complete (initial transport timeout, final scoped rescan timeout); no clean AFT result is claimed. Compiler/tests are the successful verification gates.
- Chromium production-build checks with synthetic API fixtures (not a live backend): detail → reparse navigation, selected resume feedback, shelf actions and PWA icon loading pass at 1280×900 and 390×844. No horizontal overflow, unexpected API calls or page errors in the final run.
- Preview sample fills the desktop pane (251px scroll area within a 355px pane), with only the intended 16px bottom padding after its note. Mobile prose stays bounded at 288px. Resume button top margin is 0px. Locale text is centered within 0px measured delta; the arrow remains visible and native keyboard selection successfully switches English/Traditional Chinese.
- No existing reader home or backend changed. No low-end-device benchmark or non-Chromium run; implementation adds no dependency, motion, polling or positioning listeners. Existing reader icons were shared rather than duplicated.

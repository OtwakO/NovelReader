---
status: completed
---

# Import layout and selector refinement

## Goal and Scope
Make import tasks understandable at a glance, and correct shared dropdown sizing/viewport behavior. Preserve the current theme, workflows and fine controls; no backend changes or new dependencies.

## Accepted Approach
- Distinguish chapter navigation from prose with a structured two-pane preview, stacked on mobile.
- Separate server-file selection from leftover recovery; group directory guidance, list controls and empty states by task.
- Remove the routine ready/refresh row in completed reviews; retain refresh for pending work or request recovery.
- Fix browser-owned select presentation in shared CSS: readable widths, single-line truncated outliers, viewport-aware flipping and bounded scrolling. Keep native fallbacks; no JS positioning loop.
- Two reviewable commits: shared selector/disclosure correction, then import layout. The shared commit changes native select display markup, Vue compiler recognition, picker CSS and open disclosure headers across existing screens; no workflows change. The import commit owns panel grouping, status-row removal and field/row layout. Each can be reverted independently. No palette or whole-app redesign.

## Commit boundaries
- `4a35fa8` — shared selectors/disclosure presentation and native markup only.
- `feat: distinguish import review and inbox workspaces` — import-specific layout and review-status behavior.

## Current State
Implementation and confirmation complete. Chapter/prose and file-selection/recovery panes are distinct; the review refresh action remains for pending work and failed requests only. Low-specificity form-label defaults no longer override checkbox-row layout. Shared single-value selects use native `button/selectedcontent` markup, readable widths, ellipsis and available-space positioning. The Vue nesting-validator workaround and native fallback contract live in `frontend/src/ui/README.md`. No runtime component, positioning listener, animation loop, dependency or backend change was added.

## Next Action
Manual usability feedback. No implementation remains pending in this accepted scope.

## Verification
- Full frontend run: 68 files / 259 tests passed. After the native-markup compatibility correction, 12 affected files / 50 tests passed. Final production build/typecheck, scoped ESLint and whitespace checks pass without warnings.
- Fresh isolated real-server home, production frontend, no API mocks: browser upload/review, history, empty/populated inbox and row selection verified at 1280×900 and 390×844. No horizontal overflow or page errors. Status options were single-line, 44px high and 414/290px wide.
- Native locale keyboard selection updates Vue state; the sidebar picker opens above its trigger. A separate native-control stress fixture using shipped CSS verifies long selected-label ellipsis with a retained arrow and a 30-option scrollable menu at 390×360: menu y=4–156, trigger y=160–204, no overlap/viewport overflow. Ordinary native appearance fallback was exercised in Chromium 148; other browser engines were not run.
- AFT inspection timed out and established no clean diagnostic result. No backend suite, deployment or load benchmark was run. Existing user data remains untouched; only synthetic files in an isolated temporary home were used.

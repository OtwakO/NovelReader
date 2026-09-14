---
status: completed
updated: 2026-09-15
---
# Simple TXT import experience

## Goal
Choose files → automatic processing → read. Remove technical navigation and visual clutter while preserving the existing theme and storage safety.

## Accepted approach
User explicitly approved automatic library admission for newly selected files that finish without review warnings, plus a simpler, consistent visual design. Start from the shelf; show progress and exceptions in the same surface. Keep chapter options behind progressive disclosure and handle review inline. Server inbox/history remain secondary. No backend, schema, dependency or unrelated-page redesign.

## Decisions
- Extend the existing feature-owned queue, not a second workflow framework: byte transfer stays serial; bounded status checks and admission overlap analysis and survive in-app navigation.
- Automatic approval belongs only to the exact initial interpretation of newly selected files. Never sweep old receipts into the library, approve a replacement generation, or replay an uncertain acquisition/admission. Warnings/failures require explicit inline action.
- Browser reload still drops unsent File references and this tab's automatic-admission intent. Acquired receipts remain available for recovery; older ready files require explicit Add. Logout/account switch/restore cancel both queue activities.
- Share one import surface between Shelf and Imports. Existing review URLs remain compatible; ordinary actions do not navigate through review pages. Reuse theme tokens/components; no new visual identity.

## Current state
Implemented and verified: warning-free initial generations are added automatically; one shared Shelf/Imports surface keeps review inline, advances directly to current-resume reading, and hides optional controls behind disclosures. Mobile rows are compact and Add book is prominent in the review header. No backend, schema, dependency, deployment or existing-data changes. This is an import-flow improvement, not a redesign of every app screen.

## Next action
Manual usability testing and user feedback. No implementation is pending in this accepted import-flow scope; this plan is now historical evidence.

## Verification
- Automatic-admission regressions were red before implementation. Final focused Vitest: 40 tests across eight files pass, including navigation/reset, exact-generation approval, no uncertain replay, shelf integration, inline review, router and i18n. Typecheck, production build and scoped ESLint pass.
- One desktop/mobile inspection batch and one post-correction confirmation passed against a real server and isolated temporary epoch-12 home, without API mocks. Browser File objects exercised normal uploads: two headed books automatically added, one unheaded book reviewed/added inline, literal prose opened directly in the revision-qualified reader. No required page changes during import/review or horizontal overflow at 1280×900 and 390×844.
- AFT completed but lacks authoritative Vue diagnostic reports; compiler/tests are the verification authority. Backend suites were not rerun (no backend code changed); the normal server binary was built and used for real HTTP verification. Hosted CI and broader UX usability testing were not performed.

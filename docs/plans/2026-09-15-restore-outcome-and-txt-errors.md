---
status: active
updated: 2026-09-15
---
# Restore outcome recovery and TXT failure guidance

## Goal / Scope
Correct two confirmed branch-review findings: old client work survives an uncertain restore response, and TXT analysis failures lose actionable classification. Existing reader data is untouched; tests use synthetic isolated homes.

## Accepted approach
- User chose tracked restore outcomes over manual-only verification. Extend the existing process-owned restore operation with prepared/committing/committed/failed status. Keep one operation per reader, expire terminal records, never expire or delete live commit staging. Status recovery never repeats replacement. No database schema or archive change, durable operation ledger, or new worker.
- Retire old reader requests and feature state before commit. Suspend reader-home requests while outcome is unresolved; keep restore control requests available. A reader-bound tab marker routes reload/navigation back to recovery. A missing process-local record means unknown, not success or failure; require explicit acknowledgement and fresh state rather than replay.
- Preserve sanitized TXT failure categories at the analysis boundary using existing error storage; share guidance between initial import and reparse. Old raw errors have a safe generic fallback, never string-based classification or API disclosure.

## Compatibility / rollback
Restore status adds fields to the existing authenticated GET resource. Old clients still receive ordinary commit responses. Outcome evidence is process-local and can expire or be superseded by a new preparation; no outcome is inferred from 404. Code revert needs no data migration. Analysis codes use existing storage; no epoch cutover.

## Current state
Both fixes are implemented. Restore outcomes retain live lifecycle ownership and terminal evidence; the initiating tab resets and blocks reader work before commit, recovers across navigation/reload, and does not replay uncertain operations. A prepared status must be explicitly canceled before releasing the barrier because an earlier POST could still arrive. Old-home cleanup failure after publication is a typed committed warning, not a failed replacement. Analysis stores typed, safe failure codes and both review views share localized guidance. No reader schema, archive format, migration, dependency, deployment or existing-data changes.

## Next action
Commit the verified restore correction, then the independently verified analysis-error correction with its architecture/usage note. No further implementation is pending.

## Verification
- Red: `BackupRestoreBoundary.test.ts` failed all three pre-fix assertions; the real backup service could not retrieve status after a successful replacement.
- Green: original regressions and focused frontend restore/transport/router checks pass (23 tests across seven files), with typecheck and scoped lint. Backend normal and race checks pass for backup, readerstore, API and auth; TXT parser/store/worker race checks also pass. Unix cleanup-failure regression runs as a non-root user.
- Final full frontend suite: 249 tests across 65 files pass. Production build (including typecheck) and scoped ESLint pass. Final normal Go verification passes for backup, readerstore, API, TXT parser/store/worker, auth and cmd/server (eight packages).
- No browser fault-injection run, Windows permission-path run or deployment performed. AFT inspection failed acquiring its metrics-cache writer lease; compiler/tests are the verification authority.

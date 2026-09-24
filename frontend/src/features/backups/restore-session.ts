// A tab's unresolved restore is reader-bound and survives navigation/reload.
// This is recovery intent, not evidence that the server committed anything.
const key = 'novelreader.pending-restore';
interface PendingRestore { readerId: string; operationId: string }
let pending: PendingRestore | undefined;

export function pendingRestore(readerId: string): string | undefined {
  if (!pending) {
    try {
      const stored = JSON.parse(sessionStorage.getItem(key) || 'null') as PendingRestore | null;
      if (stored && typeof stored.readerId === 'string' && typeof stored.operationId === 'string') pending = stored;
    } catch { /* No recoverable marker; do not infer a server outcome. */ }
  }
  return pending?.readerId === readerId ? pending.operationId : undefined;
}

export function rememberRestore(readerId: string, operationId: string) {
  // Failure to retain recovery intent stops commit before any server mutation.
  sessionStorage.setItem(key, JSON.stringify({ readerId, operationId }));
  pending = { readerId, operationId };
}

export function forgetRestore() {
  sessionStorage.removeItem(key);
  pending = undefined;
}

import { ApiError } from '../../api/transport';

export function importErrorKey(cause: unknown): string {
  if (!(cause instanceof ApiError)) return 'imports.errors.request';
  const keys: Record<string, string> = {
    txt_invalid_input: 'invalid', txt_too_large: 'size', txt_state_changed: 'changed',
    txt_receipt_not_found: 'missing', txt_ticket_not_found: 'uncertain', txt_interrupted: 'uncertain',
    txt_inbox_pending: 'claim', txt_inbox_changed: 'proof', txt_inbox_review_expired: 'proof',
    txt_intake_busy: 'busy', txt_inbox_busy: 'busy', txt_inbox_review_limit: 'proofLimit',
    txt_intake_unavailable: 'unavailable', txt_inbox_missing: 'inboxMissing', txt_storage_error: 'request',
  };
  return `imports.errors.${keys[cause.code] || 'request'}`;
}

export function importedTitle(filename: string): string { return filename.replace(/\.txt$/i, '').trim() || filename; }

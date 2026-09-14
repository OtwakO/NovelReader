import { ApiError } from '../../api/transport';

export function importErrorKey(cause: unknown): string {
  if (!(cause instanceof ApiError)) return 'imports.errors.request';
  if (cause.code === 'state_changed') return 'imports.reparse.stateChanged';
  if (cause.code === 'txt_resume_required' || cause.code === 'txt_invalid_resume') return 'imports.reparse.chooseResume';
  const keys: Record<string, string> = {
    txt_invalid_input: 'invalid', txt_invalid_pattern: 'pattern', txt_too_large: 'size', txt_state_changed: 'changed',
    txt_receipt_not_found: 'missing', txt_ticket_not_found: 'uncertain', txt_interrupted: 'uncertain',
    txt_inbox_pending: 'claim', txt_inbox_changed: 'proof', txt_inbox_review_expired: 'proof',
    txt_intake_busy: 'busy', txt_inbox_busy: 'busy', txt_inbox_review_limit: 'proofLimit',
    txt_intake_unavailable: 'unavailable', txt_inbox_missing: 'inboxMissing', txt_storage_error: 'request',
  };
  return `imports.errors.${keys[cause.code] || 'request'}`;
}

export function importedTitle(filename: string): string { return filename.replace(/\.txt$/i, '').trim() || filename; }

export function analysisErrorKey(code?: string): string {
  switch (code) {
    case 'txt_encoding_required': return 'imports.analysisErrors.encodingRequired';
    case 'txt_invalid_encoding': return 'imports.analysisErrors.invalidEncoding';
    case 'txt_unsupported_encoding': return 'imports.analysisErrors.unsupportedEncoding';
    case 'txt_no_readable_text': return 'imports.analysisErrors.noReadableText';
    case 'txt_non_text': return 'imports.analysisErrors.nonText';
    case 'txt_section_limit': return 'imports.analysisErrors.sectionLimit';
    case 'txt_storage_error': return 'imports.analysisErrors.storage';
    default: return 'imports.analysisErrors.generic';
  }
}

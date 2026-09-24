import { acceptTXT, getTXTReceipt, type TXTReceipt, type TXTState } from '../../api/txt-imports';
import { acceptEPUB, getEPUBReceipt, previewEPUB, type EPUBReceipt } from '../../api/epub-imports';
import type { ImportFormat } from '../../api/file-imports';
import { ApiError } from '../../api/transport';
import { importedTitle } from './import-feedback';

export type ImportReceipt = TXTReceipt | EPUBReceipt;
export const isEPUBReceipt = (receipt: ImportReceipt): receipt is EPUBReceipt => 'generation' in receipt;
export const receiptFormat = (receipt: ImportReceipt): ImportFormat => isEPUBReceipt(receipt) ? 'epub' : 'txt';
export const receiptGeneration = (receipt: ImportReceipt): number => isEPUBReceipt(receipt) ? receipt.generation : receipt.analysisVersion;

// Shared UI status is a projection, not a replacement for either wire contract.
export function receiptState(receipt: ImportReceipt): TXTState {
  if (!isEPUBReceipt(receipt)) return receipt.state;
  if (receipt.state === 'removing' || receipt.state === 'failed') return receipt.state;
  if (receipt.libraryId) return 'published';
  if (receipt.state !== 'acquired') return 'receiving';
  switch (receipt.preparationState) {
    case 'ready': return 'ready';
    case 'failed': return 'analysis_failed';
    case 'preparing': case 'finalizing': return 'analyzing';
    default: return 'received';
  }
}
export const getImportReceipt = (format: ImportFormat, id: string, signal: AbortSignal): Promise<ImportReceipt> =>
  format === 'epub' ? getEPUBReceipt(id, signal) : getTXTReceipt(id, signal);

// Only a ready EPUB needs the saved summary. Polling must remain metadata-only.
// Both queue auto-add and history bulk-add enforce this same content-review gate.
export async function additionDetails(receipt: ImportReceipt, signal: AbortSignal) {
  if (!isEPUBReceipt(receipt)) return { name: importedTitle(receipt.originalName), author: '', needsReview: receipt.state === 'needs_review', notices: [] as string[] };
  const preview = await previewEPUB(receipt.id, receipt.generation, 0, signal);
  signal.throwIfAborted();
  if (preview.generation !== receipt.generation) throw new ApiError(409, { code: 'epub_state_changed' });
  return { name: preview.title.trim() || importedTitle(receipt.originalName), author: preview.authors.join(', '), needsReview: preview.needsReview, notices: preview.notices || [] };
}
export function acceptImport(receipt: ImportReceipt, details: { name: string; author: string }, signal: AbortSignal) {
  return isEPUBReceipt(receipt)
    ? acceptEPUB(receipt.id, receipt.generation, details.name, details.author, signal)
    : acceptTXT(receipt.id, receipt.analysisVersion, details.name, details.author, signal);
}

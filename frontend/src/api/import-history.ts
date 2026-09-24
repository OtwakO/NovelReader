import { importControl, type ImportFormat } from './file-imports';
import type { TXTReceipt } from './txt-imports';
import type { EPUBReceipt } from './epub-imports';

export const historyStatuses = ['processing', 'ready', 'needs_review', 'failed', 'added', 'removing'] as const;
export type HistoryStatus = typeof historyStatuses[number];
export interface ImportHistoryItem {
  format: ImportFormat;
  status: HistoryStatus;
  receipt: TXTReceipt | EPUBReceipt;
}
export interface ImportHistoryPage { items: ImportHistoryItem[]; nextCursor?: string }

export const listImportHistory = (format: ImportFormat | '', status: HistoryStatus | '', before: string, signal: AbortSignal) =>
  importControl<ImportHistoryPage>(`/imports/receipts?${new URLSearchParams({ format, status, before, limit: '25' })}`, signal);

import { request } from './transport';

export interface BookSource {
  sourceId?: string; bookSourceUrl: string; bookSourceName: string; bookSourceGroup?: string; bookSourceType?: number; enabled: boolean; enabledExplore: boolean; collectionId?: string;
  searchUrl?: string; ruleSearch?: unknown; ruleBookInfo?: unknown; ruleToc?: unknown; ruleContent?: unknown; header?: string;
  [key: string]: unknown;
}

export function listSources() { return request<BookSource[]>('/sources'); }
export function importSources(data: string) { return request<{ imported: number; total: number }>('/sources', { method: 'POST', body: data }); }
export function updateSource(sourceId: string, source: BookSource) { return request<BookSource>(`/sources?id=${encodeURIComponent(sourceId)}`, { method: 'PUT', body: JSON.stringify(source) }); }
export function deleteSource(sourceId: string) { return request<{ status: string }>(`/sources?id=${encodeURIComponent(sourceId)}`, { method: 'DELETE' }); }

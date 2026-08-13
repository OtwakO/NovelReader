import { request } from './transport';

export interface BookSource {
  bookSourceUrl: string; bookSourceName: string; bookSourceGroup?: string; enabled: boolean; enabledExplore: boolean;
  searchUrl?: string; ruleSearch?: string; ruleBookInfo?: string; ruleToc?: string; ruleContent?: string; header?: string;
  [key: string]: unknown;
}

export function listSources() { return request<BookSource[]>('/sources'); }
export function importSources(data: string) { return request<{ imported: number; total: number }>('/sources', { method: 'POST', body: data }); }
export function deleteSource(url: string) { return request<{ status: string }>(`/sources?url=${encodeURIComponent(url)}`, { method: 'DELETE' }); }

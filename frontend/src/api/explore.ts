import type { SearchResult } from './models';
import { request } from './transport';

export interface ExploreDiagnostic { code: string; stage: string; severity: string; retryable: boolean; message: string }
export interface ExploreSource { id: string; name: string; group: string }
export interface ExploreEntry { id: string; title: string; type: 'url' | 'text' | 'button' | 'toggle' | 'select' | string; selectable: boolean; value?: string; options?: string[] }
export interface ExploreCatalog { source: ExploreSource; sessionId: string; entries: ExploreEntry[]; diagnostics: ExploreDiagnostic[] }
export interface ExplorePageResult { sourceId: string; sessionId: string; categoryId: string; page: number; nextPage: number; books: SearchResult[]; exhausted: boolean; diagnostics: ExploreDiagnostic[] }

export function listExploreSources(signal?: AbortSignal) { return request<ExploreSource[]>('/explore/sources', { signal }, 'explore'); }
export function openExplore(sourceId: string, signal?: AbortSignal) { return request<ExploreCatalog>('/explore/catalog', { method: 'POST', body: JSON.stringify({ sourceId }), signal }, 'explore'); }
export function updateExploreControl(sessionId: string, controlId: string, value: string | null, signal?: AbortSignal) { return request<ExploreCatalog>('/explore/control', { method: 'POST', body: JSON.stringify({ sessionId, controlId, value }), signal }, 'explore'); }
export function getExplorePage(sessionId: string, categoryId: string, page: number, signal?: AbortSignal) { return request<ExplorePageResult>('/explore/page', { method: 'POST', body: JSON.stringify({ sessionId, categoryId, page }), signal }, 'explore'); }

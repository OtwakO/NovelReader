import type { SearchResult } from '../../api/search';

const prefix = 'novelreader.search-preview:';
export function createPreviewKey() { return crypto.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`; }
export function savePreviewSelection(key: string, result: SearchResult) { sessionStorage.setItem(prefix + key, JSON.stringify(result)); }
export function loadPreviewSelection(key: string): SearchResult | null {
  if (!key) return null;
  try {
    const value = JSON.parse(sessionStorage.getItem(prefix + key) || 'null') as Partial<SearchResult> | null;
    if (!value || typeof value.name !== 'string' || typeof value.sourceUrl !== 'string' || typeof value.bookUrl !== 'string') return null;
    return value as SearchResult;
  } catch { return null; }
}

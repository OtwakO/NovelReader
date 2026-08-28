import type { SearchResult } from '../../api/search';

const prefix = 'novelreader.candidate-selection:';
export function createCandidateSelectionKey() { return crypto.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`; }
export function saveCandidateSelection(key: string, result: SearchResult) { sessionStorage.setItem(prefix + key, JSON.stringify(result)); }
export function loadCandidateSelection(key: string): SearchResult | null {
  if (!key) return null;
  try {
    const value = JSON.parse(sessionStorage.getItem(prefix + key) || 'null') as Partial<SearchResult> | null;
    if (!value || typeof value.name !== 'string' || typeof value.sourceId !== 'string' || typeof value.sourceUrl !== 'string' || typeof value.bookUrl !== 'string') return null;
    return value as SearchResult;
  } catch { return null; }
}

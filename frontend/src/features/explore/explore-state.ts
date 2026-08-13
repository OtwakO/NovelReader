import type { ExploreApiError } from '../../api/transport';
import type { ExploreEntry } from '../../api/explore';
import type { SearchResult } from '../../api/models';

export interface CachedExplorePage { results: SearchResult[]; nextPage: number; exhausted: boolean }
export type ExploreRecovery = { kind: 'reopen' } | { kind: 'page'; page: number } | { kind: 'retry' } | { kind: 'stop' };

export function selectedCategoryAfterRefresh(selectedId: string, entries: ExploreEntry[]): string {
  return entries.some((entry) => entry.id === selectedId && entry.selectable) ? selectedId : '';
}

export function categorySelection(currentId: string, nextId: string, cache: Record<string, CachedExplorePage>): { kind: 'current' } | { kind: 'cached'; state: CachedExplorePage } | { kind: 'load' } {
  if (currentId === nextId) return { kind: 'current' };
  return cache[nextId] ? { kind: 'cached', state: cache[nextId] } : { kind: 'load' };
}

export function classifyExploreError(error: ExploreApiError | null): ExploreRecovery {
  if (error?.code === 'session_not_found' || error?.code === 'invalid_session') return { kind: 'reopen' };
  if (error?.code === 'page_conflict' && Number.isInteger(error.nextPage) && Number(error.nextPage) > 0) return { kind: 'page', page: Number(error.nextPage) };
  return { kind: error?.retryable ? 'retry' : 'stop' };
}

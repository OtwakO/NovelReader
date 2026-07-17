// Explore recovery helpers keep opaque catalog IDs and server paging authoritative.
export function selectedCategoryAfterRefresh(selectedId, entries) {
  return entries.some((entry) => entry.id === selectedId && entry.selectable) ? selectedId : '';
}

export function categorySelection(currentId, nextId, cache) {
  if (currentId === nextId) return { kind: 'current' };
  return cache[nextId] ? { kind: 'cached', state: cache[nextId] } : { kind: 'load' };
}

export function classifyExploreError(error) {
  if (error?.code === 'session_not_found' || error?.code === 'invalid_session') {
    return { kind: 'reopen' };
  }
  if (error?.code === 'page_conflict' && Number.isInteger(error.nextPage) && error.nextPage > 0) {
    return { kind: 'page', page: error.nextPage };
  }
  return { kind: error?.retryable ? 'retry' : 'stop' };
}

import type { Pinia } from 'pinia';
import { watch } from 'vue';
import { resetReaderRequests } from '../api/transport';
import { useSessionStore } from '../stores/session';
import { useSearchStore } from '../features/search/search-store';
import { useExploreStore } from '../features/explore/explore-store';
import { clearCandidateOperations } from '../features/candidates/candidate-operation';
import { clearCandidateSelections } from '../features/search/candidate-selection';
import { resetProgressWriter } from '../features/reader/progress-writer';

const ownerKey = 'novelreader.reader-state-owner';

// The application owns identity transitions; features own their reset semantics.
// Remembering the owner preserves tab restoration for the same reader on reload.
export function installReaderStateBoundary(pinia: Pinia) {
  const session = useSessionStore(pinia);
  let previous: string | null | undefined;
  return watch(() => [session.account?.id ?? null, session.phase] as const, ([reader, phase]) => {
    if (!reader && previous === undefined && !['guest', 'setup', 'setup-unavailable'].includes(phase)) return;
    if (reader === previous) return;
    let storedOwner: string | null = null;
    try { storedOwner = sessionStorage.getItem(ownerKey); } catch { /* no restoration when storage is disabled */ }
    if (!reader || reader !== storedOwner || previous !== undefined) {
      resetReaderRequests();
      useSearchStore(pinia).resetReaderState();
      useExploreStore(pinia).resetReaderState();
      clearCandidateOperations();
      clearCandidateSelections();
      resetProgressWriter();
    }
    previous = reader;
    try {
      if (reader) sessionStorage.setItem(ownerKey, reader);
      else sessionStorage.removeItem(ownerKey);
    } catch { /* next startup safely drops state if its owner cannot be recorded */ }
  }, { immediate: true, flush: 'sync' });
}

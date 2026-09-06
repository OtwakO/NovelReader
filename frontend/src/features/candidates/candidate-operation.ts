import type { AltSource, Book } from '../../api/models';
import { readerRequestSignal, request } from '../../api/transport';
import { normalizedBookIdentity } from '../books/book-identity';

export interface BookCandidate {
  variableMap?: string;
  name: string; author?: string; coverUrl?: string; coverDisplayUrl?: string; intro?: string; kind?: string; lastChapter?: string;
  updateTime?: string; wordCount?: string; sourceName?: string; sourceGroup?: string; capabilities?: string[]; sourceId: string; sourceUrl: string; bookUrl: string; alternateSources?: AltSource[];
}

export interface CandidateSelection {
  requestedSourceId: string;
  selectedSourceId: string;
  requestedSourceUrl: string;
  selectedSourceUrl: string;
  selectedSourceName?: string;
  usedFallback: boolean;
}

export type CandidateOperationState = 'running' | 'verified' | 'committed' | 'exhausted' | 'cancelled' | 'failed';
export type CandidateOperationStage = 'book_info';

export interface CandidateOperationAttempt {
  variableMap?: string;
  sourceName?: string;
  sourceId: string;
  sourceUrl: string;
  bookUrl?: string;
  stage?: CandidateOperationStage;
  state: 'queued' | 'running' | 'ready' | 'failed' | 'verified' | 'skipped';
  reason?: string;
}

export interface CandidateOperationPreview {
  book: Omit<Book, 'id' | 'durChapterIndex' | 'durChapterPos' | 'totalChapterNum' | 'stateVersion'>;
  selection: CandidateSelection;
}

export interface CandidateOperationSnapshot {
  id: string;
  state: CandidateOperationState;
  known: number;
  completed: number;
  active: number;
  attempts: CandidateOperationAttempt[];
  preview?: CandidateOperationPreview;
  storedBook?: Book;
  created?: boolean;
  commitPending?: boolean;
  automaticCommit?: boolean;
  message?: string;
  updatedAt: string;
}

const storageKey = 'novelreader.candidate-operations.v1';
const committedPrefix = 'committed:';
const committedLogicalPrefix = 'committed-logical:';

function payload(candidate: BookCandidate, shelveBookId?: string) {
  return {
    name: candidate.name,
    ...(candidate.variableMap !== undefined ? { variableMap: candidate.variableMap } : {}),
    ...(candidate.author !== undefined ? { author: candidate.author } : {}),
    ...(candidate.coverUrl !== undefined ? { coverUrl: candidate.coverUrl } : {}),
    ...(candidate.intro !== undefined ? { intro: candidate.intro } : {}),
    ...(candidate.kind !== undefined ? { kind: candidate.kind } : {}),
    ...(candidate.lastChapter !== undefined ? { lastChapter: candidate.lastChapter } : {}),
    ...(candidate.updateTime !== undefined ? { updateTime: candidate.updateTime } : {}),
    ...(candidate.wordCount !== undefined ? { wordCount: candidate.wordCount } : {}),
    ...(candidate.sourceName !== undefined ? { sourceName: candidate.sourceName } : {}),
    ...(candidate.sourceGroup !== undefined ? { sourceGroup: candidate.sourceGroup } : {}),
    ...(candidate.capabilities !== undefined ? { capabilities: candidate.capabilities } : {}),
    sourceId: candidate.sourceId,
    sourceUrl: candidate.sourceUrl,
    bookUrl: candidate.bookUrl,
    ...(candidate.alternateSources !== undefined ? { alternateSources: candidate.alternateSources } : {}),
    ...(shelveBookId ? { shelveBookId } : {}),
  };
}

export function candidateOperationKey(candidate: BookCandidate): string {
  return bindingKey(candidate.sourceId, candidate.bookUrl);
}

export function candidateBindingSignature(candidate: BookCandidate): string {
  return JSON.stringify(candidateBindings(candidate));
}

export function candidateOperationMatches(candidate: BookCandidate, snapshot: CandidateOperationSnapshot): boolean {
  const expected = candidateBindings(candidate);
  const actual = (snapshot.attempts ?? []).map(bindingSnapshotKey);
  return expected.length === actual.length && expected.every((key, index) => key === actual[index]);
}

export function startCandidateOperation(candidate: BookCandidate, shelveBookId?: string) {
  return request<CandidateOperationSnapshot>('/candidate-resolutions', {
    method: 'POST',
    body: JSON.stringify(payload(candidate, shelveBookId)),
  });
}

export function getCandidateOperation(id: string) {
  return request<CandidateOperationSnapshot>(`/candidate-resolutions/${encodeURIComponent(id)}`);
}

export function cancelCandidateOperation(id: string) {
  return request<void>(`/candidate-resolutions/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export function commitCandidateOperation(id: string, bookId: string) {
  return request<CandidateOperationSnapshot>(`/candidate-resolutions/${encodeURIComponent(id)}/shelve`, {
    method: 'POST',
    body: JSON.stringify({ bookId }),
  });
}

export function subscribeCandidateOperation(
  id: string,
  handlers: { onSnapshot: (snapshot: CandidateOperationSnapshot) => void; onDisconnect: () => void },
): EventSource {
  const stream = new EventSource(`/api/candidate-resolutions/${encodeURIComponent(id)}/events`);
  const signal = readerRequestSignal();
  let terminal = false;
  stream.onmessage = (message) => {
    if (signal.aborted) { stream.close(); return; }
    try {
      const snapshot = JSON.parse(message.data) as CandidateOperationSnapshot;
      handlers.onSnapshot(snapshot);
      terminal = ['committed', 'exhausted', 'cancelled', 'failed'].includes(snapshot.state);
      if (terminal) stream.close();
    } catch { /* malformed operation snapshots cannot safely mutate UI state */ }
  };
  stream.onerror = () => {
    if (!signal.aborted && !terminal) handlers.onDisconnect();
  };
  return stream;
}

export function rememberCandidateOperation(candidate: BookCandidate, id: string) {
  const values = loadOperationRegistry();
  values[candidateOperationKey(candidate)] = id;
  saveOperationRegistry(values);
}

export function rememberCandidateCommitted(candidate: BookCandidate, bookId: string) {
  const values = loadOperationRegistry();
  const marker = `${committedPrefix}${bookId}`;
  values[candidateOperationKey(candidate)] = marker;
  values[logicalCommitKey(candidate)] = marker;
  saveOperationRegistry(values);
}

export function candidateWasCommitted(candidate: BookCandidate): boolean {
  const values = loadOperationRegistry();
  return [values[candidateOperationKey(candidate)], values[logicalCommitKey(candidate)]].some(value => (value || '').startsWith(committedPrefix));
}

export function recalledCandidateOperation(candidate: BookCandidate): string {
  const value = loadOperationRegistry()[candidateOperationKey(candidate)] || '';
  return value.startsWith(committedPrefix) ? '' : value;
}

export function clearCandidateCommittedBook(bookId: string) {
  const marker = `${committedPrefix}${bookId}`;
  const values = loadOperationRegistry();
  for (const [key, value] of Object.entries(values)) {
    if (value === marker) delete values[key];
  }
  saveOperationRegistry(values);
}

export function forgetCandidateOperation(candidate: BookCandidate) {
  const values = loadOperationRegistry();
  delete values[candidateOperationKey(candidate)];
  saveOperationRegistry(values);
}

function logicalCommitKey(candidate: BookCandidate) {
  const identity = normalizedBookIdentity(candidate.name, candidate.author || '');
  return `${committedLogicalPrefix}${identity.name}\u0000${identity.author}`;
}

function candidateBindings(candidate: BookCandidate): string[] {
  const seen = new Set<string>();
  const bindings = [candidate, ...(candidate.alternateSources ?? [])];
  return bindings.flatMap(binding => {
    const key = bindingKey(binding.sourceId, binding.bookUrl);
    if (!binding.sourceId || !binding.bookUrl || seen.has(key)) return [];
    seen.add(key);
    return [bindingSnapshotKey(binding)];
  });
}

// Rule variables stay opaque. Even a formatting-only change conservatively
// invalidates reuse; the frontend must not interpret source execution state.
function bindingSnapshotKey(binding: { sourceId: string; bookUrl?: string; variableMap?: string }) {
  return JSON.stringify([bindingKey(binding.sourceId, binding.bookUrl), binding.variableMap ?? '']);
}

function bindingKey(sourceId: string, bookUrl = '') {
  return `${sourceId.trim()}\u0000${bookUrl.trim()}`;
}

function loadOperationRegistry(): Record<string, string> {
  try {
    const value = JSON.parse(sessionStorage.getItem(storageKey) || '{}') as Record<string, unknown>;
    return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'));
  } catch {
    sessionStorage.removeItem(storageKey);
    return {};
  }
}

function saveOperationRegistry(value: Record<string, string>) {
  try {
    if (Object.keys(value).length) sessionStorage.setItem(storageKey, JSON.stringify(value));
    else sessionStorage.removeItem(storageKey);
  } catch { /* operation still runs if tab storage is unavailable */ }
}

export function clearCandidateOperations() {
  try { sessionStorage.removeItem(storageKey); } catch { /* tab storage may be disabled */ }
}

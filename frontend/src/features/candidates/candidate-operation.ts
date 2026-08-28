import type { AltSource, Book, Chapter } from '../../api/models';
import { request } from '../../api/transport';

export interface BookCandidate {
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
export type CandidateOperationStage = 'book_info' | 'toc' | 'content';

export interface CandidateOperationAttempt {
  sourceName?: string;
  sourceId: string;
  sourceUrl: string;
  bookUrl?: string;
  stage?: CandidateOperationStage;
  state: 'queued' | 'running' | 'failed' | 'verified' | 'skipped';
  reason?: string;
}

export interface CandidateOperationPreview {
  book: Omit<Book, 'id' | 'durChapterIndex' | 'durChapterPos' | 'totalChapterNum' | 'stateVersion'>;
  chapters: Chapter[];
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

function payload(candidate: BookCandidate, shelveBookId?: string) {
  return {
    name: candidate.name,
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
  return candidateBindings(candidate).join('\u0001');
}

export function candidateOperationMatches(candidate: BookCandidate, snapshot: CandidateOperationSnapshot): boolean {
  const expected = candidateBindings(candidate);
  const actual = (snapshot.attempts ?? []).map(attempt => bindingKey(attempt.sourceId, attempt.bookUrl));
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
  let terminal = false;
  stream.onmessage = (message) => {
    try {
      const snapshot = JSON.parse(message.data) as CandidateOperationSnapshot;
      handlers.onSnapshot(snapshot);
      terminal = ['committed', 'exhausted', 'cancelled', 'failed'].includes(snapshot.state);
      if (terminal) stream.close();
    } catch { /* malformed operation snapshots cannot safely mutate UI state */ }
  };
  stream.onerror = () => {
    if (!terminal) handlers.onDisconnect();
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
  values[candidateOperationKey(candidate)] = `${committedPrefix}${bookId}`;
  saveOperationRegistry(values);
}

export function candidateWasCommitted(candidate: BookCandidate): boolean {
  return (loadOperationRegistry()[candidateOperationKey(candidate)] || '').startsWith(committedPrefix);
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

function candidateBindings(candidate: BookCandidate): string[] {
  const seen = new Set<string>();
  const bindings = [{ sourceId: candidate.sourceId, bookUrl: candidate.bookUrl }, ...(candidate.alternateSources ?? [])];
  return bindings.flatMap(binding => {
    const key = bindingKey(binding.sourceId, binding.bookUrl);
    if (!binding.sourceId || !binding.bookUrl || seen.has(key)) return [];
    seen.add(key);
    return [key];
  });
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

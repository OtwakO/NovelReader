import { saveProgress } from '../../api/reader';

interface ProgressWrite { contentRevision: number; chapterIndex: number; position: number }
const versions = new Map<string, number>();
const pending = new Map<string, Promise<void>>();
let generation = 0;

export function setProgressVersion(bookId: string, stateVersion: number): void { versions.set(bookId, stateVersion); }
export function getProgressVersion(bookId: string): number | undefined { return versions.get(bookId); }

export async function queueProgressWrite(bookId: string, write: ProgressWrite): Promise<void> {
  await queueReadingStateWrite(bookId, version => saveProgress(bookId, write.contentRevision, version, write.chapterIndex, write.position));
}

/** Progress and bookmarks share the same server state version and must be ordered together. */
export function queueReadingStateWrite<T extends { stateVersion: number }>(bookId: string, write: (version: number) => Promise<T>): Promise<T | undefined> {
  const requestGeneration = generation;
  const previous = pending.get(bookId) ?? Promise.resolve();
  const operation = previous.catch(() => undefined).then(async () => {
    if (requestGeneration !== generation) return;
    const version = versions.get(bookId);
    if (version === undefined) throw new Error('Reading state is not initialized');
    const saved = await write(version);
    if (requestGeneration !== generation) return;
    versions.set(bookId, saved.stateVersion);
    return saved;
  });
  const barrier = operation.then(() => undefined, () => undefined);
  pending.set(bookId, barrier);
  void barrier.then(() => { if (pending.get(bookId) === barrier) pending.delete(bookId); });
  return operation;
}

export function waitForProgressWrites(bookId: string): Promise<void> { return pending.get(bookId) ?? Promise.resolve(); }
export function resetProgressWriter(): void { generation++; versions.clear(); pending.clear(); }

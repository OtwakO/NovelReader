import { saveProgress } from '../../api/reader';

interface ProgressWrite { sourceId: string; chapterIndex: number; position: number }
const versions = new Map<string, number>();
const pending = new Map<string, Promise<void>>();
let generation = 0;

export function setProgressVersion(bookId: string, stateVersion: number): void { versions.set(bookId, stateVersion); }
export function getProgressVersion(bookId: string): number | undefined { return versions.get(bookId); }

export function queueProgressWrite(bookId: string, write: ProgressWrite): Promise<void> {
  const requestGeneration = generation;
  const previous = pending.get(bookId) ?? Promise.resolve();
  const operation = previous.catch(() => undefined).then(async () => {
    if (requestGeneration !== generation) return;
    const version = versions.get(bookId);
    if (version === undefined) throw new Error('Reading state is not initialized');
    const saved = await saveProgress(bookId, write.sourceId, version, write.chapterIndex, write.position);
    if (requestGeneration === generation) versions.set(bookId, saved.stateVersion);
  });
  const barrier = operation.then(() => undefined, () => undefined);
  pending.set(bookId, barrier);
  void barrier.then(() => { if (pending.get(bookId) === barrier) pending.delete(bookId); });
  return operation;
}

export function waitForProgressWrites(bookId: string): Promise<void> { return pending.get(bookId) ?? Promise.resolve(); }
export function resetProgressWriter(): void { generation++; versions.clear(); pending.clear(); }

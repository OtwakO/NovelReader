// One versioned per-book queue prevents stale progress across Reader and source-switch lifetimes.
import { saveProgress } from '../api/client';
import { createProgressQueue } from './readingProgress.js';

const versions = new Map<string, number>();
const progressQueue = createProgressQueue(async (bookId: string, sourceUrl: string, chapterIndex: number, position: number) => {
  const stateVersion = versions.get(bookId);
  if (stateVersion === undefined) throw new Error('Reading state is not initialized');
  const saved = await saveProgress(bookId, sourceUrl, stateVersion, chapterIndex, position);
  versions.set(bookId, saved.stateVersion);
});

export function setProgressVersion(bookId: string, stateVersion: number) {
  versions.set(bookId, stateVersion);
}

export function queueProgressWrite(bookId: string, sourceUrl: string, chapterIndex: number, position: number): Promise<void> {
  return progressQueue.write(bookId, sourceUrl, chapterIndex, position);
}

export function waitForProgressWrites(bookId: string): Promise<void> {
  return progressQueue.wait(bookId);
}

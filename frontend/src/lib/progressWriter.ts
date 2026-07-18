// One per-book queue prevents progress writes from racing across Reader instances.
import { saveProgress } from '../api/client';
import { createProgressQueue } from './readingProgress.js';

const progressQueue = createProgressQueue(saveProgress);

export function queueProgressWrite(bookId: string, chapterIndex: number, position: number): Promise<void> {
  return progressQueue.write(bookId, chapterIndex, position);
}

export function waitForProgressWrites(bookId: string): Promise<void> {
  return progressQueue.wait(bookId);
}

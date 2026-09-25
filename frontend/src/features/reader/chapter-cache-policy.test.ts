import { afterEach, expect, it, vi } from 'vitest';
import type { Chapter } from '../../api/models';
import type { ChapterContent } from '../../api/reader';
import { chapterWindow, savedChapter, savedChapterIsFresh, type ChapterCacheIdentity } from './chapter-cache-policy';

afterEach(() => vi.restoreAllMocks());
it('uses main-section neighbors without filling missing predecessors', () => {
  const chapters = [0, 2, 4, 6, 8, 10, 12].map(index => ({ index, isVolume: index === 2, auxiliary: index === 6 })) as Chapter[];
  expect(chapterWindow(chapters, 8)).toEqual([0, 4, 8, 10, 12]);
  expect(chapterWindow(chapters, 6)).toEqual([]);
});
it('qualifies source content and preserves remaining age across sleep and copies', () => {
  const identity: ChapterCacheIdentity = { readerId: 'reader', homeGeneration: 'home', bookId: 'book', revision: 1, provider: 'booksource', sourceIdentity: 'definition' };
  const content: ChapterContent = { version: 1, contentRevision: 1, sourceIdentity: 'definition', freshForMs: 1000, offlineCopy: false, document: { kind: 'prose', title: '', blocks: [] } };
  const wall = vi.spyOn(Date, 'now').mockReturnValue(1500);
  vi.spyOn(performance, 'now').mockReturnValue(100);
  const saved = savedChapter(identity, 0, content, { wall: 1000, monotonic: 100 })!;
  expect(saved.expiresAt).toBe(2000);
  expect(savedChapter(identity, 0, { ...content, sourceIdentity: 'other' }, { wall: 1000, monotonic: 100 })).toBeUndefined();
  wall.mockReturnValue(2000);
  expect(savedChapterIsFresh(saved)).toBe(false);
  wall.mockReturnValue(1400);
  expect(savedChapterIsFresh(saved)).toBe(false);
});

import { describe, expect, it } from 'vitest';
import { adjacentChapterIndex, normalizedScroll, resolveChapterIndex, scrollTopForProgress } from './reading-progress';

const chapters = [{ id: 'v', bookId: 'b', index: 0, title: 'Volume', url: '', isVolume: true }, { id: '1', bookId: 'b', index: 1, title: 'One', url: '1', isVolume: false }, { id: 'v2', bookId: 'b', index: 2, title: 'Volume 2', url: '', isVolume: true }, { id: '3', bookId: 'b', index: 3, title: 'Two', url: '3', isVolume: false }];

describe('reader progress', () => {
  it('resolves requested and saved readable chapter indices', () => { expect(resolveChapterIndex(chapters, 3, 1)).toBe(3); expect(resolveChapterIndex(chapters, 2, 1)).toBe(1); expect(resolveChapterIndex([], 1, 1)).toBeNull(); });
  it('navigates around volume rows', () => { expect(adjacentChapterIndex(chapters, 1, 1)).toBe(3); expect(adjacentChapterIndex(chapters, 3, -1)).toBe(1); });
  it('normalizes and restores bounded scroll positions', () => { expect(normalizedScroll(300, 1000, 400)).toBe(.5); expect(scrollTopForProgress(.5, 1000, 400)).toBe(300); expect(scrollTopForProgress(2, 1000, 400)).toBe(600); });
});

it('keeps auxiliary locations addressable but out of resume fallback and ordinary reading order', () => {
  const sections = [
    { index: 0, title: 'Notes', isVolume: false, auxiliary: true },
    { index: 1, title: 'One', isVolume: false },
    { index: 2, title: 'More notes', isVolume: false, auxiliary: true },
    { index: 3, title: 'Two', isVolume: false },
  ];
  expect(resolveChapterIndex(sections, 0, 1)).toBe(0);
  expect(resolveChapterIndex(sections, undefined, 0)).toBe(1);
  expect(adjacentChapterIndex(sections, 1, 1)).toBe(3);
  expect(adjacentChapterIndex(sections, 3, -1)).toBe(1);
  expect(adjacentChapterIndex(sections, 2, 1)).toBeNull();
});

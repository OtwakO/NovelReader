import { describe, expect, test } from 'vitest';
import type { Book } from '../../api/models';
import { currentChapterNumber, shelfProgressPercent } from './shelf-progress';

const book = (overrides: Partial<Book> = {}): Book => ({
  id: 'book-1', name: '凡人修仙传', author: '忘语', coverUrl: '', intro: '', kind: '', sourceUrl: 'source', bookUrl: 'book',
  lastChapter: '', durChapterIndex: 24, durChapterPos: 0.5, totalChapterNum: 100, stateVersion: 1, ...overrides,
});

describe('shelf progress', () => {
  test('combines completed chapters with the saved position in the current chapter', () => {
    expect(shelfProgressPercent(book())).toBe(25);
    expect(currentChapterNumber(book())).toBe(25);
  });

  test('keeps unknown and malformed progress bounded', () => {
    expect(shelfProgressPercent(book({ totalChapterNum: 0 }))).toBe(0);
    expect(shelfProgressPercent(book({ durChapterIndex: 200, durChapterPos: 5 }))).toBe(100);
  });
});

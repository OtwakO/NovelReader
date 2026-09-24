import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '../../api/models';
import { loadShelfViewState, saveShelfViewState, visibleShelfBooks } from './shelf-view-state';

const book = (name: string, author: string, updatedAt: number, index: number, total = 10): Book => ({ id: name, name, author, coverUrl: '', intro: '', kind: '', sourceId: '', sourceUrl: '', bookUrl: '', lastChapter: '', durChapterIndex: index, durChapterPos: 0, totalChapterNum: total, provider: 'booksource', contentRevision: 0, stateVersion: 1, updatedAt, lastReadAt: updatedAt });

describe('shelf view state', () => {
  const values = new Map<string, string>();
  beforeEach(() => vi.stubGlobal('sessionStorage', { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value) }));

  it('filters title and author and sorts without mutating the shelf', () => {
    const books = [book('Beta', 'Writer', 1, 9), book('Alpha', 'Other', 3, 2), book('Gamma', 'Writer', 2, 4)];
    expect(visibleShelfBooks(books, 'writer', 'title').map(item => item.name)).toEqual(['Beta', 'Gamma']);
    expect(visibleShelfBooks(books, '', 'recent').map(item => item.name)).toEqual(['Alpha', 'Gamma', 'Beta']);
    expect(visibleShelfBooks(books, '', 'progress').at(0)?.name).toBe('Beta');
    expect(books.map(item => item.name)).toEqual(['Beta', 'Alpha', 'Gamma']);
  });

  it('orders actual reading ahead of unread additions and ignores metadata updates', () => {
    const reading = { ...book('Reading', '', 900, 0), lastReadAt: 20 };
    const olderRead = { ...book('Older read', '', 1000, 8), lastReadAt: 10 };
    const unread = { ...book('Unread', '', 2000, 0), lastReadAt: 0, createdAt: 100 };
    const newerUnread = { ...unread, name: 'Newer unread', createdAt: 200 };
    expect(visibleShelfBooks([unread, olderRead, newerUnread, reading], '', 'recent').map(item => item.name))
      .toEqual(['Reading', 'Older read', 'Newer unread', 'Unread']);
  });

  it('persists query, sort, and scroll position in the current tab', () => {
    saveShelfViewState({ query: '凡人', sort: 'author', scrollY: 420 });
    expect(loadShelfViewState()).toEqual({ query: '凡人', sort: 'author', scrollY: 420 });
  });
});

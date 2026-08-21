import type { Book } from '../../api/models';

export type ShelfSort = 'recent' | 'title' | 'author' | 'progress';
export interface ShelfViewState { query: string; sort: ShelfSort; scrollY: number }

const key = 'novelreader.shelf.view.v1';
const validSorts = new Set<ShelfSort>(['recent', 'title', 'author', 'progress']);

function storage(): Storage | null {
  try { return typeof window === 'undefined' ? null : window.sessionStorage; } catch { return null; }
}

export function loadShelfViewState(): ShelfViewState {
  try {
    const value = JSON.parse(storage()?.getItem(key) || '{}') as Partial<ShelfViewState>;
    return {
      query: typeof value.query === 'string' ? value.query : '',
      sort: validSorts.has(value.sort as ShelfSort) ? value.sort as ShelfSort : 'recent',
      scrollY: Number.isFinite(value.scrollY) ? Math.max(0, Number(value.scrollY)) : 0,
    };
  } catch { return { query: '', sort: 'recent', scrollY: 0 }; }
}

export function saveShelfViewState(state: ShelfViewState): void {
  try { storage()?.setItem(key, JSON.stringify(state)); } catch { /* tab-local restoration is optional */ }
}

export function visibleShelfBooks(books: Book[], query: string, sort: ShelfSort): Book[] {
  const normalized = query.trim().toLocaleLowerCase();
  const filtered = normalized ? books.filter(book => `${book.name}\n${book.author}`.toLocaleLowerCase().includes(normalized)) : books;
  return [...filtered].sort((left, right) => {
    if (sort === 'title') return left.name.localeCompare(right.name);
    if (sort === 'author') return left.author.localeCompare(right.author) || left.name.localeCompare(right.name);
    if (sort === 'progress') {
      const leftProgress = left.totalChapterNum > 0 ? left.durChapterIndex / left.totalChapterNum : 0;
      const rightProgress = right.totalChapterNum > 0 ? right.durChapterIndex / right.totalChapterNum : 0;
      return rightProgress - leftProgress || left.name.localeCompare(right.name);
    }
    return (right.updatedAt ?? 0) - (left.updatedAt ?? 0) || left.name.localeCompare(right.name);
  });
}

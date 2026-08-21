import type { Book } from '../../api/models';

export function shelfProgressPercent(book: Book): number {
  if (book.totalChapterNum <= 0) return 0;
  const completed = Math.max(0, book.durChapterIndex) + Math.min(1, Math.max(0, book.durChapterPos));
  return Math.min(100, Math.max(0, Math.round((completed / book.totalChapterNum) * 100)));
}

export function currentChapterNumber(book: Book): number {
  return Math.max(1, book.durChapterIndex + 1);
}

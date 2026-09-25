import type { Chapter, LibraryBook } from '../../api/models';
import type { ReadingContent } from '../../api/reader';

export const retainedBooks = 3;
export const retainedChapters = 5;

export interface ChapterCacheIdentity {
  readerId: string;
  homeGeneration: string;
  bookId: string;
  revision: number;
  provider: LibraryBook['provider'];
  sourceIdentity?: string;
}
export interface SavedChapter {
  id: string;
  scope: string;
  identity: ChapterCacheIdentity;
  index: number;
  content: ReadingContent;
  storedAt: number;
  expiresAt: number | null;
}
export interface RetainedBook { scope: string; window: number[] }

export function cacheScope(identity: ChapterCacheIdentity): string {
  return JSON.stringify([identity.readerId, identity.homeGeneration, identity.bookId, identity.revision, identity.provider, identity.sourceIdentity ?? '']);
}

// Main section order is authoritative. Notes remain on-demand and do not move
// the main window; callers keep the last committed main index for those visits.
export function chapterWindow(chapters: readonly Chapter[], current: number): number[] {
  const main = chapters.filter(chapter => !chapter.isVolume && !chapter.auxiliary);
  const position = main.findIndex(chapter => chapter.index === current);
  return position < 0 ? [] : main.slice(Math.max(0, position - 2), position + 3).map(chapter => chapter.index);
}

export function savedChapter(identity: ChapterCacheIdentity, index: number, content: ReadingContent, started: { wall: number; monotonic: number }): SavedChapter | undefined {
  if (content.contentRevision !== identity.revision || content.offlineCopy) return;
  if (content.version === 1 && content.document.blocks.some(block => block.kind === 'image' && block.resource.unavailable)) return;
  let expiresAt: number | null = null;
  if (identity.provider === 'booksource') {
    if (content.version !== 1 || !identity.sourceIdentity || content.sourceIdentity !== identity.sourceIdentity || content.freshForMs === undefined) return;
    // Conservatively account for both sleep and an adjusted wall clock.
    const remaining = content.freshForMs - Math.max(Date.now() - started.wall, performance.now() - started.monotonic);
    if (remaining <= 0) return;
    expiresAt = Date.now() + remaining;
  }
  const scope = cacheScope(identity);
  return { id: JSON.stringify([scope, index]), scope, identity, index, content, storedAt: Date.now(), expiresAt };
}

export function savedChapterIsFresh(entry: SavedChapter): boolean {
  const now = Date.now();
  return now >= entry.storedAt && (entry.expiresAt === null || now < entry.expiresAt);
}

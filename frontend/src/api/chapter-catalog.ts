import type { Chapter } from './models';

export interface ChapterCatalog { chapters: Chapter[]; contentRevision: number }

/** Catalog order is reading order; section indices remain stable across filtering. */
export function parseChapterCatalog(chapters: unknown[], contentRevision: number): ChapterCatalog {
  const indices = new Set<number>();
  return { contentRevision, chapters: chapters.map(input => {
    if (!input || typeof input !== 'object') throw new Error('Invalid catalog section');
    const value = input as Record<string, unknown>;
    if (typeof value.index !== 'number' || !Number.isSafeInteger(value.index) || value.index < 0 || indices.has(value.index) || typeof value.title !== 'string') throw new Error('Invalid catalog section');
    if (value.isVolume !== undefined && typeof value.isVolume !== 'boolean') throw new Error('Invalid catalog volume flag');
    if (value.auxiliary !== undefined && typeof value.auxiliary !== 'boolean') throw new Error('Invalid catalog auxiliary flag');
    if (value.isVolume && value.auxiliary) throw new Error('Volume cannot be an auxiliary section');
    indices.add(value.index);
    return { index: value.index, title: value.title, isVolume: value.isVolume === true, ...(value.auxiliary === undefined ? {} : { auxiliary: value.auxiliary }) };
  }) };
}

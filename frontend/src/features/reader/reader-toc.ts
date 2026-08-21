import type { Chapter } from '../../api/models';

export type TocOrder = 'ascending' | 'descending';

export function readableChapterCount(chapters: Chapter[]): number {
  return chapters.filter(chapter => !chapter.isVolume).length;
}

export function visibleTocChapters(chapters: Chapter[], query: string, order: TocOrder): Chapter[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const numericIndex = /^\d+$/.test(normalizedQuery) ? Number(normalizedQuery) - 1 : null;
  const filtered = normalizedQuery
    ? chapters.filter(chapter => numericIndex !== null ? chapter.index === numericIndex : chapter.title.toLocaleLowerCase().includes(normalizedQuery))
    : chapters;
  return order === 'descending' ? [...filtered].reverse() : [...filtered];
}

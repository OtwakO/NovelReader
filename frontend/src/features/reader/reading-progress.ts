import type { Chapter } from '../../api/models';

export function clampProgress(value: number): number {
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0;
}

export function normalizedScroll(scrollTop: number, scrollHeight: number, clientHeight: number): number {
  const range = Math.max(0, scrollHeight - clientHeight);
  return range === 0 ? 0 : clampProgress(scrollTop / range);
}

export function scrollTopForProgress(position: number, scrollHeight: number, clientHeight: number): number {
  return clampProgress(position) * Math.max(0, scrollHeight - clientHeight);
}

export function readableChapters(chapters: Chapter[]): Chapter[] {
  return chapters.filter((chapter) => !chapter.isVolume);
}

export function resolveChapterIndex(chapters: Chapter[], requestedIndex: number | undefined, savedIndex: number): number | null {
  const readable = readableChapters(chapters);
  if (!readable.length) return null;
  if (Number.isInteger(requestedIndex) && readable.some((chapter) => chapter.index === requestedIndex)) return requestedIndex ?? null;
  if (Number.isInteger(savedIndex) && readable.some((chapter) => chapter.index === savedIndex)) return savedIndex;
  return readable[0]?.index ?? null;
}

export function adjacentChapterIndex(chapters: Chapter[], currentIndex: number, direction: -1 | 1): number | null {
  const readable = readableChapters(chapters);
  const position = readable.findIndex((chapter) => chapter.index === currentIndex);
  const target = position + direction;
  return position >= 0 && target >= 0 && target < readable.length ? readable[target]?.index ?? null : null;
}

import { describe, expect, test } from 'vitest';
import type { Chapter } from '../../api/models';
import { readableChapterCount, visibleTocChapters } from './reader-toc';

const chapters: Chapter[] = [
  { id: 'v', bookId: 'book', index: 0, title: '第一卷 人界篇', url: '', isVolume: true },
  { id: '1', bookId: 'book', index: 1, title: '第一章 山边小村', url: '/1', isVolume: false },
  { id: '2', bookId: 'book', index: 2, title: '第二章 青牛镇', url: '/2', isVolume: false },
];

describe('Reader TOC tools', () => {
  test('counts readable chapters separately from volume headings', () => {
    expect(readableChapterCount(chapters)).toBe(2);
  });

  test('filters chapter titles without changing canonical indexes', () => {
    expect(visibleTocChapters(chapters, '青牛', 'ascending').map(chapter => chapter.index)).toEqual([2]);
  });

  test('uses a numeric query as an exact one-based canonical entry index', () => {
    expect(visibleTocChapters(chapters, '2', 'ascending').map(chapter => chapter.index)).toEqual([1]);
    expect(visibleTocChapters(chapters, '02', 'ascending').map(chapter => chapter.index)).toEqual([1]);
  });

  test('reverses display order without mutating the source list', () => {
    expect(visibleTocChapters(chapters, '', 'descending').map(chapter => chapter.index)).toEqual([2, 1, 0]);
    expect(chapters.map(chapter => chapter.index)).toEqual([0, 1, 2]);
  });
});

import { describe, expect, it } from 'vitest';
import { readableChapterLabel } from './book-display';

describe('readableChapterLabel', () => {
  it('keeps human chapter titles and hides URL-like source output', () => {
    expect(readableChapterLabel('第二章 青牛镇')).toBe('第二章 青牛镇');
    expect(readableChapterLabel('/novel_123.html')).toBe('');
    expect(readableChapterLabel('https://example.com/chapter')).toBe('');
    expect(readableChapterLabel('chapter?id=1')).toBe('');
  });
});

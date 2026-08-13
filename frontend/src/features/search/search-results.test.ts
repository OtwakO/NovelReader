import { describe, expect, it } from 'vitest';
import type { SearchResult } from '../../api/search';
import { mergeSearchResults } from './search-results';

const result = (sourceUrl: string, bookUrl: string, name = '凡人修仙传', author = '忘语', extras: Partial<SearchResult> = {}): SearchResult => ({ name, author, sourceUrl, bookUrl, sourceName: sourceUrl, coverUrl: '', intro: '', kind: '', lastChapter: '', ...extras });

describe('mergeSearchResults', () => {
  it('does not duplicate a retried source result', () => { const item = result('a', '/book'); expect(mergeSearchResults([item], [item], '凡人')).toEqual([item]); });
  it('consolidates the same book and keeps the alternate source', () => { const merged = mergeSearchResults([result('a', '/a')], [result('b', '/b', '凡人修仙传', '作者：忘语')], '凡人'); expect(merged).toHaveLength(1); expect(merged[0]?.alternateSources).toEqual([{ sourceUrl: 'b', bookUrl: '/b', sourceName: 'b' }]); });
  it('promotes a richer result without losing previous alternatives', () => { const [merged] = mergeSearchResults([result('a', '/a', '凡人修仙传', '忘语', { score: 60, alternateSources: [{ sourceUrl: 'b', bookUrl: '/b', sourceName: 'B' }] })], [result('c', '/c', '凡人修仙传', '忘语', { score: 80, coverUrl: 'cover' })], '凡人'); expect(merged?.sourceUrl).toBe('c'); expect(merged?.alternateSources?.map((source) => source.sourceUrl).sort()).toEqual(['a', 'b']); });
  it('keeps the same title by different authors separate', () => { expect(mergeSearchResults([result('a', '/a', '重生', '作者甲')], [result('b', '/b', '重生', '作者乙')], '重生')).toHaveLength(2); });
});

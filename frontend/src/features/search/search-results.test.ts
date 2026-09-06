import { describe, expect, it } from 'vitest';
import type { SearchResult } from '../../api/search';
import { mergeSearchResults } from './search-results';

const result = (sourceUrl: string, bookUrl: string, name = '凡人修仙传', author = '忘语', extras: Partial<SearchResult> = {}): SearchResult => ({ name, author, sourceId: sourceUrl, sourceUrl, bookUrl, sourceName: sourceUrl, coverUrl: '', intro: '', kind: '', lastChapter: '', ...extras });

describe('mergeSearchResults', () => {
  it('does not duplicate a retried source result', () => { const item = result('a', '/book'); expect(mergeSearchResults([item], [item], '凡人')).toEqual([item]); });
  it('consolidates the same book and keeps the alternate source', () => { const merged = mergeSearchResults([result('a', '/a')], [result('b', '/b', '凡人修仙传', '作者：忘语')], '凡人'); expect(merged).toHaveLength(1); expect(merged[0]?.alternateSources).toEqual([{ sourceId: 'b', sourceUrl: 'b', bookUrl: '/b', sourceName: 'b' }]); });
  it('retains authoritative shelf membership while merging provider results', () => { const [merged] = mergeSearchResults([result('a', '/a')], [result('b', '/b', '凡人修仙传', '忘语', { shelfBookId: 'shelf-book' })], '凡人'); expect(merged?.shelfBookId).toBe('shelf-book'); });
  it('promotes a richer result without losing previous alternatives', () => { const [merged] = mergeSearchResults([result('a', '/a', '凡人修仙传', '忘语', { score: 60, alternateSources: [{ sourceId: 'b', sourceUrl: 'b', bookUrl: '/b', sourceName: 'B' }] })], [result('c', '/c', '凡人修仙传', '忘语', { score: 80, coverUrl: 'cover' })], '凡人'); expect(merged?.sourceUrl).toBe('c'); expect(merged?.alternateSources?.map((source) => source.sourceUrl).sort()).toEqual(['a', 'b']); });
  it.each([0, 100])('retains each binding snapshot when incoming score is %s', (score) => {
    const [merged] = mergeSearchResults(
      [result('a', '/a', undefined, undefined, { score: 60, variableMap: '{"owner":"a"}' })],
      [result('b', '/b', undefined, undefined, { score, variableMap: '{"owner":"b"}' })], '凡人');
    for (const binding of [merged!, ...merged!.alternateSources!]) {
      expect(binding.variableMap).toBe(`{"owner":"${binding.sourceId}"}`);
    }
  });
  it('keeps the same title by different authors separate', () => { expect(mergeSearchResults([result('a', '/a', '重生', '作者甲')], [result('b', '/b', '重生', '作者乙')], '重生')).toHaveLength(2); });
  it('ranks exact normalized authors below exact titles and above partial titles', () => {
    const merged = mergeSearchResults([], [
      result('partial', '/partial', '忘语作品集', '其他作者'),
      result('author', '/author', '凡人修仙传', '作者：忘语'),
      result('title', '/title', '忘语', '其他作者'),
    ], '忘语');
    expect(merged.map((item) => item.sourceUrl)).toEqual(['title', 'author', 'partial']);
    expect(merged.map((item) => item.score)).toEqual([100, 90, 80]);
  });
});

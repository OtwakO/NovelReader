// Dependency-free cumulative search-result merge regression tests.
import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeSearchResults } from './searchResults.js';

const result = (sourceUrl, bookUrl, name = '凡人修仙传', author = '忘语', extras = {}) => ({
  name, author, sourceUrl, bookUrl, sourceName: sourceUrl, coverUrl: '', intro: '', kind: '', lastChapter: '', ...extras,
});

test('retrying a source does not duplicate its result or progress identity', () => {
  const item = result('a', '/book');
  assert.deepEqual(mergeSearchResults([item], [item], '凡人'), [item]);
});

test('same book across batches becomes one result with an alternate source', () => {
  const merged = mergeSearchResults(
    [result('a', '/a')],
    [result('b', '/b', '凡人修仙传', '作者：忘语')],
    '凡人',
  );
  assert.equal(merged.length, 1);
  assert.deepEqual(merged[0].alternateSources, [{ sourceUrl: 'b', bookUrl: '/b', sourceName: 'b' }]);
});

test('promoting a richer primary preserves every previous alternate', () => {
  const current = result('a', '/a', '凡人修仙传', '忘语', {
    score: 60,
    alternateSources: [{ sourceUrl: 'b', bookUrl: '/b', sourceName: 'B' }],
  });
  const promoted = result('c', '/c', '凡人修仙传', '忘语', { score: 80, coverUrl: 'long-cover-url' });
  const [merged] = mergeSearchResults([current], [promoted], '凡人');
  assert.equal(merged.sourceUrl, 'c');
  assert.deepEqual(
    merged.alternateSources.map(({ sourceUrl }) => sourceUrl).sort(),
    ['a', 'b'],
  );
});

test('same title with different authors remains separate', () => {
  const merged = mergeSearchResults(
    [result('a', '/a', '重生', '作者甲')],
    [result('b', '/b', '重生', '作者乙')],
    '重生',
  );
  assert.equal(merged.length, 2);
});

test('merged results stay relevance ordered', () => {
  const merged = mergeSearchResults([], [
    result('a', '/a', '别的书'),
    result('b', '/b', '凡人修仙传'),
    result('c', '/c', '凡人外传'),
  ], '凡人修仙传');
  assert.deepEqual(merged.map(({ name }) => name), ['凡人修仙传', '别的书', '凡人外传']);
});

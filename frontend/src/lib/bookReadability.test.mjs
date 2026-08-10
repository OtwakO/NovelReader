import test from 'node:test';
import assert from 'node:assert/strict';
import { alternateSourceOptions, validateReadableBook } from './bookReadability.mjs';

test('validates TOC and the first readable chapter', async () => {
  const calls = [];
  const chapter = await validateReadableBook('book-1', {
    getChapters: async () => [{ index: 0, isVolume: true }, { index: 1, isVolume: false }],
    getChapterContent: async (bookId, index) => {
      calls.push([bookId, index]);
      return { paragraphs: ['Readable'], blocks: [] };
    },
  });
  assert.equal(chapter.index, 1);
  assert.deepEqual(calls, [['book-1', 1]]);
});

test('rejects missing and empty readable chapters', async () => {
  await assert.rejects(() => validateReadableBook('book-1', {
    getChapters: async () => [{ index: 0, isVolume: true }],
    getChapterContent: async () => ({ paragraphs: [], blocks: [] }),
  }), /did not provide/);
  await assert.rejects(() => validateReadableBook('book-1', {
    getChapters: async () => [{ index: 0, isVolume: false }],
    getChapterContent: async () => ({ paragraphs: [], blocks: [] }),
  }), /empty first chapter/);
});

test('alternate options exclude duplicate source identities', () => {
  assert.deepEqual(alternateSourceOptions({
    sourceUrl: 'a', bookUrl: '/one', alternateSources: [
      { sourceUrl: 'a', bookUrl: '/one', sourceName: 'duplicate' },
      { sourceUrl: 'b', bookUrl: '/two', sourceName: 'B' },
      { sourceUrl: 'b', bookUrl: '/two', sourceName: 'B duplicate' },
    ],
  }), [{ sourceUrl: 'b', bookUrl: '/two', sourceName: 'B' }]);
});

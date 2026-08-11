import test from 'node:test';
import assert from 'node:assert/strict';
import { matchesLogicalBook, normalizedBookIdentity } from './bookIdentity.mjs';

test('normalizes punctuation whitespace case and author labels', () => {
  assert.deepEqual(normalizedBookIdentity(' 异度，旅社 ', '作者： 远瞳'), {
    name: '异度旅社', author: '远瞳',
  });
  assert.deepEqual(normalizedBookIdentity('The Book', 'AUTHOR: Alice'), {
    name: 'thebook', author: 'alice',
  });
  assert.deepEqual(normalizedBookIdentity('异度旅社', '远瞳 著'), {
    name: '异度旅社', author: '远瞳',
  });
});

test('matches only the same normalized title and author', () => {
  const book = { name: '异度旅社', author: '远瞳' };
  assert.equal(matchesLogicalBook(book, { name: '异度，旅社', author: '作者：远瞳' }), true);
  assert.equal(matchesLogicalBook(book, { name: '异度旅社', author: '另一作者' }), false);
  assert.equal(matchesLogicalBook(book, { name: '异度', author: '远瞳' }), false);
});

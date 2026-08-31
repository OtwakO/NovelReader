import { describe, expect, it } from 'vitest';
import { matchesLogicalBook, normalizedBookIdentity } from './book-identity';

describe('book identity', () => {
  it('normalizes punctuation, whitespace, case, and author labels', () => {
    expect(normalizedBookIdentity(' 异度，旅社 ', '作者： 远瞳')).toEqual({ name: '异度旅社', author: '远瞳' });
    expect(normalizedBookIdentity('The Book', 'AUTHOR: Alice')).toEqual({ name: 'thebook', author: 'alice' });
  });
  it('matches only the same normalized title and author', () => {
    const book = { name: '异度旅社', author: '远瞳' };
    expect(matchesLogicalBook(book, { name: '异度，旅社', author: '作者：远瞳' })).toBe(true);
    expect(matchesLogicalBook(book, { name: '异度旅社', author: '另一作者' })).toBe(false);
  });
});

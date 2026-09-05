import { describe, expect, it } from 'vitest';
import identityCases from '../../../../testdata/book-identity.json';
import { matchesLogicalBook, normalizedBookIdentity } from './book-identity';

describe('book identity', () => {
  it.each(identityCases)('follows the backend identity contract: $name / $author', ({ name, author, expected }) => {
    expect(normalizedBookIdentity(name, author)).toEqual(expected);
  });
  it('matches only the same normalized title and author', () => {
    const book = { name: '异度旅社', author: '远瞳' };
    expect(matchesLogicalBook(book, { name: '异度，旅社', author: '作者：远瞳' })).toBe(true);
    expect(matchesLogicalBook(book, { name: '异度旅社', author: '另一作者' })).toBe(false);
  });
});

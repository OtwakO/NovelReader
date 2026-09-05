import type { SearchResult } from '../../api/models';

// Match Go strings.TrimSpace, including Unicode whitespace absent from JS trim().
function trimIdentitySpace(value: string) {
  return value.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
}

function normalizePart(value: string | undefined, author = false) {
  // Go lowercases individual runes, without locale/context-dependent casing.
  let normalized = trimIdentitySpace([...String(value || '')].map((character) => character.toLowerCase()).join(''));
  if (author) {
    for (const prefix of ['作者：', '作者:', 'author：', 'author:']) {
      if (normalized.startsWith(prefix)) normalized = trimIdentitySpace(normalized.slice(prefix.length));
    }
    for (const suffix of [' 著', '著', ' 作', '作']) {
      if (normalized.endsWith(suffix)) normalized = trimIdentitySpace(normalized.slice(0, -suffix.length));
    }
  }
  return [...normalized].filter((character) => /[\p{L}\p{N}]/u.test(character)).join('');
}

export function normalizedBookIdentity(name: string, author: string) {
  return { name: normalizePart(name), author: normalizePart(author, true) };
}

export function matchesLogicalBook(book: { name: string; author: string }, result: Pick<SearchResult, 'name' | 'author'>) {
  const expected = normalizedBookIdentity(book.name, book.author);
  const candidate = normalizedBookIdentity(result.name, result.author);
  return Boolean(expected.name) && expected.name === candidate.name && expected.author === candidate.author;
}

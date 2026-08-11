function normalizePart(value, author = false) {
  let normalized = String(value || '').trim().toLocaleLowerCase();
  if (author) normalized = normalized
    .replace(/^(作者|author)\s*[:：]\s*/i, '')
    .replace(/\s*(著|作)\s*$/u, '');
  return [...normalized].filter((character) => /[\p{L}\p{N}]/u.test(character)).join('');
}

export function normalizedBookIdentity(name, author) {
  return {
    name: normalizePart(name),
    author: normalizePart(author, true),
  };
}

export function matchesLogicalBook(book, result) {
  const expected = normalizedBookIdentity(book.name, book.author);
  const candidate = normalizedBookIdentity(result.name, result.author);
  return Boolean(expected.name) && expected.name === candidate.name && expected.author === candidate.author;
}

// Incrementally merges streamed source results across batches and retries.

const authorPrefixes = ['作者：', '作者:', '作 者 ：'];

const identity = ({ sourceUrl, bookUrl }) => `${sourceUrl}\u0000${bookUrl}`;
const normalizeName = (name = '') => name.trim();

function normalizeAuthor(author = '') {
  let value = author.trim();
  const prefix = authorPrefixes.find((item) => value.startsWith(item));
  if (prefix) value = value.slice(prefix.length).trim();
  return value;
}

function sameBook(left, right) {
  if (normalizeName(left.name) !== normalizeName(right.name)) return false;
  const leftAuthor = normalizeAuthor(left.author);
  const rightAuthor = normalizeAuthor(right.author);
  return !leftAuthor || !rightAuthor || leftAuthor === rightAuthor;
}

function score(query, name) {
  const q = query.trim();
  const n = name.trim();
  if (!q) return 0;
  if (n === q) return 100;
  if (n.startsWith(q)) return 80;
  if (n.includes(q)) return 60;
  return 20;
}

function alternatives(items, primary) {
  const seen = new Set([identity(primary)]);
  return items.filter((item) => {
    const key = identity(item);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

/**
 * Merge newly streamed results into the compact cumulative result list.
 * @param {Array<any>} current
 * @param {Array<any>} incoming
 * @param {string} query
 * @returns {Array<any>}
 */
export function mergeSearchResults(current, incoming, query) {
  const merged = current.map((item) => item.alternateSources
    ? { ...item, alternateSources: [...item.alternateSources] }
    : { ...item });
  const known = new Set();
  for (const item of merged) {
    known.add(identity(item));
    for (const alternate of item.alternateSources || []) known.add(identity(alternate));
  }

  for (const value of incoming) {
    if (known.has(identity(value))) continue;
    const item = { ...value, score: value.score || score(query, value.name || '') };
    const match = merged.find((candidate) => sameBook(candidate, item));
    if (!match) {
      item.alternateSources = alternatives(item.alternateSources || [], item);
      merged.push(item);
      known.add(identity(item));
      for (const alternate of item.alternateSources) known.add(identity(alternate));
      continue;
    }

    const matchScore = match.score || score(query, match.name || '');
    const promote = item.score > matchScore ||
      (item.score === matchScore && (item.coverUrl || '').length > (match.coverUrl || '').length);
    if (promote) {
      const index = merged.indexOf(match);
      item.alternateSources = alternatives([
        ...(match.alternateSources || []),
        { sourceUrl: match.sourceUrl, bookUrl: match.bookUrl, sourceName: match.sourceName },
        ...(item.alternateSources || []),
      ], item);
      merged[index] = item;
    } else {
      match.alternateSources = alternatives([
        ...(match.alternateSources || []),
        { sourceUrl: item.sourceUrl, bookUrl: item.bookUrl, sourceName: item.sourceName },
        ...(item.alternateSources || []),
      ], match);
    }
    known.add(identity(item));
    for (const alternate of item.alternateSources || []) known.add(identity(alternate));
  }

  return merged.sort((left, right) => {
    const scoreDifference = (right.score || score(query, right.name || '')) -
      (left.score || score(query, left.name || ''));
    return scoreDifference || (left.name || '').length - (right.name || '').length;
  });
}

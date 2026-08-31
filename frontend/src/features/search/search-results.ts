import type { AltSource, SearchResult } from '../../api/search';

const authorPrefixes = ['作者：', '作者:', '作 者 ：'];
const identity = ({ sourceId, bookUrl }: Pick<AltSource, 'sourceId' | 'bookUrl'>) => `${sourceId}\u0000${bookUrl}`;
const normalizeName = (name = '') => name.trim();

function normalizeAuthor(author = '') {
  let value = author.trim();
  const prefix = authorPrefixes.find((item) => value.startsWith(item));
  if (prefix) value = value.slice(prefix.length).trim();
  return value;
}

function sameBook(left: SearchResult, right: SearchResult) {
  if (normalizeName(left.name) !== normalizeName(right.name)) return false;
  const leftAuthor = normalizeAuthor(left.author);
  const rightAuthor = normalizeAuthor(right.author);
  return !leftAuthor || !rightAuthor || leftAuthor === rightAuthor;
}

function relevance(query: string, name: string, author: string) {
  const normalizedQuery = query.trim();
  if (!normalizedQuery) return 0;

  const normalizedName = normalizeName(name);
  const normalizedAuthor = normalizeAuthor(author);
  if (normalizedName === normalizedQuery) return 100;
  if (normalizedAuthor === normalizedQuery) return 90;
  if (normalizedName.startsWith(normalizedQuery)) return 80;
  if (normalizedAuthor.startsWith(normalizedQuery)) return 70;
  if (normalizedName.includes(normalizedQuery)) return 60;
  if (normalizedAuthor.includes(normalizedQuery)) return 50;
  return 20;
}

function alternatives(items: AltSource[], primary: Pick<SearchResult, 'sourceId' | 'bookUrl'>) {
  const seen = new Set([identity(primary)]);
  return items.filter((item) => {
    const key = identity(item);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function mergeSearchResults(current: SearchResult[], incoming: SearchResult[], query: string): SearchResult[] {
  const merged = current.map((item) => item.alternateSources ? { ...item, alternateSources: [...item.alternateSources] } : { ...item });
  const known = new Set<string>();
  for (const item of merged) {
    known.add(identity(item));
    for (const alternate of item.alternateSources ?? []) known.add(identity(alternate));
  }

  for (const value of incoming) {
    if (known.has(identity(value))) continue;
    const item: SearchResult = { ...value, score: value.score || relevance(query, value.name || '', value.author || '') };
    const match = merged.find((candidate) => sameBook(candidate, item));
    if (!match) {
      item.alternateSources = alternatives(item.alternateSources ?? [], item);
      merged.push(item);
      known.add(identity(item));
      for (const alternate of item.alternateSources) known.add(identity(alternate));
      continue;
    }

    const matchScore = match.score || relevance(query, match.name || '', match.author || '');
    const promote = (item.score ?? 0) > matchScore || ((item.score ?? 0) === matchScore && item.coverUrl.length > match.coverUrl.length);
    if (promote) {
      const index = merged.indexOf(match);
      item.shelfBookId ||= match.shelfBookId;
      item.alternateSources = alternatives([
        ...(match.alternateSources ?? []),
        { sourceId: match.sourceId, sourceUrl: match.sourceUrl, bookUrl: match.bookUrl, sourceName: match.sourceName, sourceGroup: match.sourceGroup, capabilities: match.capabilities },
        ...(item.alternateSources ?? []),
      ], item);
      merged[index] = item;
    } else {
      match.shelfBookId ||= item.shelfBookId;
      match.alternateSources = alternatives([
        ...(match.alternateSources ?? []),
        { sourceId: item.sourceId, sourceUrl: item.sourceUrl, bookUrl: item.bookUrl, sourceName: item.sourceName, sourceGroup: item.sourceGroup, capabilities: item.capabilities },
        ...(item.alternateSources ?? []),
      ], match);
    }
    known.add(identity(item));
    for (const alternate of item.alternateSources ?? []) known.add(identity(alternate));
  }

  return merged.sort((left, right) => {
    const difference = (right.score || relevance(query, right.name, right.author)) - (left.score || relevance(query, left.name, left.author));
    return difference || left.name.length - right.name.length;
  });
}

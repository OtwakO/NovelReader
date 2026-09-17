import { convertChineseTexts } from '../../api/system';
import type { ReadingContent } from '../../api/reader';
import { mapStructuredText, structuredTextValues } from '../../api/structured-prose';
import type { Chapter } from '../../api/models';
import type { ChineseConversionMode } from './reader-preferences';

export interface ConvertedReaderDisplay<T = ReadingContent> {
  chapters: Chapter[];
  content: T | null;
}

export async function convertReaderDisplay<T extends ReadingContent = ReadingContent>(
  chapters: Chapter[],
  content: T | null,
  mode: ChineseConversionMode,
): Promise<ConvertedReaderDisplay<T>> {
  if (mode === 'original') return { chapters, content };

  const chapterTitleCount = chapters.length;
  const contentTexts = !content ? [] : content.version === 2
    ? structuredTextValues(content.document)
    : [content.document.title, ...content.document.blocks.flatMap(block => block.kind === 'paragraph' ? [block.text] : block.alt ? [block.alt] : [])];
  const expectedCount = chapterTitleCount + contentTexts.length;
  const converted = await convertChineseTexts(mode, [...chapters.map(chapter => chapter.title), ...contentTexts]);
  if (converted.length !== expectedCount) {
    throw new Error('Chinese conversion response length did not match the request');
  }
  let cursor = 0;
  const convertedChapters = chapters.map(chapter => ({ ...chapter, title: converted[cursor++] ?? chapter.title }));
  if (!content) return { chapters: convertedChapters, content: null };

  if (content.version === 2) {
    return { chapters: convertedChapters, content: { ...content, document: mapStructuredText(content.document, text => converted[cursor++] ?? text) } };
  }
  const title = converted[cursor++] ?? content.document.title;
  const blocks = content.document.blocks.map(block => {
    if (block.kind === 'paragraph') return { ...block, text: converted[cursor++] ?? block.text };
    if (block.alt) return { ...block, alt: converted[cursor++] ?? block.alt };
    return block;
  });

  return { chapters: convertedChapters, content: { ...content, document: { ...content.document, title, blocks } } };
}

/** Reader-owned memoization: canonical objects stay unchanged; discarded chapters are collectible. */
export function createReaderDisplayConverter() {
  const catalogs = new WeakMap<Chapter[], Map<ChineseConversionMode, Promise<ConvertedReaderDisplay>>>();
  const documents = new WeakMap<ReadingContent, Map<ChineseConversionMode, Promise<ConvertedReaderDisplay>>>();

  function cached<T extends object>(cache: WeakMap<T, Map<ChineseConversionMode, Promise<ConvertedReaderDisplay>>>, key: T, mode: ChineseConversionMode, convert: () => Promise<ConvertedReaderDisplay>) {
    let modes = cache.get(key);
    if (!modes) { modes = new Map(); cache.set(key, modes); }
    const existing = modes.get(mode);
    if (existing) return existing;
    const pending = convert().catch(error => { modes.delete(mode); throw error; });
    modes.set(mode, pending);
    return pending;
  }

  return async (chapters: Chapter[], content: ReadingContent | null, mode: ChineseConversionMode): Promise<ConvertedReaderDisplay> => {
    if (mode === 'original') return { chapters, content };
    const [catalog, document] = await Promise.all([
      cached(catalogs, chapters, mode, () => convertReaderDisplay<ReadingContent>(chapters, null, mode)),
      content ? cached(documents, content, mode, () => convertReaderDisplay([], content, mode)) : null,
    ]);
    return { chapters: catalog.chapters, content: document?.content ?? null };
  };
}

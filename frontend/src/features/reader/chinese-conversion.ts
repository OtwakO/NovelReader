import { convertChineseTexts } from '../../api/system';
import type { ChapterContent } from '../../api/reader';
import type { Chapter } from '../../api/models';
import type { ChineseConversionMode } from './reader-preferences';

export interface ConvertedReaderDisplay {
  chapters: Chapter[];
  content: ChapterContent | null;
}

export async function convertReaderDisplay(
  chapters: Chapter[],
  content: ChapterContent | null,
  mode: ChineseConversionMode,
): Promise<ConvertedReaderDisplay> {
  if (mode === 'original') return { chapters, content };

  const chapterTitleCount = chapters.length;
  const contentTexts = content
    ? [content.document.title, ...content.document.blocks.flatMap(block => block.kind === 'paragraph' ? [block.text] : block.alt ? [block.alt] : [])]
    : [];
  const expectedCount = chapterTitleCount + contentTexts.length;
  const converted = await convertChineseTexts(mode, [...chapters.map(chapter => chapter.title), ...contentTexts]);
  if (converted.length !== expectedCount) {
    throw new Error('Chinese conversion response length did not match the request');
  }
  let cursor = 0;
  const convertedChapters = chapters.map(chapter => ({ ...chapter, title: converted[cursor++] ?? chapter.title }));
  if (!content) return { chapters: convertedChapters, content: null };

  const title = converted[cursor++] ?? content.document.title;
  const blocks = content.document.blocks.map(block => {
    if (block.kind === 'paragraph') return { ...block, text: converted[cursor++] ?? block.text };
    if (block.alt) return { ...block, alt: converted[cursor++] ?? block.alt };
    return block;
  });

  return { chapters: convertedChapters, content: { ...content, document: { ...content.document, title, blocks } } };
}

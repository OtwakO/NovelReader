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
    ? [content.title, ...content.paragraphs, ...content.blocks.filter(block => block.type === 'text').map(block => block.text)]
    : [];
  const expectedCount = chapterTitleCount + contentTexts.length;
  const converted = await convertChineseTexts(mode, [...chapters.map(chapter => chapter.title), ...contentTexts]);
  if (converted.length !== expectedCount) {
    throw new Error('Chinese conversion response length did not match the request');
  }
  let cursor = 0;
  const convertedChapters = chapters.map(chapter => ({ ...chapter, title: converted[cursor++] ?? chapter.title }));
  if (!content) return { chapters: convertedChapters, content: null };

  const title = converted[cursor++] ?? content.title;
  const paragraphs = content.paragraphs.map(paragraph => converted[cursor++] ?? paragraph);
  const blocks = content.blocks.map(block => block.type === 'text'
    ? { ...block, text: converted[cursor++] ?? block.text }
    : block);

  return { chapters: convertedChapters, content: { ...content, title, paragraphs, blocks } };
}

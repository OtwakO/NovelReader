import type { ChapterContent } from '../../api/reader';
import type { Chapter } from '../../api/models';
import type { ChineseConversionMode } from './reader-preferences';

type Converter = (text: string) => string;

const converterLoads = new Map<Exclude<ChineseConversionMode, 'original'>, Promise<Converter>>();

function loadConverter(mode: Exclude<ChineseConversionMode, 'original'>): Promise<Converter> {
  const existing = converterLoads.get(mode);
  if (existing) return existing;
  const loading = mode === 'traditional'
    ? import('opencc-js/cn2t').then(({ Converter }) => Converter({ from: 'cn', to: 't' }))
    : import('opencc-js/t2cn').then(({ Converter }) => Converter({ from: 't', to: 'cn' }));
  converterLoads.set(mode, loading);
  return loading;
}

export async function convertChineseText(text: string, mode: ChineseConversionMode): Promise<string> {
  if (mode === 'original' || !text) return text;
  return (await loadConverter(mode))(text);
}

export async function convertChapterTitles(chapters: Chapter[], mode: ChineseConversionMode): Promise<Chapter[]> {
  if (mode === 'original') return chapters;
  const convert = await loadConverter(mode);
  return chapters.map(chapter => ({ ...chapter, title: convert(chapter.title) }));
}

export async function convertChapterContent(content: ChapterContent, mode: ChineseConversionMode): Promise<ChapterContent> {
  if (mode === 'original') return content;
  const convert = await loadConverter(mode);
  return {
    ...content,
    title: convert(content.title),
    paragraphs: content.paragraphs.map(convert),
    blocks: content.blocks.map(block => block.type === 'text' ? { ...block, text: convert(block.text) } : block),
  };
}

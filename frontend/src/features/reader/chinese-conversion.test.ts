import { describe, expect, test } from 'vitest';
import { convertChineseText, convertChapterContent, convertChapterTitles } from './chinese-conversion';

const content = {
  title: '软件与网络',
  paragraphs: ['后台保存原文，阅读时转换。'],
  blocks: [
    { type: 'text' as const, text: '这里有一个文本区块。' },
    { type: 'image' as const, index: 0 },
  ],
  offlineCopy: false,
};

describe('Reader Chinese conversion', () => {
  test('leaves text untouched in original mode', async () => {
    expect(await convertChineseText('漢字与汉字', 'original')).toBe('漢字与汉字');
    expect(await convertChapterContent(content, 'original')).toBe(content);
  });

  test('converts in both directions with generic Traditional output', async () => {
    expect(await convertChineseText('软件与网络', 'traditional')).toBe('軟件與網絡');
    expect(await convertChineseText('軟件與網絡', 'simplified')).toBe('软件与网络');
  });

  test('converts chapter titles and textual content without touching images', async () => {
    const converted = await convertChapterContent(content, 'traditional');
    expect(converted).toMatchObject({ title: '軟件與網絡', paragraphs: ['後臺保存原文，閱讀時轉換。'] });
    expect(converted.blocks.at(0)).toEqual({ type: 'text', text: '這裏有一個文本區塊。' });
    expect(converted.blocks.at(1)).toBe(content.blocks[1]);

    const chapters = [{ id: '1', bookId: 'book', index: 0, title: '第一章 网络', url: '/1', isVolume: false }];
    expect((await convertChapterTitles(chapters, 'traditional')).at(0)?.title).toBe('第一章 網絡');
    expect(chapters.at(0)?.title).toBe('第一章 网络');
  });
});

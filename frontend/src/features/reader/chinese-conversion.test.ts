import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChapterContent } from '../../api/reader';
import type { Chapter } from '../../api/models';
import { convertChineseTexts } from '../../api/system';
import { convertReaderDisplay } from './chinese-conversion';

vi.mock('../../api/system', () => ({ convertChineseTexts: vi.fn() }));

const chapters: Chapter[] = [{ id: 'chapter-1', bookId: 'book-1', index: 0, title: '软件后台', url: '/1', isVolume: false }];
const content: ChapterContent = {
  title: '这里的软件',
  paragraphs: ['鼠标和硬盘'],
  blocks: [
    { type: 'text', text: '数据库连接' },
    { type: 'image', index: 0 },
  ],
  offlineCopy: false,
};

beforeEach(() => vi.mocked(convertChineseTexts).mockReset());

describe('reader Chinese conversion', () => {
  it('returns canonical data unchanged in original mode', async () => {
    const display = await convertReaderDisplay(chapters, content, 'original');
    expect(display).toEqual({ chapters, content });
    expect(convertChineseTexts).not.toHaveBeenCalled();
  });

  it('converts one ordered display-only batch through the backend', async () => {
    vi.mocked(convertChineseTexts).mockResolvedValue(['軟體後臺', '這裡的軟體', '滑鼠和硬碟', '資料庫連線']);

    const display = await convertReaderDisplay(chapters, content, 'traditional');

    expect(convertChineseTexts).toHaveBeenCalledWith('traditional', ['软件后台', '这里的软件', '鼠标和硬盘', '数据库连接']);
    expect(display.chapters[0]?.title).toBe('軟體後臺');
    expect(display.content?.title).toBe('這裡的軟體');
    expect(display.content?.paragraphs).toEqual(['滑鼠和硬碟']);
    expect(display.content?.blocks).toEqual([
      { type: 'text', text: '資料庫連線' },
      { type: 'image', index: 0 },
    ]);
    expect(chapters[0]?.title).toBe('软件后台');
    expect(content.paragraphs[0]).toBe('鼠标和硬盘');
  });

  it('rejects incomplete conversion responses instead of mixing modes', async () => {
    vi.mocked(convertChineseTexts).mockResolvedValue(['軟體後臺']);
    await expect(convertReaderDisplay(chapters, content, 'traditional')).rejects.toThrow('response length');
  });

  it('also validates chapter-only conversion responses', async () => {
    vi.mocked(convertChineseTexts).mockResolvedValue([]);
    await expect(convertReaderDisplay(chapters, null, 'traditional')).rejects.toThrow('response length');
  });
});

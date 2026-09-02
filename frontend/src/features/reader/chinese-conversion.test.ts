import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChapterContent } from '../../api/reader';
import type { Chapter } from '../../api/models';
import { convertChineseTexts } from '../../api/system';
import { convertReaderDisplay } from './chinese-conversion';

vi.mock('../../api/system', () => ({ convertChineseTexts: vi.fn() }));

const chapters: Chapter[] = [{ id: 'chapter-1', bookId: 'book-1', index: 0, title: '软件后台', url: '/1', isVolume: false }];
const content: ChapterContent = {
  version: 1,
  document: {
    kind: 'prose',
    title: '这里的软件',
    blocks: [
      { kind: 'paragraph', text: '数据库连接' },
      { kind: 'image', resource: { href: '/api/books/book-1/chapters/0/images/0' }, alt: '鼠标和硬盘' },
    ],
  },
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
    vi.mocked(convertChineseTexts).mockResolvedValue(['軟體後臺', '這裡的軟體', '資料庫連線', '滑鼠和硬碟']);

    const display = await convertReaderDisplay(chapters, content, 'traditional');

    expect(convertChineseTexts).toHaveBeenCalledWith('traditional', ['软件后台', '这里的软件', '数据库连接', '鼠标和硬盘']);
    expect(display.chapters[0]?.title).toBe('軟體後臺');
    expect(display.content?.document.title).toBe('這裡的軟體');
    expect(display.content?.document.blocks).toEqual([
      { kind: 'paragraph', text: '資料庫連線' },
      { kind: 'image', resource: { href: '/api/books/book-1/chapters/0/images/0' }, alt: '滑鼠和硬碟' },
    ]);
    expect(chapters[0]?.title).toBe('软件后台');
    expect(content.document.blocks[0]).toEqual({ kind: 'paragraph', text: '数据库连接' });
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

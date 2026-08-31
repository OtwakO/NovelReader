import type { Book, Chapter } from './models';
import { API_BASE, request, requestForm } from './transport';
export type { Chapter } from './models';
export type ChapterContentBlock = { type: 'text'; text: string } | { type: 'image'; index: number };
export interface ChapterContent { title: string; paragraphs: string[]; blocks: ChapterContentBlock[]; offlineCopy: boolean }
export interface Bookmark { id: string; bookId: string; chapterIndex: number; chapterTitle: string; position: number; note: string; orphaned: boolean; createdAt: number }
export interface Font { id: string; name: string; fileName: string; fileSize: number }
export type CatalogResult = { state: 'ready'; chapters: Chapter[] } | { state: 'syncing' };
export interface CatalogPollingOptions { retry?: boolean; isCurrent?: () => boolean }

const catalogPollDelays = [500, 1000, 1500, 2000];

export function getCatalog(bookId: string, retry = false): Promise<CatalogResult> {
  return request<Chapter[] | { state?: unknown }>(`/books/${encodeURIComponent(bookId)}/chapters${retry ? '/sync' : ''}`, retry ? { method: 'POST' } : undefined).then((value) => {
    if (Array.isArray(value)) return { state: 'ready', chapters: value };
    if (value?.state === 'syncing') return { state: 'syncing' };
    throw new Error('Invalid catalog response');
  });
}

export async function waitForCatalog(bookId: string, options: CatalogPollingOptions = {}): Promise<Chapter[]> {
  let attempt = 0;
  let result = await getCatalog(bookId, Boolean(options.retry));
  while (result.state === 'syncing') {
    if (options.isCurrent && !options.isCurrent()) throw new Error('Catalog request superseded');
    const delay = catalogPollDelays[Math.min(attempt, catalogPollDelays.length - 1)];
    await new Promise((resolve) => setTimeout(resolve, delay));
    if (options.isCurrent && !options.isCurrent()) throw new Error('Catalog request superseded');
    result = await getCatalog(bookId);
    attempt += 1;
  }
  return result.chapters;
}
export function getChapterContent(bookId: string, chapterIdx: number) {
  return request<Record<string, unknown>>(`/books/${encodeURIComponent(bookId)}/chapters/${chapterIdx}/content`).then((data) => ({
    title: String(data.title || data.Title || ''),
    paragraphs: (data.paragraphs || data.Paragraphs || []) as string[],
    blocks: Array.isArray(data.blocks) ? data.blocks.filter((block: unknown) => {
      if (!block || typeof block !== 'object') return false;
      const value = block as Record<string, unknown>;
      return (value.type === 'text' && typeof value.text === 'string') || (value.type === 'image' && Number.isInteger(value.index) && Number(value.index) >= 0);
    }) as ChapterContentBlock[] : [],
    offlineCopy: Boolean(data.offlineCopy),
  }));
}
export function getChapterImageUrl(bookId: string, chapterIdx: number, imageIdx: number) { return `${API_BASE}/books/${encodeURIComponent(bookId)}/chapters/${chapterIdx}/images/${imageIdx}`; }
export function switchBookSource(bookId: string, sourceId: string, sourceUrl: string, bookUrl: string) { return request<{ book: Book; mapping: 'title' | 'index' }>(`/books/${encodeURIComponent(bookId)}/source`, { method: 'PUT', body: JSON.stringify({ sourceId, sourceUrl, bookUrl }) }); }
export function listBookmarks(bookId: string) { return request<Bookmark[]>(`/books/${encodeURIComponent(bookId)}/bookmarks`); }
export function addBookmark(bookId: string, bookmark: { id: string; sourceId: string; stateVersion: number; chapterIndex: number; position: number; note: string }) { return request<Bookmark>(`/books/${encodeURIComponent(bookId)}/bookmarks`, { method: 'POST', body: JSON.stringify(bookmark) }); }
export function deleteBookmark(bookId: string, bookmarkId: string) { return request<{ status: string }>(`/books/${encodeURIComponent(bookId)}/bookmarks/${encodeURIComponent(bookmarkId)}`, { method: 'DELETE' }); }
export function saveProgress(bookId: string, sourceId: string, stateVersion: number, chapterIndex: number, position: number) { return request<{ status: string; stateVersion: number }>(`/books/${encodeURIComponent(bookId)}/progress`, { method: 'PUT', body: JSON.stringify({ sourceId, stateVersion, chapterIndex, position }) }); }
export function listFonts() { return request<Font[]>('/fonts'); }
export function uploadFont(file: File, name: string) {
  const form = new FormData(); form.append('file', file); form.append('name', name);
  return requestForm<Font>('/fonts', form);
}
export function deleteFont(id: string) { return request<{ status: string }>(`/fonts/${encodeURIComponent(id)}`, { method: 'DELETE' }); }
export function getFontUrl(id: string) { return `${API_BASE}/fonts/${encodeURIComponent(id)}/file`; }

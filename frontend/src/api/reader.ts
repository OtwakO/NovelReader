import type { Book, Chapter } from './models';
import { API_BASE, request, requestForm } from './transport';
export type { Chapter } from './models';
export type ChapterContentBlock = { type: 'text'; text: string } | { type: 'image'; index: number };
export interface ChapterContent { title: string; paragraphs: string[]; blocks: ChapterContentBlock[]; offlineCopy: boolean }
export interface Bookmark { id: string; bookId: string; chapterIndex: number; chapterTitle: string; position: number; note: string; orphaned: boolean; createdAt: number }
export interface Font { id: string; name: string; fileName: string; fileSize: number }

export function getChapters(bookId: string) { return request<Chapter[]>(`/books/${encodeURIComponent(bookId)}/chapters`); }
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
export function switchBookSource(bookId: string, sourceUrl: string, bookUrl: string) { return request<{ book: Book; mapping: 'title' | 'index' }>(`/books/${encodeURIComponent(bookId)}/source`, { method: 'PUT', body: JSON.stringify({ sourceUrl, bookUrl }) }); }
export function listBookmarks(bookId: string) { return request<Bookmark[]>(`/books/${encodeURIComponent(bookId)}/bookmarks`); }
export function addBookmark(bookId: string, bookmark: { id: string; sourceUrl: string; stateVersion: number; chapterIndex: number; position: number; note: string }) { return request<Bookmark>(`/books/${encodeURIComponent(bookId)}/bookmarks`, { method: 'POST', body: JSON.stringify(bookmark) }); }
export function deleteBookmark(bookId: string, bookmarkId: string) { return request<{ status: string }>(`/books/${encodeURIComponent(bookId)}/bookmarks/${encodeURIComponent(bookmarkId)}`, { method: 'DELETE' }); }
export function saveProgress(bookId: string, sourceUrl: string, stateVersion: number, chapterIndex: number, position: number) { return request<{ status: string; stateVersion: number }>(`/books/${encodeURIComponent(bookId)}/progress`, { method: 'PUT', body: JSON.stringify({ sourceUrl, stateVersion, chapterIndex, position }) }); }
export function listFonts() { return request<Font[]>('/fonts'); }
export function uploadFont(file: File, name: string) {
  const form = new FormData(); form.append('file', file); form.append('name', name);
  return requestForm<Font>('/fonts', form);
}
export function deleteFont(id: string) { return request<{ status: string }>(`/fonts/${encodeURIComponent(id)}`, { method: 'DELETE' }); }
export function getFontUrl(id: string) { return `${API_BASE}/fonts/${encodeURIComponent(id)}/file`; }

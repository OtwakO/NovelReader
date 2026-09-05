import type { Book, Chapter } from './models';
import { API_BASE, request, requestForm } from './transport';
export type { Chapter } from './models';
export interface ContentResourceReference { href: string; mediaType?: string }
export type ProseBlock =
  | { kind: 'paragraph'; text: string }
  | { kind: 'image'; resource: ContentResourceReference; alt?: string };
export interface ProseDocument { kind: 'prose'; title: string; blocks: ProseBlock[] }
export interface ChapterContent { version: 1; document: ProseDocument; offlineCopy: boolean }
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
export function getChapterContent(bookId: string, chapterIdx: number, signal?: AbortSignal): Promise<ChapterContent> {
  return request<Record<string, unknown>>(`/books/${encodeURIComponent(bookId)}/chapters/${chapterIdx}/content`, { signal }).then(parseChapterContent);
}

function parseChapterContent(data: Record<string, unknown>): ChapterContent {
  if (data.version !== 1 || !data.document || typeof data.document !== 'object') throw new Error('Invalid chapter content response');
  const document = data.document as Record<string, unknown>;
  if (document.kind !== 'prose' || typeof document.title !== 'string' || !Array.isArray(document.blocks)) throw new Error('Invalid prose document');
  return {
    version: 1,
    document: {
      kind: 'prose',
      title: document.title,
      blocks: document.blocks.map(parseProseBlock),
    },
    offlineCopy: Boolean(data.offlineCopy),
  };
}

function parseProseBlock(block: unknown): ProseBlock {
  if (!block || typeof block !== 'object') throw new Error('Invalid prose block');
  const value = block as Record<string, unknown>;
  if (value.kind === 'paragraph' && typeof value.text === 'string') return { kind: 'paragraph', text: value.text };
  if (value.kind === 'image' && value.resource && typeof value.resource === 'object') {
    const resource = value.resource as Record<string, unknown>;
    if (typeof resource.href !== 'string' || !resource.href.startsWith(`${API_BASE}/`)) throw new Error('Invalid content resource');
    return {
      kind: 'image',
      resource: { href: resource.href, ...(typeof resource.mediaType === 'string' ? { mediaType: resource.mediaType } : {}) },
      ...(typeof value.alt === 'string' && value.alt.trim() ? { alt: value.alt.trim() } : {}),
    };
  }
  throw new Error('Invalid prose block');
}
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

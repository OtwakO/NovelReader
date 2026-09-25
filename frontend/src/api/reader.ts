import { parseStructuredChapterContent, type StructuredChapterContent } from './structured-prose';
import type { Book } from './models';
import { API_BASE, request, requestForm } from './transport';
import { parseContentResource, type ContentResourceReference } from './content-resource';
import { parseChapterCatalog, type ChapterCatalog } from './chapter-catalog';
export type { Chapter } from './models';
export type { ContentResourceReference } from './content-resource';
export type ProseBlock =
  | { kind: 'paragraph'; text: string }
  | { kind: 'image'; resource: ContentResourceReference; alt?: string };
export interface ProseDocument { kind: 'prose'; title: string; blocks: ProseBlock[] }
export interface ChapterContent { freshForMs?: number; version: 1; contentRevision: number; document: ProseDocument; offlineCopy: boolean }
export type ReadingContent = ChapterContent | StructuredChapterContent;
export interface Bookmark { id: string; bookId: string; contentRevision: number; chapterIndex: number; chapterTitle: string; position: number; note: string; orphaned: boolean; createdAt: number }
export interface Font { id: string; name: string; fileName: string; fileSize: number }
export type { ChapterCatalog } from './chapter-catalog';
export type CatalogResult = ({ state: 'ready' } & ChapterCatalog) | { state: 'syncing' };
export interface CatalogPollingOptions { retry?: boolean; isCurrent?: () => boolean }

const catalogPollDelays = [500, 1000, 1500, 2000];

export function getCatalog(bookId: string, retry = false): Promise<CatalogResult> {
  return request<{ chapters?: unknown; contentRevision?: unknown; navigation?: unknown; state?: unknown }>(`/books/${encodeURIComponent(bookId)}/chapters${retry ? '/sync' : ''}`, retry ? { method: 'POST' } : undefined).then((value) => {
    if (Array.isArray(value?.chapters) && typeof value.contentRevision === 'number' && Number.isSafeInteger(value.contentRevision) && value.contentRevision >= 0) return { state: 'ready', ...parseChapterCatalog(value.chapters, value.contentRevision, value.navigation) };
    if (value?.state === 'syncing') return { state: 'syncing' };
    throw new Error('Invalid catalog response');
  });
}

export async function waitForCatalog(bookId: string, options: CatalogPollingOptions = {}): Promise<ChapterCatalog> {
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
  return { chapters: result.chapters, contentRevision: result.contentRevision, ...(result.navigation ? { navigation: result.navigation } : {}) };
}
export function getChapterContent(bookId: string, chapterIdx: number, contentRevision: number, signal?: AbortSignal): Promise<ReadingContent> {
  return request<Record<string, unknown>>(`/books/${encodeURIComponent(bookId)}/chapters/${chapterIdx}/content?contentRevision=${contentRevision}`, { signal }).then(parseChapterContent);
}

function parseChapterContent(data: Record<string, unknown>): ReadingContent {
  if (data.version === 2) return parseStructuredChapterContent(data);
  if (typeof data.contentRevision !== 'number' || !Number.isSafeInteger(data.contentRevision) || data.contentRevision < 0 || data.version !== 1 || !data.document || typeof data.document !== 'object') throw new Error('Invalid chapter content response');
  const document = data.document as Record<string, unknown>;
  if (document.kind !== 'prose' || typeof document.title !== 'string' || !Array.isArray(document.blocks)) throw new Error('Invalid prose document');
  if (data.freshForMs !== undefined && (typeof data.freshForMs !== 'number' || !Number.isSafeInteger(data.freshForMs) || data.freshForMs < 0)) throw new Error('Invalid chapter freshness');
  return {
    ...(typeof data.freshForMs === 'number' ? { freshForMs: data.freshForMs } : {}),
    version: 1,
    contentRevision: data.contentRevision,
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
    const resource = parseContentResource(value.resource);
    return {
      kind: 'image',
      resource,
      ...(typeof value.alt === 'string' && value.alt.trim() ? { alt: value.alt.trim() } : {}),
    };
  }
  throw new Error('Invalid prose block');
}
export function switchBookSource(bookId: string, sourceId: string, sourceUrl: string, bookUrl: string) { return request<{ book: Book; mapping: 'title' | 'index' }>(`/books/${encodeURIComponent(bookId)}/source`, { method: 'PUT', body: JSON.stringify({ sourceId, sourceUrl, bookUrl }) }); }
export function listBookmarks(bookId: string) { return request<Bookmark[]>(`/books/${encodeURIComponent(bookId)}/bookmarks`); }
export function addBookmark(bookId: string, bookmark: { id: string; contentRevision: number; stateVersion: number; chapterIndex: number; position: number; note: string }) { return request<Bookmark & { stateVersion: number }>(`/books/${encodeURIComponent(bookId)}/bookmarks`, { method: 'POST', body: JSON.stringify(bookmark) }); }
export function deleteBookmark(bookId: string, bookmarkId: string, contentRevision: number, stateVersion: number) { return request<{ status: string; stateVersion: number }>(`/books/${encodeURIComponent(bookId)}/bookmarks/${encodeURIComponent(bookmarkId)}`, { method: 'DELETE', body: JSON.stringify({ contentRevision, stateVersion }) }); }
export function saveProgress(bookId: string, contentRevision: number, stateVersion: number, chapterIndex: number, position: number) { return request<{ status: string; stateVersion: number }>(`/books/${encodeURIComponent(bookId)}/progress`, { method: 'PUT', body: JSON.stringify({ contentRevision, stateVersion, chapterIndex, position }) }); }
export function listFonts() { return request<Font[]>('/fonts'); }
export function uploadFont(file: File, name: string) {
  const form = new FormData(); form.append('file', file); form.append('name', name);
  return requestForm<Font>('/fonts', form);
}
export function deleteFont(id: string) { return request<{ status: string }>(`/fonts/${encodeURIComponent(id)}`, { method: 'DELETE' }); }
export function getFontUrl(id: string) { return `${API_BASE}/fonts/${encodeURIComponent(id)}/file`; }

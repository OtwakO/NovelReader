import type { AltSource, Book, Chapter, SearchResult } from './models';
import { request } from './transport';
export type { Book } from './models';
export interface BookCandidate {
  id?: string; name: string; author?: string; coverUrl?: string; intro?: string; kind?: string; lastChapter?: string;
  updateTime?: string; wordCount?: string; sourceName?: string; sourceUrl: string; bookUrl: string; alternateSources?: AltSource[];
}
export interface BookPreview { book: Omit<Book, 'id' | 'durChapterIndex' | 'durChapterPos' | 'totalChapterNum' | 'stateVersion'>; chapters: Chapter[] }
export interface ReadableBookResult { book: Book; firstReadableChapter: number }

export function listBooks() { return request<Book[]>('/books'); }
export function getBook(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}`); }
export function addBook(book: Partial<Book>) { return request<Book>('/books', { method: 'POST', body: JSON.stringify(book) }); }
function candidatePayload(data: BookCandidate): BookCandidate {
  return {
    ...(data.id ? { id: data.id } : {}),
    name: data.name,
    ...(data.author !== undefined ? { author: data.author } : {}),
    ...(data.coverUrl !== undefined ? { coverUrl: data.coverUrl } : {}),
    ...(data.intro !== undefined ? { intro: data.intro } : {}),
    ...(data.kind !== undefined ? { kind: data.kind } : {}),
    ...(data.lastChapter !== undefined ? { lastChapter: data.lastChapter } : {}),
    ...(data.updateTime !== undefined ? { updateTime: data.updateTime } : {}),
    ...(data.wordCount !== undefined ? { wordCount: data.wordCount } : {}),
    ...(data.sourceName !== undefined ? { sourceName: data.sourceName } : {}),
    sourceUrl: data.sourceUrl,
    bookUrl: data.bookUrl,
    ...(data.alternateSources !== undefined ? { alternateSources: data.alternateSources } : {}),
  };
}

export function previewBook(data: BookCandidate) { return request<BookPreview>('/books/preview', { method: 'POST', body: JSON.stringify(candidatePayload(data)) }); }
export function shelveBook(data: BookCandidate & { id: string }) { return request<Book>('/books/shelve', { method: 'POST', body: JSON.stringify(candidatePayload(data)) }); }
export function addReadableBook(data: BookCandidate & { id: string }) { return request<ReadableBookResult>('/books/readable', { method: 'POST', body: JSON.stringify(candidatePayload(data)) }); }
export function enrichBook(data: BookCandidate & { id: string }) { return request<Book>('/books/enrich', { method: 'POST', body: JSON.stringify(candidatePayload(data)) }); }
export async function addSearchResultToShelf(result: SearchResult) {
  const preview = await previewBook(result);
  const id = crypto.randomUUID?.() ?? (Date.now().toString(36) + Math.random().toString(36).slice(2));
  return shelveBook({
    ...result,
    ...preview.book,
    id,
    sourceName: result.sourceName,
    alternateSources: result.alternateSources,
  });
}
export function mergeBookSources(id: string, sources: AltSource[]) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'POST', body: JSON.stringify({ sources }) }); }
export function clearBookSources(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'DELETE' }); }
export function deleteBook(id: string) { return request<{ status: string }>(`/books?id=${encodeURIComponent(id)}`, { method: 'DELETE' }); }

import type { AltSource, Book, LibraryBook } from './models';
import { parseChineseConversionCapability } from './system';
import { request } from './transport';
export type { Book, LibraryBook } from './models';

export function listBooks() { return request<LibraryBook[]>('/books'); }
export async function getBook(id: string): Promise<LibraryBook> {
  const book = await request<LibraryBook>(`/books/${encodeURIComponent(id)}`);
  if (book.readingContext !== undefined) {
    const context = book.readingContext;
    if (!context || typeof context !== 'object' || (context.sourceIdentity !== undefined && (typeof context.sourceIdentity !== 'string' || !context.sourceIdentity))) throw new Error('Invalid reading context');
    book.readingContext = { ...context, chineseConversion: parseChineseConversionCapability(context.chineseConversion) };
  }
  return book;
}
export function getBookSource(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}/booksource`); }
export function mergeBookSources(id: string, sources: AltSource[]) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'POST', body: JSON.stringify({ sources }) }); }
export function clearBookSources(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'DELETE' }); }
export function deleteBook(id: string, signal?: AbortSignal) { return request<{ status: string; warnings?: string[] }>(`/books?id=${encodeURIComponent(id)}`, { method: 'DELETE', signal }); }

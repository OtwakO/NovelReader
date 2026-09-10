import type { AltSource, Book, LibraryBook } from './models';
import { request } from './transport';
export type { Book, LibraryBook } from './models';

export function listBooks() { return request<LibraryBook[]>('/books'); }
export function getBook(id: string) { return request<LibraryBook>(`/books/${encodeURIComponent(id)}`); }
export function getBookSource(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}/booksource`); }
export function mergeBookSources(id: string, sources: AltSource[]) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'POST', body: JSON.stringify({ sources }) }); }
export function clearBookSources(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'DELETE' }); }
export function deleteBook(id: string) { return request<{ status: string }>(`/books?id=${encodeURIComponent(id)}`, { method: 'DELETE' }); }

import type { AltSource, Book } from './models';
import { request } from './transport';
export type { Book } from './models';

export function listBooks() { return request<Book[]>('/books'); }
export function getBook(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}`); }
export function mergeBookSources(id: string, sources: AltSource[]) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'POST', body: JSON.stringify({ sources }) }); }
export function clearBookSources(id: string) { return request<Book>(`/books/${encodeURIComponent(id)}/sources`, { method: 'DELETE' }); }
export function deleteBook(id: string) { return request<{ status: string }>(`/books?id=${encodeURIComponent(id)}`, { method: 'DELETE' }); }

// API client for NovelReader backend.
const BASE = '/api';

async function req<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

// --- Book Sources ---
export interface BookSource {
  bookSourceUrl: string;
  bookSourceName: string;
  bookSourceGroup?: string;
  enabled: boolean;
  enabledExplore: boolean;
  searchUrl?: string;
  ruleSearch?: string;
  ruleBookInfo?: string;
  ruleToc?: string;
  ruleContent?: string;
  header?: string;
  [key: string]: unknown;
}

export function listSources() {
  return req<BookSource[]>('/sources');
}

export function importSources(data: string) {
  return req<{ imported: number; total: number }>('/sources', {
    method: 'POST',
    body: data,
  });
}

export function deleteSource(url: string) {
  return req<{ status: string }>(`/sources?url=${encodeURIComponent(url)}`, {
    method: 'DELETE',
  });
}

// --- Search ---
export interface SearchResult {
  name: string;
  author: string;
  coverUrl: string;
  intro: string;
  kind: string;
  lastChapter: string;
  bookUrl: string;
  sourceUrl: string;
  sourceName: string;
}

export function searchBooks(query: string) {
  return req<SearchResult[]>(`/search?q=${encodeURIComponent(query)}`);
}

// --- Books ---
export interface Book {
  id: string;
  name: string;
  author: string;
  coverUrl: string;
  intro: string;
  kind: string;
  sourceUrl: string;
  bookUrl: string;
  lastChapter: string;
  durChapterIndex: number;
  totalChapterNum: number;
}

export function listBooks() {
  return req<Book[]>('/books');
}

export function getBook(id: string) {
  return req<Book>(`/books/${encodeURIComponent(id)}`);
}

export function addBook(book: Partial<Book>) {
  return req<Book>('/books', {
    method: 'POST',
    body: JSON.stringify(book),
  });
}

// EnrichBook adds a book and tries to fetch full info (cover, intro, chapters) from source.
// Falls back gracefully if source is unreachable.
export function enrichBook(data: {
  id: string;
  name: string;
  author?: string;
  coverUrl?: string;
  intro?: string;
  sourceUrl: string;
  bookUrl: string;
}) {
  return req<Book>('/books/enrich', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function deleteBook(id: string) {
  return req<{ status: string }>(`/books?id=${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

// --- Chapters ---
export interface Chapter {
  id: string;
  bookId: string;
  index: number;
  title: string;
  url: string;
}

export interface ChapterContent {
  title: string;
  paragraphs: string[];
}

export function getChapters(bookId: string) {
  return req<Chapter[]>(`/books/${encodeURIComponent(bookId)}/chapters`);
}

export function getChapterContent(bookId: string, chapterIdx: number) {
  return req<ChapterContent>(
    `/books/${encodeURIComponent(bookId)}/chapters/${chapterIdx}/content`
  );
}

// --- Progress ---
export function saveProgress(
  bookId: string,
  chapterIndex: number,
  position: number
) {
  return req<{ status: string }>(
    `/books/${encodeURIComponent(bookId)}/progress`,
    {
      method: 'PUT',
      body: JSON.stringify({ chapterIndex, position }),
    }
  );
}

// --- Fonts ---
export interface Font {
  id: string;
  name: string;
  fileName: string;
  fileSize: number;
}

export function listFonts() {
  return req<Font[]>('/fonts');
}

export async function uploadFont(file: File, name: string) {
  const form = new FormData();
  form.append('file', file);
  form.append('name', name);
  const res = await fetch(`${BASE}/fonts`, {
    method: 'POST',
    body: form,
  });
  if (!res.ok) throw new Error('Upload failed');
  return res.json() as Promise<Font>;
}

export function deleteFont(id: string) {
  return req<{ status: string }>(`/fonts/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export function getFontUrl(id: string) {
  return `${BASE}/fonts/${encodeURIComponent(id)}/file`;
}

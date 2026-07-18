// API client for NovelReader backend.
const BASE = '/api';

export class ExploreApiError extends Error {
  code: string;
  stage: string;
  severity: string;
  retryable: boolean;
  nextPage?: number;

  constructor(body: ExploreErrorBody) {
    super(body.message || 'Explore request failed');
    this.name = 'ExploreApiError';
    this.code = body.code || 'internal_error';
    this.stage = body.stage || 'internal';
    this.severity = body.severity || 'error';
    this.retryable = Boolean(body.retryable);
    this.nextPage = body.nextPage;
  }
}

async function req<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText })) as ExploreErrorBody;
    if (err.code) throw new ExploreApiError(err);
    throw new Error(err.error || err.message || res.statusText);
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
export interface AltSource {
  sourceUrl: string;
  bookUrl: string;
  sourceName: string;
}

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
  score?: number;
  alternateSources?: AltSource[];
}

export interface ExploreDiagnostic {
  code: string;
  stage: string;
  severity: string;
  retryable: boolean;
  message: string;
}

export interface ExploreErrorBody extends Partial<ExploreDiagnostic> {
  error?: string;
  nextPage?: number;
}

export interface ExploreSource {
  id: string;
  name: string;
  group: string;
}

export interface ExploreEntry {
  id: string;
  title: string;
  type: 'url' | 'text' | 'button' | 'toggle' | 'select' | string;
  selectable: boolean;
  value?: string;
  options?: string[];
}

export interface ExploreCatalog {
  source: ExploreSource;
  sessionId: string;
  entries: ExploreEntry[];
  diagnostics: ExploreDiagnostic[];
}

export interface ExplorePageResult {
  sourceId: string;
  sessionId: string;
  categoryId: string;
  page: number;
  nextPage: number;
  books: SearchResult[];
  exhausted: boolean;
  diagnostics: ExploreDiagnostic[];
}

export function listExploreSources(signal?: AbortSignal) {
  return req<ExploreSource[]>('/explore/sources', { signal });
}

export function openExplore(sourceId: string, signal?: AbortSignal) {
  return req<ExploreCatalog>('/explore/catalog', {
    method: 'POST', body: JSON.stringify({ sourceId }), signal,
  });
}

export function updateExploreControl(sessionId: string, controlId: string, value: string | null, signal?: AbortSignal) {
  return req<ExploreCatalog>('/explore/control', {
    method: 'POST', body: JSON.stringify({ sessionId, controlId, value }), signal,
  });
}

export function getExplorePage(sessionId: string, categoryId: string, page: number, signal?: AbortSignal) {
  return req<ExplorePageResult>('/explore/page', {
    method: 'POST', body: JSON.stringify({ sessionId, categoryId, page }), signal,
  });
}

export function searchBooks(query: string) {
  return req<SearchResult[]>(`/search?q=${encodeURIComponent(query)}`);
}

// SSE streaming search. Calls onResult per source, onDone when all complete.
// Returns an EventSource that the caller can close() to cancel.
// ponytail: no cross-source dedup — showing same book from different sources is useful (user picks source).
export function searchBooksStream(
  query: string,
  onResult: (source: string, items: SearchResult[]) => void,
  onError: (source: string, msg: string) => void,
  onDone: (total: number, sourcesDone: number) => void,
  onMerged?: (items: SearchResult[]) => void,
): EventSource {
  const es = new EventSource(`/api/search/stream?q=${encodeURIComponent(query)}`);
  let finished = false;

  es.onmessage = (e) => {
    try {
      const ev = JSON.parse(e.data);
      if (ev.type === 'results') onResult(ev.source, ev.data);
      else if (ev.type === 'error') onError(ev.source, ev.message);
      else if (ev.type === 'done') {
        finished = true;
        onDone(ev.total, ev.sourcesDone);
        if (ev.merged && onMerged) onMerged(ev.merged);
        es.close();
      }
    } catch { /* ignore malformed */ }
  };

  es.onerror = () => {
    if (!finished) {
      es.close();
      onDone(0, 0);
      if (onMerged) onMerged([]);
    }
  };

  return es;
}

export interface SearchBatchOptions {
  batchSize: number;
  concurrency: number;
  cursor?: string;
}

export interface SearchBatchStart {
  offset: number;
  eligible: number;
  sourcesInBatch: number;
  requestedConcurrency: number;
  effectiveConcurrency: number;
  retryCursor: string;
}

export interface SearchBatchDone {
  complete: boolean;
  checked: number;
  eligible: number;
  hasMore: boolean;
  nextCursor?: string;
  retryCursor: string;
  sourceFailures: number;
}

export interface SearchBatchHandlers {
  onStart: (event: SearchBatchStart) => void;
  onResult: (sourceId: string, items: SearchResult[], checked: number) => void;
  onSourceError: (sourceId: string, message: string, checked: number) => void;
  onDone: (event: SearchBatchDone) => void;
  onStale: (message: string) => void;
  onDisconnect: () => void;
}

export function searchBooksBatchStream(
  query: string,
  options: SearchBatchOptions,
  handlers: SearchBatchHandlers,
): EventSource {
  const params = new URLSearchParams({
    q: query,
    batchSize: String(options.batchSize),
    concurrency: String(options.concurrency),
  });
  if (options.cursor) params.set('cursor', options.cursor);

  const es = new EventSource(`/api/search/stream?${params}`);
  let finished = false;
  es.onmessage = (message) => {
    try {
      const event = JSON.parse(message.data);
      if (event.type === 'start') handlers.onStart(event);
      else if (event.type === 'results') handlers.onResult(event.sourceId, event.data, event.checked);
      else if (event.type === 'source_error') handlers.onSourceError(event.sourceId, event.message, event.checked);
      else if (event.type === 'stale') {
        finished = true;
        handlers.onStale(event.message);
        es.close();
      } else if (event.type === 'done') {
        finished = true;
        handlers.onDone(event);
        es.close();
      }
    } catch { /* malformed events cannot safely change search state */ }
  };
  es.onerror = () => {
    if (!finished) {
      finished = true;
      es.close();
      handlers.onDisconnect();
    }
  };
  return es;
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
  origin?: string;
  lastChapter: string;
  durChapterIndex: number;
  durChapterPos: number;
  totalChapterNum: number;
  stateVersion: number;
  alternateSources?: AltSource[];
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
  alternateSources?: AltSource[];
}) {
  return req<Book>('/books/enrich', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function addSearchResultToShelf(result: SearchResult) {
  const id = crypto.randomUUID?.() ?? (Date.now().toString(36) + Math.random().toString(36).slice(2));
  try {
    return await enrichBook({
      id, name: result.name, author: result.author || '', coverUrl: result.coverUrl || '',
      intro: result.intro || '', sourceUrl: result.sourceUrl, bookUrl: result.bookUrl,
      alternateSources: result.alternateSources,
    });
  } catch {
    return addBook({
      id, name: result.name, author: result.author, coverUrl: result.coverUrl,
      intro: result.intro, kind: result.kind, sourceUrl: result.sourceUrl,
      bookUrl: result.bookUrl, alternateSources: result.alternateSources,
    });
  }
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
  isVolume: boolean;
}

export interface ChapterContent {
  title: string;
  paragraphs: string[];
}

export function getChapters(bookId: string) {
  return req<Chapter[]>(`/books/${encodeURIComponent(bookId)}/chapters`);
}

export function getChapterContent(bookId: string, chapterIdx: number) {
  return req<any>(
    `/books/${encodeURIComponent(bookId)}/chapters/${chapterIdx}/content`
  ).then(data => ({
    title: data.title || data.Title || '',
    paragraphs: data.paragraphs || data.Paragraphs || []
  }));
}

// --- Progress ---
export function switchBookSource(bookId: string, sourceUrl: string, bookUrl: string) {
  return req<{ book: Book; mapping: 'title' | 'index' }>(`/books/${encodeURIComponent(bookId)}/source`, {
    method: 'PUT',
    body: JSON.stringify({ sourceUrl, bookUrl }),
  });
}

export interface Bookmark {
  id: string;
  bookId: string;
  chapterIndex: number;
  chapterTitle: string;
  position: number;
  note: string;
  orphaned: boolean;
  createdAt: number;
}

export function listBookmarks(bookId: string) {
  return req<Bookmark[]>(`/books/${encodeURIComponent(bookId)}/bookmarks`);
}

export function addBookmark(bookId: string, bookmark: {
  id: string; sourceUrl: string; stateVersion: number; chapterIndex: number; position: number; note: string;
}) {
  return req<Bookmark>(`/books/${encodeURIComponent(bookId)}/bookmarks`, {
    method: 'POST', body: JSON.stringify(bookmark),
  });
}

export function deleteBookmark(bookId: string, bookmarkId: string) {
  return req<{ status: string }>(`/books/${encodeURIComponent(bookId)}/bookmarks/${encodeURIComponent(bookmarkId)}`, { method: 'DELETE' });
}

export function saveProgress(
  bookId: string,
  sourceUrl: string,
  stateVersion: number,
  chapterIndex: number,
  position: number
) {
  return req<{ status: string; stateVersion: number }>(
    `/books/${encodeURIComponent(bookId)}/progress`,
    {
      method: 'PUT',
      body: JSON.stringify({ sourceUrl, stateVersion, chapterIndex, position }),
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

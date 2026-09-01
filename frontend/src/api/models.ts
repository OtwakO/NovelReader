export interface AltSource {
  sourceId: string;
  sourceUrl: string;
  bookUrl: string;
  sourceName: string;
  sourceGroup?: string;
  capabilities?: string[];
  discoveryQuery?: string;
  lastChapter?: string;
}

export interface SearchResult {
  name: string;
  author: string;
  coverUrl: string;
  coverDisplayUrl?: string;
  intro: string;
  kind: string;
  lastChapter: string;
  updateTime?: string;
  wordCount?: string;
  bookUrl: string;
  sourceId: string;
  sourceUrl: string;
  sourceName: string;
  sourceGroup?: string;
  capabilities?: string[];
  score?: number;
  shelfBookId?: string;
  alternateSources?: AltSource[];
}

export interface Book {
  id: string;
  name: string;
  author: string;
  coverUrl: string;
  coverDisplayUrl?: string;
  intro: string;
  kind: string;
  sourceId: string;
  sourceUrl: string;
  bookUrl: string;
  origin?: string;
  lastChapter: string;
  updateTime?: string;
  wordCount?: string;
  tocUrl?: string;
  downloadUrls?: string[];
  durChapterIndex: number;
  durChapterPos: number;
  totalChapterNum: number;
  stateVersion: number;
  currentChapterTitle?: string;
  activeSource?: AltSource;
  alternateSources?: AltSource[];
  createdAt?: number;
  updatedAt?: number;
}

export interface Chapter {
  id: string;
  bookId: string;
  index: number;
  title: string;
  url: string;
  isVolume: boolean;
}

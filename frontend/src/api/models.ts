export interface AltSource {
  variableMap?: string;
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
  variableMap?: string;
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

export interface LibraryBook {
  provider: string;
  originLabel?: string;
  id: string;
  name: string;
  author: string;
  coverUrl: string;
  coverDisplayUrl?: string;
  intro: string;
  kind: string;
  lastChapter: string;
  updateTime?: string;
  wordCount?: string;
  durChapterIndex: number;
  durChapterPos: number;
  totalChapterNum: number;
  contentRevision: number;
  stateVersion: number;
  currentChapterTitle?: string;
  createdAt?: number;
  updatedAt?: number;
}

// BookSource acquisition/recovery context; common shelf and reading fields are shared.
export interface Book extends LibraryBook {
  variableMap?: string;
  sourceId: string;
  sourceUrl: string;
  bookUrl: string;
  origin?: string;
  tocUrl?: string;
  downloadUrls?: string[];
  activeSource?: AltSource;
  alternateSources?: AltSource[];
}

export interface Chapter {
  index: number;
  title: string;
  isVolume: boolean;
}

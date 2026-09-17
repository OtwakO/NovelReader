import type { ChapterCatalog } from '../../api/chapter-catalog';
import type { ReadingTarget } from '../../api/structured-prose';
import { adjacentChapterIndex, clampProgress, isMainChapter } from './reading-progress';
import { ReaderRevisionConflict } from './reader-session';

export interface ReaderVisit {
  readonly chapterIndex: number;
  readonly position: number;
  readonly note: boolean;
  readonly anchor?: string;
}
export interface ReaderNavigation {
  readonly contentRevision: number;
  readonly current: ReaderVisit;
  readonly returns: readonly ReaderVisit[];
}

function section(catalog: ChapterCatalog, index: number) {
  const chapter = catalog.chapters.find(chapter => chapter.index === index && !chapter.isVolume);
  if (!chapter) throw new Error('Reading target is unavailable');
  return chapter;
}
function checkRevision(catalog: ChapterCatalog, revision: number) {
  if (catalog.contentRevision !== revision) throw new ReaderRevisionConflict();
}

/** One book's coherent catalog lifetime; discard on book/revision replacement.
 * Transitions are proposals: the reader commits them with displayed content only
 * after its existing request-generation check. Failed loads retain the old trail.
 */
export function createReaderNavigation(catalog: ChapterCatalog, chapterIndex: number, position = 0): ReaderNavigation {
  section(catalog, chapterIndex);
  return { contentRevision: catalog.contentRevision, current: { chapterIndex, position: clampProgress(position), note: false }, returns: [] };
}

export function followReaderTarget(catalog: ChapterCatalog, state: ReaderNavigation, target: ReadingTarget, departurePosition: number, note: boolean): ReaderNavigation {
  checkRevision(catalog, state.contentRevision);
  checkRevision(catalog, target.contentRevision);
  const chapter = section(catalog, target.chapterIndex);
  const visitingNote = note || chapter.auxiliary === true;
  // Capture the actual departure position, not the anchor originally used to
  // enter that document: the user may have scrolled since the initial jump.
  const origin: ReaderVisit = { chapterIndex: state.current.chapterIndex, position: clampProgress(departurePosition), note: state.current.note };
  return {
    contentRevision: state.contentRevision,
    current: { chapterIndex: target.chapterIndex, position: 0, note: visitingNote, ...(target.anchor === undefined ? {} : { anchor: target.anchor }) },
    returns: visitingNote ? [...state.returns, origin] : [],
  };
}

export function returnFromReaderNote(catalog: ChapterCatalog, state: ReaderNavigation): ReaderNavigation | null {
  checkRevision(catalog, state.contentRevision);
  const previous = state.returns.at(-1);
  if (!previous) return null;
  section(catalog, previous.chapterIndex);
  return { contentRevision: state.contentRevision, current: previous, returns: state.returns.slice(0, -1) };
}

export function canRecordReaderProgress(catalog: ChapterCatalog, state: ReaderNavigation): boolean {
  checkRevision(catalog, state.contentRevision);
  return !state.current.note && isMainChapter(section(catalog, state.current.chapterIndex));
}

/** Used for both ordinary navigation and next-section prefetch eligibility. */
export function adjacentReaderSection(catalog: ChapterCatalog, state: ReaderNavigation, direction: -1 | 1): number | null {
  return canRecordReaderProgress(catalog, state) ? adjacentChapterIndex(catalog.chapters, state.current.chapterIndex, direction) : null;
}

import { expect, it } from 'vitest';
import { adjacentReaderSection, canRecordReaderProgress, createReaderNavigation, followReaderTarget, returnFromReaderNote } from './reader-navigation';
import { ReaderRevisionConflict } from './reader-session';

const catalog = { contentRevision: 7, chapters: [
  { index: 0, title: 'First', isVolume: false },
  { index: 1, title: 'Notes', isVolume: false, auxiliary: true },
  { index: 2, title: 'Second', isVolume: false },
] };

it('returns through nested notes including a main-section note without recording main progress', () => {
  const main = createReaderNavigation(catalog, 0, .1);
  expect(adjacentReaderSection(catalog, main, 1)).toBe(2);
  const sameSection = followReaderTarget(catalog, main, { contentRevision: 7, chapterIndex: 0, anchor: 'a1' }, .3, true);
  const auxiliary = followReaderTarget(catalog, sameSection, { contentRevision: 7, chapterIndex: 1, anchor: 'a2' }, .8, false);
  expect(canRecordReaderProgress(catalog, sameSection)).toBe(false);
  expect(canRecordReaderProgress(catalog, auxiliary)).toBe(false);
  expect(adjacentReaderSection(catalog, sameSection, 1)).toBeNull();
  expect(adjacentReaderSection(catalog, auxiliary, -1)).toBeNull();
  const backToNote = returnFromReaderNote(catalog, auxiliary)!;
  expect(backToNote.current).toEqual({ chapterIndex: 0, position: .8, note: true });
  expect(canRecordReaderProgress(catalog, backToNote)).toBe(false);
  const backToMain = returnFromReaderNote(catalog, backToNote)!;
  expect(backToMain.current).toEqual({ chapterIndex: 0, position: .3, note: false });
  expect(canRecordReaderProgress(catalog, backToMain)).toBe(true);
  expect(returnFromReaderNote(catalog, backToMain)).toBeNull();
  // Proposing/abandoning a transition never consumes the displayed trail.
  expect(main.current.position).toBe(.1);
  expect(auxiliary.returns).toHaveLength(2);
});

it('opens auxiliary bookmarks without main progress and exits note context on ordinary main navigation', () => {
  const bookmark = createReaderNavigation(catalog, 1, .6);
  expect(bookmark.current.position).toBe(.6);
  expect(canRecordReaderProgress(catalog, bookmark)).toBe(false);
  expect(adjacentReaderSection(catalog, bookmark, 1)).toBeNull();
  expect(returnFromReaderNote(catalog, bookmark)).toBeNull();
  const note = followReaderTarget(catalog, bookmark, { contentRevision: 7, chapterIndex: 0 }, .6, true);
  const main = followReaderTarget(catalog, note, { contentRevision: 7, chapterIndex: 2, anchor: 'start' }, .9, false);
  expect(main.returns).toEqual([]);
  expect(main.current.anchor).toBe('start');
  expect(canRecordReaderProgress(catalog, main)).toBe(true);
});

it('rejects stale and unavailable targets rather than falling back to a different section', () => {
  const main = createReaderNavigation(catalog, 0);
  expect(() => followReaderTarget(catalog, main, { contentRevision: 6, chapterIndex: 1 }, .5, true)).toThrow(ReaderRevisionConflict);
  expect(() => followReaderTarget(catalog, main, { contentRevision: 7, chapterIndex: 99 }, .5, true)).toThrow('unavailable');
  expect(() => returnFromReaderNote({ ...catalog, contentRevision: 8 }, main)).toThrow(ReaderRevisionConflict);
  expect(main.returns).toEqual([]);
});

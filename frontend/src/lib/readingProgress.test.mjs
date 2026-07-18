import test from 'node:test';
import assert from 'node:assert/strict';
import {
  adjacentChapterIndex, chapterProgressPercent, createProgressQueue,
  normalizedScroll, resolveChapterIndex, scrollTopForProgress,
} from './readingProgress.js';

const chapters = [
  { index: 0, isVolume: true },
  { index: 1, isVolume: false },
  { index: 2, isVolume: true },
  { index: 3, isVolume: false },
];

test('resolves explicit then saved then first readable raw chapter index', () => {
  assert.equal(resolveChapterIndex(chapters, 3, 1), 3);
  assert.equal(resolveChapterIndex(chapters, undefined, 3), 3);
  assert.equal(resolveChapterIndex(chapters, 2, 2), 1);
  assert.equal(resolveChapterIndex([{ index: 0, isVolume: true }], undefined, 0), null);
});

test('navigates around volume rows using raw chapter indices', () => {
  assert.equal(adjacentChapterIndex(chapters, 1, 1), 3);
  assert.equal(adjacentChapterIndex(chapters, 3, -1), 1);
  assert.equal(adjacentChapterIndex(chapters, 1, -1), null);
});

test('normalizes and restores bounded scroll progress', () => {
  assert.equal(normalizedScroll(300, 1000, 400), 0.5);
  assert.equal(scrollTopForProgress(0.5, 1000, 400), 300);
  assert.equal(normalizedScroll(10, 100, 100), 0);
  assert.equal(scrollTopForProgress(2, 1000, 400), 600);
});

test('serializes writes per book and lets a new reader await an unmount save', async () => {
  let releaseFirst;
  const calls = [];
  const queue = createProgressQueue(async (_bookId, chapter, position) => {
    calls.push([chapter, position]);
    if (chapter === 1) await new Promise((resolve) => { releaseFirst = resolve; });
  });
  const first = queue.write('book', 1, 0.7);
  let resumed = false;
  const resume = queue.wait('book').then(() => { resumed = true; });
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(resumed, false);
  releaseFirst();
  await Promise.all([first, resume]);
  await queue.write('book', 2, 0);
  assert.deepEqual(calls, [[1, 0.7], [2, 0]]);
});

test('counts the current chapter in shelf progress', () => {
  assert.equal(chapterProgressPercent(0, 10), 10);
  assert.equal(chapterProgressPercent(9, 10), 100);
});

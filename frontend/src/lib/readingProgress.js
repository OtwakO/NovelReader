// Reader progress helpers keep raw chapter indices and normalized scroll positions predictable.
export function clampProgress(value) {
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0;
}

export function normalizedScroll(scrollTop, scrollHeight, clientHeight) {
  const range = Math.max(0, scrollHeight - clientHeight);
  return range === 0 ? 0 : clampProgress(scrollTop / range);
}

export function scrollTopForProgress(position, scrollHeight, clientHeight) {
  return clampProgress(position) * Math.max(0, scrollHeight - clientHeight);
}

export function readableChapters(chapters) {
  return chapters.filter((chapter) => !chapter.isVolume);
}

export function resolveChapterIndex(chapters, requestedIndex, savedIndex) {
  const readable = readableChapters(chapters);
  if (readable.length === 0) return null;
  if (Number.isInteger(requestedIndex) && readable.some((chapter) => chapter.index === requestedIndex)) return requestedIndex;
  if (Number.isInteger(savedIndex) && readable.some((chapter) => chapter.index === savedIndex)) return savedIndex;
  return readable[0].index;
}

export function adjacentChapterIndex(chapters, currentIndex, direction) {
  const readable = readableChapters(chapters);
  const position = readable.findIndex((chapter) => chapter.index === currentIndex);
  const target = position + direction;
  return position >= 0 && target >= 0 && target < readable.length ? readable[target].index : null;
}

export function createProgressQueue(write) {
  const pending = new Map();
  return {
    write(bookId, chapterIndex, position) {
      const previous = pending.get(bookId) || Promise.resolve();
      const operation = previous.catch(() => {}).then(() => write(bookId, chapterIndex, position));
      const barrier = operation.then(() => {}, () => {});
      pending.set(bookId, barrier);
      barrier.then(() => { if (pending.get(bookId) === barrier) pending.delete(bookId); });
      return operation;
    },
    wait(bookId) {
      return pending.get(bookId) || Promise.resolve();
    },
  };
}

export function chapterProgressPercent(chapterIndex, totalChapters) {
  if (!Number.isInteger(chapterIndex) || totalChapters <= 0) return 0;
  return Math.min(100, Math.max(0, ((chapterIndex + 1) / totalChapters) * 100));
}

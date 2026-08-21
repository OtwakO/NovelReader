export function pageCount(total: number, pageSize: number): number {
  return Math.max(1, Math.ceil(Math.max(0, total) / pageSize));
}

export function clampPage(
  page: number,
  total: number,
  pageSize: number,
): number {
  return Math.min(
    pageCount(total, pageSize),
    Math.max(1, Math.trunc(page) || 1),
  );
}

export function pageItems<T>(items: T[], page: number, pageSize: number): T[] {
  const safePage = clampPage(page, items.length, pageSize);
  const start = (safePage - 1) * pageSize;
  return items.slice(start, start + pageSize);
}

export function pageRange(
  page: number,
  pageSize: number,
  total: number,
): { start: number; end: number } {
  if (total <= 0) return { start: 0, end: 0 };
  const safePage = clampPage(page, total, pageSize);
  const start = (safePage - 1) * pageSize + 1;
  return { start, end: Math.min(total, start + pageSize - 1) };
}

export interface ReadingTarget { chapterIndex: number; contentRevision: number; anchor?: string }

/** A delayed action keeps its original revision; never requalify it implicitly. */
export function parseReadingTarget(input: unknown, revision: number): ReadingTarget {
  if (!input || typeof input !== 'object') throw new Error('Invalid reading target');
  const value = input as Record<string, unknown>;
  if (typeof value.chapterIndex !== 'number' || !Number.isSafeInteger(value.chapterIndex) || value.chapterIndex < 0 || (value.anchor !== undefined && typeof value.anchor !== 'string')) throw new Error('Invalid reading target');
  if (value.contentRevision !== revision) throw new Error('Reading target revision mismatch');
  return { chapterIndex: value.chapterIndex, contentRevision: revision, ...(value.anchor === undefined ? {} : { anchor: value.anchor }) };
}

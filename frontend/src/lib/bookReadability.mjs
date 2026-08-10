export async function validateReadableBook(bookId, { getChapters, getChapterContent }) {
  const chapters = await getChapters(bookId);
  const first = chapters.find((chapter) => !chapter.isVolume);
  if (!first) throw new Error('This source did not provide any readable chapters.');
  const content = await getChapterContent(bookId, first.index);
  if (!(content.paragraphs?.length || content.blocks?.length)) {
    throw new Error('This source returned an empty first chapter.');
  }
  return first;
}

export function alternateSourceOptions(result) {
  const seen = new Set([`${result.sourceUrl}\n${result.bookUrl}`]);
  return (result.alternateSources || []).filter((source) => {
    const key = `${source.sourceUrl}\n${source.bookUrl}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

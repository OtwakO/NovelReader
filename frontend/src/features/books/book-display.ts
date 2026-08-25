export function readableChapterLabel(value?: string): string {
  const label = value?.trim() || '';
  if (!label || /^(?:https?:\/\/|\/)|[?#=]|\.html?$/i.test(label)) return '';
  return label;
}

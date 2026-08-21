export type ReaderKeyboardAction = 'previous' | 'next' | 'page-up' | 'page-down' | 'top' | 'bottom' | 'escape' | 'none';

export function readerKeyboardAction(event: Pick<KeyboardEvent, 'key'|'target'|'defaultPrevented'|'ctrlKey'|'metaKey'|'altKey'|'shiftKey'>, overlayOpen: boolean, selection = ''): ReaderKeyboardAction {
  if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) return 'none';
  const target = event.target as HTMLElement | null;
  if (target?.closest?.('input,select,textarea,[contenteditable="true"]')) return 'none';
  if (event.key === 'Escape') return 'escape';
  if (overlayOpen || selection.trim()) return 'none';
  if (event.key === 'ArrowLeft') return 'previous';
  if (event.key === 'ArrowRight') return 'next';
  if (event.key === 'PageUp') return 'page-up';
  if (event.key === 'PageDown' || (event.key === ' ' && !event.shiftKey)) return 'page-down';
  if (event.key === ' ' && event.shiftKey) return 'page-up';
  if (event.key === 'Home') return 'top';
  if (event.key === 'End') return 'bottom';
  return 'none';
}

export type ReaderTapAction = 'previous' | 'next' | 'scroll-up' | 'scroll-down' | 'toggle-controls' | 'show-controls' | 'none';

interface TapRect {
  left: number;
  top: number;
  width: number;
  height: number;
}

export function readerTapAction(clientX: number, clientY: number, rect: TapRect, sideNavigation = true): ReaderTapAction {
  const column = Math.min(2, Math.max(0, Math.floor(((clientX - rect.left) / Math.max(1, rect.width)) * 3)));
  const row = Math.min(2, Math.max(0, Math.floor(((clientY - rect.top) / Math.max(1, rect.height)) * 3)));

  if (row === 1 && column === 0) return sideNavigation ? 'previous' : 'none';
  if (row === 1 && column === 2) return sideNavigation ? 'next' : 'none';
  if (row === 0 && column === 1) return 'scroll-up';
  if (row === 2 && column === 1) return 'scroll-down';
  if (row === 1 && column === 1) return 'toggle-controls';
  return 'show-controls';
}

export function isAtScrollBoundary(
  direction: -1 | 1,
  scrollTop: number,
  scrollHeight: number,
  clientHeight: number,
  tolerance = 8,
): boolean {
  if (direction < 0) return scrollTop <= tolerance;
  return scrollTop + clientHeight >= scrollHeight - tolerance;
}

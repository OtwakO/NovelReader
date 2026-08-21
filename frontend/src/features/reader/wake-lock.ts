export interface WakeLockSentinelLike { release(): Promise<void>; addEventListener?(type: 'release', listener: () => void): void }

export function createReaderWakeLock(enabled: () => boolean, onUnavailable?: () => void) {
  let sentinel: WakeLockSentinelLike | null = null;
  let destroyed = false;

  async function acquire() {
    if (destroyed || !enabled() || document.visibilityState !== 'visible' || sentinel) return;
    const wakeLock = (navigator as Navigator & { wakeLock?: { request(type: 'screen'): Promise<WakeLockSentinelLike> } }).wakeLock;
    if (!wakeLock) { onUnavailable?.(); return; }
    try {
      sentinel = await wakeLock.request('screen');
      sentinel.addEventListener?.('release', () => { sentinel = null; });
    } catch { onUnavailable?.(); }
  }

  async function release() {
    const current = sentinel; sentinel = null;
    if (current) await current.release().catch(() => undefined);
  }

  async function sync() { if (enabled()) await acquire(); else await release(); }
  function visibility() { if (document.visibilityState === 'visible') void acquire(); else void release(); }
  document.addEventListener('visibilitychange', visibility);

  return { sync, destroy: async () => { destroyed = true; document.removeEventListener('visibilitychange', visibility); await release(); } };
}

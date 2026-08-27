export function registerServiceWorker(): void {
  if (!('serviceWorker' in navigator) || !import.meta.env.PROD) return;

  const register = () => {
    void navigator.serviceWorker.register('/service-worker.js', { scope: '/' }).then((registration) => {
      // Newly discovered updates intentionally remain waiting during this session.
      // A later launch or reload reaches this branch and activates them at a safe boundary.
      if (!registration.waiting) return;
      navigator.serviceWorker.addEventListener('controllerchange', () => window.location.reload(), { once: true });
      registration.waiting.postMessage('activate-update');
    }).catch(() => {
      // Installation remains optional; the web app must keep working without a worker.
    });
  };

  if (document.readyState === 'loading') {
    window.addEventListener('load', register, { once: true });
  } else {
    register();
  }
}

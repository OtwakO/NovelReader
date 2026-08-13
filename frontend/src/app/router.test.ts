import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as api from '../api/client';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    getSetupStatus: vi.fn(),
    getRegistrationPolicy: vi.fn(),
    getCurrentAccount: vi.fn(),
    onAuthenticationLoss: vi.fn(() => () => undefined),
  };
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetModules();
  vi.clearAllMocks();
  location.hash = '';
});

describe('router access policy', () => {
  it('redirects an unauthenticated private route to login', async () => {
    vi.mocked(api.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    vi.mocked(api.getRegistrationPolicy).mockResolvedValue({ enabled: false, inviteRequired: false });
    vi.mocked(api.getCurrentAccount).mockRejectedValue(new api.ApiError(401, 'unauthorized'));
    const { router } = await import('./router');
    await router.push('/search');
    await router.isReady();
    expect(router.currentRoute.value.name).toBe('login');
  });
});

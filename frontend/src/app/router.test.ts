import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as authApi from '../api/auth';
import { ApiError } from '../api/transport';

vi.mock('../api/auth', async () => {
  const actual = await vi.importActual<typeof import('../api/auth')>('../api/auth');
  return { ...actual, getSetupStatus: vi.fn(), getRegistrationPolicy: vi.fn(), getCurrentAccount: vi.fn() };
});
vi.mock('../api/transport', async () => {
  const actual = await vi.importActual<typeof import('../api/transport')>('../api/transport');
  return { ...actual, onAuthenticationLoss: vi.fn(() => () => undefined) };
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetModules();
  vi.clearAllMocks();
  location.hash = '';
});

describe('router access policy', () => {
  it('redirects an unauthenticated private route to login', async () => {
    vi.mocked(authApi.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    vi.mocked(authApi.getRegistrationPolicy).mockResolvedValue({ enabled: false, inviteRequired: false });
    vi.mocked(authApi.getCurrentAccount).mockRejectedValue(new ApiError(401, 'unauthorized'));
    const { router } = await import('./router');
    await router.push('/search');
    await router.isReady();
    expect(router.currentRoute.value.name).toBe('login');
  });
});

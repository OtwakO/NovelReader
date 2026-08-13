import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useSessionStore } from './session';
import * as authApi from '../api/auth';
import { ApiError } from '../api/transport';

vi.mock('../api/auth', async () => {
  const actual = await vi.importActual<typeof import('../api/auth')>('../api/auth');
  return { ...actual, getSetupStatus: vi.fn(), getRegistrationPolicy: vi.fn(), getCurrentAccount: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn() };
});
vi.mock('../api/transport', async () => {
  const actual = await vi.importActual<typeof import('../api/transport')>('../api/transport');
  return { ...actual, onAuthenticationLoss: vi.fn(() => () => undefined) };
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

describe('session store', () => {
  it('opens setup before requesting private account state', async () => {
    vi.mocked(authApi.getSetupStatus).mockResolvedValue({ status: 'open', available: true });
    const session = useSessionStore();
    await session.initialize();
    expect(session.phase).toBe('setup');
    expect(authApi.getCurrentAccount).not.toHaveBeenCalled();
  });

  it('loads registration policy and current account after closed setup', async () => {
    vi.mocked(authApi.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    vi.mocked(authApi.getRegistrationPolicy).mockResolvedValue({ enabled: true, inviteRequired: true });
    vi.mocked(authApi.getCurrentAccount).mockResolvedValue({ id: 'u1', username: 'reader', role: 'reader' });
    const session = useSessionStore();
    await session.initialize();
    expect(session.phase).toBe('authenticated');
    expect(session.registrationEnabled).toBe(true);
    expect(session.account?.username).toBe('reader');
  });

  it('treats a current-account 401 as a guest session', async () => {
    vi.mocked(authApi.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    vi.mocked(authApi.getRegistrationPolicy).mockResolvedValue({ enabled: false, inviteRequired: false });
    vi.mocked(authApi.getCurrentAccount).mockRejectedValue(new ApiError(401, 'unauthorized'));
    const session = useSessionStore();
    await session.initialize();
    expect(session.phase).toBe('guest');
    expect(session.account).toBeNull();
  });
});

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

  it('loads registration policy and current account concurrently after closed setup', async () => {
    vi.mocked(authApi.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    let resolveRegistration!: (value: { enabled: boolean; inviteRequired: boolean }) => void;
    vi.mocked(authApi.getRegistrationPolicy).mockReturnValue(new Promise((resolve) => { resolveRegistration = resolve; }));
    vi.mocked(authApi.getCurrentAccount).mockResolvedValue({ id: 'u1', username: 'reader', role: 'reader' });
    const session = useSessionStore();
    const initialization = session.initialize();
    await vi.waitFor(() => expect(authApi.getCurrentAccount).toHaveBeenCalledOnce());
    resolveRegistration({ enabled: true, inviteRequired: true });
    await initialization;
    expect(session.phase).toBe('authenticated');
    expect(session.registrationEnabled).toBe(true);
    expect(session.account?.username).toBe('reader');
  });

  it('ends private state after a successful password change', () => {
    const session = useSessionStore(); session.account = { id: 'u1', username: 'reader', role: 'reader' }; session.phase = 'authenticated'; session.returnTo = '/account'; session.endAfterPasswordChange();
    expect(session.account).toBeNull(); expect(session.phase).toBe('guest'); expect(session.returnTo).toBe('/shelf'); expect(session.message).toBe('password-changed');
  });

  it('keeps a retryable logout failure after private state closes', async () => {
    vi.mocked(authApi.logout).mockRejectedValue(new Error('logout unavailable')); const session=useSessionStore();session.account={id:'u1',username:'reader',role:'reader'};session.phase='authenticated';await expect(session.logout()).rejects.toThrow('logout unavailable');expect(session.account).toBeNull();expect(session.phase).toBe('guest');expect(session.notice).toBe('logout-failed');
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

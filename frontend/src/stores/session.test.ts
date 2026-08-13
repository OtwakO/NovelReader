import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useSessionStore } from './session';
import * as api from '../api/client';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    getSetupStatus: vi.fn(),
    getRegistrationPolicy: vi.fn(),
    getCurrentAccount: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    onAuthenticationLoss: vi.fn(() => () => undefined),
  };
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

describe('session store', () => {
  it('opens setup before requesting private account state', async () => {
    vi.mocked(api.getSetupStatus).mockResolvedValue({ status: 'open', available: true });
    const session = useSessionStore();
    await session.initialize();
    expect(session.phase).toBe('setup');
    expect(api.getCurrentAccount).not.toHaveBeenCalled();
  });

  it('loads registration policy and current account after closed setup', async () => {
    vi.mocked(api.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    vi.mocked(api.getRegistrationPolicy).mockResolvedValue({ enabled: true, inviteRequired: true });
    vi.mocked(api.getCurrentAccount).mockResolvedValue({ id: 'u1', username: 'reader', role: 'reader' });
    const session = useSessionStore();
    await session.initialize();
    expect(session.phase).toBe('authenticated');
    expect(session.registrationEnabled).toBe(true);
    expect(session.account?.username).toBe('reader');
  });

  it('treats a current-account 401 as a guest session', async () => {
    vi.mocked(api.getSetupStatus).mockResolvedValue({ status: 'closed', available: false });
    vi.mocked(api.getRegistrationPolicy).mockResolvedValue({ enabled: false, inviteRequired: false });
    vi.mocked(api.getCurrentAccount).mockRejectedValue(new api.ApiError(401, 'unauthorized'));
    const session = useSessionStore();
    await session.initialize();
    expect(session.phase).toBe('guest');
    expect(session.account).toBeNull();
  });
});

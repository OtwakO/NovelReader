import { defineStore } from 'pinia';
import { ApiError, NetworkError, onAuthenticationLoss } from '../api/transport';
import {
  getCurrentAccount,
  getRegistrationPolicy,
  getSetupStatus,
  login as loginRequest,
  logout as logoutRequest,
  register as registerRequest,
  type AuthAccount,
} from '../api/auth';

export type SessionPhase = 'idle' | 'loading' | 'setup' | 'setup-unavailable' | 'guest' | 'authenticated' | 'error';
export type StartupFailure = '' | 'offline' | 'server-unreachable' | 'unexpected';

interface SessionState {
  phase: SessionPhase;
  account: AuthAccount | null;
  registrationEnabled: boolean;
  registrationInviteRequired: boolean;
  message: string;
  startupFailure: StartupFailure;
  notice: '' | 'authentication-lost' | 'logout-failed';
  returnTo: string;
  initialized: boolean;
}

export const useSessionStore = defineStore('session', {
  state: (): SessionState => ({
    phase: 'idle',
    account: null,
    registrationEnabled: false,
    registrationInviteRequired: false,
    message: '',
    startupFailure: '',
    notice: '',
    returnTo: '/shelf',
    initialized: false,
  }),

  getters: {
    isAuthenticated: (state): boolean => state.phase === 'authenticated' && state.account !== null,
    isAdministrator: (state): boolean => state.account?.role === 'admin',
  },

  actions: {
    installAuthenticationLossHandler(onLost: () => void) {
      return onAuthenticationLoss(() => {
        if (!this.isAuthenticated) return;
        this.account = null;
        this.phase = 'guest';
        this.notice = 'authentication-lost';
        onLost();
      });
    },

    async initialize() {
      if (this.initialized) return;
      this.phase = 'loading';
      this.message = '';
      this.startupFailure = '';
      this.notice = '';
      try {
        const setup = await getSetupStatus();
        if (setup.status !== 'closed') {
          this.phase = setup.available ? 'setup' : 'setup-unavailable';
          this.initialized = true;
          return;
        }

        const [registration, account] = await Promise.all([
          getRegistrationPolicy(),
          getCurrentAccount().then(
            (value) => ({ value, error: null }),
            (error: unknown) => ({ value: null, error }),
          ),
        ]);
        this.registrationEnabled = registration.enabled;
        this.registrationInviteRequired = registration.inviteRequired;

        if (account.error) {
          if (!(account.error instanceof ApiError) || account.error.status !== 401) throw account.error;
          this.account = null;
          this.phase = 'guest';
        } else {
          this.account = account.value;
          this.phase = 'authenticated';
        }
      } catch (cause) {
        this.phase = 'error';
        this.startupFailure = !navigator.onLine
          ? 'offline'
          : cause instanceof NetworkError
            ? 'server-unreachable'
            : 'unexpected';
        this.message = cause instanceof Error ? cause.message : '';
      } finally {
        this.initialized = true;
      }
    },

    authenticated(account: AuthAccount) {
      this.account = account;
      this.phase = 'authenticated';
      this.message = '';
      this.startupFailure = '';
      this.notice = '';
    },

    async login(username: string, password: string) {
      const account = await loginRequest(username, password);
      this.authenticated(account);
    },

    async register(username: string, password: string, inviteCode: string) {
      const account = await registerRequest(username, password, inviteCode);
      this.authenticated(account);
    },

    endAfterPasswordChange() {
      this.account = null;
      this.phase = 'guest';
      this.returnTo = '/shelf';
      this.notice = '';
      this.message = 'password-changed';
    },

    async logout() {
      this.account = null;
      this.phase = 'loading';
      this.notice = '';
      try {
        await logoutRequest();
        this.phase = 'guest';
        this.message = '';
      } catch (cause) {
        this.phase = 'guest';
        this.notice = 'logout-failed';
        this.message = cause instanceof Error ? cause.message : '';
        throw cause;
      }
    },
  },
});

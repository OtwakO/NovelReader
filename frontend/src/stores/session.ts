import { defineStore } from 'pinia';
import {
  ApiError,
  getCurrentAccount,
  getRegistrationPolicy,
  getSetupStatus,
  login as loginRequest,
  logout as logoutRequest,
  onAuthenticationLoss,
  register as registerRequest,
  type AuthAccount,
} from '../api/client';

export type SessionPhase = 'idle' | 'loading' | 'setup' | 'setup-unavailable' | 'guest' | 'authenticated' | 'error';

interface SessionState {
  phase: SessionPhase;
  account: AuthAccount | null;
  registrationEnabled: boolean;
  registrationInviteRequired: boolean;
  message: string;
  notice: '' | 'authentication-lost';
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
      this.notice = '';
      try {
        const setup = await getSetupStatus();
        if (setup.status !== 'closed') {
          this.phase = setup.available ? 'setup' : 'setup-unavailable';
          this.initialized = true;
          return;
        }

        const registration = await getRegistrationPolicy();
        this.registrationEnabled = registration.enabled;
        this.registrationInviteRequired = registration.inviteRequired;

        try {
          this.account = await getCurrentAccount();
          this.phase = 'authenticated';
        } catch (cause) {
          if (!(cause instanceof ApiError) || cause.status !== 401) throw cause;
          this.account = null;
          this.phase = 'guest';
        }
      } catch (cause) {
        this.phase = 'error';
        this.message = cause instanceof Error ? cause.message : '';
      } finally {
        this.initialized = true;
      }
    },

    authenticated(account: AuthAccount) {
      this.account = account;
      this.phase = 'authenticated';
      this.message = '';
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

    async logout() {
      this.account = null;
      this.phase = 'loading';
      try {
        await logoutRequest();
        this.phase = 'guest';
        this.message = '';
      } catch (cause) {
        this.phase = 'guest';
        this.message = cause instanceof Error ? cause.message : '';
        throw cause;
      }
    },
  },
});

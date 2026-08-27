import type { Pinia } from 'pinia';
import { createRouter, createWebHashHistory, type RouteLocationNormalized } from 'vue-router';
import { pinia } from './pinia';
import { useSessionStore } from '../stores/session';
import LoadingView from './views/LoadingView.vue';
import StartupErrorView from './views/StartupErrorView.vue';
import NotFoundView from './views/NotFoundView.vue';
import PublicLayout from './layouts/PublicLayout.vue';
import AppShell from './layouts/AppShell.vue';

const LoginView = () => import('../features/account/LoginView.vue');
const RegisterView = () => import('../features/account/RegisterView.vue');
const PasswordResetView = () => import('../features/account/PasswordResetView.vue');
const SetupView = () => import('../features/account/SetupView.vue');
const SetupUnavailableView = () => import('../features/account/SetupUnavailableView.vue');
const RecoveryView = () => import('../features/account/RecoveryView.vue');
const ShelfView = () => import('../features/shelf/ShelfView.vue');
const SearchView = () => import('../features/search/SearchView.vue');
const CandidateBookDetailView = () => import('../features/books/CandidateBookDetailView.vue');
const BookDetailView = () => import('../features/books/BookDetailView.vue');
const ReaderView = () => import('../features/reader/ReaderView.vue');
const ExploreView = () => import('../features/explore/ExploreView.vue');
const SourceManagementView = () => import('../features/sources/SourceManagementView.vue');
const SettingsView = () => import('../features/settings/SettingsView.vue');
const AccountView = () => import('../features/account/AccountView.vue');
const ReaderAdministrationView = () => import('../features/account/ReaderAdministrationView.vue');

const publicNames = new Set(['login', 'register', 'password-reset', 'recovery', 'setup', 'setup-unavailable', 'loading', 'startup-error']);

function requiresAuthentication(to: RouteLocationNormalized): boolean {
  return to.matched.some((record) => record.meta.requiresAuth === true);
}

export function createAppRouter(appPinia: Pinia = pinia) {
  const appRouter = createRouter({
    history: createWebHashHistory(),
    routes: [
      { path: '/loading', name: 'loading', component: LoadingView },
      { path: '/startup-error', name: 'startup-error', component: StartupErrorView },
      {
        path: '/', component: PublicLayout,
        children: [
          { path: 'login', name: 'login', component: LoginView },
          { path: 'register', name: 'register', component: RegisterView },
          { path: 'password-reset', name: 'password-reset', component: PasswordResetView },
          { path: 'recovery', name: 'recovery', component: RecoveryView },
          { path: 'setup', name: 'setup', component: SetupView },
          { path: 'setup-unavailable', name: 'setup-unavailable', component: SetupUnavailableView },
        ],
      },
      {
        path: '/', component: AppShell, meta: { requiresAuth: true },
        children: [
          { path: 'shelf', name: 'shelf', component: ShelfView },
          { path: 'explore', name: 'explore', component: ExploreView },
          { path: 'search', name: 'search', component: SearchView },
          { path: 'books/candidate', name: 'candidate-book-detail', component: CandidateBookDetailView },
          { path: 'books/:bookId', name: 'book-detail', component: BookDetailView },
          { path: 'books/:bookId/read/:chapterIndex?', name: 'reader', component: ReaderView },
          { path: 'sources', name: 'sources', component: SourceManagementView },
          { path: 'settings', name: 'settings', component: SettingsView },
          { path: 'account', name: 'account', component: AccountView },
          { path: 'account/readers', name: 'reader-admin', component: ReaderAdministrationView, meta: { administrator: true } },
        ],
      },
      { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
    ],
  });

  appRouter.beforeEach(async (to) => {
    const session = useSessionStore(appPinia);
    if (!session.initialized) await session.initialize();

    if (session.phase === 'error' && to.name !== 'startup-error') return { name: 'startup-error' };
    if (session.phase === 'setup' && to.name !== 'setup') return { name: 'setup' };
    if (session.phase === 'setup-unavailable' && to.name !== 'setup-unavailable') return { name: 'setup-unavailable' };

    if (session.isAuthenticated) {
      if (to.meta.administrator && !session.isAdministrator) return { name: 'shelf' };
      if (publicNames.has(String(to.name))) return session.returnTo || '/shelf';
      if (to.path === '/') return { name: 'shelf' };
      return true;
    }

    if (requiresAuthentication(to)) {
      session.returnTo = to.fullPath;
      return { name: 'login' };
    }
    if (to.name === 'register' && !session.registrationEnabled) return { name: 'login' };
    if (to.path === '/') return { name: 'login' };
    return true;
  });

  return appRouter;
}

export const router = createAppRouter();

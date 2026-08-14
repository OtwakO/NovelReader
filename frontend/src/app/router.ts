import { createRouter, createWebHashHistory, type RouteLocationNormalized } from 'vue-router';
import { pinia } from './pinia';
import { useSessionStore } from '../stores/session';
import LoadingView from './views/LoadingView.vue';
import StartupErrorView from './views/StartupErrorView.vue';
import NotFoundView from './views/NotFoundView.vue';
import PublicLayout from './layouts/PublicLayout.vue';
import AppShell from './layouts/AppShell.vue';
import LoginView from '../features/account/LoginView.vue';
import RegisterView from '../features/account/RegisterView.vue';
import PasswordResetView from '../features/account/PasswordResetView.vue';
import SetupView from '../features/account/SetupView.vue';
import SetupUnavailableView from '../features/account/SetupUnavailableView.vue';
import RecoveryView from '../features/account/RecoveryView.vue';
import ShelfView from '../features/shelf/ShelfView.vue';
import SearchView from '../features/search/SearchView.vue';
import BookPreviewView from '../features/books/BookPreviewView.vue';
import BookDetailView from '../features/books/BookDetailView.vue';
import ReaderView from '../features/reader/ReaderView.vue';
import ExploreView from '../features/explore/ExploreView.vue';
import SourceManagementView from '../features/sources/SourceManagementView.vue';
import SettingsView from '../features/settings/SettingsView.vue';
import AccountView from '../features/account/AccountView.vue';
import PlannedFeatureView from './views/PlannedFeatureView.vue';

const publicNames = new Set(['login', 'register', 'password-reset', 'recovery', 'setup', 'setup-unavailable', 'loading', 'startup-error']);

function requiresAuthentication(to: RouteLocationNormalized): boolean {
  return to.matched.some((record) => record.meta.requiresAuth === true);
}

export const router = createRouter({
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
        { path: 'books/preview', name: 'book-preview', component: BookPreviewView },
        { path: 'books/:bookId', name: 'book-detail', component: BookDetailView },
        { path: 'books/:bookId/read/:chapterIndex?', name: 'reader', component: ReaderView },
        { path: 'sources', name: 'sources', component: SourceManagementView },
        { path: 'settings', name: 'settings', component: SettingsView },
        { path: 'account', name: 'account', component: AccountView },
        { path: 'account/readers', name: 'reader-admin', component: PlannedFeatureView, meta: { administrator: true }, props: { title: 'app.planned.readersTitle', description: 'app.planned.readersDescription' } },
      ],
    },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
  ],
});

router.beforeEach(async (to) => {
  const session = useSessionStore(pinia);
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

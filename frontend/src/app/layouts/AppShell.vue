<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink, RouterView } from 'vue-router';
import { useSessionStore } from '../../stores/session';
import LocaleSwitcher from '../../ui/components/LocaleSwitcher.vue';

interface NavigationItem { to: string; labelKey: string; short: string; adminOnly?: boolean }

export default defineComponent({
  name: 'AppShell',
  components: { RouterLink, RouterView, LocaleSwitcher },
  data() {
    return {
      primaryNavigation: [
        { to: '/shelf', labelKey: 'app.navigation.shelf', short: 'S' },
        { to: '/explore', labelKey: 'app.navigation.explore', short: 'E' },
        { to: '/search', labelKey: 'app.navigation.search', short: 'Q' },
      ] as NavigationItem[],
      managementNavigation: [
        { to: '/sources', labelKey: 'app.navigation.sources', short: 'B' },
        { to: '/settings', labelKey: 'app.navigation.settings', short: 'P' },
        { to: '/account', labelKey: 'app.navigation.account', short: 'A' },
        { to: '/account/readers', labelKey: 'app.navigation.readers', short: 'R', adminOnly: true },
      ] as NavigationItem[],
      mobileMenuOpen: false,
    };
  },
  computed: {
    username(): string { return useSessionStore().account?.username ?? ''; },
    isAdministrator(): boolean { return useSessionStore().isAdministrator; },
    visibleManagementNavigation(): NavigationItem[] { return this.managementNavigation.filter((item) => !item.adminOnly || this.isAdministrator); },
  },
  watch: {
    $route() { this.mobileMenuOpen = false; },
  },
  methods: {
    async signOut() {
      const session = useSessionStore();
      try { await session.logout(); } finally { await this.$router.replace('/login'); }
    },
  },
});
</script>

<template>
  <div class="app-shell">
    <aside class="desktop-rail" :aria-label="$t('app.navigation.app')">
      <RouterLink class="brand" to="/shelf" :aria-label="`NovelReader ${$t('app.navigation.shelf')}`">N</RouterLink>
      <nav class="nav-group" :aria-label="$t('app.navigation.reading')">
        <RouterLink v-for="item in primaryNavigation" :key="item.to" :to="item.to"><span aria-hidden="true">{{ item.short }}</span>{{ $t(item.labelKey) }}</RouterLink>
      </nav>
      <nav class="nav-group nav-group--secondary" :aria-label="$t('app.navigation.management')">
        <RouterLink v-for="item in visibleManagementNavigation" :key="item.to" :to="item.to"><span aria-hidden="true">{{ item.short }}</span>{{ $t(item.labelKey) }}</RouterLink>
      </nav>
      <LocaleSwitcher class="desktop-locale" />
      <button class="account-button" type="button" :title="$t('app.navigation.signOutUser', { username })" @click="signOut">{{ $t('app.navigation.signOut') }}</button>
    </aside>

    <header class="mobile-header">
      <RouterLink class="mobile-brand" to="/shelf">NovelReader</RouterLink>
      <button type="button" class="menu-button" :aria-expanded="mobileMenuOpen" aria-controls="mobile-management" @click="mobileMenuOpen = !mobileMenuOpen">{{ username || $t('app.navigation.account') }}</button>
      <nav v-if="mobileMenuOpen" id="mobile-management" class="mobile-management" :aria-label="$t('app.navigation.accountManagement')">
        <LocaleSwitcher />
        <RouterLink v-for="item in visibleManagementNavigation" :key="item.to" :to="item.to">{{ $t(item.labelKey) }}</RouterLink>
        <button type="button" @click="signOut">{{ $t('app.navigation.signOut') }}</button>
      </nav>
    </header>

    <main class="app-content"><RouterView /></main>

    <nav class="mobile-tabs" :aria-label="$t('app.navigation.primary')">
      <RouterLink v-for="item in primaryNavigation" :key="item.to" :to="item.to"><span aria-hidden="true">{{ item.short }}</span>{{ $t(item.labelKey) }}</RouterLink>
    </nav>
  </div>
</template>

<style scoped>
.app-shell { min-height: 100dvh; display: grid; grid-template-columns: 13.5rem minmax(0, 1fr); }
.desktop-rail { position: sticky; top: 0; height: 100dvh; display: flex; flex-direction: column; gap: 1rem; padding: 1rem; border-right: 1px solid var(--color-border); background: var(--color-paper-raised); }
.brand { width: 3rem; height: 3rem; display: grid; place-items: center; margin-inline: auto; border-radius: 50%; background: var(--color-accent); color: white; font: 700 1.25rem var(--font-literary); text-decoration: none; }
.nav-group { display: grid; gap: .3rem; }
.nav-group--secondary { margin-top: auto; padding-top: 1rem; border-top: 1px solid var(--color-border); }
.nav-group a { min-height: 2.75rem; display: flex; align-items: center; gap: .75rem; padding: .55rem .75rem; border-radius: var(--radius-md); color: var(--color-ink-muted); text-decoration: none; font-weight: 650; }
.nav-group a span { width: 1.75rem; height: 1.75rem; display: grid; place-items: center; border-radius: .45rem; background: var(--color-paper-muted); font-size: .78rem; }
.nav-group a.router-link-active { background: var(--color-accent-soft); color: var(--color-accent-strong); }
.desktop-locale { display: flex; justify-content: center; }
.account-button { min-height: 2.75rem; border: 0; border-radius: var(--radius-md); background: transparent; color: var(--color-ink-muted); cursor: pointer; }
.app-content { min-width: 0; padding: clamp(1rem, 3vw, 2.5rem); }
.mobile-header, .mobile-tabs { display: none; }
@media (max-width: 48rem) {
  .app-shell { display: block; padding: 3.75rem 0 4.5rem; }
  .desktop-rail { display: none; }
  .mobile-header { position: fixed; z-index: 30; inset: 0 0 auto; height: 3.75rem; display: flex; align-items: center; justify-content: space-between; padding: .5rem 1rem; border-bottom: 1px solid var(--color-border); background: color-mix(in srgb, var(--color-paper-raised) 94%, transparent); backdrop-filter: blur(12px); }
  .mobile-brand { color: var(--color-ink); font: 700 1.15rem var(--font-literary); text-decoration: none; }
  .menu-button { min-height: 2.75rem; max-width: 9rem; overflow: hidden; text-overflow: ellipsis; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .5rem .75rem; background: var(--color-paper-raised); color: var(--color-ink); }
  .mobile-management { position: absolute; top: 3.5rem; right: 1rem; width: max-content; max-width: calc(100vw - 2rem); display: grid; padding: .5rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-card); }
  .mobile-management a, .mobile-management button { min-height: 2.75rem; display: flex; align-items: center; border: 0; border-radius: var(--radius-sm); padding: .6rem .75rem; background: transparent; color: var(--color-ink); text-decoration: none; }
  .app-content { padding: 1rem; }
  .mobile-tabs { position: fixed; z-index: 25; inset: auto 0 0; min-height: 4.25rem; display: grid; grid-template-columns: repeat(3, 1fr); padding: .35rem max(.5rem, env(safe-area-inset-right)) max(.35rem, env(safe-area-inset-bottom)) max(.5rem, env(safe-area-inset-left)); border-top: 1px solid var(--color-border); background: var(--color-paper-raised); }
  .mobile-tabs a { min-height: 3.5rem; display: grid; place-content: center; justify-items: center; gap: .15rem; border-radius: var(--radius-md); color: var(--color-ink-muted); text-decoration: none; font-size: .75rem; }
  .mobile-tabs a span { font-size: .8rem; font-weight: 800; }
  .mobile-tabs a.router-link-active { background: var(--color-accent-soft); color: var(--color-accent-strong); }
}
</style>

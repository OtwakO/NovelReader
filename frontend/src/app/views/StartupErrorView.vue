<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import { useSessionStore } from '../../stores/session';

export default defineComponent({
  name: 'StartupErrorView',
  components: { AppButton },
  computed: {
    message(): string { return useSessionStore().message || this.$t('app.startup.fallback'); },
  },
  methods: {
    async retry() {
      const session = useSessionStore();
      session.initialized = false;
      await session.initialize();
      await this.$router.replace('/');
    },
  },
});
</script>

<template>
  <main class="state-page">
    <section>
      <h1>{{ $t('app.startup.title') }}</h1>
      <p role="alert">{{ message }}</p>
      <AppButton @click="retry">{{ $t('app.common.retry') }}</AppButton>
    </section>
  </main>
</template>

<style scoped>
.state-page { min-height: 100dvh; display: grid; place-items: center; padding: 1rem; }
section { width: min(100%, 32rem); padding: 2rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); text-align: center; }
h1 { font-family: var(--font-literary); }
p { color: var(--color-ink-muted); line-height: 1.6; }
</style>

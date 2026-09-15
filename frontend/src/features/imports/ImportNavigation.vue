<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import { useImportQueue } from './import-queue';
export default defineComponent({
  components: { RouterLink }, props: { compact: Boolean },
  data: () => ({ queue: useImportQueue() }),
});
</script>

<template>
  <RouterLink v-if="!compact || queue.outstanding || queue.attention" to="/imports" :class="{ compact }" :aria-label="`${$t('imports.title')}: ${$t('imports.navigationStatus', { pending: queue.outstanding, attention: queue.attention })}`">
    <span v-if="!compact" class="import-nav-icon" aria-hidden="true">T</span>
    {{ compact ? $t('imports.activity', { count: queue.outstanding || queue.attention }) : $t('imports.title') }}
    <small v-if="!compact && (queue.outstanding || queue.attention)">{{ $t('imports.navigationStatus', { pending: queue.outstanding, attention: queue.attention }) }}</small>
  </RouterLink>
</template>

<style scoped>
a { flex-wrap: wrap; }
.import-nav-icon { display: grid; place-items: center; width: 1.65rem; height: 1.65rem; flex: 0 0 auto; border-radius: .4rem; background: var(--color-paper-muted); font-size: var(--text-caption); }
.compact { font-size: var(--text-caption); padding: .5rem; }
small { display: block; width: 100%; font-size: var(--text-caption); font-weight: var(--weight-regular); }
</style>

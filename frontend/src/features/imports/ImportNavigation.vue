<script lang="ts">
import { defineComponent } from 'vue';
import AppIcon from '../../ui/components/AppIcon.vue';
import { RouterLink } from 'vue-router';
import { useImportQueue } from './import-queue';
export default defineComponent({
  components: { RouterLink, AppIcon }, props: { compact: Boolean },
  data: () => ({ queue: useImportQueue() }),
});
</script>

<template>
  <RouterLink v-if="!compact || queue.outstanding || queue.attention" to="/imports" :class="{ compact }" :aria-label="`${$t('imports.title')}: ${$t('imports.navigationStatus', { pending: queue.outstanding, attention: queue.attention })}`">
    <AppIcon v-if="!compact" name="upload" />
    {{ compact ? $t('imports.activity', { count: queue.outstanding || queue.attention }) : $t('imports.title') }}
    <small v-if="!compact && (queue.outstanding || queue.attention)">{{ $t('imports.navigationStatus', { pending: queue.outstanding, attention: queue.attention }) }}</small>
  </RouterLink>
</template>

<style scoped>
a { flex-wrap: wrap; }
.compact { font-size: var(--text-caption); padding: .5rem; }
small { display: block; width: 100%; font-size: var(--text-caption); font-weight: var(--weight-regular); }
</style>

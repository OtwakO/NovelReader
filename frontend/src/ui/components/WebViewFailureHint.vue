<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({ name: 'WebViewFailureHint' });
</script>

<template>
  <p class="webview-hint">
    {{ $t('app.webviewFailureHint.message') }}
    <RouterLink to="/settings">{{ $t('app.webviewFailureHint.action') }}</RouterLink>
  </p>
</template>

<style scoped>
.webview-hint{margin:.65rem 0 0;color:var(--color-ink-muted);font-size:.84rem;line-height:1.5}.webview-hint a{color:var(--color-accent);font-weight:700}
</style>

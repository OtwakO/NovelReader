<script lang="ts">
import { defineComponent, type PropType } from 'vue';

export default defineComponent({
  name: 'AppButton',
  props: {
    variant: {
      type: String as PropType<'primary' | 'secondary' | 'quiet' | 'danger'>,
      default: 'primary',
    },
    type: {
      type: String as PropType<'button' | 'submit' | 'reset'>,
      default: 'button',
    },
    disabled: Boolean,
    busy: Boolean,
  },
});
</script>

<template>
  <button class="app-button" :class="`app-button--${variant}`" :type="type" :disabled="disabled || busy" :aria-busy="busy || undefined">
    <slot />
  </button>
</template>

<style scoped>
.app-button { min-height: 2.75rem; display: inline-flex; align-items: center; justify-content: center; gap: .5rem; border: 1px solid transparent; border-radius: var(--radius-md); padding: .65rem 1rem; font-weight: 700; cursor: pointer; }
.app-button:disabled { opacity: .62; cursor: wait; }
.app-button--primary { background: var(--color-accent); color: white; }
.app-button--primary:hover:not(:disabled) { background: var(--color-accent-strong); }
.app-button--secondary { border-color: var(--color-border); background: var(--color-paper-raised); color: var(--color-ink); }
.app-button--quiet { background: transparent; color: var(--color-accent); }
.app-button--danger { background: var(--color-danger); color: white; }
</style>

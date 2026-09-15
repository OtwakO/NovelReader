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

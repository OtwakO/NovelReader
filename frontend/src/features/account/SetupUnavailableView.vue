<script lang="ts">
import { defineComponent } from 'vue';
import AuthCard from '../../ui/components/AuthCard.vue';

export default defineComponent({ name: 'SetupUnavailableView', components: { AuthCard } });
</script>

<template><AuthCard :title="$t('account.setup.unavailableTitle')" :intro="$t('account.setup.unavailableIntro')" /></template>

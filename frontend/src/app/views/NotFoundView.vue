<script lang="ts">
import { defineComponent } from 'vue';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';

export default defineComponent({ name: 'NotFoundView', components: { FeatureScaffold } });
</script>

<template><FeatureScaffold :title="$t('app.notFound.title')" :description="$t('app.notFound.description')"><RouterLink to="/shelf">{{ $t('app.notFound.action') }}</RouterLink></FeatureScaffold></template>

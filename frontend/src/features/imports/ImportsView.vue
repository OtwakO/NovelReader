<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';
import ImportWorkspace from './ImportWorkspace.vue';

export default defineComponent({
  name: 'ImportsView',
  components: { FeatureScaffold, RouterLink, ImportWorkspace },
  computed: { reviewId(): string { return typeof this.$route.query.review === 'string' ? this.$route.query.review : ''; } },
  methods: {
    closeReview() { if (this.reviewId) void this.$router.replace({ path: '/imports' }); },
  },
});
</script>

<template>
  <FeatureScaffold class="imports-page" :title="$t('imports.title')" :description="$t('imports.flow.pageHint')">
    <template #actions>
      <RouterLink class="app-button app-button--secondary" to="/shelf">{{ $t('imports.flow.backToShelf') }}</RouterLink>
    </template>
    <ImportWorkspace :initial-review="reviewId" @review-closed="closeReview" />
  </FeatureScaffold>
</template>

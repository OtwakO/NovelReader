<script lang="ts">
import { defineComponent } from 'vue';
import ImportWorkspace from './ImportWorkspace.vue';
export default defineComponent({
  components: { ImportWorkspace },
  computed: { reviewId(): string { return typeof this.$route.query.review === 'string' ? this.$route.query.review : ''; } },
  methods: { closeReview() { if (this.reviewId) void this.$router.replace({ path: '/imports' }); } },
});
</script>
<template>
  <div class="imports-page">
    <header class="import-page-heading"><h1>{{ $t('imports.title') }}</h1><p>{{ $t('imports.flow.pageHint') }}</p></header>
    <ImportWorkspace :initial-review="reviewId" @review-closed="closeReview" />
  </div>
</template>

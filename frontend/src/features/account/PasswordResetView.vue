<script lang="ts">
import { defineComponent } from 'vue';
import { completePasswordReset } from '../../api/client';
import AuthCard from '../../ui/components/AuthCard.vue';
import AppButton from '../../ui/components/AppButton.vue';

export default defineComponent({
  name: 'PasswordResetView',
  components: { AuthCard, AppButton },
  data() { return { token: '', password: '', confirmation: '', error: '', submitting: false }; },
  methods: {
    async submit() {
      this.error = '';
      if (this.password !== this.confirmation) { this.error = this.$t('account.reset.mismatch'); return; }
      this.submitting = true;
      try {
        await completePasswordReset(this.token, this.password);
        await this.$router.replace({ path: '/login', query: { message: 'password-reset' } });
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('account.reset.failed');
      } finally {
        this.token = ''; this.password = ''; this.confirmation = ''; this.submitting = false;
      }
    },
  },
});
</script>

<template>
  <AuthCard :title="$t('account.reset.title')" :intro="$t('account.reset.intro')">
    <form @submit.prevent="submit">
      <label>{{ $t('account.reset.token') }}<input v-model="token" autocomplete="off" required :disabled="submitting"></label>
      <label>{{ $t('account.reset.newPassword') }}<input v-model="password" type="password" minlength="12" autocomplete="new-password" required :disabled="submitting"></label>
      <label>{{ $t('account.reset.confirm') }}<input v-model="confirmation" type="password" minlength="12" autocomplete="new-password" required :disabled="submitting"></label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <AppButton type="submit" :busy="submitting">{{ submitting ? $t('account.reset.submitting') : $t('account.reset.submit') }}</AppButton>
    </form>
    <div class="back"><RouterLink to="/login">{{ $t('account.common.backToLogin') }}</RouterLink></div>
  </AuthCard>
</template>

<style scoped>.back { margin-top: 1rem; text-align: center; }</style>

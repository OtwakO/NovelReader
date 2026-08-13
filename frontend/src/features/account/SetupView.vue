<script lang="ts">
import { defineComponent } from 'vue';
import { createInitialAdministrator } from '../../api/client';
import AuthCard from '../../ui/components/AuthCard.vue';
import AppButton from '../../ui/components/AppButton.vue';
import { useSessionStore } from '../../stores/session';

export default defineComponent({
  name: 'SetupView',
  components: { AuthCard, AppButton },
  data() { return { token: '', username: '', password: '', confirmation: '', error: '', submitting: false }; },
  methods: {
    async submit() {
      this.error = '';
      if (this.password !== this.confirmation) { this.error = this.$t('account.setup.mismatch'); return; }
      this.submitting = true;
      try {
        const account = await createInitialAdministrator(this.token, this.username, this.password);
        useSessionStore().authenticated(account);
        this.token = ''; this.password = ''; this.confirmation = '';
        await this.$router.replace('/shelf');
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('account.setup.failed');
      } finally { this.submitting = false; }
    },
  },
});
</script>

<template>
  <AuthCard :title="$t('account.setup.title')" :intro="$t('account.setup.intro')">
    <form @submit.prevent="submit">
      <label>{{ $t('account.setup.token') }}<input v-model="token" type="password" autocomplete="off" required :disabled="submitting"></label>
      <small>{{ $t('account.setup.tokenHint') }}</small>
      <label>{{ $t('account.login.username') }}<input v-model.trim="username" minlength="3" maxlength="32" autocomplete="username" required :disabled="submitting"></label>
      <label>{{ $t('account.login.password') }}<input v-model="password" type="password" minlength="12" maxlength="128" autocomplete="new-password" required :disabled="submitting"></label>
      <label>{{ $t('account.setup.confirm') }}<input v-model="confirmation" type="password" minlength="12" maxlength="128" autocomplete="new-password" required :disabled="submitting"></label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <AppButton type="submit" :busy="submitting">{{ submitting ? $t('account.setup.submitting') : $t('account.setup.submit') }}</AppButton>
    </form>
    <p class="aftercare">{{ $t('account.setup.aftercare') }}</p>
  </AuthCard>
</template>

<style scoped>small, .aftercare { color: var(--color-ink-muted); line-height: 1.5; }.aftercare { margin: 1rem 0 0; }</style>

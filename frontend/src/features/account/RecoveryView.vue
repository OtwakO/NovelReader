<script lang="ts">
import { defineComponent } from 'vue';
import { getRecoveryStatus, recoverAdministrator, type RecoveryAction } from '../../api/auth';
import { ApiError } from '../../api/transport';
import AuthCard from '../../ui/components/AuthCard.vue';
import AppButton from '../../ui/components/AppButton.vue';
import { useSessionStore } from '../../stores/session';

export default defineComponent({
  name: 'RecoveryView',
  components: { AuthCard, AppButton },
  data() {
    return { available: null as boolean | null, action: 'reset_existing' as RecoveryAction, token: '', username: '', password: '', confirmation: '', error: '', submitting: false };
  },
  async mounted() {
    try { this.available = (await getRecoveryStatus()).available; }
    catch (cause) { this.available = false; if (!(cause instanceof ApiError && cause.status === 404)) this.error = cause instanceof Error ? cause.message : this.$t('account.recovery.statusFailed'); }
  },
  methods: {
    async submit() {
      this.error = '';
      if (this.password !== this.confirmation) { this.error = this.$t('account.recovery.mismatch'); return; }
      this.submitting = true;
      try {
        const account = await recoverAdministrator(this.token, this.action, this.username, this.password);
        useSessionStore().authenticated(account);
        await this.$router.replace('/shelf');
      } catch (cause) { this.error = cause instanceof Error ? cause.message : this.$t('account.recovery.failed'); }
      finally { this.token = ''; this.password = ''; this.confirmation = ''; this.submitting = false; }
    },
  },
});
</script>

<template>
  <AuthCard :title="$t('account.recovery.title')" :intro="$t('account.recovery.intro')">
    <p v-if="available === null" aria-busy="true">{{ $t('account.recovery.checking') }}</p>
    <template v-else-if="available">
      <form @submit.prevent="submit">
        <fieldset :disabled="submitting">
          <legend>{{ $t('account.recovery.action') }}</legend>
          <label><input v-model="action" type="radio" value="reset_existing">{{ $t('account.recovery.resetExisting') }}</label>
          <label><input v-model="action" type="radio" value="create_replacement">{{ $t('account.recovery.createReplacement') }}</label>
        </fieldset>
        <label>{{ $t('account.recovery.token') }}<input v-model="token" type="password" autocomplete="off" required :disabled="submitting"></label>
        <label>{{ $t('account.recovery.username') }}<input v-model.trim="username" autocomplete="username" required :disabled="submitting"></label>
        <label>{{ $t('account.recovery.newPassword') }}<input v-model="password" type="password" minlength="12" autocomplete="new-password" required :disabled="submitting"></label>
        <label>{{ $t('account.recovery.confirm') }}<input v-model="confirmation" type="password" minlength="12" autocomplete="new-password" required :disabled="submitting"></label>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <AppButton type="submit" :busy="submitting">{{ submitting ? $t('account.recovery.submitting') : $t('account.recovery.submit') }}</AppButton>
      </form>
    </template>
    <p v-else role="status">{{ $t('account.recovery.unavailable') }}</p>
    <div class="back"><RouterLink to="/login">{{ $t('account.common.backToLogin') }}</RouterLink></div>
  </AuthCard>
</template>

<style scoped>fieldset { display: grid; gap: .6rem; margin: 0; padding: .8rem; border: 1px solid var(--color-border); border-radius: var(--radius-sm); }.back { margin-top: 1rem; text-align: center; }</style>

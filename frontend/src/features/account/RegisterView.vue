<script lang="ts">
import { defineComponent } from 'vue';
import AuthCard from '../../ui/components/AuthCard.vue';
import AppButton from '../../ui/components/AppButton.vue';
import { useSessionStore } from '../../stores/session';

export default defineComponent({
  name: 'RegisterView',
  components: { AuthCard, AppButton },
  data() { return { username: '', password: '', inviteCode: '', error: '', submitting: false }; },
  computed: {
    inviteRequired(): boolean { return useSessionStore().registrationInviteRequired; },
  },
  methods: {
    async submit() {
      this.error = '';
      this.submitting = true;
      try {
        const session = useSessionStore();
        await session.register(this.username, this.password, this.inviteCode);
        this.password = '';
        this.inviteCode = '';
        await this.$router.replace('/shelf');
      } catch (cause) {
        this.password = '';
        this.inviteCode = '';
        this.error = cause instanceof Error ? cause.message : this.$t('account.register.failed');
      } finally { this.submitting = false; }
    },
  },
});
</script>

<template>
  <AuthCard :title="$t('account.register.title')" :intro="$t('account.register.intro')">
    <form @submit.prevent="submit">
      <label>{{ $t('account.login.username') }}<input v-model.trim="username" autocomplete="username" required :disabled="submitting"></label>
      <label>{{ $t('account.login.password') }}<input v-model="password" type="password" minlength="12" autocomplete="new-password" required :disabled="submitting"></label>
      <label v-if="inviteRequired">{{ $t('account.register.invite') }}<input v-model="inviteCode" type="password" autocomplete="off" required :disabled="submitting"></label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <AppButton type="submit" :busy="submitting">{{ submitting ? $t('account.register.submitting') : $t('account.register.submit') }}</AppButton>
    </form>
    <div class="back"><RouterLink to="/login">{{ $t('account.common.backToLogin') }}</RouterLink></div>
  </AuthCard>
</template>

<style scoped>.back { margin-top: 1rem; text-align: center; }</style>

<script lang="ts">
import { defineComponent } from 'vue';
import AuthCard from '../../ui/components/AuthCard.vue';
import AppButton from '../../ui/components/AppButton.vue';
import { useSessionStore } from '../../stores/session';

export default defineComponent({
  name: 'LoginView',
  components: { AuthCard, AppButton },
  data() {
    return { username: '', password: '', error: '', submitting: false };
  },
  computed: {
    registrationEnabled(): boolean { return useSessionStore().registrationEnabled; },
    sessionMessage(): string {
      const session = useSessionStore();
      if (session.notice === 'authentication-lost') return this.$t('account.login.expired');
      return session.message;
    },
  },
  methods: {
    async submit() {
      this.error = '';
      this.submitting = true;
      try {
        const session = useSessionStore();
        await session.login(this.username, this.password);
        this.password = '';
        await this.$router.replace(session.returnTo || '/shelf');
      } catch (cause) {
        this.password = '';
        this.error = cause instanceof Error ? cause.message : this.$t('account.login.failed');
      } finally {
        this.submitting = false;
      }
    },
  },
});
</script>

<template>
  <AuthCard :title="$t('account.login.title')" :intro="$t('account.login.intro')">
    <p v-if="sessionMessage" class="session-message" role="status">{{ sessionMessage }}</p>
    <form @submit.prevent="submit">
      <label>{{ $t('account.login.username') }}<input v-model.trim="username" autocomplete="username" required :disabled="submitting"></label>
      <label>{{ $t('account.login.password') }}<input v-model="password" type="password" autocomplete="current-password" required :disabled="submitting"></label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <AppButton type="submit" :busy="submitting">{{ submitting ? $t('account.login.submitting') : $t('account.login.submit') }}</AppButton>
    </form>
    <nav class="links" :aria-label="$t('account.login.help')">
      <RouterLink v-if="registrationEnabled" to="/register">{{ $t('account.login.register') }}</RouterLink>
      <RouterLink to="/password-reset">{{ $t('account.login.reset') }}</RouterLink>
      <RouterLink to="/recovery">{{ $t('account.login.recovery') }}</RouterLink>
    </nav>
  </AuthCard>
</template>

<style scoped>
.session-message { padding: .7rem; border-radius: var(--radius-sm); background: #fff2d8; color: #704b13; }
.links { display: flex; justify-content: center; flex-wrap: wrap; gap: .6rem 1rem; margin-top: 1rem; font-size: .9rem; }
</style>

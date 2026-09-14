<template>
  <div class="login-overlay">
    <div class="login-card">
      <h2 class="login-title">{{ $t('auth.login_title') }}</h2>
      <p class="login-subtitle">{{ $t('auth.login_subtitle') }}</p>

      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-field">
          <label>{{ $t('auth.username') }}</label>
          <input
            v-model="username"
            type="text"
            :placeholder="$t('auth.username')"
            data-testid="login-username"
            autocomplete="username"
          />
        </div>

        <div class="form-field">
          <label>{{ $t('auth.password') }}</label>
          <input
            v-model="password"
            type="password"
            :placeholder="$t('auth.password')"
            data-testid="login-password"
            autocomplete="current-password"
          />
        </div>

        <div class="error-slot" :class="{ visible: error }">
          <span v-if="error">{{ error }}</span>
        </div>

        <AppButton
          variant="primary"
          :loading="loading"
          :disabled="!username || !password"
          test-id="login-submit"
          style="width: 100%"
          @click="handleLogin"
        >
          {{ $t('auth.login') }}
        </AppButton>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import AppButton from '../components/ui/AppButton.vue'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  if (!username.value || !password.value) return
  loading.value = true
  error.value = ''
  try {
    await auth.login(username.value, password.value)
    router.push('/')
  } catch (e: any) {
    error.value = 'Неверное имя пользователя или пароль'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-overlay {
  position: fixed;
  inset: 0;
  background: var(--bg);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-login);
}

.login-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 40px;
  width: 380px;
  max-width: 90vw;
  box-shadow: var(--shadow);
}

.login-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  text-align: center;
  letter-spacing: -0.4px;
}

.login-subtitle {
  font-size: 13px;
  color: var(--text-muted);
  text-align: center;
  margin-top: 8px;
  margin-bottom: 32px;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin-bottom: 6px;
}

.form-field input {
  width: 100%;
  padding: 10px 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  font-size: 13px;
  transition: border-color 0.15s;
}

.form-field input:focus {
  border-color: var(--accent);
}

.error-slot {
  min-height: 20px;
  font-size: 12px;
  color: var(--danger);
  text-align: center;
  opacity: 0;
  transition: opacity 0.15s;
}

.error-slot.visible {
  opacity: 1;
}
</style>

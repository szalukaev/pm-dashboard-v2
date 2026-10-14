<template>
  <div class="login-overlay">
    <LoginBoard class="login-backdrop" />
    <div class="login-veil"></div>
    <div class="login-center">
      <div class="login-card">
        <h2 class="login-title">{{ greeting }}</h2>

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
      <LoginCountdown class="login-countdown-below" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import i18n from '../i18n'
import AppButton from '../components/ui/AppButton.vue'
import LoginBoard from '../components/login/LoginBoard.vue'
import LoginCountdown from '../components/login/LoginCountdown.vue'

const hour = new Date().getHours()
const greeting = i18n.global.t(
  hour < 5 ? 'auth.greeting.night'
    : hour < 12 ? 'auth.greeting.morning'
      : hour < 18 ? 'auth.greeting.day'
        : 'auth.greeting.evening',
)

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
    // Возврат туда, откуда пользователя отправили на вход; только внутренний
    // адрес — чужой сайт в параметре не откроется
    const redirect = router.currentRoute.value.query.redirect
    // Переход с полной загрузкой страницы: во вкладке могли остаться данные
    // и настройки прежнего пользователя (выход, истёкшая сессия), и новому
    // они достаться не должны
    window.location.assign(typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/')
  } catch (e: any) {
    const code = e?.response?.data?.error
    const key = code === 'USER_BLOCKED' ? 'error_blocked'
      : code === 'TOO_MANY_ATTEMPTS' ? 'error_too_many_attempts'
      // Сбой на сервере — не повод говорить, что пароль неверный
      : (e?.response?.status ?? 500) >= 500 ? 'error_server'
      : 'error_invalid_credentials'
    error.value = i18n.global.t(`auth.${key}`)
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
  overflow: hidden;
}

/* Доска видна по краям и гаснет к центру, где стоит форма */
.login-veil {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(
    ellipse 30% 46% at 50% 50%,
    var(--bg) 60%,
    transparent 100%
  );
}

.login-center {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 400px;
  max-width: 90vw;
}

.login-card {
  width: 100%;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 30px 30px 26px;
  box-shadow: 0 30px 70px -20px rgba(0, 0, 0, 0.55);
}

.login-title {
  margin: 0 0 24px;
  font-size: 28px;
  font-weight: 700;
  color: var(--text-bright);
  letter-spacing: -0.8px;
}

/* Табло стоит под формой и не участвует в центрировании —
   форма остаётся ровно посередине экрана */
.login-countdown-below {
  position: absolute;
  top: calc(100% + 72px);
  left: 50%;
  transform: translateX(-50%);
  white-space: nowrap;
}

@media (max-width: 700px) {
  .login-backdrop {
    display: none;
  }
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
  margin-bottom: 6px;
}

.form-field input {
  width: 100%;
  padding: 11px 12px;
  background: var(--bg);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  font-size: 13px;
  transition: border-color 0.15s;
}

.form-field input:focus-visible {
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

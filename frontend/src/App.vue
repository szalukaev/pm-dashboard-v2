<template>
  <div id="app">
    <router-view />

    <!-- Force password change modal -->
    <AppModal
      :model-value="showForceChange"
      title="Смена пароля"
      width="400px"
    >
      <p class="force-hint">Администратор создал ваш аккаунт. Для безопасности необходимо сменить пароль.</p>
      <form @submit.prevent="submitPasswordChange" class="force-form">
        <div class="form-field">
          <label>Новый пароль</label>
          <input v-model="newPassword" type="password" minlength="6" />
        </div>
        <div class="form-field">
          <label>Подтверждение</label>
          <input v-model="confirmPassword" type="password" minlength="6" />
        </div>
        <div v-if="passwordError" class="error-msg">{{ passwordError }}</div>
      </form>
      <template #footer>
        <AppButton
          variant="primary"
          :disabled="newPassword.length < 6 || newPassword !== confirmPassword"
          @click="submitPasswordChange"
        >
          Сменить пароль
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from './stores/auth'
import AppModal from './components/ui/AppModal.vue'
import AppButton from './components/ui/AppButton.vue'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')

const showForceChange = computed(() => {
  return user.value?.force_password_change === true
})

async function submitPasswordChange() {
  if (newPassword.value.length < 6) {
    passwordError.value = 'Минимум 6 символов'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = 'Пароли не совпадают'
    return
  }
  try {
    await authStore.changePassword('', newPassword.value)
    passwordError.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch {
    passwordError.value = 'Ошибка смены пароля'
  }
}
</script>

<style scoped>
.force-hint {
  font-size: 13px;
  color: var(--text-dim);
  margin-bottom: 16px;
}

.force-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin-bottom: 4px;
}

.form-field input {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.form-field input:focus {
  border-color: var(--accent);
}

.error-msg {
  font-size: 12px;
  color: var(--danger);
}
</style>

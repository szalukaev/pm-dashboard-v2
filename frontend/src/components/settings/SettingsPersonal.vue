<template>
  <div class="settings-personal">
    <h3 class="section-title">Личные данные</h3>
    <div class="personal-layout">
      <!-- Avatar -->
      <div class="avatar-block">
        <div class="avatar-circle" @click="triggerFileInput">
          <img v-if="user?.avatar" :src="user.avatar" class="avatar-img" />
          <span v-else class="avatar-placeholder">{{ initials }}</span>
          <div class="avatar-overlay">
            <Camera :size="18" />
          </div>
        </div>
        <input ref="fileInput" type="file" accept="image/*" style="display:none" @change="onFileSelected" />
        <p class="avatar-hint">Нажмите, чтобы {{ user?.avatar ? 'изменить' : 'добавить' }} фото</p>
        <AppButton v-if="user?.avatar" variant="ghost" size="sm" @click="removeAvatar">Удалить фото</AppButton>
      </div>

      <!-- Info -->
      <div class="form-group">
        <div class="info-row">
          <span class="info-label">Отображаемое имя</span>
          <span class="info-value editable-name">
            <input
              v-model="editDisplayName"
              class="name-input"
              placeholder="Введите отображаемое имя"
              maxlength="255"
              @keyup.enter="saveDisplayName"
            />
            <AppButton
              variant="ghost"
              size="sm"
              :loading="savingName"
              :disabled="!editDisplayName.trim() || editDisplayName === (user?.display_name || '')"
              @click="saveDisplayName"
            >
              Сохранить
            </AppButton>
          </span>
        </div>
        <div class="info-row">
          <span class="info-label">Логин</span>
          <span class="info-value">{{ user?.username || '—' }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Роль</span>
          <span class="info-value">
            <span class="role-badge" :class="user?.role">{{ user?.role === 'admin' ? 'Администратор' : 'Пользователь' }}</span>
          </span>
        </div>
        <div class="info-row">
          <span class="info-label">Последний вход</span>
          <span class="info-value">{{ formatLastLogin(user?.last_login) }}</span>
        </div>
      </div>
    </div>

    <div class="form-actions">
      <AppButton variant="primary" @click="showChangePassword = true">
        Изменить пароль
      </AppButton>
    </div>

    <!-- The user's account in the data source -->
    <div v-if="user?.source?.type" class="source-block">
      <h4 class="source-title">{{ $t('source_token.title') }}</h4>
      <p class="source-hint">{{ $t('source_token.hint') }}</p>
      <SourceTokenForm allow-unlink />
    </div>

    <!-- Change password modal -->
    <AppModal v-model="showChangePassword" title="Изменение пароля" width="400px">
      <form @submit.prevent="submitPasswordChange" class="pw-form">
        <div class="form-field">
          <label>Текущий пароль</label>
          <input v-model="currentPassword" type="password" />
        </div>
        <div class="form-field">
          <label>Новый пароль</label>
          <input v-model="newPassword" type="password" minlength="6" />
        </div>
        <div class="form-field">
          <label>Подтверждение пароля</label>
          <input v-model="confirmPassword" type="password" minlength="6" />
        </div>
        <div v-if="passwordError" class="error-msg">{{ passwordError }}</div>
        <div v-if="passwordSuccess" class="success-msg">{{ passwordSuccess }}</div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="showChangePassword = false">Отмена</AppButton>
        <AppButton
          variant="primary"
          :disabled="!currentPassword || newPassword.length < 6 || newPassword !== confirmPassword"
          :loading="changingPassword"
          @click="submitPasswordChange"
        >
          Сохранить
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { storeToRefs } from 'pinia'
import { Camera } from 'lucide-vue-next'
import axios from 'axios'
import AppButton from '../ui/AppButton.vue'
import AppModal from '../ui/AppModal.vue'
import SourceTokenForm from './SourceTokenForm.vue'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const fileInput = ref<HTMLInputElement>()
const showChangePassword = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')
const passwordSuccess = ref('')
const changingPassword = ref(false)

const editDisplayName = ref(user.value?.display_name || '')
const savingName = ref(false)

async function saveDisplayName() {
  const name = editDisplayName.value.trim()
  if (!name) return
  savingName.value = true
  try {
    await authStore.updateMe(name)
  } finally {
    savingName.value = false
  }
}

const initials = computed(() => {
  const name = user.value?.display_name || user.value?.username || '?'
  return name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)
})

function triggerFileInput() {
  fileInput.value?.click()
}

async function onFileSelected(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const formData = new FormData()
  formData.append('avatar', file)
  try {
    const { data } = await axios.post('/api/auth/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    if (user.value) user.value.avatar = data.avatar
  } catch {}
  ;(e.target as HTMLInputElement).value = ''
}

async function removeAvatar() {
  try {
    await axios.delete('/api/auth/avatar')
    if (user.value) user.value.avatar = ''
  } catch {}
}

function formatLastLogin(val?: string) {
  if (!val) return '—'
  try {
    return new Date(val).toLocaleString('ru-RU', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit',
    })
  } catch {
    return val
  }
}

async function submitPasswordChange() {
  passwordError.value = ''
  passwordSuccess.value = ''
  if (newPassword.value.length < 6) {
    passwordError.value = 'Минимум 6 символов'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = 'Пароли не совпадают'
    return
  }
  changingPassword.value = true
  try {
    await authStore.changePassword(currentPassword.value, newPassword.value)
    passwordSuccess.value = 'Пароль успешно изменён'
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    setTimeout(() => { showChangePassword.value = false; passwordSuccess.value = '' }, 1500)
  } catch {
    passwordError.value = 'Ошибка смены пароля. Проверьте текущий пароль.'
  } finally {
    changingPassword.value = false
  }
}
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 20px;
}

.personal-layout {
  display: flex;
  gap: 32px;
  margin-bottom: 24px;
  align-items: flex-start;
}

.avatar-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.avatar-circle {
  width: 96px;
  height: 96px;
  border-radius: 50%;
  background: var(--surface-2);
  border: 2px solid var(--hairline);
  cursor: pointer;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border-color 0.15s;
}

.avatar-circle:hover {
  border-color: var(--accent);
}

.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-placeholder {
  font-size: 28px;
  font-weight: 600;
  color: var(--text-muted);
}

.avatar-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s;
  color: #fff;
}

.avatar-circle:hover .avatar-overlay {
  opacity: 1;
}

.avatar-hint {
  font-size: 11px;
  color: var(--text-muted);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.info-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  min-width: 180px;
}

.info-value {
  font-size: 13px;
  color: var(--text);
}

.role-badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
}

.role-badge.admin {
  background: var(--accent-bg);
  color: var(--accent);
}

.role-badge.user {
  background: var(--tag-bg);
  color: var(--text-muted);
}

.form-actions {
  display: flex;
  gap: 8px;
}

.source-block {
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid var(--border-light);
}

.source-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 6px;
}

.source-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 12px;
  max-width: 560px;
}

.pw-form {
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

.editable-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.name-input {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  padding: 6px 10px;
  font-size: 13px;
  width: 220px;
  outline: none;
}

.name-input:focus {
  border-color: var(--accent);
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

.success-msg {
  font-size: 12px;
  color: var(--success);
}
</style>

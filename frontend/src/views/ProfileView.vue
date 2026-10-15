<template>
  <!-- Профиль на телефоне: /profile, /profile/password, /profile/key -->
  <div class="profile-view" data-testid="profile-page">
    <!-- Пароль -->
    <template v-if="section === 'password'">
      <div class="p-group">
        <label class="p-field">
          <span>{{ $t('auth.password_change.current') }}</span>
          <input v-model="currentPassword" type="password" autocomplete="current-password" data-testid="profile-password-current" />
        </label>
        <label class="p-field">
          <span>{{ $t('auth.password_change.new') }}</span>
          <input v-model="newPassword" type="password" autocomplete="new-password" data-testid="profile-password-new" />
        </label>
        <label class="p-field">
          <span>{{ $t('auth.password_change.confirm') }}</span>
          <input v-model="confirmPassword" type="password" autocomplete="new-password" data-testid="profile-password-confirm" />
        </label>
      </div>
      <p v-if="passwordError" class="p-error">{{ passwordError }}</p>
      <p v-else class="p-note">{{ $t('profile.password_hint') }}</p>
      <button
        class="p-save"
        :disabled="changingPassword || !currentPassword || newPassword.length < 6 || newPassword !== confirmPassword"
        data-testid="profile-password-save"
        @click="submitPassword"
      >{{ $t('auth.password_change.change') }}</button>
    </template>

    <!-- Ключ Redmine -->
    <template v-else-if="section === 'key'">
      <p class="p-note p-note-top">{{ $t('source_token.hint') }}</p>
      <div class="p-group p-group-pad">
        <SourceTokenForm allow-unlink />
      </div>
    </template>

    <!-- Профиль -->
    <template v-else>
      <div class="p-hero">
        <button class="p-avatar" :aria-label="$t(user?.avatar ? 'settings.personal.avatar_change' : 'settings.personal.avatar_add')" @click="fileInput?.click()">
          <img v-if="user?.avatar" :src="user.avatar" alt="" />
          <span v-else>{{ initials }}</span>
          <i class="p-cam"><Camera :size="15" /></i>
        </button>
        <input ref="fileInput" type="file" accept="image/*" hidden @change="onFileSelected" />
        <b>{{ user?.display_name || user?.username }}</b>
        <span class="p-sub">
          {{ $t(user?.role === 'admin' ? 'access.users.role_admin' : 'access.users.role_user') }}
          <template v-if="user?.last_login"> · {{ $t('profile.last_login', { at: formatDateTime(user.last_login, { year: 'none', empty: '' }) }) }}</template>
        </span>
        <button v-if="user?.avatar" class="p-link" @click="removeAvatar">{{ $t('settings.personal.avatar_remove') }}</button>
      </div>

      <div class="p-label">{{ $t('settings.personal.title') }}</div>
      <div class="p-group">
        <label class="p-field">
          <span>{{ $t('settings.personal.display_name') }}</span>
          <input v-model="name" maxlength="255" :placeholder="$t('settings.personal.display_name_placeholder')" data-testid="profile-name" />
        </label>
        <label class="p-field">
          <span>{{ $t('settings.personal.email') }}</span>
          <input v-model="email" type="email" inputmode="email" maxlength="255" :placeholder="$t('settings.personal.email_placeholder')" data-testid="profile-email" />
        </label>
        <div class="p-field p-readonly">
          <span>{{ $t('settings.personal.login') }}</span>
          <div>{{ user?.username || '—' }}</div>
        </div>
      </div>
      <p v-if="emailError" class="p-error">{{ $t('settings.personal.email_invalid') }}</p>

      <div class="p-label">{{ $t('profile.security') }}</div>
      <div class="p-group">
        <router-link to="/profile/password" class="p-row" data-testid="profile-password">
          <i class="p-ic p-ic-lock"><Lock :size="17" /></i>
          <span class="p-row-label">{{ $t('profile.password') }}</span>
          <ChevronRight :size="18" class="p-chev" />
        </router-link>
        <router-link v-if="user?.source?.type" to="/profile/key" class="p-row" data-testid="profile-key">
          <i class="p-ic p-ic-key"><KeyRound :size="17" /></i>
          <span class="p-row-label">{{ $t('source_token.title') }}</span>
          <span class="p-row-value" :class="{ ok: linked }">{{ $t(linked ? 'profile.key_linked' : 'profile.key_missing') }}</span>
          <ChevronRight :size="18" class="p-chev" />
        </router-link>
      </div>

      <div class="p-group p-group-gap">
        <button class="p-row p-danger" data-testid="profile-logout" @click="logout">
          <LogOut :size="18" /> {{ $t('auth.logout') }}
        </button>
      </div>

      <!-- «Сохранить» появляется, только когда имя или адрес изменены -->
      <Transition name="p-save-fade">
        <button v-if="dirty" class="p-save p-save-fixed" :disabled="saving || !name.trim()" data-testid="profile-save" @click="save">
          {{ $t('common.save') }}
        </button>
      </Transition>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { Camera, Lock, KeyRound, ChevronRight, LogOut } from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth'
import { useSwal } from '../composables/useSwal'
import { formatDateTime } from '../utils/format'
import SourceTokenForm from '../components/settings/SourceTokenForm.vue'

const auth = useAuthStore()
const { user } = storeToRefs(auth)
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { toast } = useSwal()

const section = computed(() => String(route.params.section || ''))

const initials = computed(() => {
  const n = user.value?.display_name || user.value?.username || '?'
  return n.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)
})

// Связь с Redmine: ключ указан и пользователь найден среди сотрудников
const linked = computed(() => !!user.value?.source?.linked)

// Имя и адрес правятся прямо в списке
const name = ref('')
const email = ref('')
const saving = ref(false)
const emailError = ref(false)

watch(user, u => {
  name.value = u?.display_name || ''
  email.value = u?.email || ''
}, { immediate: true })

const dirty = computed(() => name.value !== (user.value?.display_name || '') || email.value.trim() !== (user.value?.email || ''))

async function save() {
  saving.value = true
  emailError.value = false
  try {
    await auth.updateMe(name.value.trim(), email.value.trim())
    toast(t('profile.saved'), 'success')
  } catch {
    emailError.value = true
  } finally {
    saving.value = false
  }
}

// Фото
const fileInput = ref<HTMLInputElement>()

async function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const form = new FormData()
  form.append('avatar', file)
  try {
    const { data } = await axios.post('/api/auth/avatar', form, { headers: { 'Content-Type': 'multipart/form-data' } })
    if (user.value) user.value.avatar = data.avatar
  } catch {
    toast(t('profile.avatar_failed'), 'error')
  }
  input.value = ''
}

async function removeAvatar() {
  try {
    await axios.delete('/api/auth/avatar')
    if (user.value) user.value.avatar = ''
  } catch {
    toast(t('profile.avatar_failed'), 'error')
  }
}

// Пароль
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')
const changingPassword = ref(false)

async function submitPassword() {
  passwordError.value = ''
  if (newPassword.value.length < 6) {
    passwordError.value = t('auth.password_change.too_short')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = t('auth.password_change.mismatch')
    return
  }
  changingPassword.value = true
  try {
    await auth.changePassword(currentPassword.value, newPassword.value)
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    toast(t('auth.password_change.changed'), 'success')
    router.replace('/profile')
  } catch {
    passwordError.value = t('auth.password_change.change_error_current')
  } finally {
    changingPassword.value = false
  }
}

async function logout() {
  await auth.logout()
  // Полная загрузка: данные вышедшего пользователя не остаются в памяти вкладки
  window.location.assign('/login')
}
</script>

<style scoped>
.profile-view {
  max-width: 560px;
  margin: 0 auto;
}

.p-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 6px 0 4px;
  text-align: center;
}

.p-hero b {
  margin-top: 8px;
  font-size: 20px;
  color: var(--text-bright);
}

.p-sub {
  font-size: 13px;
  color: var(--text-faint);
}

.p-avatar {
  position: relative;
  width: 92px;
  height: 92px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--accent), var(--accent-hover));
  color: #fff;
  font-size: 30px;
  font-weight: 600;
  cursor: pointer;
}

.p-avatar img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

.p-cam {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border: 3px solid var(--bg);
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
}

.p-link {
  min-height: 36px;
  padding: 0;
  margin-top: 2px;
  border: 0;
  background: none;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
}

.p-label {
  margin: 20px 6px 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.p-group {
  overflow: hidden;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.p-group-pad {
  padding: 14px;
}

.p-group-gap {
  margin-top: 22px;
}

.p-field {
  display: block;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-light);
}

.p-field:last-child {
  border-bottom: 0;
}

.p-field > span {
  display: block;
  margin-bottom: 2px;
  font-size: 12px;
  color: var(--text-faint);
}

.p-field input {
  width: 100%;
  padding: 2px 0;
  border: 0;
  background: none;
  color: var(--text-bright);
  /* 16px — иначе iOS увеличивает страницу при фокусе */
  font-size: 16px;
  outline: none;
}

.p-readonly div {
  font-size: 16px;
  color: var(--text-muted);
}

.p-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 52px;
  padding: 10px 14px;
  border: 0;
  border-bottom: 1px solid var(--border-light);
  background: none;
  color: var(--text);
  font-size: 15px;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.p-row:last-child {
  border-bottom: 0;
}

.p-row:active {
  background: var(--bg-hover);
}

.p-ic {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 8px;
  color: #fff;
}

.p-ic-lock {
  background: var(--hairline-strong);
}

.p-ic-key {
  background: var(--success);
}

.p-row-label {
  flex: 1;
}

.p-row-value {
  font-size: 13px;
  color: var(--text-faint);
}

.p-row-value.ok {
  color: var(--success);
}

.p-chev {
  flex: none;
  color: var(--text-faint);
}

.p-danger {
  justify-content: center;
  color: var(--danger);
}

.p-note,
.p-error {
  margin: 8px 6px 0;
  font-size: 12px;
  color: var(--text-faint);
}

.p-note-top {
  margin: 0 6px 10px;
}

.p-error {
  color: var(--danger);
}

.p-save {
  width: 100%;
  margin-top: 18px;
  padding: 14px;
  border: 0;
  border-radius: 12px;
  background: var(--accent);
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
}

.p-save:disabled {
  opacity: 0.5;
  cursor: default;
}

/* На экране профиля кнопка висит внизу, над краем экрана */
.p-save-fixed {
  position: fixed;
  left: 16px;
  right: 16px;
  bottom: calc(16px + env(safe-area-inset-bottom));
  z-index: var(--z-dropdown);
  width: auto;
  max-width: 528px;
  margin: 0 auto;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
}

.p-save-fade-enter-active,
.p-save-fade-leave-active {
  transition: opacity 0.2s, transform 0.2s;
}

.p-save-fade-enter-from,
.p-save-fade-leave-to {
  opacity: 0;
  transform: translateY(12px);
}
</style>

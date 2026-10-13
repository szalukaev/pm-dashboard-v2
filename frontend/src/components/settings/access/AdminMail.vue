<template>
  <div class="adm-section">
    <h4 class="adm-title">{{ $t('mail.title') }} <AppHint help :text="$t('hints.mail')" /></h4>
    <p class="adm-hint">{{ $t('mail.hint') }}</p>

    <!-- Кнопки внутри формы сами её отправляют: сохранение идёт только по их клику -->
    <form class="mail-form" data-testid="smtp-form" @submit.prevent>
      <label class="mail-check">
        <input v-model="form.enabled" type="checkbox" data-testid="smtp-enabled" />
        {{ $t('mail.enabled') }}
      </label>

      <div class="mail-row">
        <label class="mail-field wide">
          <span>{{ $t('mail.host') }}</span>
          <input v-model="form.host" type="text" class="adm-input" placeholder="smtp.example.com" data-testid="smtp-host" />
        </label>
        <!-- Порт и шифрование делят вторую колонку: её левый край совпадает
             с полями «Пароль» и «Имя отправителя» под ней -->
        <div class="mail-pair">
          <label class="mail-field">
            <span>{{ $t('mail.port') }}</span>
            <input v-model="form.port" type="text" inputmode="numeric" class="adm-input" data-testid="smtp-port" />
          </label>
          <label class="mail-field">
            <span>{{ $t('mail.security') }}</span>
            <select v-model="form.security" class="adm-input" data-testid="smtp-security" @change="onSecurityChange">
              <option value="starttls">{{ $t('mail.security_starttls') }}</option>
              <option value="tls">{{ $t('mail.security_tls') }}</option>
              <option value="none">{{ $t('mail.security_none') }}</option>
            </select>
          </label>
        </div>
      </div>

      <div class="mail-row">
        <label class="mail-field wide">
          <span>{{ $t('mail.username') }}</span>
          <input v-model="form.username" type="text" class="adm-input" autocomplete="off" data-testid="smtp-username" />
        </label>
        <label class="mail-field wide">
          <span>{{ $t('mail.password') }}</span>
          <!-- Сохранённый пароль не показывается; пустое поле его не меняет -->
          <input
            v-model="password"
            type="password"
            class="adm-input"
            autocomplete="new-password"
            :placeholder="hasPassword ? $t('mail.password_saved') : ''"
            data-testid="smtp-password"
          />
        </label>
      </div>

      <div class="mail-row">
        <label class="mail-field wide">
          <span>{{ $t('mail.from') }}</span>
          <input v-model="form.from" type="email" class="adm-input" placeholder="pm@example.com" data-testid="smtp-from" />
        </label>
        <label class="mail-field wide">
          <span>{{ $t('mail.from_name') }}</span>
          <input v-model="form.from_name" type="text" class="adm-input" placeholder="PM Dashboard" data-testid="smtp-from-name" />
        </label>
      </div>

      <!-- С адресом дашборда номер и название задачи в письме становятся ссылкой -->
      <label class="mail-field">
        <span>{{ $t('mail.base_url') }}</span>
        <input v-model="form.base_url" type="url" class="adm-input" :placeholder="origin" data-testid="smtp-base-url" />
        <span class="mail-note">{{ $t('mail.base_url_hint') }}</span>
      </label>

      <div class="mail-actions">
        <AppButton variant="primary" :loading="saving" test-id="smtp-save" @click="save">{{ $t('common.save') }}</AppButton>
        <AppButton v-if="hasPassword" variant="ghost" test-id="smtp-clear-password" @click="clearPassword">
          {{ $t('mail.clear_password') }}
        </AppButton>
      </div>
      <AppSavingOverlay cover :active="saving" />
    </form>

    <!-- Проверка идёт по сохранённым настройкам -->
    <h4 class="adm-title test-title">{{ $t('mail.test_title') }}</h4>
    <p class="adm-hint">{{ $t('mail.test_hint') }}</p>
    <div class="mail-row">
      <label class="mail-field wide">
        <span>{{ $t('mail.test_to') }}</span>
        <input v-model="testTo" type="email" class="adm-input" data-testid="smtp-test-to" @keyup.enter="sendTest" />
      </label>
      <div class="mail-test-btn">
        <AppButton variant="ghost" :loading="testing" :disabled="!testTo.trim()" test-id="smtp-test" @click="sendTest">
          {{ $t('mail.test_send') }}
        </AppButton>
      </div>
    </div>
    <div v-if="testError" class="mail-error" data-testid="smtp-test-error">{{ testError }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useSwal } from '../../../composables/useSwal'
import { useAuthStore } from '../../../stores/auth'
import AppButton from '../../ui/AppButton.vue'
import AppSavingOverlay from '../../ui/AppSavingOverlay.vue'
import AppHint from '../../ui/AppHint.vue'

const { t, te } = useI18n()
const { toast } = useSwal()
const auth = useAuthStore()

const form = reactive({
  enabled: false,
  host: '',
  port: '587',
  security: 'starttls',
  username: '',
  from: '',
  from_name: '',
  base_url: '',
})
// Адрес, по которому дашборд открыт сейчас: его обычно и надо указать
const origin = window.location.origin
// Новый пароль; пустое поле оставляет сохранённый
const password = ref('')
const hasPassword = ref(false)
const saving = ref(false)

// Пробное письмо по умолчанию уходит самому администратору
const testTo = ref(auth.user?.email || '')
const testing = ref(false)
const testError = ref('')

// Обычные порты для каждого вида шифрования
const PORTS: Record<string, string> = { starttls: '587', tls: '465', none: '25' }

// Порт меняется вместе с шифрованием, если стоял обычный для прежнего
function onSecurityChange() {
  if (Object.values(PORTS).includes(form.port)) form.port = PORTS[form.security] || form.port
}

async function load() {
  try {
    const { data } = await axios.get('/api/admin/smtp')
    form.enabled = !!data.enabled
    form.host = data.host || ''
    form.port = String(data.port || 587)
    form.security = data.security || 'starttls'
    form.username = data.username || ''
    form.from = data.from || ''
    form.from_name = data.from_name || ''
    // Пока адрес не задан, предлагается тот, по которому дашборд открыт
    form.base_url = data.base_url || origin
    hasPassword.value = !!data.has_password
  } catch {
    toast(t('mail.load_error'), 'error')
  }
}

function errorMessage(e: any): string {
  const key = 'mail.errors.' + e?.response?.data?.error
  return te(key) ? t(key) : t('mail.save_error')
}

async function put(extra: Record<string, unknown>) {
  saving.value = true
  try {
    await axios.put('/api/admin/smtp', { ...form, port: Number(form.port) || 0, ...extra })
    password.value = ''
    toast(t('mail.saved'), 'success')
    await load()
  } catch (e) {
    toast(errorMessage(e), 'error')
  } finally {
    saving.value = false
  }
}

function save() {
  // Пароль отправляется, только если его ввели
  return put(password.value ? { password: password.value } : {})
}

function clearPassword() {
  return put({ password: '' })
}

async function sendTest() {
  testing.value = true
  testError.value = ''
  try {
    await axios.post('/api/admin/smtp/test', { to: testTo.value.trim() })
    toast(t('mail.test_sent'), 'success')
  } catch (e: any) {
    // Ответ почтового сервера помогает понять, что не так с настройками
    const detail = e?.response?.data?.detail
    testError.value = errorMessage(e) + (detail ? ': ' + detail : '')
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.mail-form {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 760px;
}

.mail-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}

/* Две равные колонки: поля стоят строго друг под другом */
.mail-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  max-width: 760px;
}

.mail-pair {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: 12px;
}

.mail-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  color: var(--text-muted);
}

/* Общий стиль ограничивает поле 200 px; здесь поле занимает всю свою колонку */
.mail-field .adm-input {
  width: 100%;
  max-width: none;
  box-sizing: border-box;
}

.mail-note {
  line-height: 1.4;
}

@media (max-width: 640px) {
  .mail-row {
    grid-template-columns: minmax(0, 1fr);
  }
}

.mail-actions {
  display: flex;
  gap: 8px;
}

.test-title {
  margin-top: 28px;
}

.mail-test-btn {
  display: flex;
  align-items: flex-end;
}

.mail-error {
  margin-top: 10px;
  max-width: 760px;
  padding: 10px 14px;
  border: 1px solid var(--danger);
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
  word-break: break-word;
}
</style>

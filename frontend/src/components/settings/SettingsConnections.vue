<template>
  <div class="settings-connections">
    <h3 class="section-title">{{ $t('connections.title') }}</h3>
    <p class="section-hint">{{ $t('connections.hint') }}</p>

    <!-- База данных: те же поля, что на первом шаге мастера настройки -->
    <div class="connection-block" data-testid="connections-db">
      <h4 class="block-title">{{ $t('connections.db.title') }}</h4>
      <p class="block-hint">{{ $t('connections.db.hint') }}</p>
      <div class="form-grid">
        <div class="form-field wide">
          <label>{{ $t('setup.db_host') }}</label>
          <input v-model="db.host" type="text" placeholder="localhost" :disabled="busy" data-testid="db-host" @input="dbChanged" />
        </div>
        <div class="form-field">
          <label>{{ $t('setup.db_port') }}</label>
          <input v-model="db.port" type="text" inputmode="numeric" placeholder="5432" :disabled="busy" data-testid="db-port" @input="dbChanged" />
        </div>
        <div class="form-field wide">
          <label>{{ $t('setup.db_name') }}</label>
          <input v-model="db.dbname" type="text" :disabled="busy" data-testid="db-name" @input="dbChanged" />
        </div>
        <div class="form-field wide">
          <label>{{ $t('setup.db_user') }}</label>
          <input v-model="db.user" type="text" autocomplete="off" :disabled="busy" data-testid="db-user" @input="dbChanged" />
        </div>
        <div class="form-field wide">
          <label>{{ $t('setup.db_password') }}</label>
          <input v-model="db.password" type="password" autocomplete="new-password" :disabled="busy" data-testid="db-password" @input="dbChanged" />
        </div>
      </div>
      <div class="status-message" :class="dbStatus" data-testid="db-status">
        <span v-if="dbMessage">{{ dbMessage }}</span>
      </div>
      <div class="form-actions">
        <AppButton variant="ghost" :loading="testingDb" :disabled="busy" test-id="db-test" @click="testDb">
          {{ $t('setup.test_connection') }}
        </AppButton>
        <AppButton variant="primary" :loading="savingDb" :disabled="!dbOk || busy" test-id="db-save" @click="saveDb">
          {{ $t('common.save') }}
        </AppButton>
      </div>

      <!-- Сервер перезапускается: форма закрыта, страницу покидать нельзя -->
      <div v-if="restarting" class="restart-overlay" data-testid="db-restarting">
        <AppSpinner :size="28" />
        <p class="restart-title">{{ $t('connections.restart.title') }}</p>
        <div class="restart-bar"><span></span></div>
        <p class="restart-hint">{{ $t('connections.restart.hint') }}</p>
      </div>
    </div>

    <!-- Система-источник -->
    <div class="connection-block" data-testid="connections-source">
      <h4 class="block-title">{{ $t('connections.source.title') }}</h4>
      <p class="block-hint">{{ $t('connections.source.hint') }}</p>
      <div class="form-field">
        <label>{{ $t('setup.datasource_url') }}</label>
        <input v-model="dsUrl" type="text" placeholder="https://redmine.example.com" :disabled="busy" data-testid="ds-url" @input="dsChanged" />
      </div>
      <div class="form-field">
        <label>{{ $t('setup.datasource_api_key') }}</label>
        <input v-model="dsApiKey" type="password" autocomplete="new-password" :disabled="busy" data-testid="ds-key" @input="dsChanged" />
      </div>
      <div class="form-field checkbox-field">
        <label class="checkbox-label">
          <input v-model="dsUseBasic" type="checkbox" :disabled="busy" @change="dsChanged" />
          {{ $t('setup.use_basic_auth') }}
        </label>
      </div>
      <template v-if="dsUseBasic">
        <div class="form-field">
          <label>{{ $t('setup.basic_login') }}</label>
          <input v-model="dsBasicLogin" type="text" autocomplete="off" :disabled="busy" @input="dsChanged" />
        </div>
        <div class="form-field">
          <label>{{ $t('setup.basic_password') }}</label>
          <input v-model="dsBasicPass" type="password" autocomplete="new-password" :disabled="busy" @input="dsChanged" />
        </div>
      </template>
      <div class="status-message" :class="dsStatus" data-testid="ds-status">
        <span v-if="dsMessage">{{ dsMessage }}</span>
      </div>
      <div class="form-actions">
        <AppButton variant="ghost" :loading="testingDs" :disabled="busy" test-id="ds-test" @click="testDs">
          {{ $t('setup.test_connection') }}
        </AppButton>
        <AppButton variant="primary" :loading="savingDs" :disabled="!dsOk || busy" test-id="ds-save" @click="saveDs">
          {{ $t('common.save') }}
        </AppButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import AppButton from '../ui/AppButton.vue'
import AppSpinner from '../ui/AppSpinner.vue'
import { useSwal } from '../../composables/useSwal'

const { t, te } = useI18n()
const { confirm, toast } = useSwal()

// Сколько ждать сервер после перезапуска и как часто его спрашивать
const RESTART_FIRST_CHECK_MS = 4000
const RESTART_POLL_MS = 2000
const RESTART_TIMEOUT_MS = 120000

// ─── База данных ───

const db = reactive({ host: '', port: '5432', dbname: '', user: '', password: '' })
const testingDb = ref(false)
const savingDb = ref(false)
const restarting = ref(false)
// Сохранять можно только то, что проверено: любая правка снимает отметку
const dbOk = ref(false)
const dbMessage = ref('')
const dbStatus = ref('')

// Пока идёт запрос или перезапуск, обе формы закрыты
const busy = computed(() => testingDb.value || savingDb.value || restarting.value || testingDs.value || savingDs.value)

function dbChanged() {
  dbOk.value = false
  dbMessage.value = ''
}

// Текст ошибки по её коду; техническое описание дописывается в конце
function dbError(code?: string, detail?: string): string {
  const key = `connections.errors.${code}`
  const text = code && te(key) ? t(key) : t('setup.connection_failed')
  return detail ? `${text} ${detail}` : text
}

async function loadDb() {
  try {
    const { data } = await axios.get('/api/admin/db-config')
    Object.assign(db, { host: '', port: '5432', dbname: '', user: '', password: '' }, data.config || {})
  } catch {
    dbMessage.value = t('connections.load_error')
    dbStatus.value = 'error'
  }
}

async function testDb() {
  testingDb.value = true
  try {
    const { data } = await axios.post('/api/admin/db-config/test', { ...db })
    dbOk.value = !!data.success
    dbMessage.value = data.success ? t('setup.connection_ok') : dbError(data.error, data.detail)
    dbStatus.value = data.success ? 'success' : 'error'
  } catch (e: any) {
    dbOk.value = false
    dbMessage.value = dbError(e?.response?.data?.error)
    dbStatus.value = 'error'
  } finally {
    testingDb.value = false
  }
}

async function saveDb() {
  const answer = await confirm(t('connections.db.confirm_title'), t('connections.db.confirm_text'))
  if (!answer.isConfirmed) return
  savingDb.value = true
  try {
    const { data } = await axios.put('/api/admin/db-config', { ...db })
    if (data.restart) {
      await waitForRestart()
    } else {
      dbMessage.value = t('connections.db.unchanged')
      dbStatus.value = 'success'
    }
  } catch (e: any) {
    dbOk.value = false
    dbMessage.value = dbError(e?.response?.data?.error, e?.response?.data?.detail)
    dbStatus.value = 'error'
  } finally {
    savingDb.value = false
  }
}

function sleep(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

// Не даёт закрыть или обновить страницу, пока сервер перезапускается
function warnBeforeLeave(e: BeforeUnloadEvent) {
  e.preventDefault()
  e.returnValue = ''
}

// Сервер завершается через секунду после ответа и поднимается заново:
// спрашиваем его, пока не ответит
async function waitForRestart() {
  restarting.value = true
  window.addEventListener('beforeunload', warnBeforeLeave)
  try {
    await sleep(RESTART_FIRST_CHECK_MS)
    const deadline = Date.now() + RESTART_TIMEOUT_MS
    let up = false
    while (!up && Date.now() < deadline) {
      try {
        const { data } = await axios.get('/api/status', { timeout: RESTART_POLL_MS })
        up = data?.status === 'ok'
      } catch {}
      if (!up) await sleep(RESTART_POLL_MS)
    }
    if (up) {
      await loadDb()
      dbOk.value = false
      dbMessage.value = t('connections.restart.done')
      dbStatus.value = 'success'
      toast(t('connections.restart.done'), 'success')
    } else {
      dbMessage.value = t('connections.restart.timeout')
      dbStatus.value = 'error'
    }
  } finally {
    window.removeEventListener('beforeunload', warnBeforeLeave)
    restarting.value = false
  }
}

onBeforeUnmount(() => window.removeEventListener('beforeunload', warnBeforeLeave))

// ─── Система-источник ───

const dsUrl = ref('')
const dsApiKey = ref('')
const dsUseBasic = ref(false)
const dsBasicLogin = ref('')
const dsBasicPass = ref('')
const testingDs = ref(false)
const savingDs = ref(false)
const dsOk = ref(false)
const dsMessage = ref('')
const dsStatus = ref('')

function dsChanged() {
  dsOk.value = false
  dsMessage.value = ''
}

function dsPayload() {
  const p: Record<string, string> = { type: 'redmine', url: dsUrl.value.trim(), api_key: dsApiKey.value }
  if (dsUseBasic.value) {
    p.basic_login = dsBasicLogin.value
    p.basic_password = dsBasicPass.value
  }
  return p
}

async function testDs() {
  testingDs.value = true
  try {
    const { data } = await axios.post('/api/admin/datasource-config/test', dsPayload())
    dsOk.value = !!data.success
    dsMessage.value = data.success ? t('setup.connection_ok') : `${t('setup.connection_failed')}. ${data.error || ''}`
    dsStatus.value = data.success ? 'success' : 'error'
  } catch {
    dsOk.value = false
    dsMessage.value = t('setup.connection_failed')
    dsStatus.value = 'error'
  } finally {
    testingDs.value = false
  }
}

// Применяется на лету: перезапуск не нужен, синхронизация стартует сразу
async function saveDs() {
  savingDs.value = true
  try {
    await axios.put('/api/admin/datasource-config', dsPayload())
    dsMessage.value = t('connections.source.saved')
    dsStatus.value = 'success'
    toast(t('connections.source.saved'), 'success')
  } catch {
    dsMessage.value = t('connections.source.save_error')
    dsStatus.value = 'error'
  } finally {
    savingDs.value = false
  }
}

async function loadDs() {
  try {
    const { data } = await axios.get('/api/admin/datasource-config')
    if (data.config) {
      dsUrl.value = data.config.url || ''
      dsApiKey.value = data.config.api_key || ''
      dsUseBasic.value = !!data.config.basic_login
      dsBasicLogin.value = data.config.basic_login || ''
      dsBasicPass.value = data.config.basic_password || ''
    }
  } catch {
    dsMessage.value = t('connections.load_error')
    dsStatus.value = 'error'
  }
}

onMounted(() => {
  loadDb()
  loadDs()
})
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.section-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 24px;
}

.connection-block {
  position: relative;
  max-width: 640px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 20px;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 6px;
}

.block-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 16px;
}

/* Широкое поле и узкое рядом: адрес и порт, как в мастере настройки */
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 120px;
  column-gap: 12px;
}

.form-grid .form-field.wide {
  grid-column: span 2;
}

.form-field {
  margin-bottom: 12px;
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

.form-field input[type="text"],
.form-field input[type="password"] {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.form-field input:focus {
  border-color: var(--accent);
}

.form-field input:disabled {
  opacity: 0.6;
}

.status-message {
  min-height: 18px;
  font-size: 12px;
  margin-bottom: 8px;
  word-break: break-word;
}

.status-message.success { color: var(--success); }
.status-message.error { color: var(--danger); }

.form-actions {
  display: flex;
  gap: 8px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  text-transform: none;
  letter-spacing: 0;
}

.checkbox-label input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
}

.restart-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 20px;
  text-align: center;
  background: color-mix(in srgb, var(--surface-2) 92%, transparent);
  border-radius: 12px;
}

.restart-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.restart-hint {
  max-width: 420px;
  font-size: 12px;
  color: var(--text-muted);
}

/* Полоса без процентов: сколько займёт перезапуск, заранее неизвестно */
.restart-bar {
  width: 240px;
  height: 4px;
  overflow: hidden;
  background: var(--hairline);
  border-radius: 9999px;
}

.restart-bar span {
  display: block;
  width: 40%;
  height: 100%;
  background: var(--accent);
  border-radius: 9999px;
  animation: restart-slide 1.4s ease-in-out infinite;
}

@keyframes restart-slide {
  from { transform: translateX(-100%); }
  to { transform: translateX(250%); }
}

@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
  .form-grid .form-field.wide {
    grid-column: auto;
  }
}
</style>

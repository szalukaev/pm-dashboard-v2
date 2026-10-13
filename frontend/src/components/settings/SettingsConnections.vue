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
      <AppSavingOverlay
        cover
        :active="savingDb || restarting"
        :text="restarting ? $t('common.restarting') : undefined"
        :hint="restarting ? $t('connections.restart.hint') : undefined"
        :progress="restarting"
        guard
      />
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
      <AppSavingOverlay cover :active="savingDs" :text="$t('common.restarting')" progress guard />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import AppButton from '../ui/AppButton.vue'
import AppSavingOverlay from '../ui/AppSavingOverlay.vue'
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

// Сервер завершается через секунду после ответа и поднимается заново:
// спрашиваем его, пока не ответит. Уйти со страницы не даёт заслонка.
async function waitForRestart() {
  restarting.value = true
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
      // Если новая база при старте не ответила, сервер вернулся к прежней:
      // после перезапуска в параметрах стоит не то, что сохраняли
      const wanted = { host: db.host.trim(), port: db.port.trim() || '5432', dbname: db.dbname.trim(), user: db.user.trim() }
      await loadDb()
      dbOk.value = false
      const applied = db.host === wanted.host && db.port === wanted.port && db.dbname === wanted.dbname && db.user === wanted.user
      dbMessage.value = t(applied ? 'connections.restart.done' : 'connections.restart.rolled_back')
      dbStatus.value = applied ? 'success' : 'error'
      toast(dbMessage.value, applied ? 'success' : 'error')
    } else {
      dbMessage.value = t('connections.restart.timeout')
      dbStatus.value = 'error'
    }
  } finally {
    restarting.value = false
  }
}

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

.form-field input:focus-visible {
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

@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
  .form-grid .form-field.wide {
    grid-column: auto;
  }
}
</style>

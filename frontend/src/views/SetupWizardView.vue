<template>
  <div class="setup-overlay">
    <div class="setup-card">
      <div class="step-indicator">{{ $t('setup.step_of', { current: step, total: 3 }) }}</div>
      <h2 class="setup-title">{{ stepTitle }}</h2>

      <!-- Step 1: Database -->
      <form v-if="step === 1" @submit.prevent="testDb" class="setup-form">
        <div class="form-row">
          <div class="form-field flex-1">
            <label>{{ $t('setup.db_host') }}</label>
            <input v-model="dbHost" type="text" placeholder="localhost" @input="onDbFormChange" />
          </div>
          <div class="form-field" style="width:100px">
            <label>{{ $t('setup.db_port') }}</label>
            <input v-model="dbPort" type="text" placeholder="5432" @input="onDbFormChange" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-field flex-1">
            <label>{{ $t('setup.db_user') }}</label>
            <input v-model="dbUser" type="text" placeholder="pm_user" @input="onDbFormChange" />
          </div>
          <div class="form-field flex-1">
            <label>{{ $t('setup.db_password') }}</label>
            <input v-model="dbPassword" type="password" @input="onDbFormChange" />
          </div>
        </div>
        <div class="form-field">
          <label>{{ $t('setup.db_name') }}</label>
          <input v-model="dbName" type="text" placeholder="pm_dashboard" @input="onDbFormChange" />
        </div>
        <div class="status-message" :class="dbStatus">
          <span v-if="dbMessage">{{ dbMessage }}</span>
        </div>
        <div class="form-actions">
          <AppButton variant="primary" :loading="testingDb" @click="testDb">
            {{ $t('setup.test_connection') }}
          </AppButton>
          <AppButton variant="primary" :disabled="!dbOk || dbFormDirty" @click="saveDb">
            {{ $t('common.next') }}
          </AppButton>
        </div>
      </form>

      <!-- Step 2: Data Source -->
      <form v-if="step === 2" @submit.prevent="testDs" class="setup-form">
        <div class="form-field">
          <label>{{ $t('setup.datasource_type') }}</label>
          <select v-model="dsType" @input="onDsFormChange">
            <option value="redmine">Redmine</option>
          </select>
        </div>
        <div class="form-field">
          <label>{{ $t('setup.datasource_url') }}</label>
          <input v-model="dsUrl" type="text" placeholder="https://redmine.example.com" @input="onDsFormChange" />
        </div>
        <div class="form-field">
          <label>{{ $t('setup.datasource_api_key') }}</label>
          <input v-model="dsApiKey" type="text" placeholder="API key" @input="onDsFormChange" />
        </div>
        <div class="form-field checkbox-field">
          <label class="checkbox-label">
            <input v-model="dsUseBasic" type="checkbox" @change="onDsFormChange" />
            {{ $t('setup.use_basic_auth') }}
          </label>
        </div>
        <template v-if="dsUseBasic">
          <div class="form-field">
            <label>{{ $t('setup.basic_login') }}</label>
            <input v-model="dsBasicLogin" type="text" @input="onDsFormChange" />
          </div>
          <div class="form-field">
            <label>{{ $t('setup.basic_password') }}</label>
            <input v-model="dsBasicPass" type="password" @input="onDsFormChange" />
          </div>
        </template>
        <div class="status-message" :class="dsStatus">
          <span v-if="dsMessage">{{ dsMessage }}</span>
        </div>
        <div class="form-actions">
          <AppButton variant="ghost" @click="step = 1">{{ $t('common.back') }}</AppButton>
          <AppButton variant="primary" :loading="testingDs" @click="testDs">
            {{ $t('setup.test_connection') }}
          </AppButton>
          <AppButton variant="primary" :disabled="!dsOk || dsFormDirty" @click="saveDs">
            {{ $t('common.next') }}
          </AppButton>
        </div>
      </form>

      <!-- Step 3: Create Admin -->
      <form v-if="step === 3" @submit.prevent="createAdmin" class="setup-form">
        <div class="form-field">
          <label>{{ $t('setup.display_name') }}</label>
          <input v-model="adminName" type="text" />
        </div>
        <div class="form-field">
          <label>{{ $t('auth.username') }}</label>
          <input v-model="adminUser" type="text" />
        </div>
        <div class="form-field">
          <label>{{ $t('auth.password') }}</label>
          <input v-model="adminPass" type="password" />
        </div>
        <div class="form-field">
          <label>{{ $t('setup.confirm_password') }}</label>
          <input v-model="adminPassConfirm" type="password" />
        </div>
        <div class="form-field">
          <label>{{ $t('setup.default_language') }}</label>
          <select v-model="adminLang">
            <option value="ru">Русский</option>
            <option value="en">English</option>
          </select>
        </div>
        <div v-if="adminPassConfirm && adminPass !== adminPassConfirm" class="status-message error">
          {{ $t('setup.passwords_mismatch') }}
        </div>
        <div class="status-message" :class="adminStatus">
          <span v-if="adminMessage">{{ adminMessage }}</span>
        </div>
        <div class="form-actions">
          <AppButton variant="ghost" @click="step = 2">{{ $t('common.back') }}</AppButton>
          <AppButton
            variant="primary"
            :loading="creatingAdmin"
            :disabled="adminUser.length < 3 || adminPass.length < 6 || adminPass !== adminPassConfirm"
            @click="createAdmin"
          >
            {{ $t('setup.finish') }}
          </AppButton>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppButton from '../components/ui/AppButton.vue'

const router = useRouter()
const step = ref(1)

// Step 1: DB
const dbHost = ref('host.docker.internal')
const dbPort = ref('5432')
const dbUser = ref('pm_user')
const dbPassword = ref('')
const dbName = ref('pm_dashboard')
const testingDb = ref(false)
const dbOk = ref(false)
const dbMessage = ref('')
const dbStatus = ref('')
const dbFormDirty = ref(false)

function onDbFormChange() {
  dbFormDirty.value = true
  dbOk.value = false
}

async function testDb() {
  testingDb.value = true
  dbFormDirty.value = false
  try {
    const { data } = await axios.post('/api/setup/database/test', {
      host: dbHost.value,
      port: dbPort.value,
      user: dbUser.value,
      password: dbPassword.value,
      dbname: dbName.value,
    })
    dbOk.value = data.success
    if (data.success) {
      dbMessage.value = data.db_exists
        ? 'Указанная база данных уже существует. По завершению настроек будет произведена актуализация её структуры.'
        : 'Будет создана новая БД.'
      dbStatus.value = 'success'
    } else {
      dbMessage.value = data.error || 'Не удалось подключиться к БД.'
      dbStatus.value = 'error'
    }
  } catch {
    dbMessage.value = 'Не удалось подключиться к БД. Проверьте реквизиты.'
    dbStatus.value = 'error'
  } finally {
    testingDb.value = false
  }
}

async function saveDb() {
  try {
    await axios.post('/api/setup/database', {
      host: dbHost.value,
      port: dbPort.value,
      user: dbUser.value,
      password: dbPassword.value,
      dbname: dbName.value,
    })
    step.value = 2
  } catch {}
}

// Step 2: Data Source
const dsType = ref('redmine')
const dsUrl = ref('')
const dsApiKey = ref('')
const dsUseBasic = ref(false)
const dsBasicLogin = ref('')
const dsBasicPass = ref('')
const testingDs = ref(false)
const dsOk = ref(false)
const dsMessage = ref('')
const dsStatus = ref('')
const dsFormDirty = ref(false)

function onDsFormChange() {
  dsFormDirty.value = true
  dsOk.value = false
}

async function testDs() {
  testingDs.value = true
  dsFormDirty.value = false
  try {
    const payload: any = {
      type: dsType.value,
      url: dsUrl.value,
      api_key: dsApiKey.value,
    }
    if (dsUseBasic.value) {
      payload.basic_login = dsBasicLogin.value
      payload.basic_password = dsBasicPass.value
    }
    const { data } = await axios.post('/api/setup/datasource/test', payload)
    dsOk.value = data.success
    dsMessage.value = data.success ? 'Соединение установлено' : (data.error || 'Не удалось подключиться')
    dsStatus.value = data.success ? 'success' : 'error'
  } catch {
    dsMessage.value = 'Не удалось установить соединение с внешней системой.'
    dsStatus.value = 'error'
  } finally {
    testingDs.value = false
  }
}

async function saveDs() {
  try {
    const payload: any = {
      type: dsType.value,
      url: dsUrl.value,
      api_key: dsApiKey.value,
    }
    if (dsUseBasic.value) {
      payload.basic_login = dsBasicLogin.value
      payload.basic_password = dsBasicPass.value
    }
    await axios.post('/api/setup/datasource', payload)
    step.value = 3
  } catch {}
}

// Step 3: Admin
const adminName = ref('')
const adminUser = ref('')
const adminPass = ref('')
const adminPassConfirm = ref('')
const adminLang = ref('ru')
const creatingAdmin = ref(false)
const adminMessage = ref('')
const adminStatus = ref('')

async function createAdmin() {
  if (adminPass.value !== adminPassConfirm.value) return
  creatingAdmin.value = true
  try {
    await axios.post('/api/setup/admin', {
      username: adminUser.value,
      display_name: adminName.value || adminUser.value,
      password: adminPass.value,
      language: adminLang.value,
    })
    adminMessage.value = 'Администратор создан. Перенаправление...'
    adminStatus.value = 'success'
    setTimeout(() => router.push('/'), 1500)
  } catch (e: any) {
    adminMessage.value = e.response?.data?.error === 'USER_EXISTS'
      ? 'Пользователь с таким именем уже существует'
      : 'Ошибка создания администратора'
    adminStatus.value = 'error'
  } finally {
    creatingAdmin.value = false
  }
}

const stepTitle = computed(() => {
  switch (step.value) {
    case 1: return 'Подключение к базе данных'
    case 2: return 'Подключение к системе-источнику'
    case 3: return 'Создание администратора'
    default: return ''
  }
})
</script>

<style scoped>
.setup-overlay {
  position: fixed;
  inset: 0;
  background: var(--bg);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-login);
}

.setup-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 40px;
  width: 520px;
  max-width: 90vw;
  box-shadow: var(--shadow);
}

.step-indicator {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-align: center;
  margin-bottom: 8px;
}

.setup-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  text-align: center;
  letter-spacing: -0.4px;
  margin-bottom: 24px;
}

.setup-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 {
  flex: 1;
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

.form-field input,
.form-field select {
  width: 100%;
  padding: 10px 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  font-size: 13px;
}

.form-field input:focus,
.form-field select:focus {
  border-color: var(--accent);
}

.status-message {
  min-height: 20px;
  font-size: 12px;
  text-align: center;
}

.status-message.success {
  color: var(--success);
}

.status-message.error {
  color: var(--danger);
}

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.checkbox-field {
  margin: -4px 0;
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
</style>

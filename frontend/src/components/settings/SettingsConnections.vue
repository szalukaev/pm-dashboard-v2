<template>
  <div class="settings-connections">
    <h3 class="section-title">Подключения</h3>
    <p class="section-hint">Параметры подключения к базе данных и системе-источнику. Доступно только администратору.</p>

    <!-- Database -->
    <div class="connection-block">
      <h4 class="block-title">База данных (PostgreSQL)</h4>
      <div class="form-field">
        <label>Строка подключения (DSN)</label>
        <input v-model="dbDsn" type="text" placeholder="postgres://user:pass@host:5432/dbname" @input="dbDirty = true" />
      </div>
      <div class="status-message" :class="dbStatus">
        <span v-if="dbMessage">{{ dbMessage }}</span>
      </div>
      <div class="form-actions">
        <AppButton variant="ghost" :loading="testingDb" @click="testDb">
          Проверить доступ
        </AppButton>
        <AppButton variant="primary" :disabled="!dbOk || dbDirty" @click="saveDb">
          Сохранить
        </AppButton>
      </div>
      <p class="warning-hint" v-if="dbSaveSuccess">
        Изменение параметров БД потребует перезапуска системы.
      </p>
    </div>

    <!-- Data Source -->
    <div class="connection-block">
      <h4 class="block-title">Система-источник (Redmine)</h4>
      <div class="form-field">
        <label>URL</label>
        <input v-model="dsUrl" type="text" placeholder="https://redmine.example.com" @input="dsDirty = true" />
      </div>
      <div class="form-field">
        <label>API-токен</label>
        <input v-model="dsApiKey" type="text" placeholder="API key" @input="dsDirty = true" />
      </div>
      <div class="status-message" :class="dsStatus">
        <span v-if="dsMessage">{{ dsMessage }}</span>
      </div>
      <div class="form-actions">
        <AppButton variant="ghost" :loading="testingDs" @click="testDs">
          Проверить доступ
        </AppButton>
        <AppButton variant="primary" :disabled="!dsOk || dsDirty" @click="saveDs">
          Сохранить
        </AppButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import AppButton from '../ui/AppButton.vue'

// DB
const dbDsn = ref('')
const testingDb = ref(false)
const dbOk = ref(false)
const dbDirty = ref(false)
const dbMessage = ref('')
const dbStatus = ref('')
const dbSaveSuccess = ref(false)

async function testDb() {
  testingDb.value = true
  dbDirty.value = false
  try {
    const { data } = await axios.post('/api/setup/database/test', { dsn: dbDsn.value })
    dbOk.value = data.success
    dbMessage.value = data.success ? 'Соединение установлено' : data.error
    dbStatus.value = data.success ? 'success' : 'error'
  } catch {
    dbMessage.value = 'Ошибка подключения'
    dbStatus.value = 'error'
  } finally { testingDb.value = false }
}

async function saveDb() {
  dbSaveSuccess.value = true
}

// Data Source
const dsUrl = ref('')
const dsApiKey = ref('')
const testingDs = ref(false)
const dsOk = ref(false)
const dsDirty = ref(false)
const dsMessage = ref('')
const dsStatus = ref('')

async function testDs() {
  testingDs.value = true
  dsDirty.value = false
  try {
    const { data } = await axios.post('/api/setup/datasource/test', {
      type: 'redmine', url: dsUrl.value, api_key: dsApiKey.value,
    })
    dsOk.value = data.success
    dsMessage.value = data.success ? 'Соединение установлено' : data.error
    dsStatus.value = data.success ? 'success' : 'error'
  } catch {
    dsMessage.value = 'Ошибка подключения'
    dsStatus.value = 'error'
  } finally { testingDs.value = false }
}

async function saveDs() {
  try {
    await axios.put('/api/admin/datasource-config', {
      url: dsUrl.value, api_key: dsApiKey.value, type: 'redmine',
    })
    dsMessage.value = 'Сохранено. Изменения применятся при следующей синхронизации.'
    dsStatus.value = 'success'
  } catch {}
}

onMounted(async () => {
  try {
    const { data } = await axios.get('/api/admin/datasource-config')
    if (data.config) {
      dsUrl.value = data.config.url || ''
      dsApiKey.value = data.config.api_key || ''
    }
  } catch {}
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
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 10px;
  padding: 20px;
  margin-bottom: 20px;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 16px;
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

.form-field input {
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

.status-message {
  min-height: 18px;
  font-size: 12px;
  margin-bottom: 8px;
}

.status-message.success { color: var(--success); }
.status-message.error { color: var(--danger); }

.form-actions {
  display: flex;
  gap: 8px;
}

.warning-hint {
  font-size: 12px;
  color: var(--warning);
  margin-top: 8px;
}
</style>

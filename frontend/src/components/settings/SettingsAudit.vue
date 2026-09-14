<template>
  <div class="settings-audit">
    <h3 class="section-title">Журнал аудита</h3>
    <p class="section-hint">История действий пользователей в системе. Доступно только администратору.</p>

    <!-- Filters -->
    <div class="audit-filters">
      <select v-model="filterAction" @change="loadLogs" class="filter-select">
        <option value="">Все действия</option>
        <option value="created">Создание</option>
        <option value="updated">Изменение</option>
        <option value="deleted">Удаление</option>
        <option value="login">Вход</option>
        <option value="login_failed">Неудачный вход</option>
        <option value="logout">Выход</option>
      </select>
      <select v-model="filterEntity" @change="loadLogs" class="filter-select">
        <option value="">Все сущности</option>
        <option value="organization">Организации</option>
        <option value="contract">Договоры</option>
        <option value="sprint">Спринты</option>
        <option value="task">Задачи</option>
        <option value="auth">Авторизация</option>
      </select>
    </div>

    <table class="audit-table">
      <thead>
        <tr>
          <th>Дата/время</th>
          <th>Пользователь</th>
          <th>Действие</th>
          <th>Сущность</th>
          <th>ID</th>
          <th>IP</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in logs" :key="log.id">
          <td>{{ formatDateTime(log.occurred_at) }}</td>
          <td>{{ log.username || '—' }}</td>
          <td>
            <span class="action-badge" :class="log.action">{{ actionLabel(log.action) }}</span>
          </td>
          <td>{{ log.entity_type || '—' }}</td>
          <td>{{ log.entity_id || '—' }}</td>
          <td class="ip-cell">{{ log.ip_address || '—' }}</td>
          <td>
            <button
              v-if="log.before_state || log.after_state"
              class="detail-btn"
              @click="showDetail(log)"
              title="Детали"
            >
              <Eye :size="14" />
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="logs.length === 0" class="empty-hint">
      Нет записей
    </div>

    <!-- Detail Modal -->
    <AppModal
      :model-value="showDetailModal"
      title="Детали события"
      width="600px"
      @update:model-value="showDetailModal = $event"
    >
      <div class="detail-content" v-if="selectedLog">
        <div class="detail-section" v-if="selectedLog.before_state">
          <h4>До</h4>
          <pre class="json-block">{{ formatJSON(selectedLog.before_state) }}</pre>
        </div>
        <div class="detail-section" v-if="selectedLog.after_state">
          <h4>После</h4>
          <pre class="json-block">{{ formatJSON(selectedLog.after_state) }}</pre>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { Eye } from 'lucide-vue-next'
import AppModal from '../ui/AppModal.vue'

interface AuditEntry {
  id: number
  occurred_at: string
  user_id: number | null
  username: string | null
  action: string
  entity_type: string | null
  entity_id: string | null
  before_state: string | null
  after_state: string | null
  ip_address: string | null
}

const logs = ref<AuditEntry[]>([])
const filterAction = ref('')
const filterEntity = ref('')
const showDetailModal = ref(false)
const selectedLog = ref<AuditEntry | null>(null)

async function loadLogs() {
  try {
    const params: Record<string, string> = {}
    if (filterAction.value) params.action = filterAction.value
    if (filterEntity.value) params.entity = filterEntity.value
    const { data } = await axios.get('/api/admin/audit-log', { params })
    logs.value = data.logs || []
  } catch {}
}

function actionLabel(action: string): string {
  const map: Record<string, string> = {
    created: 'Создание',
    updated: 'Изменение',
    deleted: 'Удаление',
    login: 'Вход',
    login_failed: 'Ошибка входа',
    logout: 'Выход',
  }
  return map[action] || action
}

function showDetail(log: AuditEntry) {
  selectedLog.value = log
  showDetailModal.value = true
}

function formatDateTime(s: string): string {
  return new Date(s).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function formatJSON(s: string | null): string {
  if (!s) return ''
  try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

onMounted(loadLogs)
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
  margin-bottom: 16px;
}

.audit-filters {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.filter-select {
  padding: 6px 10px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 6px;
  color: var(--text);
}

.audit-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  overflow: hidden;
}

.audit-table th {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
}

.audit-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
}

.audit-table tr:hover td {
  background: var(--bg-hover);
}

.action-badge {
  display: inline-block;
  padding: 1px 6px;
  font-size: 10px;
  font-weight: 500;
  border-radius: 9999px;
}

.action-badge.created { background: var(--success)22; color: var(--success); }
.action-badge.updated { background: var(--accent-bg); color: var(--accent); }
.action-badge.deleted { background: var(--danger)22; color: var(--danger); }
.action-badge.login { background: var(--teal)22; color: var(--teal); }
.action-badge.login_failed { background: var(--warning)22; color: var(--warning); }
.action-badge.logout { background: var(--tag-bg); color: var(--text-muted); }

.ip-cell {
  font-family: monospace;
  font-size: 11px;
  color: var(--text-faint);
}

.detail-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 4px;
  color: var(--text-muted);
}

.detail-btn:hover {
  background: var(--surface-3);
  color: var(--text-bright);
}

.detail-section {
  margin-bottom: 16px;
}

.detail-section h4 {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.json-block {
  font-family: monospace;
  font-size: 11px;
  background: var(--surface-2);
  padding: 12px;
  border-radius: 6px;
  overflow-x: auto;
  color: var(--text-dim);
  white-space: pre-wrap;
  word-break: break-all;
}

.empty-hint {
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
  padding: 24px;
}
</style>

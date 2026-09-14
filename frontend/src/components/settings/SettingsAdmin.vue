<template>
  <div class="settings-admin">
    <h3 class="section-title">{{ $t('settings.tabs.admin') }}</h3>

    <!-- Users Management -->
    <div class="admin-section">
      <div class="section-header">
        <h4 class="block-title">Пользователи</h4>
        <AppButton variant="primary" @click="showUserForm = true" test-id="add-user-btn">
          + Добавить
        </AppButton>
      </div>

      <table class="admin-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Имя</th>
            <th>Роль</th>
            <th>Создан</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.id }}</td>
            <td>{{ u.username }}</td>
            <td>
              <select
                :value="u.role"
                @change="updateUserRole(u.id, ($event.target as HTMLSelectElement).value)"
                class="inline-select"
              >
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </td>
            <td class="date-cell">{{ formatDate(u.created_at) }}</td>
            <td>
              <button class="action-btn danger-btn" @click="deleteUser(u.id)" title="Удалить">
                <Trash2 :size="14" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Status Mapping -->
    <div class="admin-section">
      <h4 class="block-title">Маппинг статусов</h4>
      <p class="block-hint">Настройте, к какой группе относится каждый статус из системы-источника.</p>

      <table class="admin-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Название</th>
            <th>Группа</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in statuses" :key="s.external_id">
            <td>{{ s.external_id }}</td>
            <td>{{ s.name }}</td>
            <td>
              <select
                :value="s.group"
                @change="updateStatusGroup(s.external_id, ($event.target as HTMLSelectElement).value)"
                class="inline-select"
              >
                <option value="open">open</option>
                <option value="testing">testing</option>
                <option value="closed">closed</option>
              </select>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Priority Order -->
    <div class="admin-section">
      <h4 class="block-title">Порядок приоритетов</h4>

      <table class="admin-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Название</th>
            <th>Порядок</th>
            <th>Цвет</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in priorities" :key="p.external_id">
            <td>{{ p.external_id }}</td>
            <td>{{ p.name }}</td>
            <td>
              <input
                type="number"
                :value="p.sort_order"
                @change="updatePriorityOrder(p.external_id, Number(($event.target as HTMLInputElement).value))"
                class="inline-input"
                min="0"
              />
            </td>
            <td>
              <input
                type="color"
                :value="p.color"
                @change="updatePriorityColor(p.external_id, ($event.target as HTMLInputElement).value)"
                class="color-input"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add User Modal -->
    <AppModal
      :model-value="showUserForm"
      title="Новый пользователь"
      width="400px"
      @update:model-value="showUserForm = $event"
    >
      <form class="modal-form" @submit.prevent="createUser">
        <div class="form-field">
          <label>Имя пользователя</label>
          <input v-model="newUser.username" type="text" required minlength="3" />
        </div>
        <div class="form-field">
          <label>Пароль</label>
          <input v-model="newUser.password" type="password" required minlength="6" />
        </div>
        <div class="form-field">
          <label>Роль</label>
          <select v-model="newUser.role">
            <option value="user">Пользователь</option>
            <option value="admin">Администратор</option>
          </select>
        </div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="showUserForm = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" @click="createUser" :disabled="newUser.username.length < 3 || newUser.password.length < 6">
          {{ $t('common.create') }}
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import axios from 'axios'
import { Trash2 } from 'lucide-vue-next'
import AppButton from '../ui/AppButton.vue'
import AppModal from '../ui/AppModal.vue'
import { formatDate } from '../../utils/format'

const users = ref<any[]>([])
const statuses = ref<any[]>([])
const priorities = ref<any[]>([])
const showUserForm = ref(false)
const newUser = reactive({ username: '', password: '', role: 'user' })

async function loadData() {
  try {
    const [uRes, sRes, pRes] = await Promise.all([
      axios.get('/api/admin/users'),
      axios.get('/api/admin/statuses'),
      axios.get('/api/admin/priorities'),
    ])
    users.value = uRes.data.users || []
    statuses.value = sRes.data.statuses || []
    priorities.value = pRes.data.priorities || []
  } catch {
    // May fail if not admin
  }
}

async function createUser() {
  try {
    await axios.post('/api/admin/users', newUser)
    showUserForm.value = false
    newUser.username = ''
    newUser.password = ''
    newUser.role = 'user'
    await loadData()
  } catch {}
}

async function updateUserRole(id: number, role: string) {
  await axios.put(`/api/admin/users/${id}`, { role })
  await loadData()
}

async function deleteUser(id: number) {
  if (!confirm('Удалить пользователя?')) return
  await axios.delete(`/api/admin/users/${id}`)
  await loadData()
}

async function updateStatusGroup(id: number, group: string) {
  await axios.put(`/api/admin/statuses/${id}`, { group })
  await loadData()
}

async function updatePriorityOrder(id: number, order: number) {
  await axios.put(`/api/admin/priorities/${id}`, { sort_order: order })
  await loadData()
}

async function updatePriorityColor(id: number, color: string) {
  await axios.put(`/api/admin/priorities/${id}`, { color })
  await loadData()
}

onMounted(loadData)
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 20px;
}

.admin-section {
  margin-bottom: 32px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.section-header .block-title {
  margin-bottom: 0;
}

.block-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 12px;
}

.admin-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  overflow: hidden;
}

.admin-table th {
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
}

.admin-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text-dim);
}

.admin-table tr:hover td {
  background: var(--bg-hover);
}

.date-cell {
  font-size: 12px;
  color: var(--text-faint);
}

.inline-select {
  padding: 4px 8px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 6px;
  color: var(--text);
}

.inline-input {
  padding: 4px 8px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 6px;
  color: var(--text);
  width: 60px;
  text-align: center;
}

.color-input {
  width: 32px;
  height: 24px;
  border: 1px solid var(--hairline);
  border-radius: 4px;
  cursor: pointer;
  padding: 0;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  color: var(--text-muted);
  transition: all 0.15s;
}

.action-btn:hover {
  background: var(--surface-3);
  color: var(--text-bright);
}

.danger-btn:hover {
  color: var(--danger);
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
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

.form-field input,
.form-field select {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.form-field input:focus,
.form-field select:focus {
  border-color: var(--accent);
}
</style>

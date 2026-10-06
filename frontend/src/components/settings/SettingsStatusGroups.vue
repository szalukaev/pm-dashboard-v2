<template>
  <div class="status-groups">
    <h3 class="section-title">Статусы по типам</h3>
    <p class="block-hint">Распределите статусы по группам. Это влияет на фильтры вкладки «Задачи». Настройка общая для всех пользователей.</p>

    <!-- Unassigned statuses as tiles -->
    <div class="tiles-area">
      <h4 class="block-title">Все статусы</h4>
      <div class="tiles-grid">
        <div
          v-for="s in allStatuses"
          :key="s.id"
          class="status-tile"
          :class="{ selected: selectedStatus === s.id, [`group-${s.group}`]: true }"
          @click="selectStatus(s.id)"
        >
          {{ s.name }}
        </div>
      </div>
    </div>

    <!-- Three columns -->
    <div class="columns-row">
      <div
        v-for="col in columns"
        :key="col.key"
        class="group-column"
        :class="{ 'drop-active': selectedStatus !== null }"
        @click="assignSelected(col.key)"
      >
        <h4 class="col-title" :class="col.key">{{ col.label }}</h4>
        <div class="col-items">
          <div
            v-for="s in statusesByGroup(col.key)"
            :key="s.id"
            class="status-tile in-column"
            :class="{ selected: selectedStatus === s.id }"
            @click.stop="selectStatus(s.id)"
          >
            {{ s.name }}
          </div>
          <div v-if="statusesByGroup(col.key).length === 0" class="col-empty">Перетащите сюда</div>
        </div>
      </div>
    </div>

    <div v-if="isAdmin" class="hint-text">Кликните на статус, затем на столбец, чтобы переместить.</div>
    <div v-else class="hint-text">Только администратор может изменять распределение статусов.</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { storeToRefs } from 'pinia'
import axios from 'axios'

interface Status {
  id: number
  name: string
  is_closed: boolean
  group: string
}

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
const isAdmin = computed(() => user.value?.role === 'admin')

const allStatuses = ref<Status[]>([])
const selectedStatus = ref<number | null>(null)

const columns = [
  { key: 'open', label: 'Открытые' },
  { key: 'closed', label: 'Закрытые' },
  { key: 'testing', label: 'Тестирование' },
]

function statusesByGroup(group: string): Status[] {
  return allStatuses.value.filter(s => s.group === group)
}

function selectStatus(id: number) {
  if (!isAdmin.value) return
  selectedStatus.value = selectedStatus.value === id ? null : id
}

async function assignSelected(group: string) {
  if (!isAdmin.value || selectedStatus.value === null) return
  const status = allStatuses.value.find(s => s.id === selectedStatus.value)
  if (!status) return
  status.group = group
  status.is_closed = group === 'closed'
  try {
    await axios.put(`/api/admin/statuses/${status.id}`, {
      group: group,
      is_closed: group === 'closed',
    })
  } catch {}
  selectedStatus.value = null
}

onMounted(async () => {
  try {
    const { data } = await axios.get('/api/tasks/statuses')
    allStatuses.value = data.statuses || []
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

.block-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 16px;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-dim);
  margin-bottom: 8px;
}

.tiles-area {
  margin-bottom: 20px;
}

.tiles-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-tile {
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid var(--hairline);
  background: var(--surface-2);
  color: var(--text);
  transition: all 0.15s;
  user-select: none;
}

.status-tile:hover {
  border-color: var(--accent);
}

.status-tile.selected {
  border-color: var(--accent);
  background: var(--accent-bg);
  box-shadow: 0 0 0 2px var(--accent-bg);
}

.status-tile.group-open {
  border-left: 3px solid var(--success);
}

.status-tile.group-closed {
  border-left: 3px solid var(--text-faint);
}

.status-tile.group-testing {
  border-left: 3px solid var(--warning);
}

.columns-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.group-column {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 10px;
  padding: 12px;
  min-height: 120px;
  cursor: pointer;
  transition: border-color 0.15s;
}

.group-column.drop-active {
  border-color: var(--accent);
}

.group-column.drop-active:hover {
  background: var(--accent-bg);
}

.col-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--hairline);
}

.col-title.open {
  color: var(--success);
}

.col-title.closed {
  color: var(--text-muted);
}

.col-title.testing {
  color: var(--warning);
}

.col-items {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.status-tile.in-column {
  font-size: 11px;
  padding: 4px 10px;
}

.col-empty {
  font-size: 11px;
  color: var(--text-faintest);
  padding: 8px 0;
}

.hint-text {
  font-size: 11px;
  color: var(--text-faintest);
  margin-top: 8px;
}
</style>

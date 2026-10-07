<template>
  <div class="settings-data">
    <h3 class="section-title">{{ $t('settings.data.title') }}</h3>

    <!-- Projects: selected ones as chips, the full tree in a dialog -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.data.projects') }}</h4>
      <p class="block-hint">{{ $t('settings.data.projects_hint') }}</p>
      <div class="chips">
        <span v-for="p in selectedProjectItems" :key="p.id" class="chip">
          {{ p.name }} <span class="chip-id">#{{ p.id }}</span>
        </span>
        <span v-if="!selectedProjectItems.length" class="chips-empty">Проекты не выбраны</span>
      </div>
      <AppButton variant="ghost" size="sm" @click="openProjects">
        <Pencil :size="14" /> Изменить
      </AppButton>
    </div>

    <!-- Team -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.data.team') }}</h4>
      <p class="block-hint">Сотрудники, задачи которых показываются в дашборде.</p>
      <div class="chips">
        <span v-for="m in selectedTeamItems" :key="m.id" class="chip">{{ m.name }}</span>
        <span v-if="!selectedTeamItems.length" class="chips-empty">Сотрудники не выбраны — показываются все</span>
      </div>
      <AppButton variant="ghost" size="sm" @click="openTeam">
        <Pencil :size="14" /> Изменить
      </AppButton>
    </div>

    <!-- Projects dialog -->
    <AppModal v-model="projectsOpen" title="Выбор проектов" width="700px">
      <div class="picker-toolbar">
        <input v-model="projectQuery" class="picker-search" placeholder="Поиск проекта…" />
        <AppButton variant="ghost" size="sm" @click="draftProjects = projects.map(p => p.id)">Выбрать все</AppButton>
        <AppButton variant="ghost" size="sm" @click="draftProjects = []">Снять все</AppButton>
        <span class="picker-count">Выбрано: <strong>{{ draftProjects.length }}</strong> из {{ projects.length }}</span>
      </div>
      <div class="picker-list">
        <label
          v-for="item in visibleProjectTree"
          :key="item.id"
          class="picker-item"
          :class="{ selected: draftProjects.includes(item.id) }"
          :style="{ paddingLeft: (projectQuery ? 10 : item.depth * 24 + 10) + 'px' }"
        >
          <input
            type="checkbox"
            :checked="draftProjects.includes(item.id)"
            @change="toggleProject(item.id, ($event.target as HTMLInputElement).checked)"
          />
          <span class="tree-label" :class="{ 'has-children': item.hasChildren }">{{ item.name }}</span>
          <span class="picker-id">#{{ item.id }}</span>
        </label>
        <div v-if="!visibleProjectTree.length" class="chips-empty">Ничего не найдено</div>
      </div>
      <template #footer>
        <AppButton variant="ghost" @click="projectsOpen = false">Отмена</AppButton>
        <AppButton variant="primary" :loading="saving" @click="saveProjects">Сохранить</AppButton>
      </template>
    </AppModal>

    <!-- Team dialog -->
    <AppModal v-model="teamOpen" title="Выбор сотрудников" width="560px">
      <div class="picker-toolbar">
        <input v-model="teamQuery" class="picker-search" placeholder="Поиск сотрудника…" />
        <AppButton variant="ghost" size="sm" @click="draftTeam = members.map(m => m.id)">Выбрать все</AppButton>
        <AppButton variant="ghost" size="sm" @click="draftTeam = []">Снять все</AppButton>
        <span class="picker-count">Выбрано: <strong>{{ draftTeam.length }}</strong> из {{ members.length }}</span>
      </div>
      <div class="picker-list">
        <label
          v-for="m in visibleMembers"
          :key="m.id"
          class="picker-item"
          :class="{ selected: draftTeam.includes(m.id) }"
        >
          <input type="checkbox" :value="m.id" v-model="draftTeam" />
          <span>{{ m.name }}</span>
        </label>
        <div v-if="!visibleMembers.length" class="chips-empty">Ничего не найдено</div>
      </div>
      <template #footer>
        <AppButton variant="ghost" @click="teamOpen = false">Отмена</AppButton>
        <AppButton variant="primary" :loading="saving" @click="saveTeam">Сохранить</AppButton>
      </template>
    </AppModal>

    <AppButton variant="ghost" @click="resetToDefaults">
      {{ $t('settings.data.reset') }}
    </AppButton>

    <!-- Status Groups (admin only) -->
    <div v-if="isAdmin" class="setting-block">
      <SettingsStatusGroups />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { useSettingsStore } from '../../stores/settings'
import { useAuthStore } from '../../stores/auth'
import { Pencil } from 'lucide-vue-next'
import AppButton from '../ui/AppButton.vue'
import AppModal from '../ui/AppModal.vue'
import SettingsStatusGroups from './SettingsStatusGroups.vue'

const settingsStore = useSettingsStore()
const auth = useAuthStore()
const isAdmin = computed(() => auth.user?.role === 'admin')

const projects = ref<{ id: number; name: string; parent_id: number | null }[]>([])
const members = ref<{ id: number; name: string }[]>([])
const selectedProjects = ref<number[]>([])
const selectedTeam = ref<number[]>([])

async function loadData() {
  try {
    const [projRes, taskRes, memRes] = await Promise.all([
      axios.get('/api/tasks/projects'),
      axios.get('/api/settings'),
      axios.get('/api/tasks/members'),
    ])
    projects.value = projRes.data.projects || []
    members.value = memRes.data.members || []
    selectedProjects.value = taskRes.data.selected_projects || []
    selectedTeam.value = taskRes.data.selected_team || []
  } catch {}
}

// Build flat tree with depth for display
const projectTree = computed(() => {
  const byId = new Map<number, any>()
  const children = new Map<number, number[]>()
  for (const p of projects.value) {
    byId.set(p.id, p)
    const pid = p.parent_id || 0
    if (!children.has(pid)) children.set(pid, [])
    children.get(pid)!.push(p.id)
  }
  const result: { id: number; name: string; depth: number; hasChildren: boolean }[] = []
  function walk(pid: number, depth: number) {
    const kids = children.get(pid) || []
    for (const id of kids) {
      const p = byId.get(id)
      if (!p) continue
      const hasChildren = (children.get(id) || []).length > 0
      result.push({ id, name: p.name, depth, hasChildren })
      walk(id, depth + 1)
    }
  }
  walk(0, 0)
  // Also include orphan projects (parent_id points to non-existent project)
  const placed = new Set(result.map(r => r.id))
  for (const p of projects.value) {
    if (!placed.has(p.id)) {
      result.push({ id: p.id, name: p.name, depth: 0, hasChildren: false })
    }
  }
  return result
})

// Get all descendant IDs of a project
function getDescendants(id: number): number[] {
  const children = new Map<number, number[]>()
  for (const p of projects.value) {
    const pid = p.parent_id || 0
    if (!children.has(pid)) children.set(pid, [])
    children.get(pid)!.push(p.id)
  }
  const result: number[] = []
  function walk(pid: number) {
    for (const cid of children.get(pid) || []) {
      result.push(cid)
      walk(cid)
    }
  }
  walk(id)
  return result
}

// Selected values shown as chips, in tree / alphabetical order
const selectedProjectItems = computed(() =>
  projectTree.value.filter(p => selectedProjects.value.includes(p.id))
)
const selectedTeamItems = computed(() =>
  members.value.filter(m => selectedTeam.value.includes(m.id))
)

// Dialogs edit a draft copy; it is applied only on "Сохранить".
const projectsOpen = ref(false)
const teamOpen = ref(false)
const draftProjects = ref<number[]>([])
const draftTeam = ref<number[]>([])
const projectQuery = ref('')
const teamQuery = ref('')
const saving = ref(false)

const visibleProjectTree = computed(() => {
  const q = projectQuery.value.trim().toLowerCase()
  if (!q) return projectTree.value
  return projectTree.value.filter(p => p.name.toLowerCase().includes(q) || String(p.id) === q)
})

const visibleMembers = computed(() => {
  const q = teamQuery.value.trim().toLowerCase()
  return q ? members.value.filter(m => m.name.toLowerCase().includes(q)) : members.value
})

function openProjects() {
  draftProjects.value = [...selectedProjects.value]
  projectQuery.value = ''
  projectsOpen.value = true
}

function openTeam() {
  draftTeam.value = [...selectedTeam.value]
  teamQuery.value = ''
  teamOpen.value = true
}

// Selecting a parent project selects all its subprojects as well.
function toggleProject(id: number, checked: boolean) {
  const descendants = getDescendants(id)
  if (checked) {
    const toAdd = [id, ...descendants].filter(x => !draftProjects.value.includes(x))
    draftProjects.value = [...draftProjects.value, ...toAdd]
  } else {
    const toRemove = new Set([id, ...descendants])
    draftProjects.value = draftProjects.value.filter(x => !toRemove.has(x))
  }
}

async function saveProjects() {
  saving.value = true
  try {
    await settingsStore.updateSettings({ selected_projects: draftProjects.value } as any)
    selectedProjects.value = [...draftProjects.value]
    projectsOpen.value = false
  } finally {
    saving.value = false
  }
}

async function saveTeam() {
  saving.value = true
  try {
    await settingsStore.updateSettings({ selected_team: draftTeam.value } as any)
    selectedTeam.value = [...draftTeam.value]
    teamOpen.value = false
  } finally {
    saving.value = false
  }
}

async function resetToDefaults() {
  selectedProjects.value = []
  selectedTeam.value = []
  await settingsStore.updateSettings({ selected_projects: [], selected_team: [] } as any)
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

.setting-block {
  margin-bottom: 24px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 12px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  font-size: 12px;
  border-radius: 6px;
  background: var(--accent-bg);
  color: var(--accent);
  border: 1px solid var(--border-accent);
}

.chip-id {
  font-size: 10px;
  color: var(--text-faint);
}

.chips-empty {
  font-size: 13px;
  color: var(--text-faint);
}

.picker-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.picker-search {
  flex: 1;
  min-width: 180px;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  outline: none;
}

.picker-search:focus {
  border-color: var(--accent);
}

.picker-count {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-muted);
}

.picker-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 50vh;
  overflow-y: auto;
}

.picker-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  font-size: 13px;
  color: var(--text-dim);
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.1s;
}

.picker-item:hover {
  background: var(--bg-hover);
}

.picker-item.selected {
  color: var(--text-bright);
}

.picker-item input[type="checkbox"] {
  accent-color: var(--accent);
  width: 16px;
  height: 16px;
}

.picker-id {
  margin-left: auto;
  font-size: 11px;
  color: var(--text-faint);
}

.tree-label.has-children {
  font-weight: 600;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.block-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 12px;
}

</style>

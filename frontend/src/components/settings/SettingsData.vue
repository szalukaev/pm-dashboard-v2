<template>
  <div class="settings-data">
    <h3 class="section-title">{{ $t('settings.data.title') }}</h3>

    <!-- Projects tree -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.data.projects') }}</h4>
      <p class="block-hint">{{ $t('settings.data.projects_hint') }}</p>
      <div class="checkbox-list tree-list">
        <template v-for="item in projectTree" :key="item.id">
          <label class="checkbox-item tree-item" :style="{ paddingLeft: (item.depth * 24 + 8) + 'px' }">
            <input
              type="checkbox"
              :checked="selectedProjects.includes(item.id)"
              @change="toggleProject(item.id, ($event.target as HTMLInputElement).checked)"
            />
            <span class="tree-label" :class="{ 'has-children': item.hasChildren }">{{ item.name }}</span>
          </label>
        </template>
      </div>
    </div>

    <!-- Team -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.data.team') }}</h4>
      <div class="checkbox-list">
        <label v-for="m in members" :key="m.id" class="checkbox-item">
          <input
            type="checkbox"
            :value="m.id"
            v-model="selectedTeam"
            @change="saveTeam"
          />
          <span>{{ m.name }}</span>
        </label>
      </div>
    </div>

    <AppButton variant="ghost" @click="resetToDefaults">
      {{ $t('settings.data.reset') }}
    </AppButton>

    <!-- Status Groups (admin only) -->
    <div class="setting-block">
      <SettingsStatusGroups />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { useSettingsStore } from '../../stores/settings'
import AppButton from '../ui/AppButton.vue'
import SettingsStatusGroups from './SettingsStatusGroups.vue'

const settingsStore = useSettingsStore()

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

function toggleProject(id: number, checked: boolean) {
  const descendants = getDescendants(id)
  if (checked) {
    const toAdd = [id, ...descendants].filter(x => !selectedProjects.value.includes(x))
    selectedProjects.value = [...selectedProjects.value, ...toAdd]
  } else {
    const toRemove = new Set([id, ...descendants])
    selectedProjects.value = selectedProjects.value.filter(x => !toRemove.has(x))
  }
  saveProjects()
}

async function saveProjects() {
  await settingsStore.updateSettings({ selected_projects: selectedProjects.value } as any)
}

async function saveTeam() {
  await settingsStore.updateSettings({ selected_team: selectedTeam.value } as any)
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

.tree-list {
  max-height: 400px;
  overflow-y: auto;
}

.tree-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  cursor: pointer;
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

.checkbox-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 300px;
  overflow-y: auto;
  padding: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
}

.checkbox-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-dim);
  cursor: pointer;
  padding: 4px 0;
}

.checkbox-item input[type="checkbox"] {
  accent-color: var(--accent);
  width: 16px;
  height: 16px;
}
</style>

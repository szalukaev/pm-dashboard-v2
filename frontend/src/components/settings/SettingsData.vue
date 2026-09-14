<template>
  <div class="settings-data">
    <h3 class="section-title">{{ $t('settings.data.title') }}</h3>

    <!-- Projects -->
    <div class="setting-block">
      <h4 class="block-title">{{ $t('settings.data.projects') }}</h4>
      <p class="block-hint">{{ $t('settings.data.projects_hint') }}</p>
      <div class="checkbox-list">
        <label v-for="p in projects" :key="p.id" class="checkbox-item">
          <input
            type="checkbox"
            :value="p.id"
            v-model="selectedProjects"
            @change="saveProjects"
          />
          <span>{{ p.name }}</span>
        </label>
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useSettingsStore } from '../../stores/settings'
import AppButton from '../ui/AppButton.vue'

const settingsStore = useSettingsStore()

const projects = ref<{ id: number; name: string }[]>([])
const members = ref<{ id: number; name: string }[]>([])
const selectedProjects = ref<number[]>([])
const selectedTeam = ref<number[]>([])

async function loadData() {
  try {
    const [projRes, taskRes] = await Promise.all([
      axios.get('/api/tasks/projects'),
      axios.get('/api/settings'),
    ])
    projects.value = projRes.data.projects || []
    selectedProjects.value = taskRes.data.selected_projects || []
    selectedTeam.value = taskRes.data.selected_team || []
  } catch {}
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

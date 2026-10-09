<template>
  <div class="settings-admin">
    <h3 class="section-title">{{ $t('settings.tabs.admin') }}</h3>

    <div class="adm-tabs">
      <button
        v-for="tab in tabs"
        :key="tab"
        type="button"
        class="adm-tab"
        :class="{ active: activeTab === tab }"
        :data-testid="'admin-tab-' + tab"
        @click="activeTab = tab"
      >
        {{ $t('access.tabs.' + tab) }}
      </button>
    </div>

    <!-- Access control. Each screen loads its own data when it is opened,
         so a change made on one is seen on another. -->
    <AdminUsers v-if="activeTab === 'users'" :projects="projects" :members="members" />
    <AdminGroups v-else-if="activeTab === 'groups'" :projects="projects" :members="members" />
    <AdminTemplates v-else-if="activeTab === 'templates'" :projects="projects" :members="members" />

    <!-- Settings of the data shared by all users -->
    <template v-else>
      <div class="adm-section">
        <SettingsStatusGroups />
      </div>

      <div class="adm-section">
        <h4 class="adm-title">{{ $t('settings.priorities.title') }}</h4>
        <p class="adm-hint">{{ $t('settings.priorities.order_hint') }}</p>
        <div class="adm-table-wrap">
          <table class="adm-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>{{ $t('settings.priorities.name') }}</th>
                <th>{{ $t('settings.priorities.order') }}</th>
                <th>{{ $t('settings.priorities.color') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in priorities" :key="p.external_id">
                <td>{{ p.external_id }}</td>
                <td>{{ p.name }}</td>
                <td>
                  <input
                    type="text"
                    inputmode="numeric"
                    :value="p.sort_order"
                    class="adm-input order-input"
                    @change="updatePriority(p.external_id, { sort_order: Number(($event.target as HTMLInputElement).value) || 0 })"
                  />
                </td>
                <td>
                  <input
                    type="color"
                    :value="p.color"
                    class="color-input"
                    @change="updatePriority(p.external_id, { color: ($event.target as HTMLInputElement).value })"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import SettingsStatusGroups from './SettingsStatusGroups.vue'
import AdminUsers from './access/AdminUsers.vue'
import AdminGroups from './access/AdminGroups.vue'
import AdminTemplates from './access/AdminTemplates.vue'
import { useSwal } from '../../composables/useSwal'
import type { ProjectItem, MemberItem } from '../../utils/access'

interface Priority {
  external_id: number
  name: string
  sort_order: number
  color: string
}

const tabs = ['users', 'groups', 'templates', 'data'] as const
const activeTab = ref<(typeof tabs)[number]>('users')

const { t } = useI18n()
const { toast } = useSwal()

// Everything rights can be given to: an administrator gets the full lists
const projects = ref<ProjectItem[]>([])
const members = ref<MemberItem[]>([])
const priorities = ref<Priority[]>([])

async function loadPriorities() {
  try {
    const { data } = await axios.get('/api/admin/priorities')
    priorities.value = data.priorities || []
  } catch {}
}

async function updatePriority(id: number, fields: Record<string, unknown>) {
  try {
    await axios.put(`/api/admin/priorities/${id}`, fields)
  } catch {
    toast(t('access.save_error'), 'error')
  }
  await loadPriorities()
}

onMounted(async () => {
  loadPriorities()
  try {
    const [p, m] = await Promise.all([axios.get('/api/tasks/projects'), axios.get('/api/tasks/members')])
    projects.value = p.data.projects || []
    members.value = m.data.members || []
  } catch {}
})
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 16px;
}

.order-input {
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
</style>

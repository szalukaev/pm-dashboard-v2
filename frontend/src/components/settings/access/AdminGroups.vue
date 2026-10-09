<template>
  <div class="adm-section">
    <div class="adm-header">
      <h4 class="adm-title">{{ $t('access.groups.title') }}</h4>
      <AppButton variant="primary" test-id="add-group-btn" @click="openCreate">
        <Plus :size="14" /> {{ $t('access.groups.add') }}
      </AppButton>
    </div>
    <p class="adm-hint">{{ $t('access.groups.hint') }}</p>

    <div class="adm-table-wrap">
      <table class="adm-table" data-testid="groups-table">
        <thead>
          <tr>
            <th>{{ $t('access.groups.name') }}</th>
            <th>{{ $t('access.groups.members') }}</th>
            <th>{{ $t('access.groups.projects') }}</th>
            <th>{{ $t('access.groups.team') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="g in groups" :key="g.id" :data-testid="'group-row-' + g.id">
            <td>
              {{ g.name }}
              <div v-if="g.description" class="adm-muted">{{ g.description }}</div>
            </td>
            <td>{{ g.member_ids.length }}</td>
            <td>{{ g.permissions.all_projects ? $t('access.summary.all') : g.permissions.project_ids.length }}</td>
            <td>{{ g.permissions.all_team ? $t('access.summary.all') : g.permissions.team_ids.length }}</td>
            <td>
              <div class="adm-actions">
                <button class="adm-icon-btn" :title="$t('common.edit')" :data-testid="'group-edit-' + g.id" @click="openEdit(g)"><Pencil :size="15" /></button>
                <button class="adm-icon-btn" :title="$t('access.clone.title_short')" @click="openClone(g)"><Copy :size="15" /></button>
                <button class="adm-icon-btn danger" :title="$t('common.delete')" @click="deleteGroup(g)"><Trash2 :size="15" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!groups.length" class="adm-empty">{{ $t('access.groups.empty') }}</div>
    </div>

    <!-- New group -->
    <AppModal v-model="createOpen" :title="$t('access.groups.new_title')" width="440px">
      <form class="adm-form" @submit.prevent="createGroup">
        <div class="adm-field">
          <label>{{ $t('access.groups.name') }}</label>
          <input v-model="draft.name" type="text" required data-testid="new-group-name" />
        </div>
        <div class="adm-field">
          <label>{{ $t('access.groups.description') }}</label>
          <textarea v-model="draft.description" rows="2"></textarea>
        </div>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="createOpen = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" :loading="saving" :disabled="!draft.name.trim()" test-id="create-group-btn" @click="createGroup">{{ $t('common.create') }}</AppButton>
      </template>
    </AppModal>

    <!-- Edit group: members and rights -->
    <AppModal v-model="editOpen" :title="$t('access.groups.edit_title', { name: edit.name })" width="900px">
      <div class="adm-tabs">
        <button type="button" class="adm-tab" :class="{ active: editTab === 'members' }" @click="editTab = 'members'">{{ $t('access.groups.tab_members') }}</button>
        <button type="button" class="adm-tab" :class="{ active: editTab === 'rights' }" data-testid="group-tab-rights" @click="editTab = 'rights'">{{ $t('access.groups.tab_rights') }}</button>
        <button type="button" class="adm-tab" :class="{ active: editTab === 'general' }" @click="editTab = 'general'">{{ $t('access.groups.tab_general') }}</button>
      </div>

      <div v-show="editTab === 'members'">
        <div class="adm-pick-toolbar">
          <input v-model="userQuery" class="adm-pick-search" :placeholder="$t('access.groups.search_user')" />
          <span class="adm-muted">{{ $t('access.editor.selected', { n: edit.member_ids.length }) }}</span>
        </div>
        <div class="adm-pick-list members-list">
          <label v-for="u in visibleUsers" :key="u.id" class="adm-pick-item" :class="{ selected: edit.member_ids.includes(u.id) }">
            <input type="checkbox" :value="u.id" v-model="edit.member_ids" />
            <span>{{ u.display_name || u.username }}</span>
            <span class="adm-pick-id">{{ u.username }}</span>
          </label>
          <div v-if="!visibleUsers.length" class="adm-empty">{{ $t('access.editor.nothing_found') }}</div>
        </div>
      </div>

      <div v-show="editTab === 'rights'">
        <p class="adm-hint">{{ $t('access.groups.rights_hint') }}</p>
        <PermissionsEditor v-model="edit.permissions" :projects="projects" :members="members" />
      </div>

      <div v-show="editTab === 'general'" class="adm-form">
        <div class="adm-field">
          <label>{{ $t('access.groups.name') }}</label>
          <input v-model="edit.name" type="text" />
        </div>
        <div class="adm-field">
          <label>{{ $t('access.groups.description') }}</label>
          <textarea v-model="edit.description" rows="2"></textarea>
        </div>
      </div>

      <template #footer>
        <AppButton variant="ghost" @click="editOpen = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" :loading="saving" :disabled="!edit.name.trim()" test-id="save-group-btn" @click="saveGroup">{{ $t('common.save') }}</AppButton>
      </template>
    </AppModal>

    <!-- Copy rights of a group -->
    <AppModal v-model="cloneOpen" :title="$t('access.clone.group_title', { name: cloneSource?.name || '' })" width="440px">
      <div class="adm-form">
        <div class="adm-field">
          <label>{{ $t('access.clone.target_type') }}</label>
          <select v-model="clone.to_type" @change="clone.to_id = null">
            <option value="group">{{ $t('access.clone.to_group') }}</option>
            <option value="user">{{ $t('access.clone.to_user') }}</option>
          </select>
        </div>
        <div class="adm-field">
          <label>{{ $t(clone.to_type === 'user' ? 'access.clone.apply_to_user' : 'access.clone.apply_to_group') }}</label>
          <select v-model="clone.to_id">
            <option :value="null" disabled>{{ $t('access.clone.choose') }}</option>
            <template v-if="clone.to_type === 'user'">
              <option v-for="u in users.filter(x => x.role !== 'admin')" :key="u.id" :value="u.id">{{ u.username }}</option>
            </template>
            <template v-else>
              <option v-for="g in groups.filter(x => x.id !== cloneSource?.id)" :key="g.id" :value="g.id">{{ g.name }}</option>
            </template>
          </select>
        </div>
        <label class="adm-check"><input type="checkbox" v-model="clone.include_tabs" /> {{ $t('access.clone.include_tabs') }}</label>
        <label class="adm-check"><input type="checkbox" v-model="clone.include_widgets" /> {{ $t('access.clone.include_widgets') }}</label>
      </div>
      <template #footer>
        <AppButton variant="ghost" @click="cloneOpen = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" :loading="saving" :disabled="clone.to_id === null" @click="applyClone">{{ $t('access.clone.apply') }}</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { Plus, Pencil, Copy, Trash2 } from 'lucide-vue-next'
import AppButton from '../../ui/AppButton.vue'
import AppModal from '../../ui/AppModal.vue'
import PermissionsEditor from './PermissionsEditor.vue'
import { useSwal } from '../../../composables/useSwal'
import {
  clonePermissions, emptyPermissions,
  type Permissions, type ProjectItem, type MemberItem,
} from '../../../utils/access'

interface Group {
  id: number
  name: string
  description: string
  member_ids: number[]
  permissions: Permissions
}

interface UserItem {
  id: number
  username: string
  display_name: string
  role: string
}

defineProps<{
  projects: ProjectItem[]
  members: MemberItem[]
}>()

const { t, te } = useI18n()
const { toast, confirm } = useSwal()

const groups = ref<Group[]>([])
const users = ref<UserItem[]>([])
const saving = ref(false)

async function load() {
  try {
    const [g, u] = await Promise.all([axios.get('/api/admin/groups'), axios.get('/api/admin/users')])
    groups.value = g.data.groups || []
    users.value = u.data.users || []
  } catch {
    toast(t('access.load_error'), 'error')
  }
}

function errorText(e: any): string {
  const key = `access.errors.${e?.response?.data?.error}`
  return te(key) ? t(key) : t('access.save_error')
}

// ── New group ──
const createOpen = ref(false)
const draft = reactive({ name: '', description: '' })

function openCreate() {
  draft.name = ''
  draft.description = ''
  createOpen.value = true
}

async function createGroup() {
  if (!draft.name.trim()) return
  saving.value = true
  try {
    await axios.post('/api/admin/groups', draft)
    createOpen.value = false
    await load()
    // A new group is empty: open it to add members and rights right away
    const created = groups.value.find(g => g.name === draft.name.trim())
    if (created) openEdit(created)
  } catch (e) {
    toast(errorText(e), 'error')
  } finally {
    saving.value = false
  }
}

// ── Edit ──
const editOpen = ref(false)
const editTab = ref<'members' | 'rights' | 'general'>('members')
const userQuery = ref('')
const edit = reactive({
  id: 0,
  name: '',
  description: '',
  member_ids: [] as number[],
  permissions: emptyPermissions(),
})

const visibleUsers = computed(() => {
  const q = userQuery.value.trim().toLowerCase()
  return q
    ? users.value.filter(u => u.username.toLowerCase().includes(q) || (u.display_name || '').toLowerCase().includes(q))
    : users.value
})

function openEdit(g: Group) {
  Object.assign(edit, {
    id: g.id,
    name: g.name,
    description: g.description,
    member_ids: [...g.member_ids],
    permissions: clonePermissions(g.permissions),
  })
  editTab.value = 'members'
  userQuery.value = ''
  editOpen.value = true
}

async function saveGroup() {
  saving.value = true
  try {
    await axios.put(`/api/admin/groups/${edit.id}`, {
      name: edit.name,
      description: edit.description,
      member_ids: edit.member_ids,
      permissions: edit.permissions,
    })
    editOpen.value = false
    toast(t('access.groups.saved'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e), 'error')
  } finally {
    saving.value = false
  }
}

async function deleteGroup(g: Group) {
  const result = await confirm(t('access.groups.delete_title', { name: g.name }), t('access.groups.delete_text'))
  if (!result.isConfirmed) return
  try {
    await axios.delete(`/api/admin/groups/${g.id}`)
  } catch (e) {
    toast(errorText(e), 'error')
  }
  await load()
}

// ── Copy rights ──
const cloneOpen = ref(false)
const cloneSource = ref<Group | null>(null)
const clone = reactive({
  to_type: 'group' as 'user' | 'group',
  to_id: null as number | null,
  include_tabs: true,
  include_widgets: true,
})

function openClone(g: Group) {
  cloneSource.value = g
  Object.assign(clone, { to_type: 'group', to_id: null, include_tabs: true, include_widgets: true })
  cloneOpen.value = true
}

async function applyClone() {
  if (!cloneSource.value || clone.to_id === null) return
  saving.value = true
  try {
    await axios.post('/api/admin/permissions/clone', { from_type: 'group', from_id: cloneSource.value.id, ...clone })
    cloneOpen.value = false
    toast(t('access.clone.done'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e), 'error')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.members-list {
  max-height: 46vh;
}
</style>

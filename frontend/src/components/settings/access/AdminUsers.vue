<template>
  <div class="adm-section">
    <div class="adm-header">
      <h4 class="adm-title">{{ $t('access.users.title') }}</h4>
      <AppButton variant="primary" test-id="add-user-btn" @click="openCreate">
        <Plus :size="14" /> {{ $t('access.users.add') }}
      </AppButton>
    </div>
    <p class="adm-hint">{{ $t('access.users.hint') }}</p>

    <div class="adm-table-wrap">
      <table class="adm-table" data-testid="users-table">
        <thead>
          <tr>
            <th>{{ $t('access.users.login') }}</th>
            <th>{{ $t('access.users.name') }}</th>
            <th>{{ $t('access.users.role') }}</th>
            <th>{{ $t('access.users.template') }}</th>
            <th>{{ $t('access.users.member') }}</th>
            <th>{{ $t('access.users.groups') }}</th>
            <th>{{ $t('access.users.created') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id" :class="{ 'adm-blocked': u.is_blocked }" :data-testid="'user-row-' + u.id">
            <td>
              {{ u.username }}
              <span v-if="u.is_blocked" class="adm-badge danger">{{ $t('access.users.blocked') }}</span>
            </td>
            <td>
              <input
                class="adm-input"
                :value="u.display_name"
                @change="update(u, { display_name: ($event.target as HTMLInputElement).value })"
              />
            </td>
            <td>
              <select class="adm-select" :value="u.role" @change="update(u, { role: ($event.target as HTMLSelectElement).value })">
                <option value="user">{{ $t('access.users.role_user') }}</option>
                <option value="admin">{{ $t('access.users.role_admin') }}</option>
              </select>
            </td>
            <td>
              <span v-if="u.role === 'admin'" class="adm-muted">{{ $t('access.users.full_access') }}</span>
              <select
                v-else
                class="adm-select"
                :value="u.custom ? CUSTOM : (u.role_template_id ?? '')"
                @change="changeTemplate(u, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">{{ $t('access.users.no_template') }}</option>
                <option v-if="u.custom" :value="CUSTOM" disabled>{{ $t('access.users.custom') }}</option>
                <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }}</option>
              </select>
            </td>
            <!-- Set by the user themselves with a personal API key -->
            <td>
              <span v-if="u.member_id">{{ u.member_name || '#' + u.member_id }}</span>
              <span v-else class="adm-muted" :title="$t('access.users.no_member_hint')">{{ $t('access.users.no_member') }}</span>
            </td>
            <td>
              <span v-for="g in u.groups" :key="g" class="adm-badge group-badge">{{ g }}</span>
              <span v-if="!u.groups.length" class="adm-muted">—</span>
            </td>
            <td class="adm-muted">{{ formatDate(u.created_at) }}</td>
            <td>
              <div class="adm-actions">
                <button class="adm-icon-btn" :title="$t('access.users.rights')" :disabled="u.role === 'admin'" :data-testid="'user-rights-' + u.id" @click="openRights(u)">
                  <ShieldCheck :size="15" />
                </button>
                <button class="adm-icon-btn" :title="$t('access.clone.title_short')" :disabled="u.role === 'admin'" @click="openClone(u)">
                  <Copy :size="15" />
                </button>
                <button class="adm-icon-btn" :title="$t('access.users.reset_password')" @click="resetPassword(u)">
                  <KeyRound :size="15" />
                </button>
                <button
                  class="adm-icon-btn"
                  :title="$t(u.is_blocked ? 'access.users.unblock' : 'access.users.block')"
                  :disabled="u.id === auth.user?.id"
                  @click="update(u, { is_blocked: !u.is_blocked })"
                >
                  <Unlock v-if="u.is_blocked" :size="15" />
                  <Lock v-else :size="15" />
                </button>
                <button class="adm-icon-btn danger" :title="$t('common.delete')" :disabled="u.id === auth.user?.id" @click="deleteUser(u)">
                  <Trash2 :size="15" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <AppPagination
      :total="total"
      :limit="pageSize"
      :offset="offset"
      @update:offset="setPage"
      @update:limit="setPageSize"
    />

    <!-- New user -->
    <AppModal v-model="createOpen" :title="$t('access.users.new_title')" width="440px">
      <form class="adm-form" @submit.prevent="createUser">
        <div class="adm-field">
          <label>{{ $t('access.users.login') }}</label>
          <input v-model="draft.username" type="text" autocomplete="off" required minlength="3" data-testid="new-user-login" />
        </div>
        <div class="adm-field">
          <label>{{ $t('access.users.name') }}</label>
          <input v-model="draft.display_name" type="text" autocomplete="off" />
        </div>
        <div class="adm-field">
          <label>{{ $t('access.users.temp_password') }}</label>
          <input v-model="draft.password" type="text" autocomplete="off" required minlength="6" data-testid="new-user-password" />
        </div>
        <div class="adm-field">
          <label>{{ $t('access.users.role') }}</label>
          <select v-model="draft.role">
            <option value="user">{{ $t('access.users.role_user') }}</option>
            <option value="admin">{{ $t('access.users.role_admin') }}</option>
          </select>
        </div>
        <div v-if="draft.role === 'user'" class="adm-field">
          <label>{{ $t('access.users.assign_template') }}</label>
          <select v-model="draft.role_template_id">
            <option :value="null">{{ $t('access.users.no_template') }}</option>
            <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>
        </div>
        <p class="adm-muted">{{ $t('access.users.new_hint') }}</p>
      </form>
      <template #footer>
        <AppButton variant="ghost" @click="createOpen = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" :loading="saving" :disabled="draft.username.trim().length < 3 || draft.password.length < 6" test-id="create-user-btn" @click="createUser">
          {{ $t('common.create') }}
        </AppButton>
      </template>
    </AppModal>

    <!-- Rights of a user -->
    <AppModal v-model="rightsOpen" :title="$t('access.rights.title', { name: rightsUser?.username || '' })" width="900px">
      <div v-if="grants" class="rights">
        <!-- Where the rights come from -->
        <div class="rights-sources">
          <div class="rights-source">
            <span class="adm-label">{{ $t('access.rights.own_source') }}</span>
            <span v-if="grants.individual" class="adm-badge accent">{{ $t('access.users.custom') }}</span>
            <span v-else-if="grants.template" class="adm-badge">{{ $t('access.rights.from_template', { name: grants.template.name }) }}</span>
            <span v-else class="adm-badge">{{ $t('access.rights.default') }}</span>
          </div>
          <div v-for="g in grants.groups" :key="g.id" class="rights-source">
            <span class="adm-label">{{ $t('access.rights.from_group', { name: g.name }) }}</span>
            <span class="adm-muted">{{ summary(g.permissions) }}</span>
          </div>
          <div class="rights-source total">
            <span class="adm-label">{{ $t('access.rights.effective') }}</span>
            <span>{{ summary(grants.effective) }}</span>
          </div>
        </div>

        <!-- Individual rights -->
        <template v-if="editing">
          <p class="adm-hint">{{ $t('access.rights.individual_hint') }}</p>
          <PermissionsEditor v-model="editing" :projects="projects" :members="members" />
        </template>
        <p v-else class="adm-hint">{{ $t('access.rights.inherited_hint') }}</p>
      </div>
      <template #footer>
        <template v-if="editing">
          <AppButton variant="ghost" @click="saveAsTemplate">{{ $t('access.rights.save_as_template') }}</AppButton>
          <AppButton v-if="grants?.individual" variant="ghost" @click="clearRights">{{ $t('access.rights.reset') }}</AppButton>
          <AppButton variant="ghost" @click="rightsOpen = false">{{ $t('common.cancel') }}</AppButton>
          <AppButton variant="primary" :loading="saving" test-id="save-rights-btn" @click="saveRights">{{ $t('common.save') }}</AppButton>
        </template>
        <template v-else>
          <AppButton variant="ghost" @click="rightsOpen = false">{{ $t('common.close') }}</AppButton>
          <AppButton variant="primary" test-id="edit-rights-btn" @click="startEditing">{{ $t('access.rights.set_individually') }}</AppButton>
        </template>
      </template>
    </AppModal>

    <!-- Copy rights -->
    <AppModal v-model="cloneOpen" :title="$t('access.clone.title', { name: cloneSource?.username || '' })" width="440px">
      <div class="adm-form">
        <div class="adm-field">
          <label>{{ $t('access.clone.target_type') }}</label>
          <select v-model="clone.to_type" @change="clone.to_id = null">
            <option value="user">{{ $t('access.clone.to_user') }}</option>
            <option value="group">{{ $t('access.clone.to_group') }}</option>
          </select>
        </div>
        <div class="adm-field">
          <label>{{ $t(clone.to_type === 'user' ? 'access.clone.apply_to_user' : 'access.clone.apply_to_group') }}</label>
          <select v-model="clone.to_id">
            <option :value="null" disabled>{{ $t('access.clone.choose') }}</option>
            <template v-if="clone.to_type === 'user'">
              <option v-for="u in cloneTargets" :key="u.id" :value="u.id">{{ u.username }}</option>
            </template>
            <template v-else>
              <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
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

    <!-- Temporary password, shown once -->
    <AppModal v-model="passwordOpen" :title="$t('access.password.title')" width="440px">
      <p class="adm-hint">{{ $t('access.password.hint', { name: passwordUser }) }}</p>
      <div class="password-box" data-testid="temp-password">{{ tempPassword }}</div>
      <template #footer>
        <AppButton variant="ghost" @click="copyPassword"><Copy :size="14" /> {{ $t('access.password.copy') }}</AppButton>
        <AppButton variant="primary" @click="passwordOpen = false">{{ $t('common.close') }}</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { Plus, ShieldCheck, Copy, KeyRound, Lock, Unlock, Trash2 } from 'lucide-vue-next'
import AppButton from '../../ui/AppButton.vue'
import AppModal from '../../ui/AppModal.vue'
import PermissionsEditor from './PermissionsEditor.vue'
import AppPagination from '../../ui/AppPagination.vue'
import { useSettingsStore } from '../../../stores/settings'
import { useAuthStore } from '../../../stores/auth'
import { useSwal } from '../../../composables/useSwal'
import { formatDate } from '../../../utils/format'
import {
  clonePermissions, emptyPermissions,
  type Permissions, type Grants, type ProjectItem, type MemberItem,
} from '../../../utils/access'

interface AdminUser {
  id: number
  username: string
  display_name: string
  role: string
  created_at: string
  is_blocked: boolean
  member_id: number | null
  member_name: string
  role_template_id: number | null
  custom: boolean
  groups: string[]
}

const props = defineProps<{
  projects: ProjectItem[]
  members: MemberItem[]
}>()

// Value of the template select for individually set rights
const CUSTOM = 'custom'

const { t, te } = useI18n()
const { toast, confirm } = useSwal()
const auth = useAuthStore()

// The table shows one page of users
const PAGE_TABLE = 'admin_users_table'
const settingsStore = useSettingsStore()
const pageSize = computed(() => settingsStore.pageSize(PAGE_TABLE))
const users = ref<AdminUser[]>([])
const total = ref(0)
const offset = ref(0)
const templates = ref<{ id: number; name: string }[]>([])
const groups = ref<{ id: number; name: string }[]>([])
const saving = ref(false)

async function load() {
  try {
    await settingsStore.ensureLoaded()
    const usersPage = () => axios.get('/api/admin/users', { params: { limit: pageSize.value, offset: offset.value } })
    let [u, tpl, g] = await Promise.all([
      usersPage(),
      axios.get('/api/admin/role-templates'),
      axios.get('/api/admin/groups'),
    ])
    // The last user of the last page was deleted: show the page before it
    if ((u.data.users || []).length === 0 && u.data.total > 0) {
      offset.value = Math.floor((u.data.total - 1) / pageSize.value) * pageSize.value
      u = await usersPage()
    }
    users.value = u.data.users || []
    total.value = u.data.total || 0
    templates.value = tpl.data.templates || []
    groups.value = g.data.groups || []
  } catch {
    toast(t('access.load_error'), 'error')
  }
}

async function setPage(value: number) {
  offset.value = value
  await load()
}

async function setPageSize(size: number) {
  await settingsStore.setPageSize(PAGE_TABLE, size)
  offset.value = 0
  await load()
}

function errorText(e: any, fallback: string): string {
  const code = e?.response?.data?.error
  const key = `access.errors.${code}`
  return code && te(key) ? t(key) : t(fallback)
}

// One changed field of a user; the table shows what the server saved.
async function update(u: AdminUser, fields: Record<string, unknown>) {
  try {
    await axios.put(`/api/admin/users/${u.id}`, fields)
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  } finally {
    await load()
  }
}

async function changeTemplate(u: AdminUser, value: string) {
  if (value === CUSTOM) return
  // A template replaces individually set rights: ask before dropping them
  if (u.custom) {
    const result = await confirm(t('access.users.template_replaces_title'), t('access.users.template_replaces_text'))
    if (!result.isConfirmed) {
      await load()
      return
    }
  }
  await update(u, { role_template_id: value ? Number(value) : null })
}

async function deleteUser(u: AdminUser) {
  const result = await confirm(t('access.users.delete_title', { name: u.username }), t('common.confirm_delete_text'))
  if (!result.isConfirmed) return
  try {
    await axios.delete(`/api/admin/users/${u.id}`)
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  }
  await load()
}

// ── New user ──
const createOpen = ref(false)
const draft = reactive({
  username: '',
  display_name: '',
  password: '',
  role: 'user',
  role_template_id: null as number | null,
})

function openCreate() {
  Object.assign(draft, { username: '', display_name: '', password: '', role: 'user', role_template_id: null })
  createOpen.value = true
}

async function createUser() {
  if (draft.username.trim().length < 3 || draft.password.length < 6) return
  saving.value = true
  try {
    await axios.post('/api/admin/users', {
      ...draft,
      role_template_id: draft.role === 'user' ? draft.role_template_id : null,
    })
    createOpen.value = false
    toast(t('access.users.created_toast'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  } finally {
    saving.value = false
  }
}

// ── Rights ──
const rightsOpen = ref(false)
const rightsUser = ref<AdminUser | null>(null)
const grants = ref<Grants | null>(null)
// The individual rights being edited; null while they are inherited
const editing = ref<Permissions | null>(null)

function summary(p: Permissions): string {
  const parts = [
    p.all_projects ? t('access.summary.all_projects') : t('access.summary.projects', { n: p.project_ids.length }),
    p.all_team ? t('access.summary.all_team') : t('access.summary.team', { n: p.team_ids.length }),
  ]
  if (p.show_unassigned) parts.push(t('access.summary.unassigned'))
  if (p.from_source) parts.push(t('access.summary.from_source'))
  if (p.own_tasks_only) parts.push(t('access.summary.own_tasks'))
  if (p.read_only) parts.push(t('access.summary.read_only'))
  if (p.visible_tabs) parts.push(t('access.summary.tabs', { n: p.visible_tabs.length }))
  return parts.join(' · ')
}

async function openRights(u: AdminUser) {
  rightsUser.value = u
  grants.value = null
  editing.value = null
  rightsOpen.value = true
  try {
    const { data } = await axios.get(`/api/admin/users/${u.id}/permissions`)
    grants.value = data
    editing.value = data.individual ? clonePermissions(data.individual) : null
  } catch {
    rightsOpen.value = false
    toast(t('access.load_error'), 'error')
  }
}

// Individual rights start from the user's own inherited set
function startEditing() {
  if (!grants.value) return
  const base = grants.value.template?.permissions
  // Without a template the default applies: own tasks only
  editing.value = base ? clonePermissions(base) : { ...emptyPermissions(), own_tasks_only: true }
}

async function saveRights() {
  if (!rightsUser.value || !editing.value) return
  saving.value = true
  try {
    await axios.put(`/api/admin/users/${rightsUser.value.id}/permissions`, editing.value)
    rightsOpen.value = false
    toast(t('access.rights.saved'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  } finally {
    saving.value = false
  }
}

async function clearRights() {
  if (!rightsUser.value) return
  try {
    await axios.delete(`/api/admin/users/${rightsUser.value.id}/permissions`)
    rightsOpen.value = false
    toast(t('access.rights.reset_done'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  }
}

// "Create a template from the current rights": the saved rights of the user
async function saveAsTemplate() {
  if (!rightsUser.value) return
  const result = await useSwal().fire({
    title: t('access.rights.template_name_title'),
    input: 'text',
    inputPlaceholder: t('access.templates.name'),
    showCancelButton: true,
    confirmButtonText: t('common.create'),
    cancelButtonText: t('common.cancel'),
  })
  const name = String(result.value || '').trim()
  if (!result.isConfirmed || !name) return
  try {
    // Save what is on the screen first, so the template gets exactly it
    if (editing.value) await axios.put(`/api/admin/users/${rightsUser.value.id}/permissions`, editing.value)
    await axios.post('/api/admin/role-templates', { name, from_user_id: rightsUser.value.id })
    toast(t('access.templates.created_toast'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  }
}

// ── Copy rights ──
const cloneOpen = ref(false)
const cloneSource = ref<AdminUser | null>(null)
const clone = reactive({
  to_type: 'user' as 'user' | 'group',
  to_id: null as number | null,
  include_tabs: true,
  include_widgets: true,
})

// Rights may be copied to any user, not only to those on the current page
const allUsers = ref<AdminUser[]>([])
const cloneTargets = computed(() => allUsers.value.filter(u => u.id !== cloneSource.value?.id && u.role !== 'admin'))

async function openClone(u: AdminUser) {
  try {
    const { data } = await axios.get('/api/admin/users')
    allUsers.value = data.users || []
  } catch {
    allUsers.value = users.value
  }
  cloneSource.value = u
  Object.assign(clone, { to_type: 'user', to_id: null, include_tabs: true, include_widgets: true })
  cloneOpen.value = true
}

async function applyClone() {
  if (!cloneSource.value || clone.to_id === null) return
  saving.value = true
  try {
    await axios.post('/api/admin/permissions/clone', { from_type: 'user', from_id: cloneSource.value.id, ...clone })
    cloneOpen.value = false
    toast(t('access.clone.done'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  } finally {
    saving.value = false
  }
}

// ── Password reset ──
const passwordOpen = ref(false)
const passwordUser = ref('')
const tempPassword = ref('')

async function resetPassword(u: AdminUser) {
  const result = await confirm(t('access.password.confirm_title', { name: u.username }), t('access.password.confirm_text'))
  if (!result.isConfirmed) return
  try {
    const { data } = await axios.post(`/api/admin/users/${u.id}/reset-password`)
    passwordUser.value = u.username
    tempPassword.value = data.password
    passwordOpen.value = true
  } catch (e) {
    toast(errorText(e, 'access.save_error'), 'error')
  }
}

// The clipboard API exists only on https and localhost; the dashboard is
// often opened by plain http inside a network, so there is a fallback.
async function copyPassword() {
  let copied = false
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(tempPassword.value)
      copied = true
    }
  } catch {}
  if (!copied) {
    const field = document.createElement('textarea')
    field.value = tempPassword.value
    field.setAttribute('readonly', '')
    field.style.position = 'fixed'
    field.style.opacity = '0'
    document.body.appendChild(field)
    field.select()
    try {
      copied = document.execCommand('copy')
    } catch {}
    field.remove()
  }
  toast(t(copied ? 'common.copied' : 'access.password.copy_failed'), copied ? 'success' : 'error')
}

onMounted(load)
defineExpose({ load })
</script>

<style scoped>
.group-badge {
  margin-right: 4px;
}

.rights {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rights-sources {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  font-size: 13px;
  color: var(--text);
}

.rights-source {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-wrap: wrap;
}

.rights-source .adm-label {
  margin-bottom: 0;
  min-width: 200px;
}

.rights-source.total {
  padding-top: 6px;
  border-top: 1px solid var(--border-light);
  color: var(--text-bright);
}

.password-box {
  padding: 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 18px;
  letter-spacing: 1px;
  text-align: center;
  color: var(--text-bright);
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  user-select: all;
}
</style>

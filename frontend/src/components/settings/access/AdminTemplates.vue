<template>
  <div class="adm-section">
    <div class="adm-header">
      <h4 class="adm-title">{{ $t('access.templates.title') }}</h4>
      <AppButton variant="primary" test-id="add-template-btn" @click="openCreate">
        <Plus :size="14" /> {{ $t('access.templates.add') }}
      </AppButton>
    </div>
    <p class="adm-hint">{{ $t('access.templates.hint') }}</p>

    <div class="adm-table-wrap">
      <table class="adm-table" data-testid="templates-table">
        <thead>
          <tr>
            <th>{{ $t('access.templates.name') }}</th>
            <th>{{ $t('access.groups.projects') }}</th>
            <th>{{ $t('access.groups.team') }}</th>
            <th>{{ $t('access.templates.users') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="tpl in templates" :key="tpl.id" :data-testid="'template-row-' + tpl.id">
            <td>
              {{ tpl.name }}
              <span v-if="tpl.grants_admin" class="adm-badge accent">{{ $t('access.users.role_admin') }}</span>
              <span v-if="tpl.permissions.read_only" class="adm-badge">{{ $t('access.summary.read_only') }}</span>
              <span v-if="tpl.permissions.own_tasks_only" class="adm-badge">{{ $t('access.summary.own_tasks') }}</span>
              <div v-if="tpl.description" class="adm-muted">{{ tpl.description }}</div>
            </td>
            <td>{{ tpl.permissions.all_projects ? $t('access.summary.all') : tpl.permissions.project_ids.length }}</td>
            <td>{{ tpl.permissions.all_team ? $t('access.summary.all') : tpl.permissions.team_ids.length }}</td>
            <td>{{ tpl.user_count }}</td>
            <td>
              <div class="adm-actions">
                <button class="adm-icon-btn" :title="$t('common.edit')" :data-testid="'template-edit-' + tpl.id" @click="openEdit(tpl)"><Pencil :size="15" /></button>
                <button class="adm-icon-btn" :title="$t('access.templates.clone')" @click="cloneTemplate(tpl)"><Copy :size="15" /></button>
                <button class="adm-icon-btn danger" :title="$t('common.delete')" @click="deleteTemplate(tpl)"><Trash2 :size="15" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create / edit a template -->
    <AppModal v-model="editOpen" :title="edit.id ? $t('access.templates.edit_title', { name: edit.name }) : $t('access.templates.new_title')" width="900px">
      <div class="adm-form">
        <div class="name-row">
          <div class="adm-field">
            <label>{{ $t('access.templates.name') }}</label>
            <input v-model="edit.name" type="text" data-testid="template-name" />
          </div>
          <div class="adm-field">
            <label>{{ $t('access.groups.description') }}</label>
            <input v-model="edit.description" type="text" />
          </div>
        </div>
        <p v-if="edit.grants_admin" class="adm-hint">{{ $t('access.templates.admin_hint') }}</p>
        <template v-else>
          <p v-if="edit.id" class="adm-hint">{{ $t('access.templates.edit_hint') }}</p>
          <PermissionsEditor v-model="edit.permissions" :projects="projects" :members="members" />
        </template>
      </div>
      <template #footer>
        <AppButton variant="ghost" @click="editOpen = false">{{ $t('common.cancel') }}</AppButton>
        <AppButton variant="primary" :loading="saving" :disabled="!edit.name.trim()" test-id="save-template-btn" @click="saveTemplate">{{ $t('common.save') }}</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
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

interface Template {
  id: number
  name: string
  description: string
  is_system: boolean
  grants_admin: boolean
  user_count: number
  permissions: Permissions
}

defineProps<{
  projects: ProjectItem[]
  members: MemberItem[]
}>()

const { t, te } = useI18n()
const { toast, confirm, fire } = useSwal()

const templates = ref<Template[]>([])
const saving = ref(false)

async function load() {
  try {
    const { data } = await axios.get('/api/admin/role-templates')
    templates.value = data.templates || []
  } catch {
    toast(t('access.load_error'), 'error')
  }
}

function errorText(e: any): string {
  const key = `access.errors.${e?.response?.data?.error}`
  return te(key) ? t(key) : t('access.save_error')
}

const editOpen = ref(false)
const edit = reactive({
  id: 0,
  name: '',
  description: '',
  grants_admin: false,
  permissions: emptyPermissions(),
})

function openCreate() {
  Object.assign(edit, { id: 0, name: '', description: '', grants_admin: false, permissions: emptyPermissions() })
  editOpen.value = true
}

function openEdit(tpl: Template) {
  Object.assign(edit, {
    id: tpl.id,
    name: tpl.name,
    description: tpl.description,
    grants_admin: tpl.grants_admin,
    permissions: clonePermissions(tpl.permissions),
  })
  editOpen.value = true
}

async function saveTemplate() {
  if (!edit.name.trim()) return
  saving.value = true
  try {
    const body = { name: edit.name, description: edit.description, permissions: edit.permissions }
    if (edit.id) await axios.put(`/api/admin/role-templates/${edit.id}`, body)
    else await axios.post('/api/admin/role-templates', body)
    editOpen.value = false
    toast(t(edit.id ? 'access.templates.saved' : 'access.templates.created_toast'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e), 'error')
  } finally {
    saving.value = false
  }
}

async function cloneTemplate(tpl: Template) {
  const result = await fire({
    title: t('access.templates.clone_title', { name: tpl.name }),
    input: 'text',
    inputValue: t('access.templates.copy_name', { name: tpl.name }),
    showCancelButton: true,
    confirmButtonText: t('common.create'),
    cancelButtonText: t('common.cancel'),
  })
  const name = String(result.value || '').trim()
  if (!result.isConfirmed || !name) return
  try {
    await axios.post('/api/admin/role-templates', { name, description: tpl.description, from_template_id: tpl.id })
    toast(t('access.templates.created_toast'), 'success')
    await load()
  } catch (e) {
    toast(errorText(e), 'error')
  }
}

async function deleteTemplate(tpl: Template) {
  const result = await confirm(
    t('access.templates.delete_title', { name: tpl.name }),
    tpl.user_count ? t('access.templates.delete_text_users', { n: tpl.user_count }) : t('common.confirm_delete_text'),
  )
  if (!result.isConfirmed) return
  try {
    await axios.delete(`/api/admin/role-templates/${tpl.id}`)
  } catch (e) {
    toast(errorText(e), 'error')
  }
  await load()
}

onMounted(load)
</script>

<style scoped>
.name-row {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 12px;
}

@media (max-width: 768px) {
  .name-row {
    grid-template-columns: 1fr;
  }
}
</style>

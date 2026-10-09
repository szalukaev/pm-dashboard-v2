<template>
  <div class="perm-editor">
    <!-- What the user may do -->
    <div class="perm-flags">
      <label class="adm-check" :title="$t('access.editor.own_tasks_hint')">
        <input type="checkbox" v-model="model.own_tasks_only" data-testid="perm-own-tasks" />
        {{ $t('access.editor.own_tasks') }}
      </label>
      <label class="adm-check" :title="$t('access.editor.read_only_hint')">
        <input type="checkbox" v-model="model.read_only" data-testid="perm-read-only" />
        {{ $t('access.editor.read_only') }}
      </label>
      <label class="adm-check" :title="$t('access.editor.from_source_hint')">
        <input type="checkbox" v-model="model.from_source" data-testid="perm-from-source" />
        {{ $t('access.editor.from_source') }}
      </label>
    </div>

    <div class="perm-columns">
      <!-- Projects -->
      <div class="perm-block">
        <div class="perm-block-head">
          <span class="adm-label">{{ $t('access.editor.projects') }}</span>
          <label class="adm-check">
            <input type="checkbox" v-model="model.all_projects" data-testid="perm-all-projects" />
            {{ $t('access.editor.all_projects') }}
          </label>
        </div>
        <template v-if="!model.all_projects">
          <div class="adm-pick-toolbar">
            <input v-model="projectQuery" class="adm-pick-search" :placeholder="$t('access.editor.search_project')" />
            <span class="adm-muted">{{ $t('access.editor.selected', { n: model.project_ids.length }) }}</span>
            <button type="button" class="adm-link-btn" @click="model.project_ids = []">{{ $t('access.editor.clear') }}</button>
          </div>
          <div class="adm-pick-list">
            <!-- A subproject of a ticked project is covered by it: shown
                 ticked and locked -->
            <label
              v-for="item in visibleProjects"
              :key="item.id"
              class="adm-pick-item"
              :class="{ selected: projectChecked(item.id), inherited: covered.has(item.id) }"
              :style="{ paddingLeft: (projectQuery ? 8 : item.depth * 18 + 8) + 'px' }"
              :title="covered.has(item.id) ? $t('access.editor.covered_by_parent') : undefined"
            >
              <input
                type="checkbox"
                :checked="projectChecked(item.id)"
                :disabled="covered.has(item.id)"
                @change="toggleProject(item.id, ($event.target as HTMLInputElement).checked)"
              />
              <span :class="{ 'perm-parent': item.hasChildren }">{{ item.name }}</span>
              <span class="adm-pick-id">#{{ item.id }}</span>
            </label>
            <div v-if="!visibleProjects.length" class="adm-empty">{{ $t('access.editor.nothing_found') }}</div>
          </div>
          <p class="adm-muted perm-note">{{ $t('access.editor.projects_note') }}</p>
        </template>
      </div>

      <!-- Team -->
      <div class="perm-block">
        <div class="perm-block-head">
          <span class="adm-label">{{ $t('access.editor.team') }}</span>
          <label class="adm-check">
            <input type="checkbox" v-model="model.all_team" data-testid="perm-all-team" />
            {{ $t('access.editor.all_team') }}
          </label>
        </div>
        <template v-if="!model.all_team">
          <div class="adm-pick-toolbar">
            <input v-model="memberQuery" class="adm-pick-search" :placeholder="$t('access.editor.search_member')" />
            <span class="adm-muted">{{ $t('access.editor.selected', { n: model.team_ids.length }) }}</span>
            <button type="button" class="adm-link-btn" @click="model.team_ids = []">{{ $t('access.editor.clear') }}</button>
          </div>
          <div class="adm-pick-list">
            <label
              v-for="m in visibleMembers"
              :key="m.id"
              class="adm-pick-item"
              :class="{ selected: model.team_ids.includes(m.id) }"
            >
              <input type="checkbox" :value="m.id" v-model="model.team_ids" />
              <span>{{ m.name }}</span>
            </label>
            <div v-if="!visibleMembers.length" class="adm-empty">{{ $t('access.editor.nothing_found') }}</div>
          </div>
        </template>
      </div>
    </div>

    <div class="perm-columns">
      <!-- Tabs -->
      <div class="perm-block">
        <div class="perm-block-head">
          <span class="adm-label">{{ $t('access.editor.tabs') }}</span>
          <label class="adm-check">
            <input type="checkbox" :checked="model.visible_tabs === null" @change="toggleAll('visible_tabs', TABS, $event)" />
            {{ $t('access.editor.all_tabs') }}
          </label>
        </div>
        <div v-if="model.visible_tabs !== null" class="perm-keys">
          <label v-for="tab in TABS" :key="tab" class="adm-check">
            <input type="checkbox" :value="tab" v-model="model.visible_tabs" />
            {{ $t('navigation.' + tab) }}
          </label>
        </div>
      </div>

      <!-- Analytics widgets -->
      <div class="perm-block">
        <div class="perm-block-head">
          <span class="adm-label">{{ $t('access.editor.widgets') }}</span>
          <label class="adm-check">
            <input type="checkbox" :checked="model.widgets === null" @change="toggleAll('widgets', WIDGETS, $event)" />
            {{ $t('access.editor.all_widgets') }}
          </label>
        </div>
        <div v-if="model.widgets !== null" class="perm-keys">
          <label v-for="widget in WIDGETS" :key="widget" class="adm-check">
            <input type="checkbox" :value="widget" v-model="model.widgets" />
            {{ $t('access.widgets.' + widget) }}
          </label>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  TABS, WIDGETS, projectTree, withDescendants, coveredByParents,
  type Permissions, type ProjectItem, type MemberItem,
} from '../../../utils/access'

// One editor for the rights of a user, a group and a role template. The
// parent owns the object (v-model) and decides when to save it.
const model = defineModel<Permissions>({ required: true })

const props = defineProps<{
  projects: ProjectItem[]
  members: MemberItem[]
}>()

const projectQuery = ref('')
const memberQuery = ref('')

const tree = computed(() => projectTree(props.projects))

const visibleProjects = computed(() => {
  const q = projectQuery.value.trim().toLowerCase()
  if (!q) return tree.value
  return tree.value.filter(p => p.name.toLowerCase().includes(q) || String(p.id) === q)
})

const visibleMembers = computed(() => {
  const q = memberQuery.value.trim().toLowerCase()
  return q ? props.members.filter(m => m.name.toLowerCase().includes(q)) : props.members
})

// A right to a project is a right to its whole branch, including the
// subprojects created later. Only the ticked project itself is stored; its
// subprojects are covered by it.
const covered = computed(() => coveredByParents(props.projects, model.value.project_ids))

function projectChecked(id: number): boolean {
  return model.value.project_ids.includes(id) || covered.value.has(id)
}

function toggleProject(id: number, checked: boolean) {
  const current = new Set(model.value.project_ids)
  if (checked) {
    current.add(id)
    // Ticks of its subprojects are now redundant
    for (const child of withDescendants(props.projects, id)) {
      if (child !== id) current.delete(child)
    }
  } else {
    current.delete(id)
  }
  model.value.project_ids = [...current]
}

// "Everything" is stored as null; unticking it starts from the full list.
function toggleAll(field: 'visible_tabs' | 'widgets', all: string[], e: Event) {
  model.value[field] = (e.target as HTMLInputElement).checked ? null : [...all]
}
</script>

<style scoped>
.perm-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.perm-flags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
}

.perm-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.perm-block {
  min-width: 0;
}

.perm-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.perm-block-head .adm-label {
  margin-bottom: 0;
}

.perm-parent {
  font-weight: 600;
}

.perm-note {
  margin-top: 6px;
}

.adm-pick-item.inherited {
  cursor: default;
  color: var(--text-muted);
}

.perm-keys {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

@media (max-width: 768px) {
  .perm-columns {
    grid-template-columns: 1fr;
  }
}
</style>

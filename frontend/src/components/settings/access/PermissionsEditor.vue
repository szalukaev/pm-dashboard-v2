<template>
  <!-- Права по разделам: слева разделы с кратким итогом, справа — настройка раздела -->
  <div class="pe" data-testid="perm-editor">
    <nav class="pe-nav" :aria-label="$t('access.editor.sections_label')">
      <button
        v-for="s in sections"
        :key="s.id"
        type="button"
        class="pe-nav-item"
        :class="{ active: active === s.id }"
        :data-testid="'perm-section-' + s.id"
        @click="active = s.id"
      >
        <component :is="s.icon" :size="16" class="pe-nav-icon" />
        <span class="pe-nav-text">
          <span class="pe-nav-title">{{ $t('access.editor.sections.' + s.id) }}</span>
          <span class="pe-nav-status" :class="{ limited: s.limited }">{{ s.status }}</span>
        </span>
      </button>
    </nav>

    <div class="pe-pane">
      <header class="pe-head">
        <h5 class="pe-title">{{ $t('access.editor.sections.' + active) }}</h5>
        <p class="pe-desc">{{ $t('access.editor.desc.' + active) }}</p>
      </header>

      <!-- Общее -->
      <template v-if="active === 'general'">
        <label class="pe-row">
          <span class="pe-row-text">
            <span class="pe-row-title">{{ $t('access.editor.read_only') }}</span>
            <span class="pe-row-hint">{{ $t('access.editor.read_only_hint') }}</span>
          </span>
          <span class="pe-switch"><input type="checkbox" v-model="model.read_only" data-testid="perm-read-only" /><span class="pe-slider"></span></span>
        </label>
        <label class="pe-row">
          <span class="pe-row-text">
            <span class="pe-row-title">{{ $t('access.editor.own_tasks') }}</span>
            <span class="pe-row-hint">{{ $t('access.editor.own_tasks_hint') }}</span>
          </span>
          <span class="pe-switch"><input type="checkbox" v-model="model.own_tasks_only" data-testid="perm-own-tasks" /><span class="pe-slider"></span></span>
        </label>
      </template>

      <!-- Проекты -->
      <template v-else-if="active === 'projects'">
        <div class="pe-scope" role="radiogroup">
          <label class="pe-scope-option" :class="{ active: model.all_projects }">
            <input type="radio" :checked="model.all_projects" data-testid="perm-all-projects" @change="model.all_projects = true" />
            <span class="pe-scope-title">{{ $t('access.editor.all_projects') }}</span>
            <span class="pe-scope-hint">{{ $t('access.editor.all_projects_hint') }}</span>
          </label>
          <label class="pe-scope-option" :class="{ active: !model.all_projects }">
            <input type="radio" :checked="!model.all_projects" data-testid="perm-some-projects" @change="model.all_projects = false" />
            <span class="pe-scope-title">{{ $t('access.editor.some_projects') }}</span>
            <span class="pe-scope-hint">{{ $t('access.editor.selected', { n: model.project_ids.length }) }}</span>
          </label>
        </div>

        <template v-if="!model.all_projects">
          <div class="adm-pick-toolbar">
            <input v-model="projectQuery" class="adm-pick-search" :placeholder="$t('access.editor.search_project')" />
            <button type="button" class="adm-link-btn" @click="model.project_ids = []">{{ $t('access.editor.clear') }}</button>
          </div>
          <div class="adm-pick-list pe-list">
            <!-- Подпроект отмеченного проекта им покрыт: показан отмеченным и закрыт -->
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
              <span :class="{ 'pe-parent': item.hasChildren }">{{ item.name }}</span>
              <span class="adm-pick-id">#{{ item.id }}</span>
            </label>
            <div v-if="!visibleProjects.length" class="adm-empty">{{ $t('access.editor.nothing_found') }}</div>
          </div>
          <p class="pe-note">{{ $t('access.editor.projects_note') }}</p>
        </template>

        <label class="pe-row">
          <span class="pe-row-text">
            <span class="pe-row-title">{{ $t('access.editor.from_source') }}</span>
            <span class="pe-row-hint">{{ $t('access.editor.from_source_hint') }}</span>
          </span>
          <span class="pe-switch"><input type="checkbox" v-model="model.from_source" data-testid="perm-from-source" /><span class="pe-slider"></span></span>
        </label>
      </template>

      <!-- Команда -->
      <template v-else-if="active === 'team'">
        <div class="pe-scope" role="radiogroup">
          <label class="pe-scope-option" :class="{ active: model.all_team }">
            <input type="radio" :checked="model.all_team" data-testid="perm-all-team" @change="model.all_team = true" />
            <span class="pe-scope-title">{{ $t('access.editor.all_team') }}</span>
            <span class="pe-scope-hint">{{ $t('access.editor.all_team_hint') }}</span>
          </label>
          <label class="pe-scope-option" :class="{ active: !model.all_team }">
            <input type="radio" :checked="!model.all_team" data-testid="perm-some-team" @change="model.all_team = false" />
            <span class="pe-scope-title">{{ $t('access.editor.some_team') }}</span>
            <span class="pe-scope-hint">{{ $t('access.editor.selected', { n: model.team_ids.length }) }}</span>
          </label>
        </div>

        <template v-if="!model.all_team">
          <div class="adm-pick-toolbar">
            <input v-model="memberQuery" class="adm-pick-search" :placeholder="$t('access.editor.search_member')" />
            <button type="button" class="adm-link-btn" @click="model.team_ids = []">{{ $t('access.editor.clear') }}</button>
          </div>
          <div class="adm-pick-list pe-list">
            <label v-for="m in visibleMembers" :key="m.id" class="adm-pick-item" :class="{ selected: model.team_ids.includes(m.id) }">
              <input type="checkbox" :value="m.id" v-model="model.team_ids" />
              <span>{{ m.name }}</span>
            </label>
            <div v-if="!visibleMembers.length" class="adm-empty">{{ $t('access.editor.nothing_found') }}</div>
          </div>
        </template>

        <!-- Задачи без исполнителя — отдельное право -->
        <label class="pe-row">
          <span class="pe-row-text">
            <span class="pe-row-title">{{ $t('access.editor.show_unassigned') }}</span>
            <span class="pe-row-hint">{{ $t('access.editor.show_unassigned_hint') }}</span>
          </span>
          <span class="pe-switch"><input type="checkbox" v-model="model.show_unassigned" data-testid="perm-show-unassigned" /><span class="pe-slider"></span></span>
        </label>
      </template>

      <!-- Вкладки, виджеты, спринты, Kanban: «всё» либо выбранное из списка -->
      <template v-else-if="keys">
        <p v-if="keys.tab && !tabShown(keys.tab)" class="pe-warning" data-testid="perm-tab-hidden">
          {{ $t('access.editor.tab_hidden', { tab: $t('navigation.' + keys.tab) }) }}
        </p>

        <div class="pe-scope" role="radiogroup">
          <label class="pe-scope-option" :class="{ active: model[keys.field] === null }">
            <input type="radio" :checked="model[keys.field] === null" :data-testid="'perm-all-' + active" @change="setAll(true)" />
            <span class="pe-scope-title">{{ $t('access.editor.all.' + active) }}</span>
            <span class="pe-scope-hint">{{ $t('access.editor.all_hint') }}</span>
          </label>
          <label class="pe-scope-option" :class="{ active: model[keys.field] !== null }">
            <input type="radio" :checked="model[keys.field] !== null" :data-testid="'perm-some-' + active" @change="setAll(false)" />
            <span class="pe-scope-title">{{ $t('access.editor.some') }}</span>
            <span class="pe-scope-hint">{{ $t('access.editor.count', { n: (model[keys.field] || keys.all).length, total: keys.all.length }) }}</span>
          </label>
        </div>

        <div v-if="model[keys.field] !== null" class="pe-keys">
          <label v-for="key in keys.all" :key="key" class="pe-row compact" :class="{ locked: lastKey(key) }" :title="lastKey(key) ? $t('access.editor.last_one') : undefined">
            <span class="pe-row-text">
              <span class="pe-row-title">{{ $t(keys.label + key) }}</span>
              <span v-if="keys.hint" class="pe-row-hint">{{ $t(keys.hint + key) }}</span>
            </span>
            <span class="pe-switch">
              <input
                type="checkbox"
                :checked="model[keys.field]!.includes(key)"
                :disabled="lastKey(key)"
                :data-testid="'perm-' + active + '-' + key"
                @change="toggleKey(key, ($event.target as HTMLInputElement).checked)"
              />
              <span class="pe-slider"></span>
            </span>
          </label>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { SlidersHorizontal, FolderTree, Users, LayoutGrid, BarChart3, Flag, KanbanSquare } from 'lucide-vue-next'
import {
  TABS, WIDGETS, SPRINT_ACTIONS, KANBAN_MODES, projectTree, withDescendants, coveredByParents,
  type Permissions, type ProjectItem, type MemberItem,
} from '../../../utils/access'

// One editor for the rights of a user, a group and a role template. The
// parent owns the object (v-model) and decides when to save it.
const model = defineModel<Permissions>({ required: true })

const props = defineProps<{
  projects: ProjectItem[]
  members: MemberItem[]
}>()

const { t } = useI18n()

type SectionId = 'general' | 'projects' | 'team' | 'tabs' | 'widgets' | 'sprints' | 'kanban'
type KeyField = 'visible_tabs' | 'widgets' | 'sprint_actions' | 'kanban_modes'

// Разделы со списком ключей устроены одинаково. tab — вкладка, без которой
// раздел не действует; min — сколько ключей самое меньшее должно остаться.
const KEY_SECTIONS: Partial<Record<SectionId, { field: KeyField; all: string[]; label: string; hint?: string; tab?: string; min?: number }>> = {
  tabs: { field: 'visible_tabs', all: TABS, label: 'navigation.' },
  widgets: { field: 'widgets', all: WIDGETS, label: 'access.widgets.', tab: 'analytics' },
  sprints: { field: 'sprint_actions', all: SPRINT_ACTIONS, label: 'access.sprint.', tab: 'sprint' },
  // Без единого вида доски вкладка Kanban была бы пустой: один остаётся всегда
  kanban: { field: 'kanban_modes', all: KANBAN_MODES, label: 'access.kanban.', hint: 'access.kanban_hint.', tab: 'kanban', min: 1 },
}

const active = ref<SectionId>('general')
const keys = computed(() => KEY_SECTIONS[active.value])

function tabShown(tab: string): boolean {
  return model.value.visible_tabs === null || model.value.visible_tabs.includes(tab)
}

// Краткий итог списка ключей для меню: «все» или «3 из 5»
function keysStatus(field: KeyField, all: string[]): { status: string; limited: boolean } {
  const list = model.value[field]
  if (list === null) return { status: t('access.editor.status_all'), limited: false }
  return { status: t('access.editor.count', { n: list.length, total: all.length }), limited: true }
}

// Виджеты аналитики настраиваются, только пока сама вкладка «Аналитика» доступна
const sections = computed(() => {
  const m = model.value
  const list: { id: SectionId; icon: Component; status: string; limited: boolean }[] = [
    {
      id: 'general', icon: SlidersHorizontal, limited: m.read_only,
      status: t(m.read_only ? 'access.editor.status_read_only' : 'access.editor.status_write'),
    },
    {
      id: 'projects', icon: FolderTree, limited: !m.all_projects,
      status: m.all_projects ? t('access.editor.status_all') : t('access.editor.selected', { n: m.project_ids.length }),
    },
    {
      id: 'team', icon: Users, limited: !m.all_team,
      status: m.all_team ? t('access.editor.status_all') : t('access.editor.selected', { n: m.team_ids.length }),
    },
    { id: 'tabs', icon: LayoutGrid, ...keysStatus('visible_tabs', TABS) },
  ]
  if (tabShown('analytics')) list.push({ id: 'widgets', icon: BarChart3, ...keysStatus('widgets', WIDGETS) })
  list.push({ id: 'sprints', icon: Flag, ...keysStatus('sprint_actions', SPRINT_ACTIONS) })
  list.push({ id: 'kanban', icon: KanbanSquare, ...keysStatus('kanban_modes', KANBAN_MODES) })
  return list
})

// Открытый раздел исчез из меню (скрыли вкладку «Аналитика») — назад к вкладкам
watch(sections, list => {
  if (!list.some(s => s.id === active.value)) active.value = 'tabs'
})

// «Все» хранится как null; переход к выбранным начинается с полного списка
function setAll(all: boolean) {
  const section = keys.value
  if (section) model.value[section.field] = all ? null : [...section.all]
}

function toggleKey(key: string, checked: boolean) {
  const section = keys.value
  if (!section) return
  const current = new Set(model.value[section.field] || [])
  if (checked) current.add(key)
  else current.delete(key)
  // Порядок — как в полном списке
  model.value[section.field] = section.all.filter(k => current.has(k))
}

// Последний из обязательных ключей выключить нельзя
function lastKey(key: string): boolean {
  const section = keys.value
  const list = section ? model.value[section.field] : null
  return !!section?.min && !!list && list.length <= section.min && list.includes(key)
}

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
</script>

<style scoped>
.pe {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  min-height: 460px;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  overflow: hidden;
}

/* ─── Меню разделов ─── */
.pe-nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  background: var(--surface-2);
  border-right: 1px solid var(--hairline);
}

.pe-nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  text-align: left;
  color: var(--text-muted);
  border-radius: 8px;
  transition: background 0.15s, color 0.15s;
}

.pe-nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-bright);
}

.pe-nav-item.active {
  background: var(--surface-1);
  color: var(--text-bright);
  box-shadow: inset 2px 0 0 var(--accent);
}

.pe-nav-icon {
  flex-shrink: 0;
}

.pe-nav-item.active .pe-nav-icon {
  color: var(--accent);
}

.pe-nav-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.pe-nav-title {
  font-size: 13px;
  font-weight: 500;
}

.pe-nav-status {
  overflow: hidden;
  font-size: 11px;
  color: var(--text-faint);
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Раздел, в котором что-то ограничено, заметен в меню */
.pe-nav-status.limited {
  color: var(--warning);
}

/* ─── Содержимое раздела ─── */
.pe-pane {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding: 18px 20px;
}

.pe-head {
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-light);
}

.pe-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-bright);
}

.pe-desc {
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-muted);
}

/* Выбор «всё или выбранное»: две карточки */
.pe-scope {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.pe-scope-option {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 2px 8px;
  align-items: center;
  padding: 10px 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.pe-scope-option:hover {
  border-color: var(--text-faint);
}

.pe-scope-option.active {
  background: color-mix(in srgb, var(--accent) 10%, var(--surface-2));
  border-color: var(--accent);
}

.pe-scope-option input {
  width: 15px;
  height: 15px;
  accent-color: var(--accent);
}

.pe-scope-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-bright);
}

.pe-scope-hint {
  grid-column: 2;
  font-size: 11px;
  color: var(--text-muted);
}

/* Строка с переключателем */
.pe-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  cursor: pointer;
}

.pe-row.compact {
  padding: 9px 14px;
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--border-light);
  border-radius: 0;
}

.pe-row.compact:last-child {
  border-bottom: none;
}

.pe-row.locked {
  cursor: not-allowed;
}

.pe-row-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.pe-row-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-bright);
}

.pe-row-hint {
  font-size: 12px;
  line-height: 1.45;
  color: var(--text-muted);
}

.pe-keys {
  border: 1px solid var(--hairline);
  border-radius: 8px;
}

/* Переключатель */
.pe-switch {
  position: relative;
  flex-shrink: 0;
  width: 36px;
  height: 20px;
}

.pe-switch input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  opacity: 0;
  cursor: inherit;
}

.pe-slider {
  position: absolute;
  inset: 0;
  background: var(--surface-3);
  border: 1px solid var(--hairline);
  border-radius: 9999px;
  transition: background 0.15s, border-color 0.15s;
  pointer-events: none;
}

.pe-slider::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  background: var(--text-muted);
  border-radius: 50%;
  transition: transform 0.15s, background 0.15s;
}

.pe-switch input:checked + .pe-slider {
  background: var(--accent);
  border-color: var(--accent);
}

.pe-switch input:checked + .pe-slider::after {
  background: #fff;
  transform: translateX(16px);
}

.pe-switch input:focus-visible + .pe-slider {
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 40%, transparent);
}

.pe-switch input:disabled + .pe-slider {
  opacity: 0.55;
}

.pe-list {
  max-height: 240px;
}

.pe-parent {
  font-weight: 600;
}

.pe-note {
  font-size: 12px;
  color: var(--text-muted);
}

.pe-warning {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--warning);
  background: color-mix(in srgb, var(--warning) 12%, transparent);
  border-radius: 8px;
}

.adm-pick-item.inherited {
  cursor: default;
  color: var(--text-muted);
}

/* Узкий экран: разделы — лентой сверху */
@media (max-width: 768px) {
  .pe {
    grid-template-columns: 1fr;
  }
  .pe-nav {
    flex-direction: row;
    overflow-x: auto;
    border-right: none;
    border-bottom: 1px solid var(--hairline);
  }
  .pe-nav-item {
    flex-shrink: 0;
  }
  .pe-nav-item.active {
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  .pe-scope {
    grid-template-columns: 1fr;
  }
}
</style>

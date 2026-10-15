<template>
  <div class="task-accordion">
    <!-- Блок в рамке с компактной шапкой на каждый проект или исполнителя -->
    <template v-if="sortedGroups.length">
      <div v-for="group in sortedGroups" :key="group.name" class="assignee-group">
        <div
          class="assignee-header"
          @click="toggle(group.name)"
          :data-testid="'accordion-' + group.name"
        >
          <ChevronDown :size="18" class="chevron" :class="{ collapsed: !isOpen(group.name) }" />
          <Folder v-if="groupBy === 'project'" :size="18" class="person" />
          <span v-else-if="groupBy === 'status'" class="status-dot" :style="{ background: statusColor(group.name) }"></span>
          <span
            v-else-if="group.name !== NO_ASSIGNEE"
            class="avatar"
            :style="{ background: avatarColor(group.name) }"
          >{{ initials(group.name) }}</span>
          <User v-else :size="18" class="person" />
          <span class="assignee-name">{{ groupTitle(group.name) }}</span>
          <span class="count-badge">{{ group.task_count }}</span>
          <span class="group-metric">
            {{ $t('tasks.accordions.estimate') }}: <b class="estimate">{{ formatTotal(group.estimate_total) }}</b>
          </span>
          <span class="group-metric">
            {{ $t('tasks.accordions.fact') }}: <b class="fact" :class="{ over: group.fact_total > group.estimate_total && group.estimate_total > 0 }">{{ formatTotal(group.fact_total) }}</b>
          </span>
          <!-- Что в группе требует внимания -->
          <span class="group-flags">
            <span v-if="group.overdue" class="flag overdue">{{ $t('tasks.attention.group_overdue', { n: group.overdue }) }}</span>
            <span v-if="group.overrun" class="flag overrun">{{ $t('tasks.attention.group_overrun', { n: group.overrun }) }}</span>
            <span v-if="group.unassigned && groupBy !== 'assignee'" class="flag">{{ $t('tasks.attention.group_unassigned', { n: group.unassigned }) }}</span>
            <!-- Загрузка исполнителя: факт против оценки, перерасход оранжевым -->
            <span
              v-if="groupBy === 'assignee' && (group.estimate_total || group.fact_total)"
              class="load"
              :title="$t('tasks.attention.load', { fact: formatTotal(group.fact_total), estimate: formatTotal(group.estimate_total) })"
            >
              <i class="used" :style="{ width: loadUsedPct(group) + '%' }"></i>
              <i class="extra" :style="{ width: loadOverPct(group) + '%' }"></i>
            </span>
          </span>
        </div>
        <template v-if="isOpen(group.name)">
          <p v-if="group.loading" class="group-loading">{{ $t('common.loading') }}</p>
          <TaskTable
            v-else
            :tasks="group.tasks"
            flat
            @open-task="$emit('open-task', $event)"
            @sort-change="$emit('sort-change', $event)"
          />
          <AppPagination
            :total="group.task_count"
            :limit="store.pageSize"
            :offset="group.offset"
            @update:offset="store.setGroupPage(group.name, $event)"
            @update:limit="store.setPageSize"
          />
        </template>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Folder, User } from 'lucide-vue-next'
import { statusColor } from '../../utils/statusColor'
import { initials, avatarColor } from '../../utils/avatar'
import TaskTable from './TaskTable.vue'
import AppPagination from '../ui/AppPagination.vue'
import { useTasksStore, type TaskGroup } from '../../stores/tasks'

const props = defineProps<{ groups: TaskGroup[]; groupBy: string }>()
defineEmits(['open-task', 'sort-change'])

const { t } = useI18n()

// Имена групп, которые сервер даёт задачам без исполнителя и без статуса
const NO_ASSIGNEE = 'Без исполнителя'
const NO_STATUS = 'Без статуса'

function groupTitle(name: string): string {
  if (name === NO_ASSIGNEE) return t('tasks.accordions.no_assignee')
  if (name === NO_STATUS) return t('tasks.accordions.no_status')
  return name
}

// Статусы сервер отдаёт в рабочем порядке (открытые, в тестировании,
// закрытые) — его не меняем. Остальные группы: сначала те, где больше
// задач с проблемами, затем по алфавиту; «Без исполнителя» — первой
const sortedGroups = computed(() =>
  props.groupBy === 'status' ? props.groups : [...props.groups].sort((a, b) => {
    if (a.name === NO_ASSIGNEE) return -1
    if (b.name === NO_ASSIGNEE) return 1
    return problems(b) - problems(a) || a.name.localeCompare(b.name, 'ru')
  })
)

function problems(g: TaskGroup): number {
  return g.overdue + g.overrun + g.unassigned
}

// Полоса загрузки: доля факта до оценки и перерасход сверх неё
function loadUsedPct(g: TaskGroup): number {
  const max = Math.max(g.estimate_total, g.fact_total)
  return max ? Math.min(g.fact_total, g.estimate_total) / max * 100 : 0
}

function loadOverPct(g: TaskGroup): number {
  const max = Math.max(g.estimate_total, g.fact_total)
  return max && g.fact_total > g.estimate_total ? (g.fact_total - g.estimate_total) / max * 100 : 0
}

// По умолчанию группы свёрнуты; раскрытые хранятся в сторе и сохраняются
// вместе с фильтрами, поэтому переживают закрытие браузера.
const store = useTasksStore()

function toggle(name: string) {
  store.toggleGroup(name)
}

function isOpen(name: string): boolean {
  return store.isGroupExpanded(name)
}

function formatTotal(h: number): string {
  return `${(h || 0).toFixed(1)}${t('tasks.units.hours_short')}`
}
</script>

<style scoped>
.task-accordion {
  display: flex;
  flex-direction: column;
}

.chevron {
  flex-shrink: 0;
  color: var(--text-muted);
  transition: transform 0.2s;
}

.chevron.collapsed {
  transform: rotate(-90deg);
}

/* Группы */
.assignee-group {
  margin-bottom: 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}

.assignee-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: var(--bg-card);
  cursor: pointer;
  user-select: none;
  transition: background 0.1s;
}

.assignee-header:hover {
  background: var(--bg-hover);
}

.person {
  color: var(--accent);
}

.assignee-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 9999px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

.group-loading {
  padding: 12px 14px;
  font-size: 13px;
  color: var(--text-muted);
}

/* Итоги */
.group-metric {
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0;
  color: var(--text-muted);
}

.group-metric b {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.group-metric .estimate {
  color: var(--cyan);
}

.group-metric .fact {
  color: var(--success);
}

.group-metric .fact.over {
  color: var(--orange);
}

.status-dot {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.avatar {
  display: inline-grid;
  place-items: center;
  flex: none;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  color: #fff;
  font-size: 9px;
  font-weight: 600;
}

/* Метки проблем и загрузка — у правого края заголовка */
.group-flags {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}

.flag {
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--surface-3);
  color: var(--text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.flag.overdue {
  background: color-mix(in srgb, var(--danger) 15%, transparent);
  color: var(--danger);
}

.flag.overrun {
  background: color-mix(in srgb, var(--orange) 15%, transparent);
  color: var(--orange);
}

.load {
  display: flex;
  width: 110px;
  height: 6px;
  border-radius: 3px;
  background: var(--surface-3);
  overflow: hidden;
}

.load .used {
  background: var(--accent);
}

.load .extra {
  background: var(--orange);
}
</style>

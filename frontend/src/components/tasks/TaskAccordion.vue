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
          <CircleDot v-else-if="groupBy === 'status'" :size="18" class="person" />
          <User v-else :size="18" class="person" />
          <span class="assignee-name">{{ groupTitle(group.name) }}</span>
          <span class="count-badge">{{ group.task_count }}</span>
          <span class="group-metric">
            {{ $t('tasks.accordions.estimate') }}: <b class="estimate">{{ formatTotal(group.estimate_total) }}</b>
          </span>
          <span class="group-metric">
            {{ $t('tasks.accordions.fact') }}: <b class="fact">{{ formatTotal(group.fact_total) }}</b>
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
import { ChevronDown, Folder, User, CircleDot } from 'lucide-vue-next'
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
// закрытые) — его не меняем; остальные группы — по алфавиту
const sortedGroups = computed(() =>
  props.groupBy === 'status' ? props.groups : [...props.groups].sort((a, b) => {
    if (a.name === NO_ASSIGNEE) return 1
    if (b.name === NO_ASSIGNEE) return -1
    return a.name.localeCompare(b.name, 'ru')
  })
)

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
</style>

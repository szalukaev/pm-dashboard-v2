<template>
  <!-- Телефон: спринты карточками; нажатие открывает спринт на отдельном экране -->
  <div class="sprint-mobile" data-testid="sprint-mobile">
    <button
      v-for="sprint in openSprints"
      :key="sprint.id"
      class="s-card"
      :data-testid="'sprint-' + sprint.id"
      @click="openSprint(sprint.id)"
    >
      <span class="s-row">
        <span class="s-name">{{ sprint.name }}</span>
        <SprintStateBadge :sprint="sprint" />
      </span>
      <span class="s-bar">
        <i class="bar-closed" :style="{ width: share(sprint.closed_count, sprint) }"></i>
        <i class="bar-testing" :style="{ width: share(sprint.testing_count, sprint) }"></i>
      </span>
      <span class="s-row s-meta">
        <span>{{ dateRange(sprint) }}</span>
        <span class="s-counts"><b class="c-open">{{ sprint.open_count }}</b> / <b class="c-testing">{{ sprint.testing_count }}</b> / {{ sprint.task_count }}</span>
      </span>
      <span class="s-row s-meta">
        <span :class="{ 's-late': timeLeft(sprint).late }">{{ timeLeft(sprint).text }}</span>
        <span>{{ kindLabel(sprint) }}</span>
      </span>
    </button>

    <div v-if="sprints.length === 0 && backlogTotal === 0" class="s-empty" data-testid="sprint-no-data">
      {{ $t('sprint.no_data') }}
    </div>
    <div v-else-if="openSprints.length === 0" class="s-empty">{{ $t('sprint.no_open_sprints') }}</div>

    <button class="s-card s-backlog" data-testid="sprint-backlog-card" @click="openBacklog">
      <span class="s-row">
        <span class="s-name"><Inbox :size="16" /> {{ $t('sprint.backlog') }}</span>
        <span class="s-count">{{ backlogTotal }}</span>
      </span>
      <span class="s-meta">{{ $t('sprint.mobile.backlog_hint', { n: backlog.length }) }}</span>
    </button>

    <!-- Завершённые: свёрнуты, пока не раскроют -->
    <template v-if="closedSprints.length">
      <button class="s-section" @click="showClosed = !showClosed">
        <ChevronDown :size="16" class="s-chevron" :class="{ open: showClosed }" />
        {{ $t('sprint.closed_section') }}
        <span class="s-count">{{ closedSprints.length }}</span>
      </button>
      <template v-if="showClosed">
        <button
          v-for="sprint in closedSprints"
          :key="sprint.id"
          class="s-card s-closed"
          :data-testid="'sprint-' + sprint.id"
          @click="openSprint(sprint.id)"
        >
          <span class="s-row">
            <span class="s-name">{{ sprint.name }}</span>
            <SprintStateBadge :sprint="sprint" />
          </span>
          <span class="s-row s-meta">
            <span>{{ dateRange(sprint) }}</span>
            <span>{{ sprint.task_count }}</span>
          </span>
        </button>
      </template>
    </template>

    <!-- Экран спринта или бэклога -->
    <Teleport to="body">
      <Transition name="s-screen">
        <div v-if="screen" class="s-screen" data-testid="sprint-screen">
          <div class="s-screen-head">
            <div class="s-screen-top">
              <button class="s-back" :aria-label="$t('common.back')" @click="screen = null">
                <ChevronLeft :size="22" />
              </button>
              <b class="s-screen-title">{{ current ? current.name : $t('sprint.backlog') }}</b>
              <SprintStateBadge v-if="current" :sprint="current" />
            </div>

            <template v-if="current">
              <span class="s-bar">
                <i class="bar-closed" :style="{ width: share(current.closed_count, current) }"></i>
                <i class="bar-testing" :style="{ width: share(current.testing_count, current) }"></i>
              </span>
              <div class="s-row s-meta">
                <span :class="{ 's-late': timeLeft(current).late }">{{ timeLeft(current).text }}</span>
                <span class="s-counts"><b class="c-open">{{ current.open_count }}</b> / <b class="c-testing">{{ current.testing_count }}</b> / {{ current.task_count }}</span>
              </div>
              <div class="s-row s-meta">
                <span>{{ dateRange(current) }}</span>
                <span>{{ kindLabel(current) }}</span>
              </div>

              <!-- Каждое действие — отдельное право -->
              <div class="s-actions">
                <template v-if="current.status !== 'closed'">
                  <button v-if="auth.canSprint('close')" class="s-act primary" @click="act('complete')">
                    <CheckCircle2 :size="15" /> {{ $t('sprint.complete') }}
                  </button>
                  <button v-if="auth.canSprint('edit')" class="s-act" @click="act('edit')">
                    <Pencil :size="15" /> {{ $t('common.edit') }}
                  </button>
                  <button v-if="auth.canSprint('tasks')" class="s-act" :aria-label="$t('sprint.refresh_from_redmine')" @click="act('refresh')">
                    <RefreshCw :size="15" />
                  </button>
                </template>
                <button v-else-if="auth.canSprint('close')" class="s-act primary" data-testid="sprint-reopen" @click="act('reopen')">
                  <RotateCcw :size="15" /> {{ $t('sprint.reopen') }}
                </button>
                <button v-if="auth.canSprint('delete')" class="s-act danger" :aria-label="$t('common.delete')" @click="act('delete')">
                  <Trash2 :size="15" />
                </button>
              </div>
            </template>
          </div>

          <div class="s-screen-body">
            <!-- Спринт -->
            <template v-if="current">
              <p v-if="current.description" class="s-desc">{{ current.description }}</p>
              <KanbanCard
                v-for="task in current.tasks"
                :key="task.external_id"
                :card="sprintTaskToCard(task)"
                :menu="canMove && current.status !== 'closed'"
                :class="'group-' + task.status_group"
                @open-task="$emit('open-task', $event)"
                @menu="startMove(task.external_id, current.id)"
              />
              <div v-if="current.tasks.length === 0" class="s-empty">{{ $t('sprint.mobile.no_tasks') }}</div>
            </template>

            <!-- Бэклог по проектам -->
            <template v-else>
              <section v-for="group in backlog" :key="group.project_name" class="s-group">
                <div class="s-group-title">
                  <span>{{ group.project_name }}</span>
                  <span>{{ group.tasks.length }}</span>
                </div>
                <KanbanCard
                  v-for="task in group.tasks"
                  :key="task.external_id"
                  :card="sprintTaskToCard(task)"
                  :menu="canMove && openSprints.length > 0"
                  @open-task="$emit('open-task', $event)"
                  @menu="startMove(task.external_id, null)"
                />
              </section>
              <div v-if="backlog.length === 0" class="s-empty">{{ $t('sprint.no_tasks') }}</div>
            </template>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Шторка переноса задачи -->
    <Teleport to="body">
      <Transition name="s-sheet">
        <div v-if="moving" class="s-sheet-scrim" @click="moving = null">
          <div class="s-sheet" data-testid="sprint-move-sheet" @click.stop>
            <div class="s-grab"></div>
            <div class="s-sheet-title">{{ $t('sprint.mobile.move_to') }}</div>
            <button
              v-for="sprint in openSprints"
              :key="sprint.id"
              class="s-target"
              :class="{ current: sprint.id === moving.fromSprintId }"
              :disabled="sprint.id === moving.fromSprintId"
              :data-testid="'sprint-move-' + sprint.id"
              @click="moveTo(sprint.id)"
            >
              <Flag :size="15" />
              <span>{{ sprint.name }}</span>
              <small>{{ sprint.id === moving.fromSprintId ? $t('sprint.mobile.here_now') : sprint.task_count }}</small>
            </button>
            <button v-if="moving.fromSprintId" class="s-target" data-testid="sprint-move-backlog" @click="moveTo(null)">
              <Inbox :size="15" />
              <span>{{ $t('sprint.mobile.to_backlog') }}</span>
            </button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronLeft, CheckCircle2, Pencil, RefreshCw, RotateCcw, Trash2, Inbox, Flag } from 'lucide-vue-next'
import SprintStateBadge from './SprintStateBadge.vue'
import KanbanCard from '../kanban/KanbanCard.vue'
import { sprintTaskToCard } from './sprintCard'
import type { Sprint, BacklogGroup } from '../../stores/sprint'
import { useAuthStore } from '../../stores/auth'
import { formatDate as formatDay } from '../../utils/format'

const props = defineProps<{ sprints: Sprint[]; backlog: BacklogGroup[] }>()
const emit = defineEmits(['complete', 'reopen', 'edit', 'refresh', 'delete', 'open-task', 'assign-task', 'unassign-task'])

const { t } = useI18n()
const auth = useAuthStore()

const openSprints = computed(() => props.sprints.filter(s => s.status !== 'closed'))
const closedSprints = computed(() => props.sprints.filter(s => s.status === 'closed'))
const backlogTotal = computed(() => props.backlog.reduce((sum, g) => sum + g.tasks.length, 0))
const showClosed = ref(false)
const canMove = computed(() => auth.canSprint('tasks'))

// Открытый экран: номер спринта или 'backlog'
const screen = ref<number | 'backlog' | null>(null)
// Спринт берётся из свежих данных, поэтому экран обновляется вместе со списком
const current = computed(() => (typeof screen.value === 'number' ? props.sprints.find(s => s.id === screen.value) || null : null))

function openSprint(id: number) {
  screen.value = id
}

function openBacklog() {
  screen.value = 'backlog'
}

function act(action: 'complete' | 'reopen' | 'edit' | 'refresh' | 'delete') {
  if (!current.value) return
  emit(action, current.value.id)
  // Завершённый спринт закрывает свой экран; удалённый закрывается сам, когда
  // пропадёт из данных (удаление ещё нужно подтвердить)
  if (action === 'complete') screen.value = null
}

// Спринт, открытый на экране, удалили или он пропал после обновления
watch(current, value => {
  if (value === null && typeof screen.value === 'number') screen.value = null
})

const moving = ref<{ issueId: number; fromSprintId: number | null } | null>(null)

function startMove(issueId: number, fromSprintId: number | null) {
  moving.value = { issueId, fromSprintId }
}

function moveTo(sprintId: number | null) {
  if (!moving.value) return
  const { issueId, fromSprintId } = moving.value
  if (sprintId === null && fromSprintId) emit('unassign-task', { sprintId: fromSprintId, issueId })
  else if (sprintId !== null) emit('assign-task', { sprintId, issueId, fromSprintId })
  moving.value = null
}

// Доля задач для полосы готовности
function share(count: number, sprint: Sprint): string {
  return sprint.task_count ? (count / sprint.task_count) * 100 + '%' : '0%'
}

function dateRange(sprint: Sprint): string {
  if (!sprint.start_date && !sprint.due_date) return ''
  return `${sprint.start_date ? formatDay(sprint.start_date, '2-digit') : '…'} — ${sprint.due_date ? formatDay(sprint.due_date, '2-digit') : '…'}`
}

function kindLabel(sprint: Sprint): string {
  const kind = t(sprint.is_shared ? 'sprint.kind.shared' : 'sprint.kind.personal')
  return sprint.is_shared && !sprint.is_own && sprint.owner_name ? `${kind} · ${sprint.owner_name}` : kind
}

// Дни между сегодня и датой по местному времени
function daysUntil(date: string): number {
  const [y, m, d] = date.slice(0, 10).split('-').map(Number)
  const target = new Date(y, m - 1, d)
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return Math.round((target.getTime() - today.getTime()) / 86400000)
}

function timeLeft(sprint: Sprint): { text: string; late: boolean } {
  if (sprint.status === 'closed') return { text: t('sprint.state.closed'), late: false }
  if (sprint.start_date && daysUntil(sprint.start_date) > 0) {
    return { text: t('sprint.mobile.starts_in', { n: daysUntil(sprint.start_date) }), late: false }
  }
  if (!sprint.due_date) return { text: '', late: false }
  const left = daysUntil(sprint.due_date)
  if (left < 0) return { text: t('sprint.mobile.overdue_by', { n: -left }), late: true }
  if (left === 0) return { text: t('sprint.mobile.ends_today'), late: false }
  return { text: t('sprint.mobile.days_left', { n: left }), late: false }
}
</script>

<style scoped>
.sprint-mobile {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.s-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 13px 14px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.s-card:active {
  border-color: var(--accent);
}

.s-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.s-name {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 700;
  color: var(--text-bright);
}

.s-meta {
  font-size: 12px;
  color: var(--text-muted);
}

.s-counts b {
  font-weight: 700;
}

.c-open { color: var(--warning); }
.c-testing { color: var(--cyan); }

.s-late {
  color: var(--danger);
  font-weight: 600;
}

.s-bar {
  display: flex;
  height: 6px;
  overflow: hidden;
  border-radius: 3px;
  background: var(--surface-3);
}

.s-bar i {
  display: block;
  height: 100%;
}

.bar-closed { background: var(--success); }
.bar-testing { background: var(--cyan); }

.s-count {
  padding: 1px 8px;
  border-radius: 9999px;
  background: var(--accent-bg);
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
}

.s-backlog {
  border-style: dashed;
}

.s-closed {
  opacity: 0.8;
}

.s-section {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  padding: 8px 2px;
  border: 0;
  background: none;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  cursor: pointer;
}

.s-chevron {
  transform: rotate(-90deg);
  transition: transform 0.2s;
}

.s-chevron.open {
  transform: none;
}

.s-empty {
  padding: 16px;
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
}

/* Экран спринта: под окнами формы и задачи, которые открываются поверх него */
.s-screen {
  position: fixed;
  inset: 0;
  z-index: calc(var(--z-modal) - 1);
  display: flex;
  flex-direction: column;
  background: var(--bg);
}

.s-screen-head {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: calc(10px + env(safe-area-inset-top)) 14px 12px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border);
}

.s-screen-top {
  display: flex;
  align-items: center;
  gap: 6px;
}

.s-back {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  margin-left: -8px;
  border: 0;
  background: none;
  color: var(--text);
  cursor: pointer;
}

.s-screen-title {
  flex: 1;
  min-width: 0;
  font-size: 17px;
  color: var(--text-bright);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.s-actions {
  display: flex;
  gap: 6px;
  margin-top: 4px;
}

.s-act {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 40px;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg);
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.s-act.primary {
  flex: 1;
  border-color: var(--accent);
  background: var(--accent);
  color: #fff;
}

.s-act.danger {
  margin-left: auto;
  color: var(--danger);
}

.s-screen-body {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 12px calc(16px + env(safe-area-inset-bottom));
}

.s-desc {
  margin: 0 0 4px;
  font-size: 13px;
  color: var(--text-muted);
  white-space: pre-line;
}

.s-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.s-group-title {
  display: flex;
  justify-content: space-between;
  margin-top: 8px;
  font-size: 11.5px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.s-screen-enter-active,
.s-screen-leave-active {
  transition: transform 0.28s cubic-bezier(0.3, 0.8, 0.3, 1);
}

.s-screen-enter-from,
.s-screen-leave-to {
  transform: translateX(100%);
}

/* Шторка переноса */
.s-sheet-scrim {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: flex;
  align-items: flex-end;
  background: var(--modal-overlay);
}

.s-sheet {
  width: 100%;
  max-height: 80vh;
  overflow-y: auto;
  padding: 10px 16px calc(18px + env(safe-area-inset-bottom));
  background: var(--bg-card);
  border-radius: 20px 20px 0 0;
}

.s-grab {
  width: 40px;
  height: 5px;
  margin: 0 auto 12px;
  border-radius: 3px;
  background: var(--border);
}

.s-sheet-title {
  margin-bottom: 8px;
  font-size: 16px;
  font-weight: 700;
  color: var(--text-bright);
}

.s-target {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 13px 12px;
  border: 0;
  border-radius: 10px;
  background: none;
  color: var(--text-bright);
  font-size: 15px;
  text-align: left;
  cursor: pointer;
}

.s-target span {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.s-target small {
  font-size: 12px;
  color: var(--text-muted);
}

.s-target:active {
  background: var(--bg-hover);
}

.s-target.current {
  background: var(--surface-2);
  color: var(--text-muted);
  cursor: default;
}

.s-sheet-enter-active,
.s-sheet-leave-active {
  transition: opacity 0.25s;
}

.s-sheet-enter-active .s-sheet,
.s-sheet-leave-active .s-sheet {
  transition: transform 0.28s cubic-bezier(0.3, 0.8, 0.3, 1);
}

.s-sheet-enter-from,
.s-sheet-leave-to {
  opacity: 0;
}

.s-sheet-enter-from .s-sheet,
.s-sheet-leave-to .s-sheet {
  transform: translateY(100%);
}
</style>

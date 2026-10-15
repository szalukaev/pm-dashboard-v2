<template>
  <!-- Уведомления на весь экран (на телефоне сюда ведёт колокольчик) -->
  <div class="notifications-view" data-testid="notifications-page">
    <div class="n-toolbar">
      <div class="n-seg">
        <button :class="{ on: filter === 'all' }" data-testid="notifications-all" @click="filter = 'all'">{{ $t('notifications.filter_all') }}</button>
        <button :class="{ on: filter === 'unread' }" data-testid="notifications-unread" @click="filter = 'unread'">
          {{ $t('notifications.filter_unread') }}<template v-if="unread > 0"> · {{ unread }}</template>
        </button>
      </div>
      <button class="n-icon" :aria-label="$t('notifications.setup')" data-testid="notifications-settings" @click="router.push('/settings/notifications')">
        <SettingsIcon :size="20" />
      </button>
    </div>

    <div v-if="items.length > 0" class="n-readall">
      <button v-if="unread > 0" data-testid="notifications-read-all" @click="store.markAllRead()">{{ $t('notifications.read_all') }}</button>
      <button class="n-clear" data-testid="notifications-clear" @click="clearAll">{{ $t('notifications.clear') }}</button>
    </div>

    <AppLoader v-if="loading && items.length === 0" :text="$t('common.loader.notifications')" />

    <div v-else-if="visible.length === 0" class="n-empty">
      <Bell :size="40" />
      <div>{{ $t(filter === 'unread' ? 'notifications.empty_unread' : 'notifications.empty') }}</div>
    </div>

    <template v-else>
      <section v-for="group in groups" :key="group.key">
        <div class="n-group">{{ group.label }}</div>
        <!-- Под карточкой — зелёная подложка «Прочитано»: она видна, пока карточку смахивают влево -->
        <div v-for="n in group.items" :key="n.id" class="n-wrap">
          <div class="n-under"><Check :size="18" /> {{ $t('notifications.mark_read') }}</div>
          <div
            class="n-card"
            :class="{ unread: !n.read }"
            :style="swipe.id === n.id ? { transform: `translateX(${swipe.dx}px)`, transition: 'none' } : undefined"
            :data-testid="'notification-' + n.id"
            @pointerdown="startSwipe($event, n)"
            @pointermove="moveSwipe"
            @pointerup="endSwipe"
            @pointercancel="cancelSwipe"
            @click="onClick(n)"
          >
            <span class="n-kind" :class="notificationKind(n)">
              <component :is="KIND_ICONS[notificationKind(n)]" :size="18" />
            </span>
            <div class="n-text">
              <div class="n-title">{{ notificationTitle(n, t) }}</div>
              <div v-for="(line, index) in notificationLines(n, t)" :key="index" class="n-line">{{ line }}</div>
              <div class="n-time">{{ formatDateTime(n.created_at, { year: 'none', empty: '' }) }}</div>
            </div>
            <span v-if="!n.read" class="n-dot"></span>
          </div>
        </div>
      </section>
      <div ref="sentinel" class="n-sentinel"></div>
    </template>

    <TaskModal :task-id="taskId" @close="taskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Bell, Check, Settings as SettingsIcon, AlertCircle, Clock, UserPlus, PenLine, AlertTriangle } from 'lucide-vue-next'
import { useNotificationsStore, type AppNotification } from '../stores/notifications'
import { notificationTitle, notificationLines, notificationTarget, notificationKind, type NotificationKind } from '../utils/notificationText'
import { formatDateTime } from '../utils/format'
import { useSwal } from '../composables/useSwal'
import AppLoader from '../components/ui/AppLoader.vue'
import TaskModal from '../components/tasks/TaskModal.vue'

const store = useNotificationsStore()
const { items, unread, loading } = storeToRefs(store)
const router = useRouter()
const { t: translate } = useI18n()
const t = (key: string, params?: Record<string, unknown>): string => (params ? translate(key, params) : translate(key))

const KIND_ICONS: Record<NotificationKind, unknown> = {
  overdue: AlertCircle,
  soon: Clock,
  assigned: UserPlus,
  changed: PenLine,
  system: AlertTriangle,
}

const { confirm } = useSwal()

const filter = ref<'all' | 'unread'>('all')
const taskId = ref<number | null>(null)

const visible = computed(() => (filter.value === 'unread' ? items.value.filter(n => !n.read) : items.value))

// Группы по дню: сегодня, вчера, раньше
const groups = computed(() => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const yesterday = new Date(today.getTime() - 86_400_000)
  const result = [
    { key: 'today', label: t('notifications.group_today'), items: [] as AppNotification[] },
    { key: 'yesterday', label: t('notifications.group_yesterday'), items: [] as AppNotification[] },
    { key: 'earlier', label: t('notifications.group_earlier'), items: [] as AppNotification[] },
  ]
  for (const n of visible.value) {
    const at = new Date(n.created_at)
    result[at >= today ? 0 : at >= yesterday ? 1 : 2].items.push(n)
  }
  return result.filter(g => g.items.length > 0)
})

// Нажатие помечает прочитанным и ведёт к тому, о чём уведомление
function onClick(n: AppNotification) {
  if (swiped) return
  store.markRead(n.id)
  const target = notificationTarget(n)
  if (target.taskId) taskId.value = target.taskId
  else if (target.path) router.push(target.path)
}

// Очистка спрашивает подтверждение: вернуть уведомления нельзя
async function clearAll() {
  const answer = await confirm(t('notifications.clear_confirm'))
  if (answer.isConfirmed) store.clear()
}

// Смахивание влево больше чем на 80 px — прочитано
const swipe = reactive({ id: 0, x0: 0, dx: 0 })
let swiped = false

function startSwipe(e: PointerEvent, n: AppNotification) {
  swiped = false
  if (n.read) return
  swipe.id = n.id
  swipe.x0 = e.clientX
  swipe.dx = 0
}

function moveSwipe(e: PointerEvent) {
  if (!swipe.id) return
  swipe.dx = Math.min(0, e.clientX - swipe.x0)
  if (swipe.dx < -8) swiped = true
}

function endSwipe() {
  if (!swipe.id) return
  if (swipe.dx < -80) store.markRead(swipe.id)
  cancelSwipe()
}

function cancelSwipe() {
  swipe.id = 0
  swipe.dx = 0
}

// Старые уведомления догружаются, когда список долистан до конца
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

function observe() {
  observer?.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver(entries => {
    if (entries.some(e => e.isIntersecting)) store.fetchMore()
  })
  observer.observe(sentinel.value)
}

watch(() => groups.value.length > 0, () => nextTick(observe))

onMounted(() => {
  store.fetch()
  nextTick(observe)
})

onBeforeUnmount(() => observer?.disconnect())
</script>

<style scoped>
.notifications-view {
  max-width: 640px;
  margin: 0 auto;
}

.n-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.n-seg {
  flex: 1;
  display: flex;
  padding: 3px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 10px;
}

.n-seg button {
  flex: 1;
  padding: 8px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.n-seg button.on {
  background: var(--bg-hover);
  color: var(--text-bright);
}

.n-icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 10px;
  background: none;
  color: var(--text);
  cursor: pointer;
}

.n-readall {
  display: flex;
  justify-content: flex-end;
  gap: 16px;
  margin-bottom: 4px;
}

.n-readall button {
  padding: 4px 2px;
  border: 0;
  background: none;
  color: var(--accent);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.n-readall .n-clear {
  color: var(--danger);
}

.n-group {
  margin: 16px 4px 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.n-wrap {
  position: relative;
  margin-bottom: 8px;
  overflow: hidden;
  border-radius: 14px;
  background: var(--success);
}

.n-under {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  padding-right: 18px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
}

.n-card {
  position: relative;
  display: flex;
  gap: 12px;
  padding: 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  cursor: pointer;
  touch-action: pan-y;
  user-select: none;
  transition: transform 0.2s;
}

.n-card.unread {
  background: var(--bg-card-alt);
  border-color: var(--border-accent);
}

.n-kind {
  width: 36px;
  height: 36px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 10px;
}

.n-kind.overdue { background: color-mix(in srgb, var(--danger) 16%, transparent); color: var(--danger); }
.n-kind.soon { background: color-mix(in srgb, var(--warning) 16%, transparent); color: var(--warning); }
.n-kind.assigned { background: var(--accent-bg); color: var(--accent-hover); }
.n-kind.changed { background: color-mix(in srgb, var(--success) 16%, transparent); color: var(--success); }
.n-kind.system { background: var(--bg-hover); color: var(--text-muted); }

.n-text {
  flex: 1;
  min-width: 0;
  padding-right: 10px;
}

.n-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.n-line {
  margin-top: 2px;
  font-size: 13px;
  color: var(--text-muted);
  overflow-wrap: anywhere;
}

.n-time {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-faint);
}

.n-dot {
  position: absolute;
  top: 14px;
  right: 12px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent);
}

.n-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 60px 20px;
  color: var(--text-faint);
  text-align: center;
}

.n-sentinel {
  height: 1px;
}
</style>

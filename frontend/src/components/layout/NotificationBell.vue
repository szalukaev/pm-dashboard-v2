<template>
  <div ref="root" class="bell">
    <button
      type="button"
      class="bell-btn"
      :class="{ active: open }"
      :title="$t('notifications.title')"
      data-testid="notification-bell"
      @click="toggle"
    >
      <Bell :size="18" />
      <span v-if="unread > 0" class="bell-badge" data-testid="notification-count">{{ unread > 99 ? '99+' : unread }}</span>
    </button>

    <div v-if="open" class="bell-panel" data-testid="notification-panel">
      <div class="bell-head">
        <span class="bell-title">{{ $t('notifications.title') }}</span>
        <button v-if="unread > 0" type="button" class="bell-link" @click="store.markAllRead()">
          {{ $t('notifications.read_all') }}
        </button>
        <button type="button" class="bell-link" @click="openSettings">{{ $t('notifications.setup') }}</button>
      </div>

      <div v-if="items.length === 0" class="bell-empty">{{ $t('notifications.empty') }}</div>

      <ul v-else class="bell-list" @scroll="onScroll">
        <li
          v-for="n in items"
          :key="n.id"
          class="bell-item"
          :class="{ unread: !n.read }"
          :data-testid="'notification-' + n.id"
          @click="openItem(n)"
        >
          <span class="bell-dot"></span>
          <div class="bell-text">
            <div class="bell-item-title">{{ notificationTitle(n, t) }}</div>
            <div v-for="(line, index) in notificationLines(n, t)" :key="index" class="bell-line">{{ line }}</div>
            <div class="bell-time">{{ timeText(n.created_at) }}</div>
          </div>
        </li>
      </ul>
    </div>

    <!-- Задача из уведомления открывается здесь же, на любой вкладке -->
    <TaskModal :task-id="taskId" @close="taskId = null" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Bell } from 'lucide-vue-next'
import { useNotificationsStore } from '../../stores/notifications'
import type { AppNotification } from '../../stores/notifications'
import { useSettingsStore } from '../../stores/settings'
import { useWebSocket } from '../../composables/useWebSocket'
import { useSwal } from '../../composables/useSwal'
import { notificationTitle, notificationLines } from '../../utils/notificationText'
import TaskModal from '../tasks/TaskModal.vue'

const store = useNotificationsStore()
const settingsStore = useSettingsStore()
const { items, unread } = storeToRefs(store)
const { t: translate, locale } = useI18n()
// Переводчик в том виде, в каком его ждут функции текста уведомлений
const t = (key: string, params?: Record<string, unknown>): string => (params ? translate(key, params) : translate(key))
const router = useRouter()
const route = useRoute()
const { on, off } = useWebSocket()
const { toast } = useSwal()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const taskId = ref<number | null>(null)

function toggle() {
  open.value = !open.value
  if (open.value) store.fetch()
}

// Клик по уведомлению помечает его прочитанным и ведёт к тому, о чём оно
function openItem(n: AppNotification) {
  store.markRead(n.id)
  if (n.entity_type === 'issue' && n.entity_id) {
    open.value = false
    taskId.value = n.entity_id
    return
  }
  // Остальные уведомления ведут на свою вкладку
  const page = PAGES[n.entity_type]
  if (page) {
    open.value = false
    router.push(page)
  }
}

const PAGES: Record<string, string> = {
  contract: '/payments',
  invoice: '/payments',
  sprint: '/sprint',
  system: '/settings',
}

function openSettings() {
  open.value = false
  router.push('/settings')
}

// Старые уведомления догружаются, когда список долистан до конца
function onScroll(e: Event) {
  const el = e.target as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 40) store.fetchMore()
}

function timeText(iso: string): string {
  const date = new Date(iso)
  if (isNaN(date.getTime())) return ''
  return date.toLocaleString(locale.value === 'en' ? 'en-GB' : 'ru-RU', {
    day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit',
  })
}

// Новое уведомление приходит сразу; всплывающее сообщение показывается, если
// пользователь их не отключил
function onNotification(n: AppNotification | null) {
  if (!n || !n.id) return
  store.add(n)
  if (settingsStore.settings.notifications_enabled) {
    const lines = notificationLines(n, t)
    toast(notificationTitle(n, t) + (lines.length > 0 ? ': ' + lines[0] : ''), 'info')
  }
}

function onDocumentClick(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}

// Ссылка из письма: /tasks?task=<номер> открывает карточку задачи. Параметр
// сразу убирается из адреса, чтобы задача не открывалась заново при обновлении
// страницы.
watch(() => route.query.task, value => {
  const id = Number(typeof value === 'string' ? value : '')
  if (!Number.isInteger(id) || id <= 0) return
  taskId.value = id
  const rest = { ...route.query }
  delete rest.task
  router.replace({ path: route.path, query: rest })
}, { immediate: true })

onMounted(() => {
  settingsStore.ensureLoaded()
  store.fetch()
  on('notification', onNotification)
  document.addEventListener('click', onDocumentClick)
})

onBeforeUnmount(() => {
  off('notification', onNotification)
  document.removeEventListener('click', onDocumentClick)
})
</script>

<style scoped>
.bell {
  position: relative;
  display: inline-flex;
}

.bell-btn {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  color: var(--text-muted);
  cursor: pointer;
}

.bell-btn:hover,
.bell-btn.active {
  background: var(--surface-2);
  color: var(--text-bright);
}

.bell-badge {
  position: absolute;
  top: 0;
  right: -2px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  box-sizing: border-box;
  border-radius: 9999px;
  background: var(--danger);
  font-size: 10px;
  font-weight: 600;
  line-height: 16px;
  color: #fff;
  text-align: center;
}

.bell-panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: var(--z-dropdown);
  width: 380px;
  max-width: calc(100vw - 24px);
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  box-shadow: var(--shadow);
}

.bell-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--hairline);
}

.bell-title {
  flex: 1;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
}

.bell-link {
  padding: 0;
  border: none;
  background: none;
  font-size: 12px;
  color: var(--accent);
  cursor: pointer;
}

.bell-link:hover {
  text-decoration: underline;
}

.bell-empty {
  padding: 32px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
}

.bell-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 440px;
  overflow-y: auto;
}

.bell-item {
  display: flex;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-light);
  cursor: pointer;
}

.bell-item:last-child {
  border-bottom: none;
}

.bell-item:hover {
  background: var(--bg-hover);
}

.bell-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  margin-top: 5px;
  border-radius: 50%;
  background: transparent;
}

.bell-item.unread .bell-dot {
  background: var(--accent);
}

.bell-text {
  min-width: 0;
  flex: 1;
}

.bell-item-title {
  font-size: 13px;
  color: var(--text-dim);
}

.bell-item.unread .bell-item-title {
  font-weight: 600;
  color: var(--text-bright);
}

.bell-line {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.bell-time {
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-faint);
}
</style>

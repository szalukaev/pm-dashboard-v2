<template>
  <header class="app-header">
    <!-- Телефон: название раздела, колокольчик и аватар; вкладки — «островом» внизу -->
    <template v-if="isMobile">
      <span class="m-title">{{ currentTitle }}</span>
      <div class="m-actions">
        <NotificationBell />
        <button class="header-avatar" :aria-label="$t('navigation.account')" data-testid="account-menu" @click="sheetOpen = true">
          <img v-if="auth.user?.avatar" :src="auth.user.avatar" class="avatar-img" />
          <span v-else class="avatar-initials">{{ avatarInitials }}</span>
        </button>
      </div>

      <Teleport to="body">
        <nav class="m-island" data-testid="mobile-nav">
          <router-link
            v-for="tab in tabs"
            v-show="auth.tabVisible(tab.testId)"
            :key="tab.path"
            :to="tab.path"
            class="m-island-tab"
            :class="{ active: isActive(tab.path) }"
            :aria-label="$t(tab.label)"
            :data-testid="'nav-' + tab.testId"
            @click="pendingPath = tab.path"
          >
            <component :is="tab.icon" :size="20" />
            <span class="m-island-label">{{ $t(tab.label) }}</span>
          </router-link>
        </nav>

        <Transition name="m-sheet">
          <div v-if="sheetOpen" class="m-scrim" @click="sheetOpen = false"></div>
        </Transition>
        <Transition name="m-sheet-up">
          <div v-if="sheetOpen" class="m-sheet" data-testid="account-sheet">
            <div class="m-grab"></div>
            <div class="m-me">
              <div class="header-avatar m-me-avatar">
                <img v-if="auth.user?.avatar" :src="auth.user.avatar" class="avatar-img" />
                <span v-else class="avatar-initials">{{ avatarInitials }}</span>
              </div>
              <div class="m-me-text">
                <b>{{ auth.user?.display_name || auth.user?.username }}</b>
                <span v-if="auth.user?.email">{{ auth.user.email }}</span>
              </div>
            </div>
            <div class="m-row m-row-static">
              <RefreshCw :size="20" /> {{ $t('navigation.sync') }}
              <AppSyncStatus class="m-row-end" />
            </div>
            <button class="m-row" data-testid="theme-toggle" @click="toggleMode">
              <Moon v-if="currentMode === 'dark'" :size="20" />
              <Sun v-else :size="20" />
              {{ $t('common.toggle_theme') }}
            </button>
            <button class="m-row" data-testid="settings-button" @click="openSettings">
              <SettingsIcon :size="20" /> {{ $t('navigation.settings') }}
            </button>
            <div class="m-sep"></div>
            <button class="m-row m-danger" data-testid="logout-button" @click="handleLogout">
              <LogOut :size="20" /> {{ $t('auth.logout') }}
            </button>
          </div>
        </Transition>
      </Teleport>
    </template>

    <template v-else>
    <div class="header-logo">
      <img src="/favicon.svg" alt="" class="logo-icon" />
      <span class="logo-text">PM Dashboard</span>
    </div>

    <nav class="header-tabs">
      <router-link
        v-for="(tab, index) in tabs"
        v-show="auth.tabVisible(tab.testId)"
        :key="tab.path"
        :to="tab.path"
        class="tab"
        :class="{ active: isActive(tab.path) }"
        :data-testid="'nav-' + tab.testId"
        draggable="true"
        @dragstart="onTabDragStart($event, index)"
        @dragover.prevent="onTabDragOver($event, index)"
        @drop="onTabDrop($event, index)"
        @dragend="onTabDragEnd"
      >
        <GripVertical :size="10" class="tab-grip" />
        <component :is="tab.icon" :size="16" />
        <span class="tab-label">{{ $t(tab.label) }}</span>
      </router-link>
    </nav>

    <div class="header-actions">
      <AppSyncStatus />
      <NotificationBell />
      <button
        class="action-btn"
        @click="toggleMode"
        :title="$t('common.toggle_theme')"
        data-testid="theme-toggle"
      >
        <Moon v-if="currentMode === 'dark'" :size="18" />
        <Sun v-else :size="18" />
      </button>
      <button
        class="action-btn"
        @click="router.push('/settings')"
        :title="$t('navigation.settings')"
        data-testid="settings-button"
      >
        <SettingsIcon :size="18" />
      </button>
      <div class="header-avatar" @click="router.push('/settings')" :title="auth.user?.display_name || auth.user?.username">
        <img v-if="auth.user?.avatar" :src="auth.user.avatar" class="avatar-img" />
        <span v-else class="avatar-initials">{{ avatarInitials }}</span>
      </div>
      <button
        class="action-btn"
        @click="handleLogout"
        :title="$t('auth.logout')"
        data-testid="logout-button"
      >
        <LogOut :size="18" />
      </button>
    </div>
    </template>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  Users,
  Activity,
  ListChecks,
  CreditCard,
  Puzzle,
  Settings as SettingsIcon,
  Moon,
  Sun,
  LogOut,
  GripVertical,
  RefreshCw
} from 'lucide-vue-next'
import { useTheme } from '../../composables/useTheme'
import { useAuthStore } from '../../stores/auth'
import { useSettingsStore } from '../../stores/settings'
import AppSyncStatus from '../ui/AppSyncStatus.vue'
import NotificationBell from './NotificationBell.vue'
import { useMobileTasks } from '../../composables/useMobileTasks'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const route = useRoute()
const { currentMode, toggleMode } = useTheme()
const auth = useAuthStore()
const avatarInitials = computed(() => {
  const name = auth.user?.display_name || auth.user?.username || '?'
  return name.split(' ').map((w: string) => w[0]).join('').toUpperCase().slice(0, 2)
})

const tabs = ref([
  { path: '/tasks', icon: ListChecks, label: 'navigation.tasks', testId: 'tasks' },
  { path: '/analytics', icon: Activity, label: 'navigation.analytics', testId: 'analytics' },
  { path: '/kanban', icon: Puzzle, label: 'navigation.kanban', testId: 'kanban' },
  { path: '/sprint', icon: Users, label: 'navigation.sprint', testId: 'sprint' },
  { path: '/payments', icon: CreditCard, label: 'navigation.payments', testId: 'payments' },
])

const TAB_ORDER_KEY = 'pm-dashboard-tab-order'
const settingsStore = useSettingsStore()

let dragTabIndex = -1

function onTabDragStart(e: DragEvent, index: number) {
  dragTabIndex = index
  e.dataTransfer!.effectAllowed = 'move'
}

function onTabDragOver(e: DragEvent, index: number) {
  e.dataTransfer!.dropEffect = 'move'
}

function onTabDrop(e: DragEvent, index: number) {
  if (dragTabIndex === index) return
  const moved = tabs.value.splice(dragTabIndex, 1)[0]
  tabs.value.splice(index, 0, moved)
  dragTabIndex = -1
  // Порядок хранится в настройках пользователя на сервере и идёт за ним на
  // любое устройство; копия в браузере нужна, чтобы вкладки не прыгали при загрузке
  const order = tabs.value.map(t => t.path)
  localStorage.setItem(TAB_ORDER_KEY, JSON.stringify(order))
  settingsStore.updateSettings({ tab_order: order })
}

function onTabDragEnd() {
  dragTabIndex = -1
}

// Расставляет вкладки в сохранённом порядке; вкладки, которых в нём нет, идут в конце
function applyTabOrder(order: unknown) {
  if (!Array.isArray(order) || order.length === 0) return
  const ordered = tabs.value.filter(t => order.includes(t.path)).sort((a, b) => order.indexOf(a.path) - order.indexOf(b.path))
  const remaining = tabs.value.filter(t => !order.includes(t.path))
  tabs.value = [...ordered, ...remaining]
}

// Сразу — по копии в браузере, после загрузки настроек — по серверу
try {
  applyTabOrder(JSON.parse(localStorage.getItem(TAB_ORDER_KEY) || '[]'))
} catch {}

watch(() => settingsStore.settings.tab_order, order => {
  if (!Array.isArray(order) || order.length === 0) return
  applyTabOrder(order)
  localStorage.setItem(TAB_ORDER_KEY, JSON.stringify(order))
  // immediate: настройки могли загрузиться раньше шапки (повторный вход без перезагрузки)
}, { immediate: true })

const { isMobile } = useMobileTasks()
const { t } = useI18n()

// Вкладка, на которую только что нажали: подсвечивается сразу, не дожидаясь
// окончания перехода
const pendingPath = ref('')
const removeAfterEach = router.afterEach(() => { pendingPath.value = '' })
onUnmounted(removeAfterEach)

function isActive(path: string) {
  return (pendingPath.value || route.path) === path
}

// Название раздела в шапке телефона
const currentTitle = computed(() => {
  const tab = tabs.value.find(tab => tab.path === route.path)
  if (tab) return t(tab.label)
  if (route.path === '/settings') return t('navigation.settings')
  return 'PM Dashboard'
})

// Шторка пользователя на телефоне: синхронизация, тема, настройки, выход
const sheetOpen = ref(false)
watch(() => route.path, () => { sheetOpen.value = false })

function openSettings() {
  sheetOpen.value = false
  router.push('/settings')
}

async function handleLogout() {
  await auth.logout()
  // Полная загрузка страницы: данные вышедшего пользователя не остаются в
  // памяти вкладки
  window.location.assign('/login')
}
</script>

<style scoped>
.app-header {
  display: flex;
  align-items: center;
  padding: 0 24px;
  border-bottom: 1px solid var(--hairline);
  background: var(--bg);
  gap: 24px;
}

.header-logo {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.logo-icon {
  width: 28px;
  height: 28px;
}

.logo-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.3px;
}

.header-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
  overflow-x: auto;
}

.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  text-decoration: none;
  transition: all 0.15s;
  white-space: nowrap;
}

.tab:hover {
  background: var(--bg-hover);
  color: var(--text-bright);
}

.tab-grip {
  color: var(--text-faintest);
  cursor: grab;
  opacity: 0;
  transition: opacity 0.15s;
}

.tab:hover .tab-grip {
  opacity: 1;
}

.tab {
  border: 1px solid transparent;
}

.tab.active {
  background: var(--surface-2);
  border-color: var(--hairline);
  color: var(--text-bright);
  font-weight: 600;
}

.tab.active :deep(svg:not(.tab-grip)) {
  color: var(--accent);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  color: var(--text-muted);
  transition: all 0.15s;
}

.action-btn:hover {
  background: var(--bg-hover);
  color: var(--text-bright);
}

/* На планшетной ширине название уступает место вкладкам — иначе последняя
   вкладка обрезается */
@media (max-width: 1100px) {
  .logo-text {
    display: none;
  }

  .app-header {
    gap: 16px;
  }
}

/* На узком экране вкладки идут отдельной строкой под логотипом и кнопками:
   в одной строке с ними вкладкам остаётся около 20 px */
@media (max-width: 768px) {
  .app-header {
    flex-wrap: wrap;
    padding: 0 12px;
    column-gap: 12px;
    row-gap: 0;
  }

  .header-logo,
  .header-actions {
    height: 52px;
  }

  .header-actions {
    margin-left: auto;
  }

  .header-tabs {
    order: 3;
    flex: 0 0 100%;
    padding-bottom: 8px;
  }

  .tab {
    flex: 1 0 auto;
    justify-content: center;
  }

  /* Перетаскивание вкладок на сенсорном экране не работает */
  .tab-grip {
    display: none;
  }
}

/* На телефоне подпись стоит под значком, все вкладки помещаются без прокрутки */
@media (max-width: 640px) {
  .tab {
    flex: 1 1 0;
    min-width: 0;
    flex-direction: column;
    gap: 2px;
    padding: 6px 2px;
    font-size: 10px;
  }

  .tab-label {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>

<style scoped>
.header-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  cursor: pointer;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: border-color 0.15s;
}

.header-avatar:hover {
  border-color: var(--accent);
}

.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-initials {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
}

.header-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  cursor: pointer;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: border-color 0.15s;
}
.header-avatar:hover {
  border-color: var(--accent);
}
.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-initials {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
}
/* ── Телефон: шапка, «остров» вкладок и шторка пользователя ── */
.m-title {
  flex: 1;
  min-width: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.3px;
  color: var(--text-bright);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.m-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 52px;
}

button.header-avatar {
  padding: 0;
}

.m-island {
  position: fixed;
  left: 50%;
  bottom: calc(16px + env(safe-area-inset-bottom));
  transform: translateX(-50%);
  z-index: var(--z-dropdown, 100);
  display: flex;
  gap: 2px;
  padding: 6px;
  max-width: calc(100vw - 24px);
  background: color-mix(in srgb, var(--surface-2) 92%, transparent);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border: 1px solid var(--hairline-strong);
  border-radius: 9999px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.45);
}

.m-island-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 44px;
  min-width: 44px;
  justify-content: center;
  padding: 0 12px;
  border-radius: 9999px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  text-decoration: none;
  -webkit-tap-highlight-color: transparent;
  transition: background 0.15s, color 0.15s, padding 0.15s;
}

/* Подпись — только у активной вкладки */
.m-island-label {
  display: none;
}

.m-island-tab.active {
  padding: 0 16px;
  background: var(--accent);
  color: #fff;
}

.m-island-tab.active .m-island-label {
  display: inline;
}

.m-scrim {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal, 1000);
  background: rgba(0, 0, 0, 0.6);
}

.m-sheet {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: calc(var(--z-modal, 1000) + 1);
  padding: 8px 14px calc(20px + env(safe-area-inset-bottom));
  background: var(--bg-card);
  border-top: 1px solid var(--border);
  border-radius: 20px 20px 0 0;
}

.m-grab {
  width: 36px;
  height: 4px;
  margin: 0 auto 12px;
  border-radius: 2px;
  background: var(--hairline-strong);
}

.m-me {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 4px 14px;
  margin-bottom: 6px;
  border-bottom: 1px solid var(--border);
}

.m-me-avatar {
  width: 40px;
  height: 40px;
  cursor: default;
}

.m-me-text {
  min-width: 0;
}

.m-me-text b {
  display: block;
  color: var(--text-bright);
}

.m-me-text span {
  display: block;
  font-size: 12px;
  color: var(--text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
}

.m-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 12px 6px;
  border: 0;
  border-radius: 10px;
  background: none;
  color: var(--text);
  font-size: 15px;
  text-align: left;
  cursor: pointer;
}

.m-row:active {
  background: var(--bg-hover);
}

.m-row-static {
  cursor: default;
}

.m-row-end {
  margin-left: auto;
}

.m-danger {
  color: var(--danger);
}

.m-sep {
  height: 1px;
  margin: 6px 0;
  background: var(--border);
}

.m-sheet-enter-active,
.m-sheet-leave-active {
  transition: opacity 0.2s;
}

.m-sheet-enter-from,
.m-sheet-leave-to {
  opacity: 0;
}

.m-sheet-up-enter-active,
.m-sheet-up-leave-active {
  transition: transform 0.25s cubic-bezier(0.3, 0.8, 0.3, 1);
}

.m-sheet-up-enter-from,
.m-sheet-up-leave-to {
  transform: translateY(100%);
}

@media (max-width: 768px) {
  .app-header {
    padding: 0 16px;
  }
}
</style>

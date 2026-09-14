<template>
  <header class="app-header">
    <div class="header-logo">
      <img src="/favicon.svg" alt="" class="logo-icon" />
      <span class="logo-text">PM Dashboard</span>
    </div>

    <nav class="header-tabs">
      <router-link
        v-for="(tab, index) in tabs"
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
      <button
        class="action-btn"
        @click="toggleTheme"
        :title="$t('common.toggle_theme')"
        data-testid="theme-toggle"
      >
        <Moon v-if="currentTheme === 'dark'" :size="18" />
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
      <button
        class="action-btn"
        @click="handleLogout"
        :title="$t('auth.logout')"
        data-testid="logout-button"
      >
        <LogOut :size="18" />
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref } from 'vue'
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
  GripVertical
} from 'lucide-vue-next'
import { useTheme } from '../../composables/useTheme'
import { useAuthStore } from '../../stores/auth'
import AppSyncStatus from '../ui/AppSyncStatus.vue'

const router = useRouter()
const route = useRoute()
const { currentTheme, setTheme } = useTheme()
const auth = useAuthStore()

const tabs = ref([
  { path: '/tasks', icon: ListChecks, label: 'navigation.tasks', testId: 'tasks' },
  { path: '/analytics', icon: Activity, label: 'navigation.analytics', testId: 'analytics' },
  { path: '/kanban', icon: Puzzle, label: 'navigation.kanban', testId: 'kanban' },
  { path: '/sprint', icon: Users, label: 'navigation.sprint', testId: 'sprint' },
  { path: '/payments', icon: CreditCard, label: 'navigation.payments', testId: 'payments' },
])

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
  // Save tab order
  const order = tabs.value.map(t => t.path)
  localStorage.setItem('pm-dashboard-tab-order', JSON.stringify(order))
}

function onTabDragEnd() {
  dragTabIndex = -1
}

// Restore tab order from localStorage
const savedOrder = localStorage.getItem('pm-dashboard-tab-order')
if (savedOrder) {
  try {
    const order = JSON.parse(savedOrder)
    const ordered = order.map((path: string) => tabs.value.find(t => t.path === path)).filter(Boolean)
    const remaining = tabs.value.filter(t => !order.includes(t.path))
    tabs.value = [...ordered, ...remaining]
  } catch {}
}

function isActive(path: string) {
  return route.path === path
}

function toggleTheme() {
  const themes = ['dark', 'light', 'nord', 'amber', 'forest', 'dusk']
  const idx = themes.indexOf(currentTheme.value)
  const next = themes[(idx + 1) % themes.length]
  setTheme(next)
}

function handleLogout() {
  auth.logout()
  router.push('/login')
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

.tab.active {
  background: var(--surface-2);
  color: var(--text-bright);
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

@media (max-width: 768px) {
  .tab-label {
    display: none;
  }

  .logo-text {
    display: none;
  }

  .app-header {
    padding: 0 12px;
    gap: 12px;
  }
}
</style>

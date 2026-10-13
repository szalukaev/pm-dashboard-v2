<template>
  <div class="settings-view">
    <!-- Настроенная система — надпись справа от заголовка; иначе чек-лист во всю строку под ним -->
    <div class="settings-head">
      <h1 class="page-title">{{ $t('settings.title') }}</h1>
      <SetupChecklist v-if="isAdmin" ref="checklist" @open="openStep" />
    </div>

    <div class="settings-layout">
      <!-- Меню слева -->
      <nav class="settings-menu">
        <button
          v-for="tab in visibleTabs"
          :key="tab.key"
          class="menu-item"
          :class="{ active: activeTab === tab.key }"
          @click="activeTab = tab.key"
          :data-testid="'settings-tab-' + tab.key"
        >
          <component :is="tab.icon" :size="16" />
          <span>{{ $t(tab.label) }}</span>
        </button>
      </nav>

      <!-- Содержимое справа -->
      <div class="settings-content">
        <SettingsPersonal v-if="activeTab === 'personal'" />
        <SettingsData v-if="activeTab === 'data'" />
        <SettingsInterface v-if="activeTab === 'interface'" />
        <SettingsNotifications v-if="activeTab === 'notifications'" />
        <SettingsConnections v-if="isAdmin && activeTab === 'connections'" />
        <SettingsSync v-if="isAdmin && activeTab === 'sync'" />
        <SettingsAudit v-if="isAdmin && activeTab === 'audit'" />
        <SettingsAdmin v-if="isAdmin && activeTab === 'admin'" :tab="adminTab" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { User, Database, Palette, Bell, Shield, Link, RefreshCw, ScrollText } from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth'
import SettingsPersonal from '../components/settings/SettingsPersonal.vue'
import SettingsData from '../components/settings/SettingsData.vue'
import SettingsInterface from '../components/settings/SettingsInterface.vue'
import SettingsNotifications from '../components/settings/SettingsNotifications.vue'
import SettingsConnections from '../components/settings/SettingsConnections.vue'
import SettingsSync from '../components/settings/SettingsSync.vue'
import SettingsAudit from '../components/settings/SettingsAudit.vue'
import SettingsAdmin from '../components/settings/SettingsAdmin.vue'
import SetupChecklist from '../components/settings/SetupChecklist.vue'

const activeTab = ref('personal')
// Вкладка внутри «Администрирования», на которую ведёт пункт чек-листа
const adminTab = ref('')
const checklist = ref<{ reload: () => void } | null>(null)

async function openStep(target: { tab: string; sub?: string }) {
  // Сначала сброс: повторный клик по тому же пункту тоже должен сработать
  adminTab.value = ''
  await nextTick()
  adminTab.value = target.sub || ''
  activeTab.value = target.tab
}

// Шаги выполняются на других вкладках: при переходе чек-лист перечитывается
watch(activeTab, () => checklist.value?.reload())

const tabs = [
  { key: 'personal', icon: User, label: 'settings.tabs.personal' },
  { key: 'data', icon: Database, label: 'settings.tabs.data' },
  { key: 'interface', icon: Palette, label: 'settings.tabs.interface' },
  { key: 'notifications', icon: Bell, label: 'settings.tabs.notifications' },
  { key: 'connections', icon: Link, label: 'settings.tabs.connections', adminOnly: true },
  { key: 'sync', icon: RefreshCw, label: 'settings.tabs.sync', adminOnly: true },
  { key: 'audit', icon: ScrollText, label: 'settings.tabs.audit', adminOnly: true },
  { key: 'admin', icon: Shield, label: 'settings.tabs.admin', adminOnly: true },
]

const auth = useAuthStore()
const isAdmin = computed(() => auth.user?.role === 'admin')
const visibleTabs = computed(() => tabs.filter(tab => !tab.adminOnly || isAdmin.value))
</script>

<style scoped>
.settings-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
}

.settings-layout {
  display: grid;
  grid-template-columns: 240px 1fr;
  gap: 24px;
  min-height: 400px;
}

.settings-menu {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  border-radius: 8px;
  transition: all 0.15s;
  text-align: left;
}

.menu-item:hover {
  background: var(--bg-hover);
  color: var(--text-bright);
}

.menu-item.active {
  background: var(--surface-2);
  color: var(--text-bright);
}

.settings-content {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 24px;
  /* Без этого широкая таблица растягивает блок за край окна вместо того,
     чтобы прокручиваться внутри себя */
  min-width: 0;
}

@media (max-width: 768px) {
  .settings-layout {
    /* Не шире окна: колонка «1fr» растягивается по самому широкому блоку */
    grid-template-columns: minmax(0, 1fr);
  }
  .settings-menu {
    flex-direction: row;
    overflow-x: auto;
  }
  /* Пункты меню в строке не сжимаются и не переносят подпись */
  .menu-item {
    flex-shrink: 0;
    white-space: nowrap;
  }
}
</style>

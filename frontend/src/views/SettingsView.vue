<template>
  <div class="settings-view">
    <h1 class="page-title">{{ $t('settings.title') }}</h1>

    <div class="settings-layout">
      <!-- Left menu -->
      <nav class="settings-menu">
        <button
          v-for="tab in tabs"
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

      <!-- Right content -->
      <div class="settings-content">
        <SettingsPersonal v-if="activeTab === 'personal'" />
        <SettingsData v-if="activeTab === 'data'" />
        <SettingsInterface v-if="activeTab === 'interface'" />
        <SettingsNotifications v-if="activeTab === 'notifications'" />
        <SettingsConnections v-if="activeTab === 'connections'" />
        <SettingsSync v-if="activeTab === 'sync'" />
        <SettingsAudit v-if="activeTab === 'audit'" />
        <SettingsAdmin v-if="activeTab === 'admin'" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { User, Database, Palette, Bell, Shield, Link, RefreshCw, ScrollText } from 'lucide-vue-next'
import SettingsPersonal from '../components/settings/SettingsPersonal.vue'
import SettingsData from '../components/settings/SettingsData.vue'
import SettingsInterface from '../components/settings/SettingsInterface.vue'
import SettingsNotifications from '../components/settings/SettingsNotifications.vue'
import SettingsConnections from '../components/settings/SettingsConnections.vue'
import SettingsSync from '../components/settings/SettingsSync.vue'
import SettingsAudit from '../components/settings/SettingsAudit.vue'
import SettingsAdmin from '../components/settings/SettingsAdmin.vue'

const activeTab = ref('personal')

const tabs = [
  { key: 'personal', icon: User, label: 'settings.tabs.personal' },
  { key: 'data', icon: Database, label: 'settings.tabs.data' },
  { key: 'interface', icon: Palette, label: 'settings.tabs.interface' },
  { key: 'notifications', icon: Bell, label: 'settings.tabs.notifications' },
  { key: 'connections', icon: Link, label: 'settings.tabs.connections' },
  { key: 'sync', icon: RefreshCw, label: 'settings.tabs.sync' },
  { key: 'audit', icon: ScrollText, label: 'settings.tabs.audit' },
  { key: 'admin', icon: Shield, label: 'settings.tabs.admin' },
]
</script>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
  margin-bottom: 24px;
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
}

@media (max-width: 768px) {
  .settings-layout {
    grid-template-columns: 1fr;
  }
  .settings-menu {
    flex-direction: row;
    overflow-x: auto;
  }
}
</style>

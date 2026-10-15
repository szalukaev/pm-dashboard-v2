<template>
  <!-- Телефон, без выбранного раздела: список разделов, каждый открывается своим экраном -->
  <div v-if="isMobile && !section" class="settings-mobile" data-testid="settings-list">
    <router-link to="/profile" class="m-me" data-testid="settings-profile">
      <span class="m-me-avatar">
        <img v-if="auth.user?.avatar" :src="auth.user.avatar" alt="" />
        <template v-else>{{ initials }}</template>
      </span>
      <span class="m-me-text">
        <b>{{ auth.user?.display_name || auth.user?.username }}</b>
        <span>{{ $t('settings.mobile.profile_hint') }}</span>
      </span>
      <ChevronRight :size="18" class="m-chev" />
    </router-link>

    <SetupChecklist v-if="isAdmin" ref="checklist" class="m-checklist" @open="openStep" />

    <div class="m-label">{{ $t('settings.mobile.mine') }}</div>
    <div class="m-group">
      <router-link v-for="tab in mineTabs" :key="tab.key" :to="'/settings/' + tab.key" class="m-row" :data-testid="'settings-tab-' + tab.key">
        <i class="m-ic" :class="'m-ic-' + tab.key"><component :is="tab.icon" :size="17" /></i>
        <span class="m-row-label">{{ $t(tab.label) }}</span>
        <span v-if="tab.key === 'interface'" class="m-row-value">{{ themeValue }}</span>
        <ChevronRight :size="18" class="m-chev" />
      </router-link>
    </div>

    <template v-if="isAdmin">
      <div class="m-label">{{ $t('settings.tabs.admin') }}</div>
      <div class="m-group">
        <router-link v-for="tab in adminTabs" :key="tab.key" :to="'/settings/' + tab.key" class="m-row" :data-testid="'settings-tab-' + tab.key">
          <i class="m-ic" :class="'m-ic-' + tab.key"><component :is="tab.icon" :size="17" /></i>
          <span class="m-row-label">{{ $t(tab.label) }}</span>
          <ChevronRight :size="18" class="m-chev" />
        </router-link>
      </div>
    </template>
  </div>

  <!-- Телефон, раздел открыт: только его содержимое; название — в шапке -->
  <div v-else-if="isMobile" class="settings-mobile-section">
    <component :is="sectionComponent" v-if="sectionComponent" v-bind="activeTab === 'admin' ? { tab: adminTab } : {}" />
  </div>

  <div v-else class="settings-view">
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
          @click="setTab(tab.key)"
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
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { User, Database, Palette, Bell, Shield, Link, RefreshCw, ScrollText, ChevronRight } from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth'
import { useMobileTasks } from '../composables/useMobileTasks'
import { useTheme, THEMES } from '../composables/useTheme'
import SettingsPersonal from '../components/settings/SettingsPersonal.vue'
import SettingsData from '../components/settings/SettingsData.vue'
import SettingsInterface from '../components/settings/SettingsInterface.vue'
import SettingsNotifications from '../components/settings/SettingsNotifications.vue'
import SettingsConnections from '../components/settings/SettingsConnections.vue'
import SettingsSync from '../components/settings/SettingsSync.vue'
import SettingsAudit from '../components/settings/SettingsAudit.vue'
import SettingsAdmin from '../components/settings/SettingsAdmin.vue'
import SetupChecklist from '../components/settings/SetupChecklist.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { isMobile } = useMobileTasks()
const { currentTheme, modePreference } = useTheme()

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

// Раздел хранится в адресе: /settings/<раздел>. Без раздела на компьютере
// открываются «Личные данные», на телефоне — список разделов
const section = computed(() => {
  const key = String(route.params.section || '')
  return visibleTabs.value.some(tab => tab.key === key) ? key : ''
})
const activeTab = computed(() => section.value || 'personal')

function setTab(key: string) {
  router.replace('/settings/' + key)
}

// На телефоне личные данные — это экран профиля
watch([isMobile, section], ([mobile, key]) => {
  if (mobile && key === 'personal') router.replace('/profile')
}, { immediate: true })

// На телефоне: «Личные данные» заменяет карточка профиля сверху
const mineTabs = computed(() => visibleTabs.value.filter(tab => !tab.adminOnly && tab.key !== 'personal'))
const adminTabs = computed(() => visibleTabs.value.filter(tab => tab.adminOnly))

const COMPONENTS: Record<string, unknown> = {
  data: SettingsData,
  interface: SettingsInterface,
  notifications: SettingsNotifications,
  connections: SettingsConnections,
  sync: SettingsSync,
  audit: SettingsAudit,
  admin: SettingsAdmin,
}
const sectionComponent = computed(() => COMPONENTS[section.value])

const initials = computed(() => {
  const n = auth.user?.display_name || auth.user?.username || '?'
  return n.split(' ').map((w: string) => w[0]).join('').toUpperCase().slice(0, 2)
})

// Текущая тема справа от «Интерфейса», например «Nord, системная»
const themeValue = computed(() => {
  const name = THEMES.find(item => item.id === currentTheme.value)?.name || ''
  return name + ', ' + t('settings.interface.mode_' + modePreference.value).toLowerCase()
})

// Вкладка внутри «Администрирования», на которую ведёт пункт чек-листа
const adminTab = ref('')
const checklist = ref<{ reload: () => void } | null>(null)

async function openStep(target: { tab: string; sub?: string }) {
  // Сначала сброс: повторный клик по тому же пункту тоже должен сработать
  adminTab.value = ''
  await nextTick()
  adminTab.value = target.sub || ''
  setTab(target.tab)
}

// Шаги выполняются на других вкладках: при переходе чек-лист перечитывается
watch(activeTab, () => checklist.value?.reload())
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
/* ── Телефон: список разделов ── */
.m-me {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  text-decoration: none;
  color: var(--text);
}

.m-me-avatar {
  width: 46px;
  height: 46px;
  flex: none;
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--accent), var(--accent-hover));
  color: #fff;
  font-size: 16px;
  font-weight: 600;
}

.m-me-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.m-me-text {
  flex: 1;
  min-width: 0;
}

.m-me-text b {
  display: block;
  color: var(--text-bright);
}

.m-me-text span {
  font-size: 12px;
  color: var(--text-faint);
}

.m-checklist {
  margin-top: 12px;
}

.m-label {
  margin: 20px 6px 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.m-group {
  overflow: hidden;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.m-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 52px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-light);
  color: var(--text);
  font-size: 15px;
  text-decoration: none;
}

.m-row:last-child {
  border-bottom: 0;
}

.m-row:active {
  background: var(--bg-hover);
}

.m-ic {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
}

.m-ic-notifications { background: var(--danger); }
.m-ic-interface { background: var(--accent-hover); }
.m-ic-data { background: var(--hairline-strong); }
.m-ic-sync { background: var(--success); }
.m-ic-audit { background: var(--warning); }

.m-row-label {
  flex: 1;
}

.m-row-value {
  max-width: 50%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--text-faint);
}

.m-chev {
  flex: none;
  color: var(--text-faint);
}

/* Раздел на телефоне: та же форма, что на компьютере, в одну колонку */
.settings-mobile-section {
  min-width: 0;
  padding: 16px;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 14px;
}

.settings-mobile-section :deep(.section-title) {
  display: none;
}
</style>


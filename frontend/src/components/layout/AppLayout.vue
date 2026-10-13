<template>
  <div class="app-layout" :class="{ 'with-banner': showBanner }">
    <AppHeader />

    <!-- Проблема с лицензией показывается каждому пользователю, а не только администраторам -->
    <div v-if="showBanner" class="license-banner" :class="license.state" data-testid="license-banner">
      <AlertTriangle :size="16" />
      <span>{{ bannerText }}</span>
      <router-link v-if="auth.isAdmin" to="/license" class="banner-link">{{ $t('license.banner.open') }}</router-link>
    </div>

    <main class="app-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { formatDateTime } from '../../utils/format'
import { useI18n } from 'vue-i18n'
import { AlertTriangle } from 'lucide-vue-next'
import AppHeader from './AppHeader.vue'
import { useAuthStore } from '../../stores/auth'
import { useLicenseStore } from '../../stores/license'

const { t } = useI18n()
const auth = useAuthStore()
const license = useLicenseStore()

const showBanner = computed(() => license.state === 'grace' || license.state === 'read_only')

const bannerText = computed(() => {
  if (license.state === 'read_only') return t('license.banner.read_only')
  const ends = formatDateTime(license.graceEndsAt, { empty: '' })
  return t('license.banner.grace', { time: ends })
})

// Состояние перечитывается, пока страница открыта: сервер проверяет
// лицензию каждый час
let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  license.fetchStatus()
  timer = setInterval(() => license.fetchStatus(true), 5 * 60_000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.app-layout {
  display: grid;
  /* Колонка не шире окна: иначе на узком экране шапка растягивает макет по
     своему содержимому и кнопки справа уезжают за край */
  grid-template-columns: minmax(0, 1fr);
  grid-template-rows: 56px 1fr;
  height: 100vh;
  overflow: hidden;
}

.app-layout.with-banner {
  grid-template-rows: 56px auto 1fr;
}

.license-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 24px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-bright);
  background: var(--surface-2);
  border-bottom: 1px solid var(--warning);
}

.license-banner svg {
  flex-shrink: 0;
  color: var(--warning);
}

.license-banner.read_only {
  border-bottom-color: var(--danger);
}

.license-banner.read_only svg {
  color: var(--danger);
}

.banner-link {
  margin-left: auto;
  color: var(--accent);
  white-space: nowrap;
}

.banner-link:hover {
  text-decoration: underline;
}

.app-content {
  overflow-y: auto;
  padding: 24px;
}

@media (max-width: 768px) {
  /* Шапка на узком экране в две строки — высоту задаёт её содержимое */
  .app-layout {
    grid-template-rows: auto 1fr;
  }

  .app-layout.with-banner {
    grid-template-rows: auto auto 1fr;
  }

  .app-content {
    padding: 16px;
  }

  .license-banner {
    padding: 8px 16px;
  }
}
</style>

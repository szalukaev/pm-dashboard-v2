import { createRouter, createWebHistory } from 'vue-router'
import axios from 'axios'
import i18n from '../i18n'

const APP_VERSION = '3.0.0'
// Вкладки, которые администратор может скрыть от пользователя (имена маршрутов)
const DATA_TABS = ['tasks', 'analytics', 'kanban', 'sprint', 'payments']
let versionWarningShown = false
// /api/setup/status не меняется, пока открыто приложение, — запрашивается один раз.
let setupStatusChecked = false
let setupIsComplete = false

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/LoginView.vue')
  },
  {
    path: '/setup',
    name: 'setup',
    component: () => import('../views/SetupWizardView.vue')
  },
  {
    path: '/',
    component: () => import('../components/layout/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/tasks' },
      { path: 'tasks', name: 'tasks', component: () => import('../views/TasksView.vue') },
      { path: 'analytics', name: 'analytics', component: () => import('../views/AnalyticsView.vue') },
      { path: 'kanban', name: 'kanban', component: () => import('../views/KanbanView.vue') },
      { path: 'sprint', name: 'sprint', component: () => import('../views/SprintView.vue') },
      { path: 'payments', name: 'payments', component: () => import('../views/PaymentsView.vue') },
      { path: 'settings/:section?', name: 'settings', component: () => import('../views/SettingsView.vue') },
      // Телефон: уведомления и профиль — отдельные экраны с кнопкой «назад»
      { path: 'notifications', name: 'notifications', component: () => import('../views/NotificationsView.vue'), meta: { back: '/tasks', title: 'notifications.title' } },
      { path: 'profile/:section?', name: 'profile', component: () => import('../views/ProfileView.vue'), meta: { back: '/settings', title: 'profile.title' } },
      { path: 'license', name: 'license', component: () => import('../views/LicenseView.vue') },
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, _from, next) => {
  // Состояние настройки (раз за сеанс — без перезагрузки оно измениться не может)
  if (!setupStatusChecked) {
    try {
      const { data } = await axios.get('/api/setup/status')
      setupIsComplete = !!data.isComplete
      setupStatusChecked = true
    } catch {
      // API недоступен — переход разрешается, повтор при следующем переходе
    }
  }
  if (setupStatusChecked) {
    if (!setupIsComplete && to.name !== 'setup') {
      return next({ name: 'setup' })
    }
    if (setupIsComplete && to.name === 'setup') {
      return next({ name: 'tasks' })
    }
  }

  // Проверка версии (раз за сеанс) — с видимым предупреждением
  if (!versionWarningShown) {
    try {
      const { data } = await axios.get('/api/status')
      // Версия сверяется один раз за сеанс, совпала она или нет:
      // иначе каждый переход между вкладками ждёт лишний запрос
      versionWarningShown = true
      if (data.version && data.version !== APP_VERSION) {
        // Предупреждение показывается плашкой, а не только в консоли
        const banner = document.createElement('div')
        banner.style.cssText = 'position:fixed;top:0;left:0;right:0;background:var(--warning);color:#111;padding:8px 16px;text-align:center;font-size:13px;font-weight:500;z-index:99999;'
        banner.textContent = i18n.global.t('common.version_mismatch', { frontend: APP_VERSION, backend: data.version })
        banner.onclick = () => banner.remove()
        document.body.appendChild(banner)
      }
    } catch {}
  }

  // Проверка входа — fetchMe кладёт данные пользователя в стор auth
  if (to.meta.requiresAuth) {
    try {
      const { useAuthStore } = await import('../stores/auth')
      const authStore = useAuthStore()
      // Пользователь уже загружен — вкладка открывается сразу, а вход
      // перепроверяется в фоне; истёк сеанс — переход на экран входа.
      // Ждать сервер на каждом клике нельзя: при медленной связи
      // переключение вкладки растягивалось до минуты
      if (authStore.isAuthenticated && authStore.user) {
        authStore.fetchMe().then(() => {
          if (!authStore.isAuthenticated) {
            router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
          }
        })
      } else {
        await authStore.fetchMe()
      }
      if (authStore.isAuthenticated) {
        // Без активированной лицензии работает только экран лицензии
        const { useLicenseStore } = await import('../stores/license')
        const licenseStore = useLicenseStore()
        // Лицензия уже известна и активна — обновляется в фоне, переход не ждёт
        if (licenseStore.loaded && !licenseStore.notActivated) {
          licenseStore.fetchStatus()
        } else {
          await licenseStore.fetchStatus(licenseStore.notActivated)
        }
        if (licenseStore.notActivated && to.name !== 'license') {
          return next({ name: 'license' })
        }
        // Вкладка скрыта администратором: переход на первую разрешённую
        const tab = typeof to.name === 'string' ? to.name : ''
        if (DATA_TABS.includes(tab) && !authStore.tabVisible(tab)) {
          const first = DATA_TABS.find(t => authStore.tabVisible(t))
          return next({ name: first || 'settings' })
        }
        next()
      } else {
        // После входа пользователь попадёт туда, куда шёл, — например к
        // задаче по ссылке из письма
        next({ name: 'login', query: { redirect: to.fullPath } })
      }
    } catch {
      next({ name: 'login', query: { redirect: to.fullPath } })
    }
  } else {
    next()
  }
})

export default router

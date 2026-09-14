import { createRouter, createWebHistory } from 'vue-router'
import axios from 'axios'

const APP_VERSION = '3.0.0'
let versionWarningShown = false

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
      { path: 'settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, _from, next) => {
  // Check setup status
  try {
    const { data } = await axios.get('/api/setup/status')
    if (!data.isComplete && to.name !== 'setup') {
      return next({ name: 'setup' })
    }
    if (data.isComplete && to.name === 'setup') {
      return next({ name: 'tasks' })
    }
  } catch {
    // API not available — allow navigation
  }

  // Version check (once per session) — show visible warning
  if (!versionWarningShown) {
    try {
      const { data } = await axios.get('/api/status')
      if (data.version && data.version !== APP_VERSION) {
        versionWarningShown = true
        // Show visible warning banner (not just console)
        const banner = document.createElement('div')
        banner.style.cssText = 'position:fixed;top:0;left:0;right:0;background:var(--warning);color:#111;padding:8px 16px;text-align:center;font-size:13px;font-weight:500;z-index:99999;'
        banner.textContent = `Версии фронтенда и бэкенда не совпадают (frontend: ${APP_VERSION}, backend: ${data.version}). Обновите страницу или обратитесь к администратору.`
        banner.onclick = () => banner.remove()
        document.body.appendChild(banner)
      }
    } catch {}
  }

  // Auth check
  if (to.meta.requiresAuth) {
    try {
      await axios.get('/api/auth/me')
      next()
    } catch {
      next({ name: 'login' })
    }
  } else {
    next()
  }
})

export default router

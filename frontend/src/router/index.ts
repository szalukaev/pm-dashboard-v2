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

  // Version check (once per session)
  if (!versionWarningShown) {
    try {
      const { data } = await axios.get('/api/status')
      if (data.version && data.version !== APP_VERSION) {
        versionWarningShown = true
        console.warn(`Version mismatch: frontend=${APP_VERSION}, backend=${data.version}`)
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

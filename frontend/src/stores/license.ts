import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

// Состояние установки по отношению к её лицензии:
//   not_activated — работает только экран активации;
//   active        — работает всё;
//   grace         — работает всё, идёт отсчёт до режима «только чтение»;
//   read_only     — данные можно читать, но не менять.
export type LicenseState = 'not_activated' | 'active' | 'grace' | 'read_only'

const REFRESH_MS = 60_000

export const useLicenseStore = defineStore('license', () => {
  // Пока сервер не ответил, считается, что всё в порядке
  const state = ref<LicenseState>('active')
  const graceEndsAt = ref<string | null>(null)
  let loadedAt = 0

  const notActivated = computed(() => state.value === 'not_activated')
  const readOnly = computed(() => state.value === 'read_only')

  function apply(data: { state?: string; grace_ends_at?: string | null }) {
    if (['not_activated', 'active', 'grace', 'read_only'].includes(data?.state || '')) {
      state.value = data.state as LicenseState
    }
    graceEndsAt.value = data?.grace_ends_at || null
    loadedAt = Date.now()
  }

  // force: спросить сервер, даже если ответ свежий
  async function fetchStatus(force = false) {
    if (!force && Date.now() - loadedAt < REFRESH_MS) return
    try {
      const { data } = await axios.get('/api/license/status')
      apply(data)
    } catch {}
  }

  return { state, graceEndsAt, notActivated, readOnly, apply, fetchStatus }
})

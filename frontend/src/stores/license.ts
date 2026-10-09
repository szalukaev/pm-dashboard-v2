import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

// State of the installation with respect to its license:
//   not_activated — only the activation screen works;
//   active        — everything works;
//   grace         — everything works, the countdown to read-only is running;
//   read_only     — data can be read but not changed.
export type LicenseState = 'not_activated' | 'active' | 'grace' | 'read_only'

const REFRESH_MS = 60_000

export const useLicenseStore = defineStore('license', () => {
  // Until the server answers, nothing is assumed to be wrong
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

  // force: ask the server even if the answer is fresh
  async function fetchStatus(force = false) {
    if (!force && Date.now() - loadedAt < REFRESH_MS) return
    try {
      const { data } = await axios.get('/api/license/status')
      apply(data)
    } catch {}
  }

  return { state, graceEndsAt, notActivated, readOnly, apply, fetchStatus }
})

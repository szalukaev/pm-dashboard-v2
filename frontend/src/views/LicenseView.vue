<template>
  <div class="license-view" v-if="!status?.is_active">
    <div class="license-card">
      <h2 class="license-title">Активация лицензии</h2>
      <p class="license-subtitle" v-if="status?.is_grace_period">
        Грейс-период. До блокировки записи: {{ graceRemaining }}
      </p>
      <p class="license-subtitle" v-if="status?.is_read_only" style="color: var(--danger)">
        Система в режиме только чтения. Активируйте лицензию для восстановления полного доступа.
      </p>

      <div class="form-field">
        <label>Лицензионный токен</label>
        <textarea
          v-model="licenseBlob"
          rows="4"
          placeholder="Вставьте лицензионный токен..."
        ></textarea>
      </div>

      <div class="status-message" :class="activateStatus" v-if="activateMessage">
        {{ activateMessage }}
      </div>

      <AppButton
        variant="primary"
        :loading="activating"
        :disabled="!licenseBlob.trim()"
        @click="activate"
        style="width: 100%"
      >
        Активировать
      </AppButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import AppButton from '../components/ui/AppButton.vue'

interface LicenseStatus {
  is_active: boolean
  is_grace_period: boolean
  is_read_only: boolean
  client: string
  edition: string
  grace_ends_at: string | null
  activated_at: string | null
  last_check_ok: string | null
}

const status = ref<LicenseStatus | null>(null)
const licenseBlob = ref('')
const activating = ref(false)
const activateMessage = ref('')
const activateStatus = ref('')

const graceRemaining = computed(() => {
  if (!status.value?.grace_ends_at) return ''
  const end = new Date(status.value.grace_ends_at)
  const now = new Date()
  const diff = end.getTime() - now.getTime()
  if (diff <= 0) return '0 дней'
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  return `${days} дн. ${hours} ч.`
})

let pollInterval: ReturnType<typeof setInterval>

async function checkStatus() {
  try {
    const { data } = await axios.get('/api/license/status')
    status.value = data
  } catch {}
}

async function activate() {
  activating.value = true
  activateMessage.value = ''
  try {
    const { data } = await axios.post('/api/license/activate', { license_blob: licenseBlob.value })
    activateMessage.value = `Лицензия активирована для: ${data.client} (${data.edition})`
    activateStatus.value = 'success'
    await checkStatus()
  } catch (e: any) {
    activateMessage.value = e.response?.data?.error === 'LICENSE_SIGNATURE_INVALID'
      ? 'Недействительная подпись лицензии'
      : 'Ошибка активации'
    activateStatus.value = 'error'
  } finally {
    activating.value = false
  }
}

onMounted(() => {
  checkStatus()
  pollInterval = setInterval(checkStatus, 60000)
})

onUnmounted(() => clearInterval(pollInterval))
</script>

<style scoped>
.license-view {
  position: fixed;
  inset: 0;
  background: var(--modal-overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-modal);
}

.license-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 40px;
  width: 480px;
  max-width: 90vw;
  box-shadow: var(--shadow);
}

.license-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  text-align: center;
  margin-bottom: 8px;
}

.license-subtitle {
  font-size: 13px;
  color: var(--warning);
  text-align: center;
  margin-bottom: 24px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin-bottom: 6px;
}

.form-field textarea {
  width: 100%;
  padding: 10px 12px;
  font-size: 12px;
  font-family: monospace;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  resize: vertical;
}

.form-field textarea:focus {
  border-color: var(--accent);
}

.status-message {
  min-height: 20px;
  font-size: 12px;
  text-align: center;
  margin: 12px 0;
}

.status-message.success { color: var(--success); }
.status-message.error { color: var(--danger); }
</style>

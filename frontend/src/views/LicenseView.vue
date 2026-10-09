<template>
  <div class="license-view">
    <h1 class="page-title">{{ $t('license.title') }}</h1>

    <!-- Everybody sees the state -->
    <div class="license-card state-card" :class="license.state" data-testid="license-state">
      <component :is="stateIcon" :size="22" class="state-icon" />
      <div>
        <div class="state-title">{{ $t('license.state.' + license.state) }}</div>
        <div class="state-text">{{ stateText }}</div>
      </div>
    </div>

    <!-- The rest is for administrators -->
    <p v-if="!auth.isAdmin" class="license-note">{{ $t('license.ask_admin') }}</p>

    <template v-else>
      <div v-if="details" class="license-grid">
        <!-- What is licensed -->
        <div v-if="details.status.client" class="license-card">
          <h4 class="card-title">{{ $t('license.info') }}</h4>
          <dl class="facts">
            <dt>{{ $t('license.client') }}</dt>
            <dd>{{ details.status.client }}</dd>
            <dt>{{ $t('license.edition') }}</dt>
            <dd>{{ details.status.edition || '—' }}</dd>
            <dt>{{ $t('license.grace_days') }}</dt>
            <dd>{{ details.status.grace_period_days || '—' }}</dd>
            <dt>{{ $t('license.activated_at') }}</dt>
            <dd>{{ formatDateTime(details.status.activated_at) }}</dd>
            <dt>{{ $t('license.last_check_ok') }}</dt>
            <dd>{{ formatDateTime(details.status.last_check_ok) }}</dd>
          </dl>
        </div>

        <!-- Hardware fingerprint to send to the vendor -->
        <div class="license-card">
          <h4 class="card-title">{{ $t('license.hwid') }}</h4>
          <template v-if="details.hwid_available">
            <p class="card-hint">{{ $t('license.hwid_hint') }}</p>
            <div class="hwid-box" data-testid="license-hwid">{{ details.hwid }}</div>
            <AppButton variant="ghost" @click="copyHwid"><Copy :size="14" /> {{ $t('license.copy') }}</AppButton>
          </template>
          <p v-else class="card-hint danger" data-testid="license-hwid-missing">{{ $t('license.hwid_missing') }}</p>
        </div>
      </div>

      <!-- Load a license -->
      <div class="license-card">
        <h4 class="card-title">{{ $t(license.notActivated ? 'license.activate_title' : 'license.replace_title') }}</h4>
        <p class="card-hint">{{ $t('license.token_hint') }}</p>
        <textarea
          v-model="token"
          rows="4"
          class="token-input"
          spellcheck="false"
          :placeholder="$t('license.token_placeholder')"
          data-testid="license-token"
        ></textarea>
        <div class="token-actions">
          <label class="file-btn">
            <Upload :size="14" /> {{ $t('license.load_file') }}
            <input type="file" accept=".lic,.txt,text/plain" class="file-input" @change="onFile" />
          </label>
          <AppButton variant="primary" :loading="activating" :disabled="!token.trim()" test-id="license-activate" @click="activate">
            {{ $t('license.activate') }}
          </AppButton>
        </div>
        <p v-if="message" class="token-message" :class="messageKind" data-testid="license-message">{{ message }}</p>
      </div>

      <!-- Journal -->
      <div v-if="details?.events.length" class="license-card">
        <h4 class="card-title">{{ $t('license.events') }}</h4>
        <div class="adm-table-wrap">
          <table class="adm-table">
            <thead>
              <tr>
                <th>{{ $t('license.event_time') }}</th>
                <th>{{ $t('license.event_name') }}</th>
                <th>{{ $t('license.event_details') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(e, i) in details.events" :key="i">
                <td class="adm-muted">{{ formatDateTime(e.at) }}</td>
                <td>{{ label('license.event.' + e.event, e.event) }}</td>
                <td>{{ label('license.reason.' + e.details, e.details) || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { ShieldCheck, ShieldAlert, ShieldX, ShieldOff, Copy, Upload } from 'lucide-vue-next'
import AppButton from '../components/ui/AppButton.vue'
import { useAuthStore } from '../stores/auth'
import { useLicenseStore } from '../stores/license'
import { useSwal } from '../composables/useSwal'

interface Details {
  status: {
    state: string
    reason?: string
    client?: string
    edition?: string
    grace_period_days?: number
    grace_ends_at?: string | null
    activated_at?: string | null
    last_check_ok?: string | null
  }
  hwid: string
  hwid_available: boolean
  events: { at: string; event: string; details: string }[]
}

const { t, te, locale } = useI18n()
const { toast } = useSwal()
const auth = useAuthStore()
const license = useLicenseStore()

const details = ref<Details | null>(null)
const token = ref('')
const activating = ref(false)
const message = ref('')
const messageKind = ref<'success' | 'error'>('success')

const stateIcon = computed(() => ({
  active: ShieldCheck,
  grace: ShieldAlert,
  read_only: ShieldX,
  not_activated: ShieldOff,
}[license.state]))

function formatDateTime(value?: string | null): string {
  if (!value) return '—'
  return new Date(value).toLocaleString(locale.value === 'en' ? 'en-GB' : 'ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

// A dictionary entry when there is one, otherwise the raw value
function label(key: string, raw: string): string {
  return te(key) ? t(key) : raw
}

const stateText = computed(() => {
  const ends = formatDateTime(license.graceEndsAt)
  const reason = details.value?.status.reason
  const why = reason ? ' ' + label('license.reason.' + reason, reason) + '.' : ''
  switch (license.state) {
    case 'grace': return t('license.text.grace', { time: ends }) + why
    case 'read_only': return t('license.text.read_only') + why
    case 'not_activated': return t('license.text.not_activated')
    default: return t('license.text.active')
  }
})

async function load() {
  await license.fetchStatus(true)
  if (!auth.isAdmin) return
  try {
    const { data } = await axios.get('/api/license/details')
    details.value = data
    license.apply(data.status)
  } catch {
    toast(t('license.load_error'), 'error')
  }
}

function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  file.text().then(text => { token.value = text.trim() })
  input.value = ''
}

async function activate() {
  if (!token.value.trim() || activating.value) return
  activating.value = true
  message.value = ''
  try {
    const { data } = await axios.post('/api/license/activate', { license_blob: token.value.trim() })
    token.value = ''
    messageKind.value = 'success'
    message.value = t('license.activated', { client: data.status?.client || '' })
    await load()
  } catch (e: any) {
    const key = `license.errors.${e?.response?.data?.error}`
    messageKind.value = 'error'
    message.value = te(key) ? t(key) : t('license.errors.generic')
    // A failed attempt is written to the journal
    load()
  } finally {
    activating.value = false
  }
}

// The clipboard API exists only on https and localhost, hence the fallback
async function copyHwid() {
  const text = details.value?.hwid || ''
  let copied = false
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      copied = true
    }
  } catch {}
  if (!copied) {
    const field = document.createElement('textarea')
    field.value = text
    field.setAttribute('readonly', '')
    field.style.position = 'fixed'
    field.style.opacity = '0'
    document.body.appendChild(field)
    field.select()
    try {
      copied = document.execCommand('copy')
    } catch {}
    field.remove()
  }
  toast(t(copied ? 'common.copied' : 'license.copy_failed'), copied ? 'success' : 'error')
}

onMounted(load)
</script>

<style scoped>
.license-view {
  max-width: 980px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-bright);
  letter-spacing: -0.4px;
}

.license-card {
  padding: 20px;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
}

.license-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.state-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.state-icon {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--text-muted);
}

.state-card.active .state-icon { color: var(--success); }
.state-card.grace .state-icon { color: var(--warning); }
.state-card.read_only .state-icon,
.state-card.not_activated .state-icon { color: var(--danger); }

.state-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 4px;
}

.state-text,
.license-note,
.card-hint {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}

.card-hint {
  margin-bottom: 12px;
}

.card-hint.danger {
  color: var(--danger);
}

.card-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
}

.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 16px;
  font-size: 13px;
}

.facts dt {
  color: var(--text-muted);
}

.facts dd {
  color: var(--text);
}

.hwid-box,
.token-input {
  width: 100%;
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.hwid-box {
  margin-bottom: 12px;
  overflow-wrap: anywhere;
  user-select: all;
}

.token-input {
  resize: vertical;
}

.token-input:focus {
  border-color: var(--accent);
}

.token-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
}

.file-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--accent);
  cursor: pointer;
}

.file-btn:hover {
  text-decoration: underline;
}

.file-input {
  display: none;
}

.token-message {
  margin-top: 12px;
  font-size: 13px;
}

.token-message.success { color: var(--success); }
.token-message.error { color: var(--danger); }

@media (max-width: 768px) {
  .license-grid {
    grid-template-columns: 1fr;
  }
}
</style>

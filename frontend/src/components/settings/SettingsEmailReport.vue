<template>
  <div class="email-report" data-testid="email-report">
    <h4 class="block-title">{{ $t('email_report.title') }}</h4>
    <p class="block-hint">{{ $t('email_report.hint', { zone: settingsStore.settings.server_timezone || '—' }) }}</p>

    <label class="report-check">
      <input v-model="form.enabled" type="checkbox" data-testid="report-enabled" />
      {{ $t('email_report.enabled') }}
    </label>

    <div class="report-row">
      <label class="report-field wide">
        <span>{{ $t('email_report.email') }}</span>
        <!-- Пустое поле — письмо уходит на адрес из личных данных -->
        <input v-model="form.email" type="email" :placeholder="profileEmail || $t('email_report.email_placeholder')" data-testid="report-email" />
      </label>
      <label class="report-field">
        <span>{{ $t('email_report.period') }}</span>
        <select v-model="form.period" data-testid="report-period">
          <option value="daily">{{ $t('email_report.periods.daily') }}</option>
          <option value="weekly">{{ $t('email_report.periods.weekly') }}</option>
          <option value="monthly">{{ $t('email_report.periods.monthly') }}</option>
        </select>
      </label>
      <label class="report-field narrow">
        <span>{{ $t('email_report.time') }}</span>
        <input v-model="form.time" type="time" data-testid="report-time" />
      </label>
    </div>

    <div class="report-sections">
      <div class="report-label">{{ $t('email_report.sections') }}</div>
      <label v-for="section in SECTIONS" :key="section" class="report-check">
        <input type="checkbox" :checked="form.sections.includes(section)" @change="toggleSection(section)" />
        {{ $t('email_report.section_names.' + section) }}
      </label>
      <label class="report-check">
        <input v-model="form.overdue_list" type="checkbox" data-testid="report-overdue" />
        {{ $t('email_report.overdue_list') }}
      </label>
    </div>

    <div v-if="message" class="report-message" :class="{ failed }" data-testid="report-message">{{ message }}</div>

    <div class="report-actions">
      <AppButton variant="primary" :loading="saving" test-id="report-save" @click="save">{{ $t('common.save') }}</AppButton>
      <AppButton variant="ghost" :loading="testing" test-id="report-test" @click="sendTest">{{ $t('email_report.test') }}</AppButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '../../stores/settings'
import type { EmailReport } from '../../stores/settings'
import { useAuthStore } from '../../stores/auth'
import AppButton from '../ui/AppButton.vue'

// Разделы сводки в порядке письма
const SECTIONS = ['stats', 'team_load', 'distribution', 'deadlines']

const { t, te } = useI18n()
const settingsStore = useSettingsStore()
const auth = useAuthStore()

const form = reactive<EmailReport>({
  enabled: false,
  email: '',
  period: 'weekly',
  time: '09:00',
  sections: [...SECTIONS],
  overdue_list: true,
})
const profileEmail = computed(() => auth.user?.email || '')

const saving = ref(false)
const testing = ref(false)
const message = ref('')
const failed = ref(false)

function toggleSection(section: string) {
  form.sections = form.sections.includes(section)
    ? form.sections.filter(s => s !== section)
    : SECTIONS.filter(s => s === section || form.sections.includes(s))
}

function payload(): EmailReport {
  return { ...form, email: form.email.trim(), sections: [...form.sections] }
}

function show(text: string, isError: boolean) {
  message.value = text
  failed.value = isError
}

function errorText(e: any, fallback: string): string {
  const key = 'email_report.errors.' + e?.response?.data?.error
  return te(key) ? t(key) : t(fallback)
}

async function save() {
  saving.value = true
  try {
    await axios.put('/api/settings', { email_report: payload() })
    settingsStore.settings.email_report = payload()
    show(t('email_report.saved'), false)
  } catch (e) {
    show(errorText(e, 'email_report.save_error'), true)
  } finally {
    saving.value = false
  }
}

// Пробная сводка уходит по тому, что сейчас в форме, — до сохранения
async function sendTest() {
  testing.value = true
  try {
    const { data } = await axios.post('/api/settings/email-report/test', payload())
    show(t('email_report.test_sent', { to: data.to || '' }), false)
  } catch (e) {
    show(errorText(e, 'email_report.test_error'), true)
  } finally {
    testing.value = false
  }
}

onMounted(async () => {
  await settingsStore.ensureLoaded()
  const saved = settingsStore.settings.email_report
  if (saved) {
    Object.assign(form, { ...saved, sections: Array.isArray(saved.sections) ? [...saved.sections] : [...SECTIONS] })
  }
})
</script>

<style scoped>
.block-title {
  margin: 32px 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-bright);
}

.block-hint {
  margin: 0 0 14px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-muted);
}

.report-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}

.report-check input {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
}

.report-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin: 14px 0;
}

.report-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1 1 200px;
  font-size: 12px;
  color: var(--text-muted);
}

.report-field.wide {
  flex: 2 1 260px;
}

.report-field.narrow {
  flex: 0 1 120px;
}

.report-field input,
.report-field select {
  padding: 8px 10px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  font-size: 13px;
  color: var(--text);
}

.report-field input:focus,
.report-field select:focus {
  border-color: var(--accent);
}

.report-sections {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.report-label {
  font-size: 12px;
  color: var(--text-muted);
}

.report-message {
  margin-top: 14px;
  padding: 10px 14px;
  border: 1px solid var(--success);
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
}

.report-message.failed {
  border-color: var(--danger);
}

.report-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}
</style>

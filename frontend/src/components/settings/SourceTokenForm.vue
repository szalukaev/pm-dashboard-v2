<template>
  <!-- The personal API key form of the Redmine connector. Another connector
       identifies a person its own way and gets its own form. -->
  <form v-if="source?.type === 'redmine'" class="token-form" @submit.prevent="save">
    <p v-if="source.linked" class="token-status linked" data-testid="source-linked">
      <CheckCircle2 :size="16" />
      <span>
        {{ $t('source_token.linked', { name: source.member_name || '#' + source.member_id }) }}
        <span v-if="source.token_hint" class="token-hint">{{ $t('source_token.key_ends', { hint: source.token_hint }) }}</span>
      </span>
    </p>
    <p v-else class="token-status">{{ $t('source_token.not_linked') }}</p>

    <label class="token-label" for="source-token-input">
      {{ $t(source.linked ? 'source_token.new_key' : 'source_token.key') }}
    </label>
    <div class="token-row">
      <input
        id="source-token-input"
        v-model="token"
        type="password"
        autocomplete="off"
        spellcheck="false"
        class="token-input"
        :placeholder="$t('source_token.placeholder')"
        data-testid="source-token-input"
      />
      <AppButton variant="primary" :loading="saving" :disabled="!token.trim()" test-id="source-token-save" @click="save">
        {{ $t(source.linked ? 'source_token.replace' : 'source_token.link') }}
      </AppButton>
    </div>
    <p class="token-help">{{ $t('source_token.help') }}</p>
    <p v-if="error" class="token-error" data-testid="source-token-error">{{ error }}</p>

    <button v-if="source.linked && allowUnlink" type="button" class="token-unlink" @click="unlink">
      {{ $t('source_token.unlink') }}
    </button>
  </form>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle2 } from 'lucide-vue-next'
import AppButton from '../ui/AppButton.vue'
import { useAuthStore } from '../../stores/auth'
import { useSwal } from '../../composables/useSwal'

defineProps<{ allowUnlink?: boolean }>()
const emit = defineEmits(['linked'])

const { t, te } = useI18n()
const { toast, confirm } = useSwal()
const auth = useAuthStore()

const source = computed(() => auth.user?.source)
const token = ref('')
const saving = ref(false)
const error = ref('')

async function save() {
  if (!token.value.trim() || saving.value) return
  saving.value = true
  error.value = ''
  try {
    await auth.setSourceToken(token.value.trim())
    token.value = ''
    toast(t('source_token.linked_toast', { name: source.value?.member_name || '' }), 'success')
    emit('linked')
  } catch (e: any) {
    const key = `source_token.errors.${e?.response?.data?.error}`
    error.value = te(key) ? t(key) : t('source_token.errors.generic')
  } finally {
    saving.value = false
  }
}

async function unlink() {
  const result = await confirm(t('source_token.unlink_title'), t('source_token.unlink_text'))
  if (!result.isConfirmed) return
  try {
    await auth.clearSourceToken()
  } catch {
    toast(t('source_token.errors.generic'), 'error')
  }
}
</script>

<style scoped>
.token-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 560px;
}

.token-status {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  color: var(--text-muted);
}

.token-status.linked {
  color: var(--success);
}

.token-status.linked span {
  color: var(--text);
}

.token-hint {
  margin-left: 6px;
  font-size: 12px;
  color: var(--text-faint) !important;
}

.token-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.token-row {
  display: flex;
  gap: 8px;
}

.token-input {
  flex: 1;
  min-width: 0;
  padding: 8px 12px;
  font-size: 13px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}

.token-input:focus {
  border-color: var(--accent);
}

.token-help {
  font-size: 12px;
  color: var(--text-faint);
}

.token-error {
  font-size: 12px;
  color: var(--danger);
}

.token-unlink {
  align-self: flex-start;
  font-size: 12px;
  color: var(--text-muted);
}

.token-unlink:hover {
  color: var(--danger);
  text-decoration: underline;
}
</style>

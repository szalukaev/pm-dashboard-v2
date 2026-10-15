<template>
  <!-- Телефон: поиск на виду, тип договора и закрытые — за иконкой на отдельном экране, как на «Задачах» -->
  <div v-if="isMobile" class="mobile-filters">
    <div class="m-search">
      <Search :size="16" class="m-search-icon" />
      <input
        v-model="search"
        type="search"
        enterkeyhint="search"
        :placeholder="$t('payments.filters.search_placeholder')"
        data-testid="payments-search"
        @keyup.enter="$emit('search', search)"
      />
      <button
        class="m-filter-btn"
        :class="{ active: activeChips.length > 0 }"
        :aria-label="$t('payments.filters.title')"
        data-testid="payments-filter-open"
        @click="panelOpen = true"
      >
        <Filter :size="18" />
        <span v-if="activeChips.length" class="m-badge">{{ activeChips.length }}</span>
      </button>
    </div>

    <div v-if="activeChips.length" class="m-chips">
      <button v-for="chip in activeChips" :key="chip.key" class="m-chip" @click="chip.clear()">
        {{ chip.label }} <X :size="12" />
      </button>
    </div>

    <Teleport to="body">
      <Transition name="m-panel">
        <div v-if="panelOpen" class="m-panel" data-testid="payments-filter-panel">
          <div class="m-panel-head">
            <button class="m-back" :aria-label="$t('common.back')" @click="panelOpen = false">
              <ChevronLeft :size="22" />
            </button>
            <b>{{ $t('payments.filters.title') }}</b>
            <button class="m-reset" @click="resetAll">{{ $t('payments.filters.reset') }}</button>
          </div>

          <div class="m-panel-body">
            <div class="m-label">{{ $t('payments.filters.type') }}</div>
            <div class="m-pills">
              <button
                v-for="o in typeOptions"
                :key="o.value"
                :class="{ active: type === o.value }"
                @click="$emit('update:type', o.value)"
              >{{ $t(o.label) }}</button>
            </div>

            <div class="m-label">{{ $t('payments.filters.show') }}</div>
            <label class="m-switch">
              {{ $t('payments.filters.closed_contracts') }}
              <input type="checkbox" :checked="closed" @change="$emit('update:closed', !closed)" />
              <span class="m-track"></span>
            </label>
          </div>

          <div class="m-panel-foot">
            <button class="m-show" @click="panelOpen = false">
              {{ $t('payments.filters.show_contracts', { n: total }) }}
            </button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>

  <!-- Компьютер: тип договора и закрытые — чипсами, поиск справа -->
  <div v-else class="filter-row">
    <button
      v-for="o in typeOptions"
      :key="o.value"
      class="chip"
      :class="{ active: type === o.value }"
      :data-testid="'payments-type-' + (o.value || 'all')"
      @click="$emit('update:type', o.value)"
    >{{ $t(o.label) }}</button>
    <span class="divider"></span>
    <button class="chip" :class="{ active: closed }" data-testid="payments-closed" @click="$emit('update:closed', !closed)">
      <Check v-if="closed" :size="12" /> {{ $t('payments.filters.show_closed') }}
    </button>
    <input
      v-model="search"
      type="text"
      :placeholder="$t('payments.filters.search_placeholder')"
      class="search-input"
      data-testid="payments-search"
      @keyup.enter="$emit('search', search)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, Filter, X, ChevronLeft, Check } from 'lucide-vue-next'
import { useMobileTasks } from '../../composables/useMobileTasks'

const props = defineProps<{ type: string; closed: boolean; total: number }>()
const emit = defineEmits<{
  'update:type': [value: string]
  'update:closed': [value: boolean]
  search: [value: string]
}>()

const { t } = useI18n()
const { isMobile } = useMobileTasks()

const typeOptions = [
  { value: '', label: 'payments.filters.all' },
  { value: 'service', label: 'payments.filters.service' },
  { value: 'onetime', label: 'payments.filters.onetime' },
]

const search = ref('')
const panelOpen = ref(false)

// Включённые фильтры — чипсами под поиском; нажатие снимает фильтр
const activeChips = computed(() => {
  const chips: { key: string; label: string; clear: () => void }[] = []
  const typeOption = typeOptions.find(o => o.value === props.type)
  if (props.type && typeOption) chips.push({ key: 'type', label: t(typeOption.label), clear: () => emit('update:type', '') })
  if (props.closed) chips.push({ key: 'closed', label: t('payments.filters.show_closed'), clear: () => emit('update:closed', false) })
  return chips
})

function resetAll() {
  if (props.type) emit('update:type', '')
  if (props.closed) emit('update:closed', false)
}
</script>

<style scoped>
.filter-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 9999px;
  background: var(--bg-card);
  border: 1px solid var(--hairline);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}

.chip:hover {
  border-color: var(--hairline-strong);
  color: var(--text-bright);
}

.chip.active {
  background: var(--accent-bg);
  border-color: var(--border-accent);
  color: var(--accent-hover);
}

.divider {
  width: 1px;
  height: 18px;
  margin: 0 4px;
  background: var(--hairline);
}

.search-input {
  padding: 6px 12px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
  /* Подсказка «Поиск по договору или организации…» должна помещаться целиком */
  min-width: 260px;
  margin-left: auto;
}

.mobile-filters {
  margin-bottom: 12px;
}

.m-search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 44px;
  padding: 0 4px 0 12px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
}

.m-search:focus-within {
  border-color: var(--accent);
}

.m-search-icon {
  flex: none;
  color: var(--text-muted);
}

.m-search input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: none;
  color: var(--text-bright);
  /* 16px — иначе iOS увеличивает страницу при фокусе */
  font-size: 16px;
  outline: none;
}

.m-filter-btn {
  position: relative;
  flex: none;
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 9px;
  background: none;
  color: var(--text);
  cursor: pointer;
}

.m-filter-btn.active {
  background: var(--accent);
  color: #fff;
}

.m-badge {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 17px;
  height: 17px;
  padding: 0 4px;
  display: grid;
  place-items: center;
  border: 2px solid var(--bg-card);
  border-radius: 9px;
  background: var(--danger);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
}

.m-chips {
  display: flex;
  gap: 6px;
  margin-top: 8px;
  overflow-x: auto;
  scrollbar-width: none;
}

.m-chips::-webkit-scrollbar {
  display: none;
}

.m-chip {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border: 0;
  border-radius: 9999px;
  background: var(--accent-bg);
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
}

.m-panel {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: flex;
  flex-direction: column;
  background: var(--bg);
}

.m-panel-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: calc(10px + env(safe-area-inset-top)) 12px 10px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border);
}

.m-panel-head b {
  flex: 1;
  font-size: 17px;
  color: var(--text-bright);
}

.m-back,
.m-reset {
  border: 0;
  background: none;
  cursor: pointer;
}

.m-back {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  color: var(--text);
}

.m-reset {
  padding: 8px;
  color: var(--accent);
  font-size: 14px;
  font-weight: 600;
}

.m-panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px 16px 16px;
}

.m-label {
  margin: 18px 0 8px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.m-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.m-pills button {
  padding: 8px 13px;
  border: 1px solid var(--border);
  border-radius: 9999px;
  background: var(--bg-card);
  color: var(--text);
  font-size: 14px;
  cursor: pointer;
}

.m-pills button.active {
  border-color: var(--accent);
  background: var(--accent-bg);
  color: var(--accent);
  font-weight: 600;
}

/* Переключатель «Закрытые договоры» */
.m-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-card);
  color: var(--text);
  font-size: 14px;
  cursor: pointer;
}

.m-switch input {
  display: none;
}

.m-track {
  position: relative;
  width: 44px;
  height: 26px;
  flex: none;
  border-radius: 13px;
  background: var(--hairline-strong);
  transition: background 0.15s;
}

.m-track::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #fff;
  transition: left 0.15s;
}

.m-switch input:checked + .m-track {
  background: var(--success);
}

.m-switch input:checked + .m-track::after {
  left: 21px;
}

.m-panel-foot {
  padding: 12px 16px calc(14px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--border);
  background: var(--bg-card);
}

.m-show {
  width: 100%;
  padding: 14px;
  border: 0;
  border-radius: 11px;
  background: var(--accent);
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
}

.m-panel-enter-active,
.m-panel-leave-active {
  transition: transform 0.28s cubic-bezier(0.3, 0.8, 0.3, 1);
}

.m-panel-enter-from,
.m-panel-leave-to {
  transform: translateX(100%);
}
</style>

<template>
  <!-- Чек-лист первого запуска: виден только администратору, над меню настроек -->
  <!-- Всё настроено: одна надпись, родитель ставит её в правый верхний угол -->
  <div v-if="complete" class="checklist-done" data-testid="setup-checklist-done">
    <CheckCircle2 :size="16" />
    {{ $t('checklist.complete') }}
  </div>
  <div v-else-if="items.length" class="checklist" data-testid="setup-checklist">
    <button type="button" class="checklist-head" :aria-expanded="open" data-testid="checklist-toggle" @click="toggle">
      <ListChecks :size="16" class="head-icon" />
      <span class="head-title">{{ $t('checklist.title') }}</span>
      <span class="head-count">{{ $t('checklist.progress', { done, total: items.length }) }}</span>
      <span class="head-bar"><span :style="{ width: percent + '%' }"></span></span>
      <ChevronDown :size="16" class="head-chevron" :class="{ collapsed: !open }" />
    </button>

    <div v-if="open" class="checklist-items">
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        class="checklist-item"
        :class="state(item)"
        :data-testid="'checklist-' + item.key"
        @click="$emit('open', TARGETS[item.key] || { tab: 'admin' })"
      >
        <CheckCircle2 v-if="item.done" :size="16" class="item-icon" />
        <AlertTriangle v-else-if="item.required" :size="16" class="item-icon" />
        <Clock v-else :size="16" class="item-icon" />
        <span class="item-text">
          <span class="item-name">{{ $t('checklist.items.' + item.key + '.name') }}</span>
          <span class="item-state">{{ $t('checklist.items.' + item.key + (item.done ? '.done' : '.todo')) }}</span>
        </span>
        <ChevronRight :size="14" class="item-go" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { CheckCircle2, ListChecks, AlertTriangle, Clock, ChevronDown, ChevronRight } from 'lucide-vue-next'

// Шаг настройки; required — без него система не работает, остальные — рекомендации
interface Item {
  key: string
  done: boolean
  required: boolean
}

// Куда ведёт пункт: вкладка настроек и, для «Администрирования», вкладка внутри него
const TARGETS: Record<string, { tab: string; sub?: string }> = {
  source: { tab: 'connections' },
  statuses: { tab: 'admin', sub: 'data' },
  projects: { tab: 'sync' },
  users: { tab: 'admin', sub: 'users' },
  rights: { tab: 'admin', sub: 'users' },
  schedule: { tab: 'admin', sub: 'workers' },
}

// Свёрнут или развёрнут незавершённый чек-лист — помнит браузер; пока не
// выбирали, он развёрнут
const OPEN_KEY = 'pm-dashboard-checklist-open'

// open: перейти к вкладке шага — { tab, sub }
defineEmits(['open'])

const items = ref<Item[]>([])
const chosen = ref<boolean | null>(null)

const done = computed(() => items.value.filter(i => i.done).length)
const complete = computed(() => items.value.length > 0 && done.value === items.value.length)
const percent = computed(() => (items.value.length ? Math.round((done.value / items.value.length) * 100) : 0))
const open = computed(() => chosen.value ?? true)

function state(item: Item): string {
  return item.done ? 'done' : item.required ? 'required' : 'advised'
}

function toggle() {
  chosen.value = !open.value
  try {
    localStorage.setItem(OPEN_KEY, chosen.value ? '1' : '0')
  } catch {}
}

async function load() {
  try {
    const { data } = await axios.get('/api/admin/setup-checklist')
    items.value = data.items || []
  } catch {
    // Без чек-листа настройки работают как прежде
    items.value = []
  }
}

onMounted(() => {
  try {
    const saved = localStorage.getItem(OPEN_KEY)
    if (saved !== null) chosen.value = saved === '1'
  } catch {}
  load()
})

// Состояние шагов меняется на других вкладках: родитель перечитывает при переходах
defineExpose({ reload: load })
</script>

<style scoped>
/* Незавершённый чек-лист занимает всю строку под заголовком страницы */
.checklist {
  flex-basis: 100%;
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
}

.checklist-done {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  color: var(--success);
  background: color-mix(in srgb, var(--success) 12%, transparent);
  border-radius: 9999px;
}

.checklist-head {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 12px 16px;
  font-size: 13px;
  text-align: left;
  color: var(--text-bright);
  border-radius: 12px;
}

.head-icon {
  flex-shrink: 0;
  color: var(--accent);
}

.head-title {
  font-weight: 600;
}

.head-count {
  color: var(--text-muted);
  white-space: nowrap;
}

.head-bar {
  flex: 1;
  min-width: 60px;
  height: 6px;
  overflow: hidden;
  background: var(--surface-3);
  border-radius: 9999px;
}

.head-bar span {
  display: block;
  height: 100%;
  background: var(--accent);
  border-radius: 9999px;
  transition: width 0.3s;
}

.head-chevron {
  flex-shrink: 0;
  color: var(--text-muted);
  transition: transform 0.15s;
}

.head-chevron.collapsed {
  transform: rotate(-90deg);
}

.checklist-items {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 8px;
  padding: 0 16px 14px;
}

.checklist-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  text-align: left;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  transition: border-color 0.15s;
}

.checklist-item:hover {
  border-color: var(--accent);
}

.item-icon {
  flex-shrink: 0;
  margin-top: 1px;
}

.checklist-item.done .item-icon { color: var(--success); }
.checklist-item.required .item-icon { color: var(--warning); }
.checklist-item.advised .item-icon { color: var(--text-muted); }

.item-text {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.item-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-bright);
}

.item-state {
  font-size: 12px;
  color: var(--text-muted);
}

.item-go {
  flex-shrink: 0;
  margin-top: 3px;
  color: var(--text-faint);
}
</style>

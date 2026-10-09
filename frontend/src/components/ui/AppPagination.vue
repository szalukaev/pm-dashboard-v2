<template>
  <!-- A list shorter than the smallest page needs neither pages nor a page size -->
  <div v-if="total > sizes[0]" class="app-pagination" data-testid="pagination">
    <span class="range">{{ $t('common.pagination.range', { from, to, total }) }}</span>

    <div class="pages">
      <button
        type="button"
        class="page-btn"
        :disabled="page <= 1"
        :title="$t('common.pagination.prev')"
        data-testid="page-prev"
        @click="go(page - 1)"
      >
        <ChevronLeft :size="16" />
      </button>
      <span class="page-of">{{ $t('common.pagination.page', { page, pages }) }}</span>
      <button
        type="button"
        class="page-btn"
        :disabled="page >= pages"
        :title="$t('common.pagination.next')"
        data-testid="page-next"
        @click="go(page + 1)"
      >
        <ChevronRight :size="16" />
      </button>
    </div>

    <label class="page-size">
      {{ $t('common.pagination.per_page') }}
      <select :value="limit" data-testid="page-size" @change="onSize">
        <option v-for="size in sizes" :key="size" :value="size">{{ size }}</option>
      </select>
    </label>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { PAGE_SIZES } from '../../stores/settings'

const props = withDefaults(defineProps<{
  total: number
  limit: number
  offset: number
  sizes?: number[]
}>(), {
  sizes: () => PAGE_SIZES,
})

const emit = defineEmits<{
  (e: 'update:offset', offset: number): void
  (e: 'update:limit', limit: number): void
}>()

const pages = computed(() => Math.max(1, Math.ceil(props.total / props.limit)))
const page = computed(() => Math.min(pages.value, Math.floor(props.offset / props.limit) + 1))
const from = computed(() => (props.total === 0 ? 0 : (page.value - 1) * props.limit + 1))
const to = computed(() => Math.min(props.total, page.value * props.limit))

function go(target: number) {
  const clamped = Math.min(pages.value, Math.max(1, target))
  if (clamped !== page.value) emit('update:offset', (clamped - 1) * props.limit)
}

function onSize(e: Event) {
  emit('update:limit', Number((e.target as HTMLSelectElement).value))
}
</script>

<style scoped>
.app-pagination {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 16px;
  padding: 10px 12px;
  font-size: 12px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.range {
  margin-right: auto;
}

.pages {
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--text);
  cursor: pointer;
}

.page-btn:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--text-bright);
}

.page-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.page-size {
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-size select {
  padding: 4px 8px;
  font-size: 12px;
  background: var(--surface-2);
  border: 1px solid var(--hairline);
  border-radius: 8px;
  color: var(--text);
}
</style>

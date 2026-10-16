<template>
  <!-- Телефон: колонки стопкой, раскрываются по нажатию; перенос — через меню карточки -->
  <div class="mobile-board" data-testid="kanban-mobile-board">
    <section
      v-for="col in columns"
      :key="col.id"
      class="m-column"
      :class="{ open: isOpen(col.id) }"
      :data-testid="'kanban-column-' + col.id"
    >
      <button class="m-column-head" @click="toggle(col.id)">
        <span class="m-column-name">{{ columnName(col) }}</span>
        <span class="m-column-count">{{ col.tasks.length }}</span>
        <ChevronDown :size="18" class="m-chevron" />
      </button>

      <!-- Свёрнутая колонка: полоска из цветов приоритетов её задач -->
      <div v-if="!isOpen(col.id) && col.tasks.length" class="m-strip">
        <i
          v-for="card in col.tasks.slice(0, STRIP_MAX)"
          :key="card.external_id"
          :style="{ background: priorityColor({ id: card.priority_id, name: card.priority_name }) }"
        ></i>
      </div>

      <div v-if="isOpen(col.id)" class="m-column-body">
        <KanbanCard
          v-for="card in col.tasks"
          :key="card.external_id"
          :card="card"
          menu
          @open-task="$emit('open-task', $event)"
          @menu="moving = card"
        />
        <div v-if="col.tasks.length === 0" class="m-empty">—</div>
      </div>
    </section>

    <!-- Шторка переноса задачи в другую колонку -->
    <Teleport to="body">
      <Transition name="m-sheet">
        <div v-if="moving" class="m-sheet-scrim" @click="moving = null">
          <div class="m-sheet" data-testid="kanban-move-sheet" @click.stop>
            <div class="m-grab"></div>
            <div class="m-sheet-task">#{{ moving.external_id }} · {{ moving.subject }}</div>
            <div class="m-sheet-title">{{ $t('kanban.move_to') }}</div>
            <button
              v-for="col in columns"
              :key="col.id"
              class="m-target"
              :class="{ current: col.id === currentColumnId }"
              :disabled="col.id === currentColumnId"
              :data-testid="'kanban-move-' + col.id"
              @click="move(col.id)"
            >
              <span>{{ columnName(col) }}</span>
              <span v-if="col.id === currentColumnId" class="m-current">{{ $t('kanban.current_column') }}</span>
              <span v-else class="m-target-count">{{ col.tasks.length }}</span>
            </button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown } from 'lucide-vue-next'
import KanbanCard from './KanbanCard.vue'
import { UNASSIGNED, type KanbanColumn, type KanbanCard as Card } from '../../stores/kanban'
import { priorityColor } from '../../utils/statusColor'

const props = defineProps<{ columns: KanbanColumn[] }>()
const emit = defineEmits(['open-task', 'move'])

const { t } = useI18n()

// Сколько отметок приоритетов помещается в полоску свёрнутой колонки
const STRIP_MAX = 24

const openIds = ref<Set<string>>(new Set())
const moving = ref<Card | null>(null)

// При заходе все колонки свёрнуты
function isOpen(id: string) {
  return openIds.value.has(id)
}

function toggle(id: string) {
  const next = new Set(openIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  openIds.value = next
}

function columnName(col: KanbanColumn) {
  return col.id === UNASSIGNED ? t('kanban.unassigned') : col.name
}

const currentColumnId = computed(() =>
  moving.value ? props.columns.find(c => c.tasks.some(x => x.external_id === moving.value!.external_id))?.id : undefined,
)

function move(columnId: string) {
  if (!moving.value) return
  emit('move', { issueId: moving.value.external_id, columnId })
  moving.value = null
}
</script>

<style scoped>
.mobile-board {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.m-column {
  background: var(--surface-4);
  border: 1px solid var(--border);
  border-top: 3px solid var(--accent);
  border-radius: 12px;
  overflow: hidden;
}

.m-column-head {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border: 0;
  background: none;
  color: var(--text-bright);
  font-size: 14px;
  font-weight: 700;
  text-align: left;
  cursor: pointer;
}

.m-column-name {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.m-column-count {
  padding: 1px 8px;
  border-radius: 9999px;
  background: var(--accent-bg);
  color: var(--accent);
  font-size: 12px;
}

.m-chevron {
  color: var(--text-muted);
  transition: transform 0.2s;
}

.m-column.open .m-chevron {
  transform: rotate(180deg);
}

.m-strip {
  display: flex;
  gap: 3px;
  padding: 0 14px 12px;
  margin-top: -4px;
}

.m-strip i {
  flex: 1;
  max-width: 28px;
  height: 4px;
  border-radius: 2px;
}

.m-column-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 10px 10px;
}

.m-empty {
  padding: 8px;
  text-align: center;
  color: var(--text-faint);
}

.m-sheet-scrim {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: flex;
  align-items: flex-end;
  background: var(--modal-overlay);
}

.m-sheet {
  width: 100%;
  max-height: 80vh;
  overflow-y: auto;
  padding: 10px 16px calc(18px + env(safe-area-inset-bottom));
  background: var(--bg-card);
  border-radius: 20px 20px 0 0;
}

.m-grab {
  width: 40px;
  height: 5px;
  margin: 0 auto 12px;
  border-radius: 3px;
  background: var(--border);
}

.m-sheet-task {
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.m-sheet-title {
  margin-bottom: 10px;
  font-size: 16px;
  font-weight: 700;
  color: var(--text-bright);
}

.m-target {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 12px;
  border: 0;
  border-radius: 10px;
  background: none;
  color: var(--text-bright);
  font-size: 15px;
  text-align: left;
  cursor: pointer;
}

.m-target span:first-child {
  flex: 1;
}

.m-target:active {
  background: var(--bg-hover);
}

.m-target.current {
  background: var(--surface-2);
  color: var(--text-muted);
  cursor: default;
}

.m-current,
.m-target-count {
  font-size: 12px;
  color: var(--text-muted);
}

.m-sheet-enter-active,
.m-sheet-leave-active {
  transition: opacity 0.25s;
}

.m-sheet-enter-active .m-sheet,
.m-sheet-leave-active .m-sheet {
  transition: transform 0.28s cubic-bezier(0.3, 0.8, 0.3, 1);
}

.m-sheet-enter-from,
.m-sheet-leave-to {
  opacity: 0;
}

.m-sheet-enter-from .m-sheet,
.m-sheet-leave-to .m-sheet {
  transform: translateY(100%);
}
</style>

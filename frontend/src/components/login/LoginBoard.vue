<template>
  <div class="login-board" aria-hidden="true">
    <div class="board-head">
      <div v-for="(name, i) in columns" :key="i" class="board-col-name">
        <span>{{ name }}</span>
        <span class="board-col-count">{{ counts[i] }}</span>
      </div>
    </div>
    <div class="board-cards">
      <div
        v-for="card in cards"
        :key="card.id"
        class="board-card"
        :class="{ hidden: card.hidden }"
        :style="{ transform: `translate(calc(${card.col} * (100% + ${GAP}px)), ${rowOf(card) * ROW}px)` }"
      >
        <i>#{{ card.id }}</i>{{ card.title }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

// Декоративная доска на экране входа: вымышленные задачи сами переезжают
// из колонки в колонку. Настоящих данных до входа нет и быть не должно.

interface BoardCard { id: number; title: string; col: number; hidden: boolean }

const GAP = 18
const ROW = 50
const MAX_IN_LAST = 4

const { tm, rt } = useI18n()
const columns = computed(() => (tm('auth.board.columns') as unknown[]).map(c => rt(c as any)))
const titles = (tm('auth.board.tasks') as unknown[]).map(c => rt(c as any))

const cards = ref<BoardCard[]>(titles.map((title, i) => ({
  id: 4511 + i * 3,
  title,
  col: i % 5,
  hidden: false,
})))

function rowOf(card: BoardCard) {
  return cards.value.filter(c => c.col === card.col).indexOf(card)
}

const counts = computed(() => columns.value.map((_, i) => cards.value.filter(c => c.col === i).length))

function step() {
  const list = cards.value
  const sources = [0, 1, 2, 3].filter(col => list.filter(c => c.col === col).length > 1)
  if (!sources.length) return
  const from = sources[Math.floor(Math.random() * sources.length)]
  const inFrom = list.filter(c => c.col === from)
  const card = inFrom[inFrom.length - 1]
  // Переезжающая карточка встаёт первой в следующей колонке
  list.splice(list.indexOf(card), 1)
  card.col = from + 1
  const firstInTarget = list.findIndex(c => c.col === card.col)
  list.splice(firstInTarget === -1 ? list.length : firstInTarget, 0, card)

  // Последняя колонка не переполняется: лишняя карточка гаснет и
  // появляется в конце первой
  const inLast = list.filter(c => c.col === 4)
  if (inLast.length > MAX_IN_LAST) {
    const extra = inLast[inLast.length - 1]
    extra.hidden = true
    setTimeout(() => {
      list.splice(list.indexOf(extra), 1)
      extra.col = 0
      list.push(extra)
      extra.hidden = false
    }, 500)
  }
}

let timer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  timer = setInterval(step, 2400)
})

onBeforeUnmount(() => clearInterval(timer))
</script>

<style scoped>
.login-board {
  position: absolute;
  inset: 0;
  padding: 36px;
  overflow: hidden;
  pointer-events: none;
  user-select: none;
}

.board-head {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 18px;
  margin-bottom: 12px;
}

.board-col-name {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.board-cards {
  position: relative;
}

.board-card {
  position: absolute;
  top: 0;
  left: 0;
  width: calc((100% - 4 * 18px) / 5);
  height: 40px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 12.5px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: transform 1.4s cubic-bezier(0.6, 0, 0.2, 1), opacity 0.4s;
}

.board-card i {
  font-style: normal;
  font-weight: 600;
  color: var(--accent);
  margin-right: 6px;
}

.board-card.hidden {
  opacity: 0;
}
</style>

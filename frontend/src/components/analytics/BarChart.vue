<template>
  <div class="bar-chart">
    <div v-if="labels.length === 0" class="empty-hint">{{ $t('common.empty') }}</div>

    <template v-else>
      <div v-if="datasets.length > 1" class="legend">
        <span v-for="d in datasets" :key="d.label" class="legend-item">
          <span class="legend-dot" :style="{ background: 'var(' + d.color + ')' }"></span>
          {{ d.label }}
        </span>
      </div>
      <div class="chart-box" :style="{ height: height + 'px' }">
        <canvas ref="canvas"></canvas>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Chart, BarController, BarElement, CategoryScale, LinearScale, Tooltip } from 'chart.js'
import type { TooltipItem } from 'chart.js'
import { cssVar, tooltipTheme } from '../../utils/chartTheme'

Chart.register(BarController, BarElement, CategoryScale, LinearScale, Tooltip)

// labels: the categories; datasets: a bar per category in each, color is the
// name of a theme variable; horizontal: bars go from left to right, one row
// per category; hint: an extra line of the tooltip for a category.
const props = defineProps<{
  labels: string[]
  datasets: { label: string; data: number[]; color: string }[]
  horizontal?: boolean
  hint?: (index: number) => string
}>()

const VERTICAL_HEIGHT = 260
const ROW_GAP = 14
const BAR_HEIGHT = 14
const AXIS_HEIGHT = 40

// A horizontal chart grows with the number of its rows
const height = computed(() =>
  props.horizontal
    ? Math.max(140, props.labels.length * (props.datasets.length * BAR_HEIGHT + ROW_GAP) + AXIS_HEIGHT)
    : VERTICAL_HEIGHT
)

const canvas = ref<HTMLCanvasElement | null>(null)
let chart: Chart<'bar'> | null = null

function destroy() {
  if (chart) {
    chart.destroy()
    chart = null
  }
}

function render() {
  destroy()
  if (!canvas.value || props.labels.length === 0) return

  const tickColor = cssVar('--text-muted', '#8a8f98')
  const gridColor = cssVar('--hairline', '#23252a')
  const categoryAxis = { grid: { display: false }, ticks: { color: tickColor } }
  const valueAxis = { beginAtZero: true, grid: { color: gridColor }, ticks: { color: tickColor, precision: 0 } }

  chart = new Chart(canvas.value, {
    type: 'bar',
    data: {
      labels: props.labels,
      datasets: props.datasets.map(d => ({
        label: d.label,
        data: d.data,
        backgroundColor: cssVar(d.color, '#5e6ad2'),
        borderRadius: 4,
        maxBarThickness: 28,
      })),
    },
    options: {
      indexAxis: props.horizontal ? 'y' : 'x',
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          ...tooltipTheme(),
          callbacks: {
            afterLabel: (item: TooltipItem<'bar'>) => (props.hint ? props.hint(item.dataIndex) : ''),
          },
        },
      },
      scales: {
        x: props.horizontal ? valueAxis : categoryAxis,
        y: props.horizontal ? categoryAxis : valueAxis,
      },
    },
  })
}

// The canvas appears together with the first data
function draw() {
  nextTick(render)
}

watch(() => [props.labels, props.datasets, props.horizontal], draw, { deep: true })
onMounted(draw)
onBeforeUnmount(destroy)
</script>

<style scoped>
.empty-hint {
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  padding: 24px;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 12px;
  font-size: 12px;
  color: var(--text-dim);
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.chart-box {
  position: relative;
  width: 100%;
}
</style>

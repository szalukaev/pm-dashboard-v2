<template>
  <div class="distribution-section">
    <div v-if="distribution.length === 0" class="empty-hint">
      {{ $t('common.empty') }}
    </div>

    <div class="charts-grid">
      <div
        v-for="proj in distribution"
        :key="proj.project_name"
        class="chart-card"
        :class="{ clickable: nested(proj) > 1 }"
        :title="nested(proj) > 1 ? $t('analytics.distribution_open') : undefined"
        @click="nested(proj) > 1 && $emit('open', proj)"
      >
        <h4 class="chart-title">{{ proj.project_name }}</h4>
        <div v-if="nested(proj) > 1" class="chart-note">{{ $t('analytics.distribution_projects', { n: nested(proj) }) }}</div>
        <div class="chart-container">
          <canvas :ref="el => setCanvas(el, proj.project_name)"></canvas>
        </div>
        <div class="legend">
          <div
            v-for="(a, idx) in proj.assignees"
            :key="a.name"
            class="legend-item"
          >
            <span class="legend-dot" :style="{ background: getChartColors()[idx % getChartColors().length] }"></span>
            <span class="legend-name">{{ a.name }}</span>
            <span class="legend-count">{{ a.count }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { Chart, PieController, ArcElement, Tooltip, Legend } from 'chart.js'
import type { DistChart } from '../../stores/analytics'

Chart.register(PieController, ArcElement, Tooltip, Legend)

const props = defineProps<{ distribution: DistChart[] }>()

// open: a chart that stands for several projects was clicked
defineEmits<{
  (e: 'open', chart: DistChart): void
}>()

// How many projects a chart stands for
function nested(chart: DistChart): number {
  return chart.parts ? chart.parts.length : 1
}

const canvasMap = ref<Map<string, HTMLCanvasElement>>(new Map())
const chartInstances = ref<Map<string, Chart<'pie'>>>(new Map())

function getChartColors(): string[] {
  const s = getComputedStyle(document.documentElement)
  return [
    s.getPropertyValue('--accent').trim(),
    s.getPropertyValue('--purple').trim(),
    s.getPropertyValue('--orange').trim(),
    s.getPropertyValue('--cyan').trim(),
    s.getPropertyValue('--pink').trim(),
    s.getPropertyValue('--teal').trim(),
    s.getPropertyValue('--lime').trim(),
    s.getPropertyValue('--warning').trim(),
    s.getPropertyValue('--danger').trim(),
    s.getPropertyValue('--success').trim(),
  ]
}

function setCanvas(el: any, name: string) {
  if (el) {
    canvasMap.value.set(name, el as HTMLCanvasElement)
  }
}

function renderCharts(distribution: DistChart[]) {
  // Destroy old charts
  chartInstances.value.forEach(chart => chart.destroy())
  chartInstances.value.clear()

  const colors = getChartColors()

  distribution.forEach(proj => {
    const canvas = canvasMap.value.get(proj.project_name)
    if (!canvas) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const chart = new Chart(ctx, {
      type: 'pie',
      data: {
        labels: proj.assignees.map(a => a.name),
        datasets: [{
          data: proj.assignees.map(a => a.count),
          backgroundColor: proj.assignees.map((_, i) => colors[i % colors.length]),
          borderWidth: 0,
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: getComputedStyle(document.documentElement).getPropertyValue('--surface-3').trim() || 'rgba(15, 16, 17, 0.9)',
            titleColor: getComputedStyle(document.documentElement).getPropertyValue('--text-bright').trim() || '#f7f8f8',
            bodyColor: getComputedStyle(document.documentElement).getPropertyValue('--text-dim').trim() || '#d0d6e0',
            borderColor: getComputedStyle(document.documentElement).getPropertyValue('--hairline').trim() || '#23252a',
            borderWidth: 1,
            cornerRadius: 8,
            padding: 8,
          }
        }
      }
    })
    chartInstances.value.set(proj.project_name, chart)
  })
}

// The charts are drawn once their canvases are on the page: when the widget
// appears (also after being expanded) and whenever the data changes
function draw() {
  nextTick(() => renderCharts(props.distribution))
}

watch(() => props.distribution, draw, { deep: true })
onMounted(draw)
onBeforeUnmount(() => {
  chartInstances.value.forEach(chart => chart.destroy())
  chartInstances.value.clear()
})
</script>

<style scoped>
.empty-hint {
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  padding: 24px;
}

.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.chart-card {
  background: var(--surface-1);
  border: 1px solid var(--hairline);
  border-radius: 12px;
  padding: 20px;
}

.chart-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 12px;
  letter-spacing: -0.2px;
}

.chart-card.clickable {
  cursor: pointer;
}

.chart-card.clickable:hover {
  border-color: var(--accent);
}

.chart-card.clickable .chart-title {
  margin-bottom: 2px;
}

.chart-note {
  margin-bottom: 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.chart-container {
  width: 100%;
  max-width: 200px;
  margin: 0 auto 12px;
}

.legend {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.legend-name {
  color: var(--text-dim);
  flex: 1;
}

.legend-count {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

@container (max-width: 600px) {
  .charts-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

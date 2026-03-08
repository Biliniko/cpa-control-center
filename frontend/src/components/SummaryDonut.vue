<script setup lang="ts">
import { PieChart, type PieSeriesOption } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  type TitleComponentOption,
  type TooltipComponentOption,
} from 'echarts/components'
import { getInstanceByDom, init, use, type ComposeOption, type ECharts } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { computed, nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DashboardSummary } from '@/types'
import { emitDebug, emitDebugError } from '@/utils/debug'

use([PieChart, TitleComponent, TooltipComponent, CanvasRenderer])

type DonutChartOption = ComposeOption<
  PieSeriesOption |
  TitleComponentOption |
  TooltipComponentOption
>

type DonutItem = {
  key: string
  value: number
  name: string
  color: string
}

const { t } = useI18n()

const props = defineProps<{
  summary: DashboardSummary
}>()

const shell = ref<HTMLDivElement | null>(null)
const chartRoot = ref<HTMLDivElement | null>(null)
const shellWidth = ref(0)
const shellHeight = ref(0)
let chart: ECharts | null = null
let resizeObserver: ResizeObserver | null = null
let renderFrame = 0

const items = computed<DonutItem[]>(() => [
  { key: 'pending', value: props.summary.pendingCount, name: t('states.pending'), color: '#7c9eb2' },
  { key: 'normal', value: props.summary.normalCount, name: t('states.normal'), color: '#2f7d61' },
  { key: 'invalid_401', value: props.summary.invalid401Count, name: t('states.invalid_401'), color: '#c2410c' },
  { key: 'quota_limited', value: props.summary.quotaLimitedCount, name: t('states.quota_limited'), color: '#d97706' },
  { key: 'recovered', value: props.summary.recoveredCount, name: t('states.recovered'), color: '#3b82f6' },
  { key: 'error', value: props.summary.errorCount, name: t('states.error'), color: '#6b7280' },
].filter((item) => item.value > 0))

const displayItems = computed<DonutItem[]>(() => {
  if (items.value.length > 0) {
    return items.value
  }
  return [
    {
      key: 'not_available',
      value: 0,
      name: t('common.notAvailable'),
      color: '#d6d0c0',
    },
  ]
})

const layoutMode = computed(() => {
  const width = shellWidth.value
  const height = shellHeight.value

  return {
    compact: width < 420 || height < 250,
    stacked: width < 360 || height < 220,
    tiny: width < 310 || height < 195,
  }
})

const shellStyle = computed(() => ({
  '--donut-gap': layoutMode.value.tiny ? '0.52rem' : layoutMode.value.compact ? '0.65rem' : '0.8rem',
}))

const chartStyle = computed(() => ({
  '--donut-chart-size': layoutMode.value.tiny ? '152px' : layoutMode.value.compact ? '176px' : '210px',
}))

const legendColumns = computed(() => {
  const total = Math.max(displayItems.value.length, 1)
  if (layoutMode.value.stacked) {
    return 1
  }
  return Math.min(total, 2)
})

const legendStyle = computed(() => ({
  gridTemplateColumns: `repeat(${legendColumns.value}, minmax(0, 1fr))`,
  '--legend-gap': layoutMode.value.tiny ? '0.5rem' : layoutMode.value.compact ? '0.58rem' : '0.7rem',
  '--legend-item-padding': layoutMode.value.tiny ? '0.52rem 0.62rem' : layoutMode.value.compact ? '0.58rem 0.72rem' : '0.68rem 0.82rem',
  '--legend-name-size': layoutMode.value.tiny ? '0.76rem' : layoutMode.value.compact ? '0.82rem' : '0.88rem',
  '--legend-value-size': layoutMode.value.tiny ? '0.95rem' : layoutMode.value.compact ? '1.02rem' : '1.14rem',
  '--legend-swatch-size': layoutMode.value.tiny ? '10px' : '12px',
}))

function chartMetrics(width: number, height: number) {
  const basis = Math.min(width, height)
  const compact = basis < 150
  const dense = basis < 130

  return {
    radius: dense ? ['63%', '83%'] : compact ? ['65%', '85%'] : ['67%', '87%'],
    center: ['50%', '47%'],
    titleTop: dense ? '38%' : '39%',
    subTitleTop: dense ? '55%' : '54%',
    titleSize: dense ? 16 : compact ? 19 : 26,
    subTitleSize: dense ? 8 : 10,
    borderWidth: dense ? 3 : 4,
    emphasisScale: dense ? 4 : 6,
  }
}

function updateBounds() {
  if (!shell.value) {
    return
  }
  shellWidth.value = shell.value.clientWidth
  shellHeight.value = shell.value.clientHeight
}

function renderChart() {
  if (!chartRoot.value) {
    emitDebug('summary-donut', 'render skipped: no chart root')
    return
  }
  if (chartRoot.value.clientWidth <= 0 || chartRoot.value.clientHeight <= 0) {
    emitDebug('summary-donut', 'render skipped: zero-size container', {
      width: chartRoot.value.clientWidth,
      height: chartRoot.value.clientHeight,
    })
    return
  }

  try {
    if (!chart) {
      chart = getInstanceByDom(chartRoot.value) ?? init(chartRoot.value)
    } else {
      chart.resize()
    }

    const metrics = chartMetrics(chartRoot.value.clientWidth, chartRoot.value.clientHeight)
    const seriesData = items.value.length > 0
      ? items.value.map(({ color, name, value }) => ({
          value,
          name,
          itemStyle: { color },
        }))
      : [{ value: 1, name: t('common.notAvailable'), itemStyle: { color: '#d6d0c0' } }]

    chart.setOption<DonutChartOption>({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
      },
      title: [
        {
          text: `${props.summary.filteredAccounts || 0}`,
          left: 'center',
          top: metrics.titleTop,
          textStyle: {
            color: '#201b14',
            fontSize: metrics.titleSize,
            fontWeight: 800,
            lineHeight: metrics.titleSize + 4,
          },
        },
        {
          text: t('dashboard.trackedAccounts'),
          left: 'center',
          top: metrics.subTitleTop,
          textStyle: {
            color: '#7a705f',
            fontSize: metrics.subTitleSize,
            fontWeight: 700,
          },
        },
      ],
      series: [
        {
          type: 'pie',
          radius: metrics.radius,
          center: metrics.center,
          silent: false,
          label: {
            show: false,
          },
          labelLine: {
            show: false,
          },
          itemStyle: {
            borderColor: '#f5efe2',
            borderWidth: metrics.borderWidth,
          },
          emphasis: {
            scale: true,
            scaleSize: metrics.emphasisScale,
          },
          data: seriesData,
        },
      ],
    })
    emitDebug('summary-donut', 'rendered', {
      width: chartRoot.value.clientWidth,
      height: chartRoot.value.clientHeight,
      items: items.value.map((item) => ({ key: item.key, value: item.value })),
      filtered: props.summary.filteredAccounts,
    })
  } catch (error) {
    emitDebugError('summary-donut', 'render failed', error)
    console.error('SummaryDonut render failed', error)
  }
}

function queueRender() {
  if (renderFrame) {
    window.cancelAnimationFrame(renderFrame)
  }
  renderFrame = window.requestAnimationFrame(() => {
    renderFrame = 0
    renderChart()
  })
}

function handleResize() {
  updateBounds()
  queueRender()
}

watch([items, () => props.summary.filteredAccounts], async () => {
  await nextTick()
  queueRender()
}, { deep: true })

onMounted(() => {
  emitDebug('summary-donut', 'mounted')
  updateBounds()
  queueRender()
  window.addEventListener('resize', handleResize)
  if (shell.value || chartRoot.value) {
    resizeObserver = new ResizeObserver(() => {
      updateBounds()
      queueRender()
    })
    if (shell.value) {
      resizeObserver.observe(shell.value)
    }
    if (chartRoot.value) {
      resizeObserver.observe(chartRoot.value)
    }
  }
})

onActivated(() => {
  emitDebug('summary-donut', 'activated')
  handleResize()
})

onDeactivated(() => {
  emitDebug('summary-donut', 'deactivated')
  if (renderFrame) {
    window.cancelAnimationFrame(renderFrame)
    renderFrame = 0
  }
})

onBeforeUnmount(() => {
  emitDebug('summary-donut', 'before unmount')
  window.removeEventListener('resize', handleResize)
  resizeObserver?.disconnect()
  resizeObserver = null
  if (renderFrame) {
    window.cancelAnimationFrame(renderFrame)
    renderFrame = 0
  }
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div
    ref="shell"
    class="summary-donut"
    :class="{
      'summary-donut--compact': layoutMode.compact,
      'summary-donut--stacked': layoutMode.stacked,
      'summary-donut--tiny': layoutMode.tiny,
    }"
    :style="shellStyle"
  >
    <div ref="chartRoot" class="summary-donut__chart" :style="chartStyle" />
    <div class="summary-donut__legend" :style="legendStyle">
      <div v-for="item in displayItems" :key="item.key" class="summary-donut__legend-item">
        <span class="summary-donut__legend-swatch" :style="{ backgroundColor: item.color }" />
        <span class="summary-donut__legend-name">{{ item.name }}</span>
        <strong class="summary-donut__legend-value">{{ item.value }}</strong>
      </div>
    </div>
  </div>
</template>

<style scoped>
.summary-donut {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--donut-gap, 0.9rem);
  width: 100%;
  height: 100%;
  min-height: 0;
  margin: 0 auto;
}

.summary-donut__chart {
  width: var(--donut-chart-size, 204px);
  height: var(--donut-chart-size, 204px);
  flex: none;
}

.summary-donut__legend {
  display: grid;
  gap: var(--legend-gap, 0.75rem);
  width: min(100%, 380px);
  align-content: start;
}

.summary-donut__legend-item {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
  padding: var(--legend-item-padding, 0.7rem 0.8rem);
  border-radius: 16px;
  background: rgba(245, 239, 226, 0.88);
  border: 1px solid rgba(38, 34, 27, 0.08);
}

.summary-donut__legend-swatch {
  flex: none;
  width: var(--legend-swatch-size, 12px);
  height: var(--legend-swatch-size, 12px);
  border-radius: 999px;
  box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.72);
}

.summary-donut__legend-name {
  flex: 1 1 auto;
  min-width: 0;
  color: #4e4639;
  font-size: var(--legend-name-size, 0.92rem);
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: normal;
  line-height: 1.2;
}

.summary-donut__legend-value {
  color: #1f1a14;
  font-size: var(--legend-value-size, 1.25rem);
  font-weight: 800;
  line-height: 1;
}

.summary-donut--stacked .summary-donut__legend {
  width: 100%;
}

.summary-donut--stacked .summary-donut__legend {
  grid-template-columns: 1fr !important;
}

.summary-donut--tiny .summary-donut__legend-item {
  border-radius: 14px;
}
</style>

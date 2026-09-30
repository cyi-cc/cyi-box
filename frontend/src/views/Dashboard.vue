<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { NIcon, NGrid, NGridItem } from 'naive-ui'
import {
  ServerOutline, GlobeOutline, KeyOutline, LibraryOutline, CubeOutline,
  LockClosedOutline, BookmarkOutline, CloudOutline, TimeOutline, PulseOutline,
  CashOutline, ReceiptOutline, ArrowForwardOutline
} from '@vicons/ionicons5'
import * as echarts from 'echarts'
import { client } from '../lib/api'
import { useAuthStore } from '../stores/auth'
import type statsView from '../api/statsView'

const auth = useAuthStore()
const router = useRouter()
const stats = ref<statsView | null>(null)
const chartEl = ref<HTMLElement | null>(null)
const chart = shallowRef<echarts.ECharts | null>(null)

let timer: ReturnType<typeof setInterval> | null = null
let observer: ResizeObserver | null = null

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

function fen2yuan(fen: number): string { return (fen / 100).toFixed(2) }

function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d} 天 ${h} 小时`
  if (h > 0) return `${h} 小时 ${m} 分`
  return `${m} 分 ${sec % 60} 秒`
}

function fmtTime(ts: number): string {
  if (!ts) return '-'
  const d = new Date(ts * 1000)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

const modules = computed(() => [
  { name: '服务器', value: stats.value?.servers, sub: '台主机', to: '/servers', icon: ServerOutline, cls: 'i1' },
  { name: '代理池', value: stats.value == null ? null : `${stats.value.aliveProxies}/${stats.value.proxies}`, sub: '存活/总数', to: '/proxies', icon: GlobeOutline, cls: 'i2' },
  { name: '授权卡密', value: stats.value == null ? null : `${stats.value.licenseActive}/${stats.value.licenseCards}`, sub: '激活/总数', to: '/licenses', icon: KeyOutline, cls: 'i3' },
  { name: '记忆库', value: stats.value?.memories, sub: `${stats.value?.memoryProjects ?? 0} 个项目`, to: '/memories', icon: LibraryOutline, cls: 'i4' },
  { name: '数据库', value: stats.value?.dbconns, sub: '个连接', to: '/dbm', icon: CubeOutline, cls: 'i1' },
  { name: '保险库', value: stats.value?.vaultItems, sub: '条机密', to: '/vault', icon: LockClosedOutline, cls: 'i2' },
  { name: '书签', value: stats.value?.bookmarks, sub: '个收藏', to: '/bookmarks', icon: BookmarkOutline, cls: 'i3' },
  { name: '云盘', value: stats.value?.files, sub: '个文件', to: '/disk', icon: CloudOutline, cls: 'i4' },
])

async function load() {
  const r = await client.dashboardSvc.stats()
  if (r.status === 0 && r.data) {
    stats.value = r.data
    renderChart()
  }
}

function renderChart() {
  if (!chartEl.value || !stats.value) return
  if (!chart.value) chart.value = echarts.init(chartEl.value)
  chart.value.setOption({
    grid: { left: 48, right: 40, top: 46, bottom: 30 },
    legend: {
      top: 2, left: 0, icon: 'roundRect', itemWidth: 14, itemHeight: 10, itemGap: 18,
      textStyle: { color: '#57534d', fontSize: 11, fontWeight: 700 }
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#fff', borderColor: '#1a1a1a', borderWidth: 2,
      textStyle: { color: '#1a1a1a', fontSize: 12, fontWeight: 700 },
      padding: [8, 12],
      extraCssText: 'box-shadow: 4px 4px 0 #1a1a1a; border-radius: 10px;',
      formatter: (params: any) => {
        const items = Array.isArray(params) ? params : [params]
        const day = items[0]?.axisValue ?? ''
        const lines = items.map((p: any) =>
          `${p.marker} ${p.seriesName}：${p.seriesName === '成交金额' ? '¥' + p.value : p.value + ' 单'}`)
        return `<b>${day}</b><br/>${lines.join('<br/>')}`
      }
    },
    xAxis: {
      type: 'category',
      data: stats.value.payTrend.map(p => p.day.slice(5)),
      axisLine: { lineStyle: { color: '#1a1a1a', width: 2 } },
      axisTick: { show: false },
      axisLabel: { color: '#57534d', fontSize: 11, fontWeight: 600 }
    },
    yAxis: [
      {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(26,26,26,0.12)', type: 'dashed' } },
        axisLabel: {
          color: '#57534d', fontSize: 11, fontWeight: 600,
          formatter: (v: number) => `¥${v}`
        }
      },
      {
        type: 'value', minInterval: 1,
        splitLine: { show: false },
        axisLabel: { color: '#57534d', fontSize: 11, fontWeight: 600 }
      }
    ],
    series: [
      {
        name: '成交金额', type: 'bar', barWidth: '55%', barMaxWidth: 22,
        data: stats.value.payTrend.map(p => +(p.money / 100).toFixed(2)),
        itemStyle: { color: '#fef08a', borderColor: '#1a1a1a', borderWidth: 2, borderRadius: [4, 4, 0, 0] }
      },
      {
        name: '成交单数', type: 'line', yAxisIndex: 1, smooth: 0.3,
        symbol: 'circle', symbolSize: 8,
        data: stats.value.payTrend.map(p => p.count),
        lineStyle: { width: 3.5, color: '#ec5b13', cap: 'round' },
        itemStyle: { color: '#fff', borderColor: '#1a1a1a', borderWidth: 2.5 }
      }
    ]
  })
}

onMounted(() => {
  void load()
  timer = setInterval(load, 60_000)
  if (chartEl.value) {
    observer = new ResizeObserver(() => chart.value?.resize())
    observer.observe(chartEl.value)
  }
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  observer?.disconnect()
  chart.value?.dispose()
})
</script>

<template>
  <div>
    <div class="dash-hello" style="margin-bottom: 22px">
      <h2>{{ greeting }}，{{ auth.user?.name }}</h2>
      <p>这是你的工作箱概览</p>
    </div>

    <n-grid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
      <n-grid-item span="4 m:1">
        <div class="stat-card" style="cursor:pointer" @click="router.push('/pay')">
          <div class="stat-icon i1"><n-icon><CashOutline /></n-icon></div>
          <div>
            <div class="stat-label">今日营收</div>
            <div class="stat-value">¥{{ stats ? fen2yuan(stats.todayMoney) : '-' }}</div>
            <div class="stat-sub">今日 {{ stats?.todayOrders ?? 0 }} 单</div>
          </div>
        </div>
      </n-grid-item>
      <n-grid-item span="4 m:1">
        <div class="stat-card" style="cursor:pointer" @click="router.push('/pay')">
          <div class="stat-icon i2"><n-icon><ReceiptOutline /></n-icon></div>
          <div>
            <div class="stat-label">累计收入</div>
            <div class="stat-value">¥{{ stats ? fen2yuan(stats.totalMoney) : '-' }}</div>
            <div class="stat-sub">待支付 {{ stats?.pendingOrders ?? 0 }} 单</div>
          </div>
        </div>
      </n-grid-item>
      <n-grid-item span="4 m:1">
        <div class="stat-card">
          <div class="stat-icon i3"><n-icon><PulseOutline /></n-icon></div>
          <div>
            <div class="stat-label">在线会话</div>
            <div class="stat-value">{{ stats?.activeSessions ?? '-' }}</div>
            <div class="stat-sub">今日登录 {{ stats?.todayLogins ?? 0 }} 次</div>
          </div>
        </div>
      </n-grid-item>
      <n-grid-item span="4 m:1">
        <div class="stat-card">
          <div class="stat-icon i4"><n-icon><TimeOutline /></n-icon></div>
          <div>
            <div class="stat-label">运行时长</div>
            <div class="stat-value" style="font-size:20px">{{ stats ? fmtUptime(stats.uptimeSeconds) : '-' }}</div>
            <div class="stat-sub">{{ stats?.totalUsers ?? 0 }} 位用户</div>
          </div>
        </div>
      </n-grid-item>
    </n-grid>

    <n-grid :cols="8" :x-gap="12" :y-gap="12" responsive="screen" item-responsive style="margin-top: 16px">
      <n-grid-item v-for="m in modules" :key="m.name" span="8 s:4 m:2 l:1">
        <div class="mod-card" @click="router.push(m.to)">
          <div class="mod-head">
            <div class="stat-icon sm" :class="m.cls"><n-icon size="17"><component :is="m.icon" /></n-icon></div>
            <n-icon class="mod-arrow" size="13"><ArrowForwardOutline /></n-icon>
          </div>
          <div class="mod-value">{{ m.value ?? '-' }}</div>
          <div class="mod-name">{{ m.name }}<span class="mod-sub">{{ m.sub }}</span></div>
        </div>
      </n-grid-item>
    </n-grid>

    <n-grid :cols="3" :x-gap="16" :y-gap="16" responsive="screen" item-responsive style="margin-top: 16px">
      <n-grid-item span="3 m:2">
        <div class="panel">
          <div class="panel-title">
            近 14 天营收
            <span class="hint">已支付订单 · 金额 + 单数</span>
          </div>
          <div ref="chartEl" style="height: 280px"></div>
        </div>
      </n-grid-item>
      <n-grid-item span="3 m:1">
        <div class="panel" style="height: 100%">
          <div class="panel-title">最近成交</div>
          <div v-if="!stats?.recentOrders?.length" class="empty-hint">暂无已支付订单</div>
          <div v-for="o in stats?.recentOrders" :key="o.tradeNo" class="act-row">
            <div class="act-main">
              <span class="act-title">{{ o.subject }}</span>
              <span class="act-time">{{ fmtTime(o.paidAt) }}</span>
            </div>
            <span class="act-money">¥{{ fen2yuan(o.money) }}</span>
          </div>
          <div class="panel-title" style="margin-top:18px">最近记忆</div>
          <div v-if="!stats?.recentMemories?.length" class="empty-hint">暂无记忆</div>
          <div v-for="m in stats?.recentMemories" :key="m.key" class="act-row" style="cursor:pointer" @click="router.push('/memories')">
            <div class="act-main">
              <span class="act-title mono">{{ m.key }}</span>
              <span class="act-time">{{ fmtTime(m.updatedAt) }}</span>
            </div>
            <span class="act-tag">{{ m.project }}</span>
          </div>
        </div>
      </n-grid-item>
    </n-grid>
  </div>
</template>

<style scoped>
.stat-sub { font-size: 11px; color: var(--cb-ink-3); font-weight: 600; margin-top: 2px; }

.stat-icon.sm { width: 30px; height: 30px; border-radius: 8px; }

.mod-card {
  background: #fff; border: 2px solid var(--cb-ink); border-radius: 12px;
  box-shadow: 3px 3px 0 var(--cb-ink); padding: 12px 14px; cursor: pointer;
  transition: transform .12s, box-shadow .12s;
}
.mod-card:hover { transform: translate(-1px, -1px); box-shadow: 4px 4px 0 var(--cb-ink); }
.mod-card:active { transform: translate(1px, 1px); box-shadow: 2px 2px 0 var(--cb-ink); }
.mod-head { display: flex; align-items: center; justify-content: space-between; }
.mod-arrow { color: var(--cb-ink-3); opacity: 0; transition: opacity .12s; }
.mod-card:hover .mod-arrow { opacity: 1; }
.mod-value { font-size: 22px; font-weight: 900; color: var(--cb-ink); margin-top: 8px; font-variant-numeric: tabular-nums; }
.mod-name { font-size: 11.5px; font-weight: 700; color: var(--cb-ink-2); display: flex; align-items: baseline; gap: 5px; }
.mod-sub { font-size: 10.5px; color: var(--cb-ink-3); font-weight: 600; }

.act-row {
  display: flex; align-items: center; justify-content: space-between; gap: 10px;
  padding: 7px 0; border-bottom: 1px dashed var(--cb-border);
}
.act-row:last-child { border-bottom: none; }
.act-main { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.act-title { font-size: 12.5px; font-weight: 700; color: var(--cb-ink); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.act-time { font-size: 10.5px; color: var(--cb-ink-3); font-weight: 600; }
.act-money { font-size: 13px; font-weight: 900; color: var(--cb-ink); flex-shrink: 0; }
.act-tag {
  font-size: 10.5px; font-weight: 800; color: var(--cb-ink-2); flex-shrink: 0;
  background: var(--cb-cream-2); border: 1.5px solid var(--cb-ink); border-radius: 6px; padding: 1px 7px;
}
.empty-hint { font-size: 12px; color: var(--cb-ink-3); font-weight: 600; padding: 8px 0; }
.mono { font-family: ui-monospace, monospace; }
</style>

<script setup lang="ts">
import { computed, onActivated, onMounted, onUnmounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NSelect, NEmpty, NPagination, NTag
} from 'naive-ui'
import {
  SearchOutline,
  CheckmarkCircleOutline, CloseCircleOutline, ChevronDownOutline, ChevronUpOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import type proxyView from '../api/proxyView'
import type proxyRegionView from '../api/proxyRegionView'
import type proxySyncView from '../api/proxySyncView'
import type proxyStatsView from '../api/proxyStatsView'

const stats = ref<proxyStatsView | null>(null)
const regions = ref<proxyRegionView[]>([])
const syncs = ref<proxySyncView[]>([])
const items = ref<proxyView[]>([])
const total = ref(0)
const loading = ref(false)

const regionId = ref(0)
const keyword = ref('')
const aliveOnly = ref(true)
const page = ref(1)
const pageSize = 30
const expandedSync = ref(0)

async function loadStats() {
  const r = await client.proxySvc.stats()
  if (r.status === 0) stats.value = r.data ?? null
}
async function loadRegions() {
  const r = await client.proxySvc.regions()
  if (r.status === 0) regions.value = r.data ?? []
}
async function loadSyncs() {
  const r = await client.proxySvc.syncs()
  if (r.status === 0) syncs.value = r.data ?? []
}
async function loadProxies() {
  loading.value = true
  try {
    const r = await client.proxySvc.list({
      regionId: regionId.value, protocol: '',
      keyword: keyword.value, alive: aliveOnly.value ? 1 : -1,
      page: page.value, pageSize
    })
    if (r.status === 0 && r.data) {
      items.value = r.data.items ?? []
      total.value = r.data.total
    }
  } finally {
    loading.value = false
  }
}

function refreshAll() {
  void loadStats(); void loadRegions(); void loadSyncs(); void loadProxies()
}
onMounted(refreshAll)
onActivated(refreshAll)

// 同步/测活进行中时轮询刷新
let timer: number | undefined
onMounted(() => {
  timer = window.setInterval(() => {
    if (stats.value?.syncing || stats.value?.checking) refreshAll()
  }, 5000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

function pickRegion(id: number) {
  regionId.value = regionId.value === id ? 0 : id
  page.value = 1
  void loadProxies()
}

function search() {
  page.value = 1
  void loadProxies()
}

function fmtTime(ts: number): string {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

function regionLabel(r: proxyRegionView): string {
  return r.zhName || r.name || r.code
}

const groupedRegions = computed(() => {
  const groups = new Map<string, proxyRegionView[]>()
  for (const r of regions.value) {
    const k = r.region || 'Other'
    if (!groups.has(k)) groups.set(k, [])
    groups.get(k)!.push(r)
  }
  return [...groups.entries()]
})

const regionZh: Record<string, string> = {
  Europe: '欧洲', Asia: '亚洲', Americas: '美洲', Africa: '非洲', Pacific: '太平洋', Other: '其他'
}

interface SyncDetail { added?: string[]; removed?: string[] }
function syncDetail(s: proxySyncView): SyncDetail {
  try { return s.detailJson ? JSON.parse(s.detailJson) : {} } catch { return {} }
}
</script>

<template>
  <div class="proxies-page">
    <div class="panel">
      <div class="panel-title">
        代理池
        <span class="title-actions">
          <n-tag v-if="stats?.syncing" size="small" type="warning">同步中…</n-tag>
          <n-tag v-if="stats?.checking" size="small" type="info">测活中…</n-tag>
        </span>
      </div>
      <div class="stat-strip">
        <div class="stat-cell">
          <div class="stat-num">{{ stats?.total ?? 0 }}</div>
          <div class="stat-lb">存活代理</div>
        </div>
        <div class="stat-cell">
          <div class="stat-num">{{ stats?.regions ?? 0 }}</div>
          <div class="stat-lb">覆盖地区</div>
        </div>
        <div class="stat-cell">
          <div class="stat-num">{{ fmtTime(stats?.lastSyncAt ?? 0) }}</div>
          <div class="stat-lb">上次同步</div>
        </div>
        <div class="stat-cell">
          <div class="stat-num">{{ fmtTime(stats?.lastCheckAt ?? 0) }}</div>
          <div class="stat-lb">上次测活</div>
        </div>
      </div>
    </div>

    <div class="proxies-body">
      <div class="panel regions-panel">
        <div class="panel-title">地区 <span class="hint">{{ regions.length }} 个</span></div>
        <div class="region-list">
          <div
            class="region-row all" :class="{ on: regionId === 0 }"
            @click="pickRegion(0)"
          >
            <span class="region-code">ALL</span>
            <span class="region-name">全部地区</span>
            <span class="region-count">{{ stats?.total ?? 0 }}</span>
          </div>
          <template v-for="[grp, list] in groupedRegions" :key="grp">
            <div class="region-grp">{{ regionZh[grp] ?? grp }}</div>
            <div
              v-for="r in list" :key="r.id"
              class="region-row" :class="{ on: regionId === r.id }"
              @click="pickRegion(r.id)"
            >
              <span class="region-code">{{ r.code }}</span>
              <span class="region-name">{{ regionLabel(r) }}</span>
              <span class="region-count">{{ r.aliveCount }}</span>
            </div>
          </template>
          <n-empty v-if="!regions.length" description="暂无数据，等待每日 00:00 自动同步" style="padding:40px 0" />
        </div>
      </div>

      <div class="panel list-panel">
        <div class="filters">
          <n-select
            v-model:value="aliveOnly" size="small" style="width:110px"
            :options="[{ label: '仅存活', value: true }, { label: '全部', value: false }]"
            @update:value="search"
          />
          <n-input
            v-model:value="keyword" size="small" placeholder="搜 IP…" style="width:180px"
            clearable @keyup.enter="search" @clear="search"
          >
            <template #prefix><n-icon><SearchOutline /></n-icon></template>
          </n-input>
          <n-button size="small" @click="search">查询</n-button>
          <span class="hint" style="margin-left:auto">共 {{ total }} 条</span>
        </div>

        <div class="proxy-table" :class="{ loading }">
          <div class="pt-head">
            <span class="c-addr">地址</span>
            <span class="c-region">地区</span>
            <span class="c-lat">延迟</span>
            <span class="c-st">状态</span>
          </div>
          <div v-for="p in items" :key="p.id" class="pt-row" :class="{ dead: !p.alive }">
            <span class="c-addr mono">{{ p.address }}</span>
            <span class="c-region">
              <span class="region-code sm">{{ p.regionCode }}</span>
              {{ p.regionZhName || p.regionName }}
            </span>
            <span class="c-lat" :class="{ slow: p.latency > 800 }">{{ p.latency <= 0 ? '<1ms' : p.latency + 'ms' }}</span>
            <span class="c-st">
              <n-icon v-if="p.alive" color="#16a34a" :size="16"><CheckmarkCircleOutline /></n-icon>
              <n-icon v-else color="#dc2626" :size="16"><CloseCircleOutline /></n-icon>
            </span>
          </div>
          <n-empty v-if="!items.length && !loading" description="没有匹配的代理" style="padding:40px 0" />
        </div>

        <div class="pager">
          <n-pagination
            v-model:page="page" :item-count="total" :page-size="pageSize"
            @update:page="loadProxies"
          />
        </div>
      </div>
    </div>

    <div class="panel syncs-panel">
      <div class="panel-title">同步记录 <span class="hint">每日 00:00 自动执行 · 最近 {{ syncs.length }} 次</span></div>
      <div v-for="s in syncs" :key="s.id" class="sync-row">
        <span class="sync-status" :class="s.status">{{ s.status === 'ok' ? '成功' : '失败' }}</span>
        <span class="sync-time mono">{{ fmtTime(s.startedAt) }}</span>
        <span class="sync-nums">
          共 {{ s.total }} · <b class="up">+{{ s.added }}</b> · <b class="dn">-{{ s.removed }}</b> · 更新 {{ s.updated }}
        </span>
        <span v-if="s.error" class="sync-err">{{ s.error }}</span>
        <n-button
          v-if="s.added || s.removed" size="tiny" quaternary
          @click="expandedSync = expandedSync === s.id ? 0 : s.id"
        >
          <template #icon><n-icon><ChevronDownOutline v-if="expandedSync !== s.id" /><ChevronUpOutline v-else /></n-icon></template>
          变更明细
        </n-button>
        <div v-if="expandedSync === s.id" class="sync-detail">
          <div v-if="syncDetail(s).added?.length" class="detail-col">
            <div class="detail-title up">新增（前 {{ syncDetail(s).added!.length }} 条）</div>
            <div class="mono detail-ips">{{ syncDetail(s).added!.join('  ') }}</div>
          </div>
          <div v-if="syncDetail(s).removed?.length" class="detail-col">
            <div class="detail-title dn">移除（前 {{ syncDetail(s).removed!.length }} 条）</div>
            <div class="mono detail-ips">{{ syncDetail(s).removed!.join('  ') }}</div>
          </div>
        </div>
      </div>
      <n-empty v-if="!syncs.length" description="还没有同步记录" style="padding:30px 0" />
    </div>
  </div>
</template>

<style scoped>
.proxies-page { display: flex; flex-direction: column; gap: 16px; }
.title-actions { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.stat-strip {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px;
  margin-top: 12px;
}
.stat-cell {
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  background: var(--cb-cream-2, #f1eee8); padding: 10px 14px;
}
.stat-cell:nth-child(1) { background: var(--cb-mint); }
.stat-cell:nth-child(2) { background: var(--cb-yellow); }
.stat-num { font-size: 20px; font-weight: 900; font-family: Consolas, monospace; line-height: 1.2; }
.stat-lb { font-size: 11px; font-weight: 800; color: var(--cb-ink-3); margin-top: 2px; }

.proxies-body { display: grid; grid-template-columns: 280px 1fr; gap: 16px; align-items: start; }

.regions-panel { height: calc(100vh - 280px); min-height: 400px; display: flex; flex-direction: column; }
.region-list { flex: 1; overflow-y: auto; }
.region-grp {
  font-size: 11px; font-weight: 800; color: var(--cb-ink-3);
  letter-spacing: 0.08em; padding: 10px 4px 4px; text-transform: uppercase;
}
.region-row {
  display: flex; align-items: center; gap: 8px; padding: 6px 8px;
  border-radius: 8px; cursor: pointer; border: 2px solid transparent;
}
.region-row:hover { background: var(--cb-cream-2, #f1eee8); }
.region-row.on { background: var(--cb-yellow); border-color: var(--cb-ink); }
.region-code {
  font-size: 11px; font-weight: 800; font-family: Consolas, monospace;
  border: 2px solid var(--cb-ink); border-radius: 6px; padding: 0 6px;
  background: var(--cb-blue); flex: none; min-width: 34px; text-align: center;
}
.region-code.sm { min-width: 0; font-size: 10px; padding: 0 4px; }
.region-name { flex: 1; font-size: 13px; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.region-count { font-size: 12px; font-weight: 800; font-family: Consolas, monospace; }

.list-panel { display: flex; flex-direction: column; height: calc(100vh - 280px); min-height: 400px; }
.filters { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; }
.proxy-table { flex: 1; overflow-y: auto; border: 2.5px solid var(--cb-ink); border-radius: 12px; }
.proxy-table.loading { opacity: 0.6; }
.pt-head, .pt-row {
  display: grid; align-items: center; gap: 8px;
  grid-template-columns: 1.6fr 1.2fr 0.8fr 0.4fr;
  padding: 8px 14px;
}
.pt-head {
  background: var(--cb-yellow); border-bottom: 2.5px solid var(--cb-ink);
  font-weight: 800; font-size: 12px; position: sticky; top: 0; z-index: 1;
}
.pt-row { border-bottom: 1.5px solid #e5e1d8; font-size: 13px; }
.pt-row:last-child { border-bottom: none; }
.pt-row.dead { opacity: 0.5; }
.mono { font-family: Consolas, monospace; }
.proto-chip {
  display: inline-block; font-size: 10px; font-weight: 800;
  border: 1.5px solid var(--cb-ink); border-radius: 5px; padding: 0 5px;
  margin-right: 4px; background: var(--cb-mint);
}
.c-lat.slow { color: #dc2626; font-weight: 700; }
.pager { display: flex; justify-content: center; padding-top: 10px; }

.syncs-panel { }
.sync-row {
  display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
  padding: 8px 4px; border-bottom: 1.5px solid #e5e1d8; font-size: 13px;
}
.sync-row:last-child { border-bottom: none; }
.sync-status {
  font-size: 11px; font-weight: 800; border: 2px solid var(--cb-ink);
  border-radius: 6px; padding: 1px 8px; background: var(--cb-mint);
}
.sync-status.error { background: var(--cb-pink); }
.sync-nums { font-weight: 600; }
.up { color: #15803d; } .dn { color: #dc2626; }
.sync-err { font-size: 12px; color: #dc2626; }
.sync-detail { flex-basis: 100%; padding: 8px 0 4px; }
.detail-title { font-size: 12px; font-weight: 800; margin: 6px 0 2px; }
.detail-ips {
  font-size: 11px; line-height: 1.8; word-break: break-all;
  max-height: 120px; overflow-y: auto;
}
</style>

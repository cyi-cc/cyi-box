<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NButton, NDatePicker, NIcon, NInput, NSelect, useMessage } from 'naive-ui'
import { CopyOutline, TimeOutline } from '@vicons/ionicons5'
import { copyText } from '../../lib/kit'

const message = useMessage()

// ---- 当前时刻 ----
const now = ref(Date.now())
let timer = 0
onMounted(() => { timer = window.setInterval(() => { now.value = Date.now() }, 1000) })
onBeforeUnmount(() => clearInterval(timer))

const unitFactors: Record<string, number> = { s: 1, ms: 1e3, us: 1e6, ns: 1e9 }

// ---- 时区 ----
const tzOptions = [
  { label: '本地时区', value: '' },
  { label: 'UTC', value: 'UTC' },
  { label: 'UTC+8 北京', value: 'Asia/Shanghai' },
  { label: 'UTC+9 东京', value: 'Asia/Tokyo' },
  { label: 'UTC+0 伦敦', value: 'Europe/London' },
  { label: 'UTC-5 纽约', value: 'America/New_York' },
  { label: 'UTC-8 洛杉矶', value: 'America/Los_Angeles' }
]
const tz = ref('')

function fmt(ms: number, timeZone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('zh-CN', {
      timeZone: timeZone || undefined,
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
    }).formatToParts(new Date(ms))
    const g = (t: string) => parts.find(p => p.type === t)?.value ?? ''
    return `${g('year')}-${g('month')}-${g('day')} ${g('hour')}:${g('minute')}:${g('second')}`
  } catch {
    return new Date(ms).toLocaleString()
  }
}

function weekday(ms: number): string {
  return '周' + '日一二三四五六'[new Date(ms).getDay()]
}

function relative(ms: number): string {
  const diff = ms - Date.now()
  const abs = Math.abs(diff)
  const units: [number, string][] = [
    [365 * 86400e3, '年'], [30 * 86400e3, '个月'], [86400e3, '天'],
    [3600e3, '小时'], [60e3, '分钟'], [1e3, '秒']
  ]
  for (const [u, label] of units) {
    if (abs >= u) {
      const n = Math.floor(abs / u)
      return diff > 0 ? `${n} ${label}后` : `${n} ${label}前`
    }
  }
  return '刚刚'
}

// ---- 时间戳 → 日期 ----
const tsInput = ref('')
const tsUnit = ref('auto')
const unitOptions = [
  { label: '自动识别', value: 'auto' },
  { label: '秒 s', value: 's' },
  { label: '毫秒 ms', value: 'ms' },
  { label: '微秒 µs', value: 'us' },
  { label: '纳秒 ns', value: 'ns' }
]

function detectUnit(raw: string): string | null {
  if (!/^-?\d+$/.test(raw)) return null
  const len = raw.replace('-', '').length
  if (len <= 11) return 's'
  if (len <= 14) return 'ms'
  if (len <= 17) return 'us'
  return 'ns'
}

const tsResult = computed(() => {
  const raw = tsInput.value.trim()
  if (!raw) return null
  const unit = tsUnit.value === 'auto' ? detectUnit(raw) : tsUnit.value
  if (!unit) return { error: '请输入纯数字时间戳' }
  const ms = Math.round(Number(raw) / unitFactors[unit] * 1e3)
  if (!Number.isFinite(ms) || Math.abs(ms) > 8.64e15) return { error: '数值超出有效时间范围' }
  const d = new Date(ms)
  return {
    ms,
    local: fmt(ms, ''),
    zoned: fmt(ms, tz.value),
    iso: d.toISOString(),
    utc: fmt(ms, 'UTC'),
    weekday: weekday(ms),
    rel: relative(ms)
  }
})

// ---- 日期 → 时间戳 ----
const dateVal = ref<number | null>(Date.now())
const dateResult = computed(() => {
  if (dateVal.value === null) return null
  const ms = dateVal.value
  return {
    s: Math.floor(ms / 1e3), ms,
    us: ms * 1e3, ns: ms * 1e6,
    iso: new Date(ms).toISOString()
  }
})

// ---- 批量 ----
const batchInput = ref('')
const batchRows = computed(() => {
  return batchInput.value.split('\n').map(l => l.trim()).filter(Boolean).map(raw => {
    const unit = detectUnit(raw)
    if (!unit) return { raw, out: '无效时间戳' }
    const ms = Math.round(Number(raw) / unitFactors[unit] * 1e3)
    return { raw, out: `${fmt(ms, tz.value)}  (${unit})` }
  })
})

async function copy(v: string | number) {
  message.success(await copyText(String(v)) ? '已复制' : '复制失败')
}
</script>

<template>
  <div class="ts-page">
    <div class="panel now-panel">
      <div class="now-left">
        <n-icon :size="20"><TimeOutline /></n-icon>
        <span class="now-label">当前时间</span>
      </div>
      <div class="now-item" @click="copy(Math.floor(now / 1000))">
        <div class="k">Unix 秒</div><div class="v">{{ Math.floor(now / 1000) }}</div>
      </div>
      <div class="now-item" @click="copy(now)">
        <div class="k">毫秒</div><div class="v">{{ now }}</div>
      </div>
      <div class="now-item wide">
        <div class="k">本地</div><div class="v">{{ fmt(now, '') }}</div>
      </div>
      <div class="now-item wide">
        <div class="k">ISO 8601</div><div class="v">{{ new Date(now).toISOString() }}</div>
      </div>
    </div>

    <div class="grid2">
      <div class="panel">
        <div class="panel-title">时间戳 → 日期 <span class="hint">输入数字自动识别单位</span></div>
        <div style="display:flex;gap:10px;margin-bottom:12px">
          <n-input v-model:value="tsInput" placeholder="例如 1727587200 或 1727587200000" style="flex:1" clearable />
          <n-select v-model:value="tsUnit" :options="unitOptions" style="width:130px" />
          <n-select v-model:value="tz" :options="tzOptions" style="width:150px" />
        </div>
        <div v-if="tsResult && 'error' in tsResult" class="ts-error">{{ tsResult.error }}</div>
        <div v-else-if="tsResult" class="kv-table">
          <div class="kv-row" @click="copy(tsResult.zoned)"><span>所选时区</span><b>{{ tsResult.zoned }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(tsResult.local)"><span>本地</span><b>{{ tsResult.local }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(tsResult.utc)"><span>UTC</span><b>{{ tsResult.utc }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(tsResult.iso)"><span>ISO 8601</span><b>{{ tsResult.iso }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row"><span>星期</span><b>{{ tsResult.weekday }}</b></div>
          <div class="kv-row"><span>相对</span><b>{{ tsResult.rel }}</b></div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-title">日期 → 时间戳</div>
        <n-date-picker v-model:value="dateVal" type="datetime" style="width:100%;margin-bottom:12px" clearable />
        <div v-if="dateResult" class="kv-table">
          <div class="kv-row" @click="copy(dateResult.s)"><span>秒 s</span><b>{{ dateResult.s }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(dateResult.ms)"><span>毫秒 ms</span><b>{{ dateResult.ms }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(dateResult.us)"><span>微秒 µs</span><b>{{ dateResult.us }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(dateResult.ns)"><span>纳秒 ns</span><b>{{ dateResult.ns }}</b><n-icon><CopyOutline /></n-icon></div>
          <div class="kv-row" @click="copy(dateResult.iso)"><span>ISO 8601</span><b>{{ dateResult.iso }}</b><n-icon><CopyOutline /></n-icon></div>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">批量转换 <span class="hint">一行一个时间戳</span></div>
      <div class="batch-grid">
        <n-input
          v-model:value="batchInput" type="textarea" :autosize="{ minRows: 5, maxRows: 10 }"
          placeholder="1727587200&#10;1727587200000&#10;1699999999"
        />
        <div class="batch-out">
          <div v-for="(r, i) in batchRows" :key="i" class="kv-row" @click="copy(r.raw)">
            <span class="mono">{{ r.raw }}</span><b>{{ r.out }}</b>
          </div>
          <div v-if="!batchRows.length" class="batch-empty">结果显示在这里</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ts-page { display: flex; flex-direction: column; gap: 14px; }
.now-panel { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
.now-left { display: flex; align-items: center; gap: 8px; font-weight: 900; font-size: 15px; }
.now-item {
  border: 2.5px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-yellow); padding: 6px 14px; cursor: pointer;
  transition: transform 0.12s ease;
}
.now-item:nth-of-type(3) { background: var(--cb-mint); }
.now-item:nth-of-type(4) { background: var(--cb-blue); }
.now-item:nth-of-type(5) { background: var(--cb-purple); }
.now-item:hover { transform: translate(-1px, -1px); }
.now-item .k { font-size: 11px; font-weight: 700; color: var(--cb-ink-2); }
.now-item .v { font-weight: 900; font-size: 15px; font-variant-numeric: tabular-nums; }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
@media (max-width: 900px) { .grid2 { grid-template-columns: 1fr; } }
.kv-table { display: flex; flex-direction: column; gap: 8px; }
.kv-row {
  display: flex; align-items: center; gap: 10px;
  border: 2px solid var(--cb-ink); border-radius: 8px;
  padding: 7px 12px; font-size: 13px; cursor: pointer; background: #fff;
}
.kv-row span { color: var(--cb-ink-3); font-weight: 600; min-width: 70px; }
.kv-row b { flex: 1; font-variant-numeric: tabular-nums; }
.kv-row .mono { font-family: Consolas, monospace; }
.ts-error { color: #dc2626; font-weight: 700; font-size: 13px; }
.batch-grid { display: grid; grid-template-columns: 1fr 1.4fr; gap: 14px; }
.batch-out { display: flex; flex-direction: column; gap: 8px; overflow: auto; max-height: 240px; }
.batch-empty { color: var(--cb-ink-3); font-weight: 600; text-align: center; padding: 40px 0; }
@media (max-width: 900px) { .batch-grid { grid-template-columns: 1fr; } }
</style>

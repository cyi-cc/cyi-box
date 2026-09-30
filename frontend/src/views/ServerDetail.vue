<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton, NDataTable, NEmpty, NIcon, NInput, NModal, NProgress,
  NSpin, NTabPane, NTabs, NText, useDialog, useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  ArrowBackOutline, ArrowDownOutline, ArrowUpOutline, CreateOutline,
  DownloadOutline, FolderOutline, DocumentOutline, FolderOpenOutline,
  HardwareChipOutline, RefreshOutline, ServerOutline, TerminalOutline,
  FileTrayOutline, TrashOutline
} from '@vicons/ionicons5'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { client, readToken } from '../lib/api'
import { useServersStore } from '../stores/servers'
import type serverMetricsView from '../api/serverMetricsView'
import type sftpEntryView from '../api/sftpEntryView'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const store = useServersStore()

const serverId = Number(route.params.id)
const server = computed(() => store.find(serverId))
const tab = ref('overview')

onMounted(async () => {
  if (!store.loaded) await store.refresh()
  void loadMetrics()
})

// ============ 工具函数 ============
function fmtBytes(n: number): string {
  if (n === 0) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log2(n) / 10), u.length - 1)
  const v = n / Math.pow(1024, i)
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${u[i]}`
}
function fmtRate(n: number): string { return fmtBytes(n) + '/s' }
function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d} 天 ${h} 小时`
  if (h > 0) return `${h} 小时 ${m} 分`
  return `${m} 分`
}
function fmtTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false })
}

// ============ 概览：指标 ============
const metrics = ref<serverMetricsView | null>(null)
const metricsErr = ref('')
const metricsLoading = ref(false)
let metricsTimer: ReturnType<typeof setInterval> | null = null

async function loadMetrics() {
  if (metricsLoading.value) return
  metricsLoading.value = true
  try {
    const r = await client.serverSvc.metrics({ id: serverId })
    if (r.status === 0 && r.data) {
      metrics.value = r.data
      metricsErr.value = ''
    } else {
      metricsErr.value = r.msg || '采集失败'
    }
  } finally {
    metricsLoading.value = false
  }
}

onMounted(() => {
  metricsTimer = setInterval(() => {
    if (tab.value === 'overview') void loadMetrics()
  }, 5000)
})
onBeforeUnmount(() => {
  if (metricsTimer) clearInterval(metricsTimer)
  termWs?.close()
  term?.dispose()
})

const memPercent = computed(() => {
  const m = metrics.value
  return m && m.memTotal > 0 ? Math.round((m.memUsed / m.memTotal) * 100) : 0
})
const diskPercent = computed(() => {
  const m = metrics.value
  return m && m.diskTotal > 0 ? Math.round((m.diskUsed / m.diskTotal) * 100) : 0
})

const diskColumns: DataTableColumns<serverMetricsView['disks'][number]> = [
  { title: '挂载点', key: 'mount', render: r => h('code', { style: 'font-weight:700' }, r.mount) },
  { title: '文件系统', key: 'fs', render: r => h(NText, { depth: 3 }, { default: () => r.fs }) },
  {
    title: '用量', key: 'used', width: 240,
    render: r => h('div', { style: 'display:flex;align-items:center;gap:10px' }, [
      h(NProgress, {
        type: 'line', percentage: r.percent, showIndicator: false,
        color: r.percent > 90 ? '#e5484d' : r.percent > 70 ? '#ec5b13' : '#1a1a1a',
        style: 'flex:1', borderRadius: '0', railStyle: 'border:2px solid #1a1a1a;background:#fff'
      }),
      h('span', { style: 'font-size:12px;font-weight:700;width:38px' }, `${r.percent}%`)
    ])
  },
  { title: '已用 / 总量', key: 'size', align: 'right', render: r => `${fmtBytes(r.used)} / ${fmtBytes(r.total)}` }
]

// ============ 文件：SFTP ============
const cwd = ref('/')
const entries = ref<sftpEntryView[]>([])
const filesLoading = ref(false)

const crumbs = computed(() => {
  const parts = cwd.value.split('/').filter(Boolean)
  const list = [{ name: '/', path: '/' }]
  let acc = ''
  for (const p of parts) {
    acc += '/' + p
    list.push({ name: p, path: acc })
  }
  return list
})

async function loadDir(path?: string) {
  if (path !== undefined) cwd.value = path
  filesLoading.value = true
  try {
    const r = await client.serverSvc.sftpList({ id: serverId, path: cwd.value })
    if (r.status === 0 && r.data) {
      cwd.value = r.data.path
      entries.value = [...r.data.entries].sort((a, b) =>
        Number(b.isDir) - Number(a.isDir) || a.name.localeCompare(b.name))
    } else {
      message.error(r.msg || '读取目录失败')
    }
  } finally {
    filesLoading.value = false
  }
}

function onEntryClick(e: sftpEntryView) {
  if (e.isDir) void loadDir(join(cwd.value, e.name))
}

function join(dir: string, name: string): string {
  return (dir.endsWith('/') ? dir : dir + '/') + name
}

function download(e: sftpEntryView) {
  const p = encodeURIComponent(join(cwd.value, e.name))
  const url = `/api/files/download?id=${serverId}&path=${p}&token=${encodeURIComponent(readToken())}`
  const a = document.createElement('a')
  a.href = url
  a.download = e.name
  a.click()
}

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)

function pickUpload() { fileInput.value?.click() }

async function doUpload(ev: Event) {
  const files = (ev.target as HTMLInputElement).files
  if (!files?.length) return
  uploading.value = true
  try {
    for (const f of Array.from(files)) {
      const fd = new FormData()
      fd.append('file', f, f.name)
      const resp = await fetch(
        `/api/files/upload?id=${serverId}&dir=${encodeURIComponent(cwd.value)}&token=${encodeURIComponent(readToken())}`,
        { method: 'POST', body: fd })
      const j = await resp.json().catch(() => null)
      if (!resp.ok || j?.status !== 0) {
        message.error(`上传 ${f.name} 失败`)
      } else {
        message.success(`已上传 ${f.name}`)
      }
    }
    await loadDir()
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

const mkdirVisible = ref(false)
const mkdirName = ref('')
async function doMkdir() {
  const name = mkdirName.value.trim()
  if (!name || name.includes('/')) {
    message.warning('目录名无效')
    return
  }
  const r = await client.serverSvc.sftpMkdir({ id: serverId, path: join(cwd.value, name) })
  if (r.status === 0) {
    message.success('已创建')
    mkdirVisible.value = false
    mkdirName.value = ''
    void loadDir()
  } else {
    message.error(r.msg || '创建失败')
  }
}

const renameVisible = ref(false)
const renameTarget = ref<sftpEntryView | null>(null)
const renameValue = ref('')
function openRename(e: sftpEntryView) {
  renameTarget.value = e
  renameValue.value = e.name
  renameVisible.value = true
}
async function doRename() {
  const t = renameTarget.value
  const name = renameValue.value.trim()
  if (!t || !name) return
  const r = await client.serverSvc.sftpRename({ id: serverId, path: join(cwd.value, t.name), newName: name })
  if (r.status === 0) {
    message.success('已重命名')
    renameVisible.value = false
    void loadDir()
  } else {
    message.error(r.msg || '重命名失败')
  }
}

function removeEntry(e: sftpEntryView) {
  dialog.warning({
    title: '删除',
    content: `确定删除 ${e.isDir ? '目录' : '文件'}「${e.name}」吗？目录需为空。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.serverSvc.sftpRemove({ id: serverId, path: join(cwd.value, e.name) })
      if (r.status === 0) {
        message.success('已删除')
        void loadDir()
      } else {
        message.error(r.msg || '删除失败')
      }
    }
  })
}

const fileColumns: DataTableColumns<sftpEntryView> = [
  {
    title: '名称', key: 'name',
    render: e => h('div', {
      style: 'display:flex;align-items:center;gap:8px;font-weight:' + (e.isDir ? '700' : '500') +
        (e.isDir ? ';cursor:pointer' : ''),
      onClick: () => onEntryClick(e)
    }, [
      h(NIcon, { size: 17, color: e.isDir ? '#ec5b13' : '#8a90a6' },
        { default: () => h(e.isDir ? FolderOutline : DocumentOutline) }),
      h('span', e.name)
    ])
  },
  { title: '大小', key: 'size', width: 110, align: 'right', render: e => e.isDir ? '—' : fmtBytes(e.size) },
  { title: '权限', key: 'mode', width: 110, render: e => h('code', { style: 'font-size:11px' }, e.mode) },
  { title: '修改时间', key: 'modTime', width: 170, render: e => h(NText, { depth: 3 }, { default: () => fmtTime(e.modTime) }) },
  {
    title: '', key: 'ops', width: 120, align: 'right',
    render: e => h('div', { style: 'display:flex;gap:2px;justify-content:flex-end' }, [
      !e.isDir ? h(NButton, { size: 'tiny', quaternary: true, onClick: () => download(e) },
        { icon: () => h(NIcon, { component: DownloadOutline }) }) : null,
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => openRename(e) },
        { icon: () => h(NIcon, { component: CreateOutline }) }),
      h(NButton, { size: 'tiny', quaternary: true, type: 'error', onClick: () => removeEntry(e) },
        { icon: () => h(NIcon, { component: TrashOutline }) })
    ])
  }
]

// ============ 终端：xterm + websocket ============
const termEl = ref<HTMLElement | null>(null)
const termMounted = ref(false)
const termStatus = ref<'idle' | 'connecting' | 'open' | 'closed' | 'error'>('idle')
const termErr = ref('')
let term: Terminal | null = null
let termWs: WebSocket | null = null
let fitAddon: FitAddon | null = null
let resizeHandler: (() => void) | null = null

function sendResize() {
  if (term && termWs?.readyState === WebSocket.OPEN) {
    termWs.send('\x01' + JSON.stringify({ cols: term.cols, rows: term.rows }))
  }
}

function onTabChange(v: string) {
  tab.value = v
  if (v === 'files' && !entries.value.length) void loadDir('/')
  if (v === 'terminal' && !termMounted.value) {
    termMounted.value = true
    requestAnimationFrame(() => void initTerm())
  }
}

async function initTerm() {
  if (!termEl.value) return
  termStatus.value = 'connecting'
  term = new Terminal({
    fontFamily: '"JetBrains Mono", "Fira Code", Consolas, monospace',
    fontSize: 13,
    cursorBlink: true,
    theme: {
      background: '#14141c', foreground: '#e8e8ec', cursor: '#ec5b13',
      selectionBackground: '#3d3d4d'
    }
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(termEl.value)
  fitAddon.fit()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${location.host}/api/ws/terminal?id=${serverId}&token=${encodeURIComponent(readToken())}`
  const ws = new WebSocket(url)
  termWs = ws
  ws.binaryType = 'arraybuffer'
  ws.onopen = () => {
    termStatus.value = 'open'
    fitAddon?.fit()
    sendResize()
  }
  ws.onmessage = ev => {
    if (typeof ev.data === 'string') {
      term?.write(ev.data)
    } else {
      term?.write(new Uint8Array(ev.data))
    }
  }
  ws.onclose = () => {
    termStatus.value = 'closed'
    term?.write('\r\n\x1b[33m[连接已断开]\x1b[0m\r\n')
  }
  ws.onerror = () => {
    termStatus.value = 'error'
    termErr.value = '连接失败'
  }
  term.onData(d => { if (ws.readyState === WebSocket.OPEN) ws.send(d) })
  resizeHandler = () => { fitAddon?.fit(); sendResize() }
  window.addEventListener('resize', resizeHandler)
}

function reconnectTerm() {
  termWs?.close()
  term?.dispose()
  term = null
  termErr.value = ''
  requestAnimationFrame(() => void initTerm())
}

onBeforeUnmount(() => {
  if (resizeHandler) window.removeEventListener('resize', resizeHandler)
})
</script>

<template>
  <div>
    <div class="svd-head">
      <n-button quaternary @click="router.push('/servers')">
        <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
        返回
      </n-button>
      <div class="brand-badge" style="width:38px;height:38px;font-size:18px;margin:0 12px 0 4px">
        <n-icon><ServerOutline /></n-icon>
      </div>
      <div>
        <div style="font-weight:900;font-size:17px">{{ server?.name || '服务器' }}</div>
        <div style="font-size:12px;color:var(--cb-ink-2)">
          {{ server ? `${server.username}@${server.host}:${server.port}` : '' }}
        </div>
      </div>
    </div>

    <n-tabs :value="tab" type="line" @update:value="onTabChange" class="svd-tabs">
      <n-tab-pane name="overview" tab="概览">
        <div v-if="metricsErr && !metrics" class="panel" style="padding:40px;text-align:center">
          <n-icon :size="40" color="#e5484d"><ServerOutline /></n-icon>
          <div style="font-weight:700;margin:12px 0 6px">连不上这台服务器</div>
          <div style="color:var(--cb-ink-2);font-size:13px">{{ metricsErr }}</div>
          <n-button style="margin-top:16px" :loading="metricsLoading" @click="loadMetrics">重试</n-button>
        </div>
        <template v-else>
          <n-spin :show="metricsLoading && !metrics">
            <div class="metric-grid">
              <div class="panel metric-card">
                <div class="metric-label"><n-icon><HardwareChipOutline /></n-icon> CPU</div>
                <div class="metric-big">{{ metrics?.cpuPercent ?? '—' }}<small>%</small></div>
                <n-progress type="line" :percentage="metrics?.cpuPercent ?? 0" :show-indicator="false"
                  :color="(metrics?.cpuPercent ?? 0) > 80 ? '#e5484d' : '#ec5b13'"
                  rail-style="border:2px solid #1a1a1a;background:#fff" :border-radius="0" />
                <div class="metric-sub">load {{ metrics?.load1 || '-' }} / {{ metrics?.load5 || '-' }} / {{ metrics?.load15 || '-' }}</div>
              </div>
              <div class="panel metric-card">
                <div class="metric-label"><n-icon><HardwareChipOutline /></n-icon> 内存</div>
                <div class="metric-big">{{ metrics ? memPercent : '—' }}<small>%</small></div>
                <n-progress type="line" :percentage="memPercent" :show-indicator="false"
                  :color="memPercent > 85 ? '#e5484d' : '#3b82f6'"
                  rail-style="border:2px solid #1a1a1a;background:#fff" :border-radius="0" />
                <div class="metric-sub">{{ metrics ? `${fmtBytes(metrics.memUsed)} / ${fmtBytes(metrics.memTotal)}` : '' }}</div>
              </div>
              <div class="panel metric-card">
                <div class="metric-label"><n-icon><FileTrayOutline /></n-icon> 磁盘 /</div>
                <div class="metric-big">{{ metrics ? diskPercent : '—' }}<small>%</small></div>
                <n-progress type="line" :percentage="diskPercent" :show-indicator="false"
                  :color="diskPercent > 90 ? '#e5484d' : '#16a34a'"
                  rail-style="border:2px solid #1a1a1a;background:#fff" :border-radius="0" />
                <div class="metric-sub">{{ metrics ? `剩 ${fmtBytes(metrics.diskAvail)} / ${fmtBytes(metrics.diskTotal)}` : '' }}</div>
              </div>
              <div class="panel metric-card">
                <div class="metric-label"><n-icon><TerminalOutline /></n-icon> 网络 IO</div>
                <div class="metric-net">
                  <span><n-icon color="#16a34a"><ArrowDownOutline /></n-icon>{{ metrics ? fmtRate(metrics.netRxRate) : '—' }}</span>
                  <span><n-icon color="#ec5b13"><ArrowUpOutline /></n-icon>{{ metrics ? fmtRate(metrics.netTxRate) : '—' }}</span>
                </div>
                <div class="metric-sub">累计 ↓{{ metrics ? fmtBytes(metrics.netRxTotal) : '—' }} ↑{{ metrics ? fmtBytes(metrics.netTxTotal) : '—' }}</div>
              </div>
            </div>

            <div class="panel" style="padding:16px 20px;margin-top:16px;display:flex;gap:28px;flex-wrap:wrap;align-items:center">
              <div class="info-item"><label>主机名</label><b>{{ metrics?.hostname || '—' }}</b></div>
              <div class="info-item"><label>系统</label><b>{{ metrics?.os || '—' }}</b></div>
              <div class="info-item"><label>内核</label><b>{{ metrics?.kernel || '—' }}</b></div>
              <div class="info-item"><label>已运行</label><b>{{ metrics ? fmtUptime(metrics.uptimeSeconds) : '—' }}</b></div>
              <div class="info-item"><label>交换分区</label><b>{{ metrics && metrics.swapTotal > 0 ? `${fmtBytes(metrics.swapUsed)} / ${fmtBytes(metrics.swapTotal)}` : '—' }}</b></div>
              <div style="flex:1"></div>
              <n-button size="small" :loading="metricsLoading" @click="loadMetrics">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>
                刷新
              </n-button>
            </div>

            <div v-if="metrics && metrics.disks.length > 1" class="panel" style="margin-top:16px;overflow:hidden">
              <n-data-table :columns="diskColumns" :data="metrics.disks" size="small" :bordered="false" :single-line="false" />
            </div>
          </n-spin>
        </template>
      </n-tab-pane>

      <n-tab-pane name="files" tab="文件">
        <div class="panel" style="padding:14px 16px;margin-bottom:14px;display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <n-icon :size="17" color="#ec5b13"><FolderOpenOutline /></n-icon>
          <template v-for="(c, i) in crumbs" :key="c.path">
            <span v-if="i > 0" style="color:#b6bccd">/</span>
            <a class="crumb" @click="loadDir(c.path)">{{ c.name === '/' ? 'root' : c.name }}</a>
          </template>
          <div style="flex:1"></div>
          <n-button size="small" @click="mkdirVisible = true">新建目录</n-button>
          <n-button size="small" :loading="uploading" @click="pickUpload">上传</n-button>
          <n-button size="small" quaternary @click="loadDir()">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>
          </n-button>
          <input ref="fileInput" type="file" multiple style="display:none" @change="doUpload" />
        </div>
        <div class="panel" style="overflow:hidden">
          <n-data-table :columns="fileColumns" :data="entries" :loading="filesLoading"
            size="small" :bordered="false" :single-line="false" />
        </div>
        <n-empty v-if="!filesLoading && !entries.length" description="空目录" style="padding:40px" />
      </n-tab-pane>

      <n-tab-pane name="terminal" tab="终端">
        <div class="panel term-panel">
          <div class="term-bar">
            <span class="term-dot" :class="termStatus"></span>
            <span style="font-size:12px;font-weight:700">
              {{ { idle: '未连接', connecting: '连接中…', open: '已连接', closed: '已断开', error: '连接失败' }[termStatus] }}
            </span>
            <span v-if="termErr" style="font-size:12px;color:#e5484d">{{ termErr }}</span>
            <div style="flex:1"></div>
            <n-button v-if="termStatus === 'closed' || termStatus === 'error'" size="tiny" @click="reconnectTerm">重连</n-button>
          </div>
          <div ref="termEl" class="term-box" v-show="termMounted"></div>
        </div>
      </n-tab-pane>
    </n-tabs>

    <n-modal v-model:show="mkdirVisible" preset="card" title="新建目录" style="width: 380px">
      <n-input v-model:value="mkdirName" placeholder="目录名" @keyup.enter="doMkdir" />
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:10px">
          <n-button @click="mkdirVisible = false">取消</n-button>
          <n-button type="primary" @click="doMkdir">创建</n-button>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="renameVisible" preset="card" title="重命名" style="width: 380px">
      <n-input v-model:value="renameValue" placeholder="新名字" @keyup.enter="doRename" />
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:10px">
          <n-button @click="renameVisible = false">取消</n-button>
          <n-button type="primary" @click="doRename">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.svd-head { display: flex; align-items: center; margin-bottom: 16px; }
.svd-tabs :deep(.n-tabs-tab) { font-weight: 700; }
.metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px; }
.metric-card { padding: 18px 20px; }
.metric-label { display: flex; align-items: center; gap: 6px; font-size: 12px; font-weight: 800; color: var(--cb-ink-2); letter-spacing: 0.4px; }
.metric-big { font-size: 34px; font-weight: 900; margin: 8px 0 10px; font-variant-numeric: tabular-nums; }
.metric-big small { font-size: 16px; font-weight: 800; margin-left: 2px; }
.metric-sub { font-size: 12px; color: var(--cb-ink-2); margin-top: 8px; font-variant-numeric: tabular-nums; }
.metric-net { display: flex; flex-direction: column; gap: 6px; font-size: 20px; font-weight: 900; margin: 8px 0 10px; font-variant-numeric: tabular-nums; }
.metric-net span { display: flex; align-items: center; gap: 6px; }
.info-item { display: flex; flex-direction: column; gap: 2px; }
.info-item label { font-size: 11px; color: var(--cb-ink-2); font-weight: 700; }
.info-item b { font-size: 13px; font-weight: 800; }
.crumb { font-weight: 700; cursor: pointer; color: #1a1a1a; }
.crumb:hover { color: #ec5b13; }
.term-panel { overflow: hidden; }
.term-bar { display: flex; align-items: center; gap: 8px; padding: 8px 14px; border-bottom: 2.5px solid #1a1a1a; background: #fff; }
.term-dot { width: 9px; height: 9px; border-radius: 50%; background: #b6bccd; border: 1.5px solid #1a1a1a; }
.term-dot.open { background: #16a34a; }
.term-dot.connecting { background: #fef08a; }
.term-dot.error, .term-dot.closed { background: #e5484d; }
.term-box { height: 480px; background: #14141c; padding: 8px; }
</style>

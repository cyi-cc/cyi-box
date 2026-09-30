<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import {
  NButton, NIcon, NInput, NSelect, NTag, useMessage
} from 'naive-ui'
import {
  AddOutline, SendOutline, TrashOutline, TimeOutline,
  CopyOutline, FlashOutline
} from '@vicons/ionicons5'
import { client } from '../../lib/api'
import { copyText, highlightJson } from '../../lib/kit'

const message = useMessage()

// ---- 请求编辑 ----
const method = ref('GET')
const url = ref('')
const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
  .map(v => ({ label: v, value: v }))

type KV = { k: string; v: string }
const params = reactive<KV[]>([{ k: '', v: '' }])
const headers = reactive<KV[]>([{ k: 'Content-Type', v: 'application/json' }, { k: '', v: '' }])
const bodyType = ref('none')
const body = ref('')
const bodyTypeOptions = [
  { label: '无 Body', value: 'none' },
  { label: 'JSON', value: 'json' },
  { label: 'Text', value: 'text' },
  { label: 'Form 表单', value: 'form' }
]

const tab = ref<'params' | 'headers' | 'body'>('params')

function addRow(list: KV[]) { list.push({ k: '', v: '' }) }
function delRow(list: KV[], i: number) { list.splice(i, 1); if (!list.length) addRow(list) }

const finalUrl = computed(() => {
  const u = url.value.trim()
  const qs = params.filter(p => p.k.trim()).map(p => `${encodeURIComponent(p.k.trim())}=${encodeURIComponent(p.v)}`).join('&')
  if (!qs) return u
  return u + (u.includes('?') ? '&' : '?') + qs
})

// ---- 响应 ----
const sending = ref(false)
const resp = ref<{ status: number; elapsedMs: number; headers: [string, string][]; body: string; truncated: boolean } | null>(null)
const respTab = ref<'body' | 'headers'>('body')
const pretty = ref(true)

const statusColor = computed(() => {
  const s = resp.value?.status ?? 0
  if (s >= 200 && s < 300) return '#a7f3d0'
  if (s >= 300 && s < 400) return '#bae6fd'
  if (s >= 400 && s < 500) return '#fef08a'
  return '#ffc1cc'
})

const respBodyView = computed(() => {
  if (!resp.value) return ''
  if (pretty.value) {
    try {
      return highlightJson(JSON.stringify(JSON.parse(resp.value.body), null, 2))
    } catch { /* 非 JSON 原样显示 */ }
  }
  return ''
})

// ---- 历史（localStorage，最近 15 条）----
const HISTORY_KEY = 'cyibox_postman_history'
type HistoryItem = { method: string; url: string; params: KV[]; headers: KV[]; bodyType: string; body: string; at: number }
const history = ref<HistoryItem[]>(JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]'))

function pushHistory() {
  history.value.unshift({
    method: method.value, url: url.value,
    params: params.filter(p => p.k.trim()).map(p => ({ ...p })),
    headers: headers.filter(h => h.k.trim()).map(h => ({ ...h })),
    bodyType: bodyType.value, body: body.value, at: Date.now()
  })
  history.value = history.value.slice(0, 15)
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value))
}

function loadHistory(h: HistoryItem) {
  method.value = h.method
  url.value = h.url
  params.splice(0, params.length, ...(h.params.length ? h.params.map(p => ({ ...p })) : [{ k: '', v: '' }]))
  headers.splice(0, headers.length, ...(h.headers.length ? h.headers.map(x => ({ ...x })) : [{ k: '', v: '' }]))
  bodyType.value = h.bodyType
  body.value = h.body
}

async function send() {
  if (!url.value.trim()) {
    message.warning('先填 URL')
    return
  }
  let u = finalUrl.value
  if (!/^https?:\/\//.test(u)) u = 'https://' + u
  const hdr: Record<string, string> = {}
  for (const h of headers) if (h.k.trim()) hdr[h.k.trim()] = h.v
  let payload = ''
  if (bodyType.value === 'json' || bodyType.value === 'text') payload = body.value
  if (bodyType.value === 'form') {
    payload = body.value.split('\n').map(l => {
      const i = l.indexOf('=')
      return i < 0 ? encodeURIComponent(l.trim()) + '=' : `${encodeURIComponent(l.slice(0, i).trim())}=${encodeURIComponent(l.slice(i + 1))}`
    }).join('&')
    if (!hdr['Content-Type']) hdr['Content-Type'] = 'application/x-www-form-urlencoded'
  }
  sending.value = true
  try {
    const r = await client.webSvc.fetch({
      method: method.value, url: u,
      headersJson: JSON.stringify(hdr),
      body: payload
    })
    if (r.status === 0 && r.data) {
      resp.value = {
        status: r.data.status,
        elapsedMs: r.data.elapsedMs,
        headers: Object.entries(JSON.parse(r.data.headersJson || '{}')),
        body: r.data.body,
        truncated: r.data.truncated
      }
      pushHistory()
    } else {
      message.error(r.msg || '请求失败')
    }
  } finally {
    sending.value = false
  }
}

function loadExample() {
  method.value = 'GET'
  url.value = 'https://api.github.com/zen'
  headers.splice(0, headers.length, { k: 'Accept', v: 'text/plain' }, { k: '', v: '' })
}
</script>

<template>
  <div class="pm-page">
    <div class="pm-main">
      <div class="panel">
        <div style="display:flex;gap:10px">
          <n-select v-model:value="method" :options="methodOptions" style="width:120px" />
          <n-input v-model:value="url" placeholder="https://api.example.com/path" style="flex:1" @keyup.enter="send" />
          <n-button type="primary" :loading="sending" @click="send">
            <template #icon><n-icon><SendOutline /></n-icon></template>发送
          </n-button>
          <n-button @click="loadExample">
            <template #icon><n-icon><FlashOutline /></n-icon></template>示例
          </n-button>
        </div>

        <div class="pm-tabs">
          <div v-for="t in (['params', 'headers', 'body'] as const)" :key="t"
            class="pm-tab" :class="{ on: tab === t }" @click="tab = t">
            {{ t === 'params' ? 'Params' : t === 'headers' ? 'Headers' : 'Body' }}
            <n-tag v-if="t === 'params' && params.filter(p => p.k).length" size="tiny" round :bordered="false">{{ params.filter(p => p.k).length }}</n-tag>
            <n-tag v-if="t === 'headers' && headers.filter(h => h.k).length" size="tiny" round :bordered="false">{{ headers.filter(h => h.k).length }}</n-tag>
          </div>
        </div>

        <div v-if="tab === 'params'" class="kv-edit">
          <div v-for="(p, i) in params" :key="i" class="kv-line">
            <n-input v-model:value="p.k" placeholder="key" size="small" style="width:220px" />
            <n-input v-model:value="p.v" placeholder="value" size="small" style="flex:1" />
            <n-button size="small" quaternary @click="delRow(params, i)"><template #icon><n-icon><TrashOutline /></n-icon></template></n-button>
          </div>
          <n-button size="small" @click="addRow(params)"><template #icon><n-icon><AddOutline /></n-icon></template>加一行</n-button>
          <div class="url-preview">最终 URL：<span class="mono">{{ finalUrl || '—' }}</span></div>
        </div>

        <div v-else-if="tab === 'headers'" class="kv-edit">
          <div v-for="(h, i) in headers" :key="i" class="kv-line">
            <n-input v-model:value="h.k" placeholder="Header" size="small" style="width:220px" />
            <n-input v-model:value="h.v" placeholder="Value" size="small" style="flex:1" />
            <n-button size="small" quaternary @click="delRow(headers, i)"><template #icon><n-icon><TrashOutline /></n-icon></template></n-button>
          </div>
          <n-button size="small" @click="addRow(headers)"><template #icon><n-icon><AddOutline /></n-icon></template>加一行</n-button>
        </div>

        <div v-else class="kv-edit">
          <n-select v-model:value="bodyType" :options="bodyTypeOptions" style="width:180px" />
          <n-input
            v-if="bodyType !== 'none'" v-model:value="body" type="textarea"
            :autosize="{ minRows: 6, maxRows: 14 }"
            :placeholder="bodyType === 'form' ? 'key=value，一行一个' : bodyType === 'json' ? '{ &quot;key&quot;: &quot;value&quot; }' : '文本内容'"
          />
        </div>
      </div>

      <div v-if="resp" class="panel">
        <div class="resp-head">
          <span class="status-chip" :style="{ background: statusColor }">{{ resp.status }}</span>
          <span class="resp-meta">{{ resp.elapsedMs }} ms · {{ resp.body.length < 1024 ? resp.body.length + ' B' : (resp.body.length / 1024).toFixed(1) + ' KB' }}<template v-if="resp.truncated"> · 已截断</template></span>
          <div style="flex:1"></div>
          <div class="pm-tab" :class="{ on: respTab === 'body' }" @click="respTab = 'body'">Body</div>
          <div class="pm-tab" :class="{ on: respTab === 'headers' }" @click="respTab = 'headers'">Headers</div>
          <n-button v-if="respTab === 'body' && respBodyView" size="tiny" :type="pretty ? 'primary' : 'default'" @click="pretty = !pretty">美化</n-button>
          <n-button size="tiny" quaternary @click="copyText(resp.body).then(ok => message[ok ? 'success' : 'error'](ok ? '已复制' : '失败'))">
            <template #icon><n-icon><CopyOutline /></n-icon></template>
          </n-button>
        </div>
        <pre v-if="respTab === 'body' && respBodyView && pretty" class="resp-body" v-html="respBodyView"></pre>
        <pre v-else-if="respTab === 'body'" class="resp-body">{{ resp.body }}</pre>
        <div v-else class="kv-edit" style="margin-top:10px">
          <div v-for="[k, v] in resp.headers" :key="k" class="hdr-line">
            <span class="hdr-k">{{ k }}</span><span class="mono">{{ v }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="panel pm-side">
      <div class="panel-title"><n-icon style="vertical-align:-2px"><TimeOutline /></n-icon> 历史</div>
      <div v-if="history.length" class="hist-list">
        <div v-for="(h, i) in history" :key="i" class="hist-item" @click="loadHistory(h)">
          <span class="hist-method" :class="'m-' + h.method.toLowerCase()">{{ h.method }}</span>
          <span class="hist-url">{{ h.url }}</span>
        </div>
      </div>
      <div v-else class="hist-empty">发过的请求会留在这里</div>
    </div>
  </div>
</template>

<style scoped>
.pm-page { display: grid; grid-template-columns: 1fr 300px; gap: 14px; align-items: start; }
@media (max-width: 1000px) { .pm-page { grid-template-columns: 1fr; } }
.pm-main { display: flex; flex-direction: column; gap: 14px; }
.pm-tabs { display: flex; gap: 8px; margin: 14px 0 10px; }
.pm-tab {
  border: 2px solid var(--cb-ink); border-radius: 8px; padding: 4px 14px;
  font-size: 13px; font-weight: 800; cursor: pointer; background: #fff;
  display: flex; align-items: center; gap: 6px;
}
.pm-tab.on { background: var(--cb-yellow); }
.kv-edit { display: flex; flex-direction: column; gap: 8px; }
.kv-line { display: flex; gap: 8px; align-items: center; }
.url-preview { font-size: 12px; color: var(--cb-ink-3); font-weight: 600; margin-top: 4px; }
.mono { font-family: Consolas, monospace; word-break: break-all; }
.resp-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.status-chip {
  border: 2.5px solid var(--cb-ink); border-radius: 8px;
  padding: 3px 14px; font-weight: 900; font-size: 15px;
}
.resp-meta { font-size: 13px; color: var(--cb-ink-2); font-weight: 600; }
.resp-body {
  margin: 12px 0 0; padding: 14px; border: 2px solid var(--cb-ink);
  border-radius: 10px; background: #fff; max-height: 480px; overflow: auto;
  font-family: Consolas, monospace; font-size: 12.5px; line-height: 1.7;
  white-space: pre-wrap; word-break: break-all;
}
.resp-body :deep(.j-key) { color: #7c3aed; font-weight: 700; }
.resp-body :deep(.j-str) { color: #059669; }
.resp-body :deep(.j-num) { color: #ea580c; }
.resp-body :deep(.j-bool) { color: #2563eb; font-weight: 700; }
.resp-body :deep(.j-null) { color: #8a857e; font-style: italic; }
.hdr-line { display: flex; gap: 10px; font-size: 12.5px; border-bottom: 1px dashed #e5e0da; padding: 4px 0; }
.hdr-k { font-weight: 800; min-width: 180px; }
.hist-list { display: flex; flex-direction: column; gap: 6px; }
.hist-item {
  display: flex; align-items: center; gap: 8px; cursor: pointer;
  border: 2px solid var(--cb-ink); border-radius: 8px; padding: 5px 8px; background: #fff;
}
.hist-item:hover { background: var(--cb-yellow); }
.hist-method { font-size: 11px; font-weight: 900; min-width: 48px; color: var(--cb-primary); }
.hist-url { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hist-empty { color: var(--cb-ink-3); font-size: 12px; font-weight: 600; text-align: center; padding: 24px 0; }
</style>

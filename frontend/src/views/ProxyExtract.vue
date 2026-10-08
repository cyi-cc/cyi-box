<script setup lang="ts">
import { computed, onActivated, onMounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NSelect, NEmpty, NModal, NForm, NFormItem,
  useDialog, useMessage
} from 'naive-ui'
import {
  KeyOutline, AddOutline, TrashOutline, CopyOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type proxyKeyView from '../api/proxyKeyView'
import type proxyRegionView from '../api/proxyRegionView'

const message = useMessage()
const dialog = useDialog()

const keys = ref<proxyKeyView[]>([])
const regions = ref<proxyRegionView[]>([])

const createVisible = ref(false)
const creating = ref(false)
const form = ref({ name: '', regionId: 0 })

const regionOpts = computed(() => [
  { label: '全部地区', value: 0 },
  ...regions.value.map(r => ({ label: `${r.code} ${r.zhName || r.name}`, value: r.id }))
])

async function refresh() {
  const r = await client.proxySvc.listKeys()
  if (r.status === 0) keys.value = r.data ?? []
  const rr = await client.proxySvc.regions()
  if (rr.status === 0) regions.value = rr.data ?? []
}
onMounted(refresh)
onActivated(refresh)

async function openCreate() {
  createVisible.value = true
  const rr = await client.proxySvc.regions()
  if (rr.status === 0) regions.value = rr.data ?? []
}

async function create() {
  creating.value = true
  try {
    const r = await client.proxySvc.createKey({
      name: form.value.name, regionId: form.value.regionId
    })
    if (r.status === 0) {
      message.success('密钥已生成')
      createVisible.value = false
      form.value = { name: '', regionId: 0 }
      void refresh()
    } else {
      message.error(r.msg || '创建失败')
    }
  } finally {
    creating.value = false
  }
}

const withScheme = ref(false)

function extractUrl(k: proxyKeyView): string {
  return `${location.origin}/api/v1/proxy?key=${k.key}${withScheme.value ? '&fmt=url' : ''}`
}

async function copyUrl(k: proxyKeyView) {
  message.success(await copyText(extractUrl(k)) ? '链接已复制' : '复制失败')
}

function removeKey(k: proxyKeyView) {
  dialog.warning({
    title: '删除提取密钥',
    content: `「${k.name || k.key}」将立即失效，使用该密钥的提取链接全部作废`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.proxySvc.deleteKey({ id: k.id })
      if (r.status === 0) { message.success('已删除'); void refresh() }
      else message.error(r.msg || '删除失败')
    }
  })
}

function fmtTime(ts: number): string {
  if (!ts) return '从未'
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

function regionLabel(k: proxyKeyView): string {
  if (!k.regionId) return '全部地区'
  return `${k.regionCode} ${k.regionZhName}`.trim()
}
</script>

<template>
  <div class="panel">
    <div class="panel-title">
      提取密钥 <span class="hint">{{ keys.length }} 个</span>
      <span class="fmt-toggle">带协议前缀 <n-switch v-model:value="withScheme" size="small" /></span>
      <n-button size="small" type="primary" style="margin-left:auto" @click="openCreate">
        <template #icon><n-icon><AddOutline /></n-icon></template>生成密钥
      </n-button>
    </div>

    <div v-if="keys.length" class="key-list">
      <div v-for="k in keys" :key="k.id" class="key-row">
        <div class="key-ic"><n-icon :size="18"><KeyOutline /></n-icon></div>
        <div class="key-meta">
          <div class="key-name">{{ k.name || '未命名' }} <span class="key-region">{{ regionLabel(k) }}</span></div>
          <div class="key-str mono">{{ k.key }}</div>
        </div>
        <div class="key-stat">
          <div>使用 {{ k.usedCount }} 次</div>
          <div class="dim">{{ fmtTime(k.lastUsedAt) }}</div>
        </div>
        <n-button size="small" @click="copyUrl(k)">
          <template #icon><n-icon><CopyOutline /></n-icon></template>复制链接
        </n-button>
        <n-button size="small" quaternary @click="removeKey(k)">
          <template #icon><n-icon><TrashOutline /></n-icon></template>
        </n-button>
      </div>
    </div>
    <n-empty v-else description="还没有密钥，点右上角生成" style="padding:60px 0" />

    <div class="usage panel-inset">
      <b>使用方式</b>
      <div class="mono usage-line">GET /api/v1/proxy?key=&lt;密钥&gt;{{ withScheme ? '&fmt=url' : '' }}</div>
      <div class="dim">每次请求实时探活，返回一个可用的 <span class="mono">ip:port</span>（纯文本）；加 <span class="mono">&amp;fmt=url</span> 返回带协议的 <span class="mono">scheme://ip:port</span>（socks5h/http/socks4）。地区由密钥绑定，「全部地区」密钥随机返回任意国家。</div>
    </div>

    <n-modal v-model:show="createVisible" preset="card" title="生成提取密钥" style="width: 420px">
      <n-form>
        <n-form-item label="备注名">
          <n-input v-model:value="form.name" placeholder="如：爬虫专用" />
        </n-form-item>
        <n-form-item label="地区">
          <n-select
            v-model:value="form.regionId" :options="regionOpts"
            filterable placeholder="全部地区"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:8px">
          <n-button @click="createVisible = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="create">生成</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.key-list { display: flex; flex-direction: column; gap: 8px; }
.key-row {
  display: flex; align-items: center; gap: 12px;
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  padding: 10px 14px; background: #fff;
}
.key-ic {
  width: 36px; height: 36px; border: 2.5px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-mint); display: flex; align-items: center; justify-content: center; flex: none;
}
.key-meta { flex: 1; min-width: 0; }
.key-name { font-weight: 800; font-size: 14px; }
.key-region {
  font-size: 11px; font-weight: 800; border: 2px solid var(--cb-ink);
  border-radius: 6px; padding: 0 6px; background: var(--cb-yellow); margin-left: 6px;
}
.key-str { font-size: 12px; color: var(--cb-ink-3); }
.key-stat { font-size: 12px; font-weight: 700; text-align: right; flex: none; min-width: 90px; }
.mono { font-family: Consolas, monospace; }
.dim { color: var(--cb-ink-3); font-weight: 500; }
.fmt-toggle { display: inline-flex; align-items: center; gap: 6px; margin-left: 14px; font-size: 12px; font-weight: 700; color: var(--cb-ink-2); }
.usage { margin-top: 16px; }
.usage-line { margin: 8px 0 4px; font-size: 13px; }
.panel-inset {
  border: 2px dashed var(--cb-ink); border-radius: 12px;
  padding: 12px 14px; background: var(--cb-cream-2, #f1eee8);
  font-size: 13px;
}
</style>

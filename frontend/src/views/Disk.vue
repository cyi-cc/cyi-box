<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NInputNumber, NModal, NSelect, NEmpty,
  useDialog, useMessage
} from 'naive-ui'
import {
  CloudUploadOutline, DownloadOutline, ShareSocialOutline,
  TrashOutline, CopyOutline, TimeOutline, DocumentOutline
} from '@vicons/ionicons5'
import { client, readToken } from '../lib/api'
import { copyText } from '../lib/kit'
import type fileView from '../api/fileView'

const message = useMessage()
const dialog = useDialog()

const files = ref<fileView[]>([])
const uploading = ref(false)
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

async function refresh() {
  const r = await client.diskSvc.list()
  if (r.status === 0) files.value = r.data ?? []
}
onMounted(() => void refresh())

function fmtSize(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}
function fmtTime(ts: number): string {
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

// ---- 上传 ----
async function upload(list: FileList | File[]) {
  if (!list.length || uploading.value) return
  uploading.value = true
  try {
    for (const file of Array.from(list)) {
      const fd = new FormData()
      fd.append('file', file)
      const r = await fetch(`/api/disk/upload?token=${encodeURIComponent(readToken())}`, {
        method: 'POST', body: fd
      })
      const j = await r.json().catch(() => null)
      if (j?.status === 0) {
        message.success(`「${file.name}」上传完成`)
      } else {
        message.error(`「${file.name}」上传失败：${j?.msg ?? r.status}`)
      }
    }
    void refresh()
  } finally {
    uploading.value = false
  }
}
function onPick(e: Event) {
  const el = e.target as HTMLInputElement
  if (el.files) void upload(el.files)
  el.value = ''
}
function onDrop(e: DragEvent) {
  dragOver.value = false
  if (e.dataTransfer?.files.length) void upload(e.dataTransfer.files)
}

// ---- 下载 / 删除 ----
function download(f: fileView) {
  window.open(`/api/disk/download?id=${f.id}&token=${encodeURIComponent(readToken())}`, '_blank')
}
function removeFile(f: fileView) {
  dialog.warning({
    title: '删除文件',
    content: `确定删除「${f.name}」？关联的分享链接会一并失效`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.diskSvc.deleteFile({ id: f.id })
      if (r.status === 0) { message.success('已删除'); void refresh() }
      else message.error(r.msg || '删除失败')
    }
  })
}

// ---- 分享 ----
const shareVisible = ref(false)
const sharing = ref(false)
const shareFile = ref<fileView | null>(null)
const expireChoice = ref('0')
const customMinutes = ref<number | null>(null)
const expireOptions = [
  { label: '永久有效', value: '0' },
  { label: '10 分钟', value: '10' },
  { label: '30 分钟', value: '30' },
  { label: '1 小时', value: '60' },
  { label: '6 小时', value: '360' },
  { label: '1 天', value: '1440' },
  { label: '3 天', value: '4320' },
  { label: '7 天', value: '10080' },
  { label: '自定义分钟', value: 'custom' }
]
const newShareLink = ref('')

function openShare(f: fileView) {
  shareFile.value = f
  expireChoice.value = '0'
  customMinutes.value = null
  newShareLink.value = ''
  shareVisible.value = true
}

async function doShare() {
  let minutes = expireChoice.value === 'custom' ? (customMinutes.value ?? 0) : Number(expireChoice.value)
  if (expireChoice.value === 'custom' && (!customMinutes.value || customMinutes.value < 1)) {
    message.warning('自定义有效期至少 1 分钟')
    return
  }
  if (!shareFile.value) return
  sharing.value = true
  try {
    const r = await client.diskSvc.createShare({ fileId: shareFile.value.id, expireMinutes: minutes })
    if (r.status === 0 && r.data) {
      newShareLink.value = `${location.origin}/api/d/${r.data.code}`
      message.success('链接已生成，可在「分享链接」页管理')
    } else {
      message.error(r.msg || '创建失败')
    }
  } finally {
    sharing.value = false
  }
}
</script>

<template>
  <div class="panel disk-panel">
    <div class="panel-title">
      我的文件 <span class="hint">{{ files.length }} 个文件 · 管理链接去「分享链接」页</span>
      <span style="flex:1"></span>
      <n-button type="primary" :loading="uploading" @click="fileInput?.click()">
        <template #icon><n-icon><CloudUploadOutline /></n-icon></template>上传文件
      </n-button>
    </div>
    <input ref="fileInput" type="file" multiple style="display:none" @change="onPick" />

    <div
      class="file-list" :class="{ over: dragOver }"
      @dragover.prevent="dragOver = true" @dragleave="dragOver = false" @drop.prevent="onDrop"
    >
      <div v-for="f in files" :key="f.id" class="file-row">
        <div class="file-ic"><n-icon :size="20"><DocumentOutline /></n-icon></div>
        <div class="file-meta">
          <div class="file-name">{{ f.name }}</div>
          <div class="file-sub">{{ fmtSize(f.size) }} · {{ fmtTime(f.createdAt) }}</div>
        </div>
        <div class="file-acts">
          <n-button size="small" quaternary @click="download(f)">
            <template #icon><n-icon><DownloadOutline /></n-icon></template>
          </n-button>
          <n-button size="small" type="primary" @click="openShare(f)">
            <template #icon><n-icon><ShareSocialOutline /></n-icon></template>分享
          </n-button>
          <n-button size="small" quaternary @click="removeFile(f)">
            <template #icon><n-icon><TrashOutline /></n-icon></template>
          </n-button>
        </div>
      </div>
      <n-empty v-if="!files.length" description="还没有文件，点右上角上传或直接把文件拖进来" style="padding:60px 0">
        <template #icon><n-icon :size="44"><CloudUploadOutline /></n-icon></template>
      </n-empty>
    </div>
    <div class="drop-hint">支持拖拽文件到虚线区直接上传</div>

    <n-modal v-model:show="shareVisible" preset="card" title="创建分享链接" style="width: 460px">
      <div style="display:flex;flex-direction:column;gap:14px">
        <div class="share-file">
          <n-icon :size="18"><DocumentOutline /></n-icon>
          <b>{{ shareFile?.name }}</b>
        </div>
        <div>
          <div class="fm-label">有效期</div>
          <n-select v-model:value="expireChoice" :options="expireOptions" />
          <div v-if="expireChoice === 'custom'" style="margin-top:10px;display:flex;gap:8px;align-items:center">
            <n-input-number v-model:value="customMinutes" :min="1" :max="525600" placeholder="分钟" style="flex:1" />
            <span style="font-size:12px;color:var(--cb-ink-3)">分钟（最长一年）</span>
          </div>
        </div>
        <div v-if="newShareLink" class="new-link">
          <div class="fm-label">链接已生成</div>
          <n-input :value="newShareLink" readonly />
          <n-button size="small" type="primary" style="margin-top:8px" @click="copyText(newShareLink).then(ok => message[ok?'success':'error'](ok?'已复制':'失败'))">
            <template #icon><n-icon><CopyOutline /></n-icon></template>复制链接
          </n-button>
        </div>
      </div>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:10px">
          <n-button @click="shareVisible = false">关闭</n-button>
          <n-button type="primary" :loading="sharing" @click="doShare">
            <template #icon><n-icon><TimeOutline /></n-icon></template>生成链接
          </n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.disk-panel {
  display: flex; flex-direction: column;
  height: calc(100vh - 130px); min-height: 420px;
}
.file-list {
  flex: 1; overflow-y: auto;
  border: 2.5px dashed transparent; border-radius: 12px;
  display: flex; flex-direction: column; gap: 8px; min-height: 60px;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.file-list.over { border-color: var(--cb-ink); background: var(--cb-yellow); }
.file-row {
  display: flex; align-items: center; gap: 12px;
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  padding: 10px 14px; background: #fff;
}
.file-ic {
  width: 40px; height: 40px; border: 2.5px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-blue); display: flex; align-items: center; justify-content: center; flex: none;
}
.file-meta { flex: 1; min-width: 0; }
.file-name { font-weight: 800; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-sub { font-size: 12px; color: var(--cb-ink-3); font-weight: 600; margin-top: 2px; }
.file-acts { display: flex; gap: 8px; flex: none; }
.drop-hint { margin-top: 8px; font-size: 12px; color: var(--cb-ink-3); font-weight: 600; }
.share-file {
  display: flex; align-items: center; gap: 8px;
  border: 2px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-yellow); padding: 10px 14px; font-size: 14px;
}
.fm-label { font-size: 12px; font-weight: 800; color: var(--cb-ink-2); margin-bottom: 6px; }
.new-link { border-top: 2px dashed var(--cb-ink); padding-top: 12px; }
</style>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NIcon, NEmpty, useDialog, useMessage } from 'naive-ui'
import { LinkOutline, CopyOutline, TrashOutline, DownloadOutline } from '@vicons/ionicons5'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type shareView from '../api/shareView'

const message = useMessage()
const dialog = useDialog()
const shares = ref<shareView[]>([])

async function refresh() {
  const r = await client.diskSvc.listShares()
  if (r.status === 0) shares.value = r.data ?? []
}
onMounted(() => void refresh())

function fmtTime(ts: number): string {
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

function shareLink(code: string): string {
  return `${location.origin}/api/d/${code}`
}

function shareState(s: shareView): { text: string; color: string } {
  if (s.expiresAt === 0) return { text: '永久', color: 'var(--cb-mint)' }
  if (s.expired) return { text: '已过期', color: 'var(--cb-pink)' }
  const left = s.expiresAt * 1000 - Date.now()
  if (left < 3600e3) return { text: `剩 ${Math.ceil(left / 60e3)} 分钟`, color: 'var(--cb-yellow)' }
  if (left < 86400e3) return { text: `剩 ${Math.ceil(left / 3600e3)} 小时`, color: 'var(--cb-yellow)' }
  return { text: `${fmtTime(s.expiresAt)} 到期`, color: 'var(--cb-blue)' }
}

async function copyShare(s: shareView) {
  message.success(await copyText(shareLink(s.code)) ? '链接已复制' : '复制失败')
}

function removeShare(s: shareView) {
  dialog.warning({
    title: '删除分享链接',
    content: `「${s.fileName}」的这个链接将立即失效`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.diskSvc.deleteShare({ id: s.id })
      if (r.status === 0) { message.success('已删除'); void refresh() }
      else message.error(r.msg || '删除失败')
    }
  })
}
</script>

<template>
  <div class="panel shares-panel">
    <div class="panel-title">
      分享链接 <span class="hint">{{ shares.length }} 条 · 在「网盘」页点文件行的分享生成</span>
    </div>
    <div v-if="shares.length" class="share-list">
      <div v-for="s in shares" :key="s.id" class="share-row" :class="{ dead: s.expired }">
        <div class="share-ic"><n-icon :size="18"><LinkOutline /></n-icon></div>
        <div class="share-meta">
          <div class="file-name">{{ s.fileName }}</div>
          <div class="share-url mono">{{ shareLink(s.code) }}</div>
        </div>
        <span class="share-badge" :style="{ background: shareState(s).color }">{{ shareState(s).text }}</span>
        <span class="share-dl"><n-icon style="vertical-align:-2px"><DownloadOutline /></n-icon> {{ s.downloads }}</span>
        <n-button size="small" @click="copyShare(s)">
          <template #icon><n-icon><CopyOutline /></n-icon></template>复制
        </n-button>
        <n-button size="small" quaternary @click="removeShare(s)">
          <template #icon><n-icon><TrashOutline /></n-icon></template>
        </n-button>
      </div>
    </div>
    <n-empty v-else description="还没有分享链接，去「网盘」页点文件行的「分享」生成" style="padding:60px 0" />
  </div>
</template>

<style scoped>
.shares-panel {
  display: flex; flex-direction: column;
  height: calc(100vh - 130px); min-height: 420px;
}
.share-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; }
.share-row {
  display: flex; align-items: center; gap: 12px;
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  padding: 10px 14px; background: #fff;
}
.share-row.dead { opacity: 0.55; }
.share-ic {
  width: 36px; height: 36px; border: 2.5px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-purple); display: flex; align-items: center; justify-content: center; flex: none;
}
.share-meta { flex: 1; min-width: 0; }
.file-name { font-weight: 800; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.share-url { font-size: 12px; color: var(--cb-ink-3); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono { font-family: Consolas, monospace; }
.share-badge {
  border: 2px solid var(--cb-ink); border-radius: 8px; padding: 2px 10px;
  font-size: 12px; font-weight: 800; flex: none;
}
.share-dl { font-size: 12px; font-weight: 700; color: var(--cb-ink-2); flex: none; min-width: 40px; }
</style>

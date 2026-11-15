<script setup lang="ts">
import { onActivated, onMounted, ref } from 'vue'
import { NButton, NIcon, NEmpty, NTag } from 'naive-ui'
import { DocumentTextOutline, RefreshOutline } from '@vicons/ionicons5'
import { client } from '../lib/api'
import type captchaLogView from '../api/captchaLogView'

const logs = ref<captchaLogView[]>([])

async function refresh() {
  const r = await client.captchaSvc.logs()
  if (r.status === 0 && r.data) logs.value = r.data
}
onMounted(refresh)
onActivated(refresh)

function fmtTime(ts: number): string {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
</script>

<template>
  <div class="panel">
    <div class="panel-title">
      <n-icon style="vertical-align:-2px"><DocumentTextOutline /></n-icon> 调用流水
      <span class="hint">最近 {{ logs.length }} 条 · 保留 3 天 / 500 条</span>
      <n-button size="small" style="margin-left:auto" @click="refresh">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>刷新
      </n-button>
    </div>

    <div v-if="logs.length" class="log-list">
      <div v-for="l in logs" :key="l.id" class="log-row">
        <span class="log-time mono">{{ fmtTime(l.createdAt) }}</span>
        <n-tag size="tiny" :bordered="false" :type="l.event === 'check' ? 'info' : l.event === 'verify' ? 'warning' : 'default'" class="log-ev">
          {{ l.event }}
        </n-tag>
        <n-tag size="tiny" :bordered="false" :type="l.ok ? 'success' : 'error'">{{ l.ok ? '通过' : '失败' }}</n-tag>
        <span class="log-sk mono">{{ l.sitekey || '—' }}</span>
        <span class="log-ip mono">{{ l.ip }}</span>
        <span class="log-msg dim">{{ l.msg }}</span>
      </div>
    </div>
    <n-empty v-else description="还没有调用记录" style="padding:60px 0" />
  </div>
</template>

<style scoped>
.mono { font-family: Consolas, monospace; }
.dim { color: var(--cb-ink-3); font-weight: 500; }
.log-list { display: flex; flex-direction: column; gap: 4px; }
.log-row {
  display: flex; align-items: center; gap: 8px; font-size: 12px;
  border: 1.5px solid var(--cb-ink); border-radius: 8px; padding: 5px 10px; background: #fff;
}
.log-time { color: var(--cb-ink-3); flex: none; }
.log-ev { flex: none; }
.log-sk { color: var(--cb-ink-2, #555); flex: none; }
.log-ip { color: var(--cb-ink-3); flex: none; }
.log-msg { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>

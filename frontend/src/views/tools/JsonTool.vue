<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NIcon, useMessage } from 'naive-ui'
import {
  CodeOutline, ContractOutline, CopyOutline, DownloadOutline,
  TrashOutline, FlashOutline
} from '@vicons/ionicons5'
import { copyText, downloadText, highlightJson } from '../../lib/kit'

const message = useMessage()
const input = ref('')
const mode = ref<'pretty' | 'mini'>('pretty')
const error = ref('')

const output = computed(() => {
  error.value = ''
  const src = input.value.trim()
  if (!src) return ''
  try {
    const val = JSON.parse(src)
    if (mode.value === 'mini') return JSON.stringify(val)
    return JSON.stringify(val, null, 2)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    return ''
  }
})

const outputHtml = computed(() => (output.value ? highlightJson(output.value) : ''))
const stats = computed(() => {
  if (!output.value) return ''
  const size = new Blob([output.value]).size
  return `${size < 1024 ? size + ' B' : (size / 1024).toFixed(1) + ' KB'} · ${output.value.split('\n').length} 行`
})

const SAMPLE = JSON.stringify(
  { name: '池易工作箱', tags: ['工具', '后台'], version: 1.4, nested: { ok: true, nothing: null } }
)

async function copyOut() {
  if (!output.value) return
  message.success(await copyText(output.value) ? '已复制' : '复制失败')
}
</script>

<template>
  <div>
    <div class="tool-bar panel">
      <span class="bar-label"><n-icon><CodeOutline /></n-icon> 输入</span>
      <n-button size="small" :type="mode === 'pretty' ? 'primary' : 'default'" @click="mode = 'pretty'">格式化</n-button>
      <n-button size="small" :type="mode === 'mini' ? 'primary' : 'default'" @click="mode = 'mini'">
        <template #icon><n-icon><ContractOutline /></n-icon></template>压缩
      </n-button>
      <n-button size="small" @click="input = SAMPLE">
        <template #icon><n-icon><FlashOutline /></n-icon></template>示例
      </n-button>
      <div style="flex:1"></div>
      <n-button size="small" :disabled="!output" @click="copyOut">
        <template #icon><n-icon><CopyOutline /></n-icon></template>复制
      </n-button>
      <n-button size="small" :disabled="!output" @click="downloadText('data.json', output, 'application/json')">
        <template #icon><n-icon><DownloadOutline /></n-icon></template>下载
      </n-button>
      <n-button size="small" quaternary @click="input = ''">
        <template #icon><n-icon><TrashOutline /></n-icon></template>
      </n-button>
    </div>

    <div class="split-pane">
      <textarea
        v-model="input"
        class="pane-editor"
        spellcheck="false"
        placeholder='粘贴 JSON，右边实时格式化&#10;&#10;{ "hello": "world" }'
      ></textarea>
      <div class="pane-view">
        <div v-if="error" class="json-error">
          <div class="err-title">JSON 解析失败</div>
          <div class="err-msg">{{ error }}</div>
        </div>
        <template v-else-if="output">
          <pre class="json-out" v-html="outputHtml"></pre>
          <div class="pane-foot">{{ stats }}</div>
        </template>
        <div v-else class="pane-empty">输出预览</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tool-bar {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px; margin-bottom: 14px;
}
.bar-label { font-weight: 800; font-size: 13px; display: flex; align-items: center; gap: 6px; }
.split-pane {
  display: grid; grid-template-columns: 1fr 1fr; gap: 14px;
  height: calc(100vh - 210px); min-height: 420px;
}
.pane-editor, .pane-view {
  border: var(--cb-border); border-radius: var(--cb-radius);
  box-shadow: var(--cb-shadow-sm); background: var(--cb-panel);
  overflow: auto;
}
.pane-editor {
  resize: none; padding: 16px; outline: none;
  font-family: 'JetBrains Mono', 'SF Mono', Consolas, monospace;
  font-size: 13px; line-height: 1.7; color: var(--cb-ink);
}
.pane-editor:focus { box-shadow: var(--cb-shadow); }
.pane-view { position: relative; display: flex; flex-direction: column; }
.json-out {
  margin: 0; padding: 16px; flex: 1;
  font-family: 'JetBrains Mono', 'SF Mono', Consolas, monospace;
  font-size: 13px; line-height: 1.7; white-space: pre-wrap; word-break: break-all;
}
.json-out :deep(.j-key) { color: #7c3aed; font-weight: 700; }
.json-out :deep(.j-str) { color: #059669; }
.json-out :deep(.j-num) { color: #ea580c; }
.json-out :deep(.j-bool) { color: #2563eb; font-weight: 700; }
.json-out :deep(.j-null) { color: #8a857e; font-style: italic; }
.pane-foot {
  border-top: 2px solid var(--cb-ink); padding: 6px 14px;
  font-size: 12px; color: var(--cb-ink-3); font-weight: 600;
}
.pane-empty {
  flex: 1; display: flex; align-items: center; justify-content: center;
  color: var(--cb-ink-3); font-weight: 700; font-size: 14px;
}
.json-error { margin: 16px; padding: 14px 16px; border: 2.5px solid var(--cb-ink); border-radius: 12px; background: var(--cb-pink); }
.err-title { font-weight: 900; margin-bottom: 6px; }
.err-msg { font-family: Consolas, monospace; font-size: 12px; }
@media (max-width: 900px) { .split-pane { grid-template-columns: 1fr; height: auto; } .pane-editor, .pane-view { min-height: 300px; } }
</style>

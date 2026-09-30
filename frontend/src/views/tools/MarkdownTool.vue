<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NIcon, useMessage } from 'naive-ui'
import { CopyOutline, DownloadOutline, DocumentTextOutline } from '@vicons/ionicons5'
import MarkdownIt from 'markdown-it'
import { copyText, downloadText } from '../../lib/kit'

const message = useMessage()
const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

const input = ref(`# Markdown 编辑器

左边写，右边实时渲染。

## 功能
- **加粗**、*斜体*、~~删除线~~
- \`行内代码\` 和代码块
- [链接](https://example.com) 自动识别

> 引用块

\`\`\`go
fmt.Println("hello")
\`\`\`

| 列 A | 列 B |
| --- | --- |
| 1 | 2 |
`)

const html = computed(() => md.render(input.value))
const mode = ref<'split' | 'edit' | 'preview'>('split')

async function copyHtml() {
  message.success(await copyText(html.value) ? 'HTML 已复制' : '复制失败')
}

function downloadHtml() {
  const doc = `<!doctype html><html><head><meta charset="utf-8"><title>export</title></head><body>${html.value}</body></html>`
  downloadText('document.html', doc, 'text/html')
}
</script>

<template>
  <div>
    <div class="tool-bar panel">
      <span class="bar-label"><n-icon><DocumentTextOutline /></n-icon> Markdown</span>
      <n-button size="small" :type="mode === 'edit' ? 'primary' : 'default'" @click="mode = 'edit'">编辑</n-button>
      <n-button size="small" :type="mode === 'split' ? 'primary' : 'default'" @click="mode = 'split'">分屏</n-button>
      <n-button size="small" :type="mode === 'preview' ? 'primary' : 'default'" @click="mode = 'preview'">预览</n-button>
      <div style="flex:1"></div>
      <n-button size="small" @click="copyHtml">
        <template #icon><n-icon><CopyOutline /></n-icon></template>复制 HTML
      </n-button>
      <n-button size="small" @click="downloadText('document.md', input)">
        <template #icon><n-icon><DownloadOutline /></n-icon></template>.md
      </n-button>
      <n-button size="small" @click="downloadHtml">
        <template #icon><n-icon><DownloadOutline /></n-icon></template>.html
      </n-button>
    </div>

    <div class="split-pane" :class="`m-${mode}`">
      <textarea
        v-if="mode !== 'preview'"
        v-model="input" class="pane-editor" spellcheck="false"
        placeholder="# 用 Markdown 语法书写"
      ></textarea>
      <div v-if="mode !== 'edit'" class="pane-view md-view" v-html="html"></div>
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
.split-pane.m-edit, .split-pane.m-preview { grid-template-columns: 1fr; }
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
.md-view { padding: 18px 22px; }

/* Markdown 排版（v-html 需要 :deep） */
.md-view :deep(h1) { font-size: 26px; font-weight: 900; border-bottom: 3px solid var(--cb-ink); padding-bottom: 8px; }
.md-view :deep(h2) { font-size: 21px; font-weight: 900; margin-top: 24px; }
.md-view :deep(h3) { font-size: 17px; font-weight: 800; }
.md-view :deep(p) { line-height: 1.8; }
.md-view :deep(a) { color: var(--cb-primary); font-weight: 700; }
.md-view :deep(code) {
  background: var(--cb-yellow); border: 1.5px solid var(--cb-ink);
  border-radius: 5px; padding: 1px 6px; font-family: Consolas, monospace; font-size: 12.5px;
}
.md-view :deep(pre) {
  background: #1a1a1a; color: #f8f6f6; border-radius: 10px;
  padding: 14px 16px; overflow: auto; border: var(--cb-border);
}
.md-view :deep(pre code) { background: none; border: none; color: inherit; padding: 0; }
.md-view :deep(blockquote) {
  margin: 12px 0; padding: 8px 16px; border-left: 5px solid var(--cb-primary);
  background: var(--cb-yellow); border-radius: 0 8px 8px 0;
}
.md-view :deep(table) { border-collapse: collapse; margin: 12px 0; }
.md-view :deep(th), .md-view :deep(td) { border: 2px solid var(--cb-ink); padding: 6px 14px; }
.md-view :deep(th) { background: var(--cb-yellow); font-weight: 800; }
.md-view :deep(img) { max-width: 100%; border: var(--cb-border); border-radius: 10px; }
.md-view :deep(ul), .md-view :deep(ol) { line-height: 1.9; }
.md-view :deep(hr) { border: none; border-top: 3px solid var(--cb-ink); margin: 20px 0; }
@media (max-width: 900px) { .split-pane { grid-template-columns: 1fr; height: auto; } .pane-editor, .pane-view { min-height: 320px; } }
</style>

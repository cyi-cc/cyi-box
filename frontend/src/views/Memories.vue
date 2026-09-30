<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NSelect, NTag, NEmpty, NModal, NDrawer, NDrawerContent,
  NPopconfirm, NSpin, useMessage
} from 'naive-ui'
import {
  BulbOutline, AddOutline, TrashOutline, CopyOutline, SearchOutline,
  FlashOutline, RefreshOutline, PencilOutline
} from '@vicons/ionicons5'
import MarkdownIt from 'markdown-it'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type memoryMetaView from '../api/memoryMetaView'
import type memoryProjectView from '../api/memoryProjectView'
import type memoryView from '../api/memoryView'
import type memoryMcpView from '../api/memoryMcpView'

const message = useMessage()
const md = new MarkdownIt({ linkify: true, breaks: true })

const projects = ref<memoryProjectView[]>([])
const curProject = ref('')
const items = ref<memoryMetaView[]>([])
const loading = ref(false)
const query = ref('')

const totalProjects = computed(() => projects.value.reduce((n, p) => n + p.count, 0))
const projectOptions = computed(() => [
  { label: '全部项目', value: '' },
  ...projects.value.map(p => ({ label: `${p.name}（${p.count}）`, value: p.name }))
])

async function refresh() {
  loading.value = true
  try {
    const r = await client.memorySvc.list({
      project: curProject.value, tag: '', query: query.value.trim(), limit: 200
    })
    if (r.status === 0 && r.data) {
      items.value = r.data.items
    } else {
      message.error(r.msg || '加载失败')
    }
    const p = await client.memorySvc.projects()
    if (p.status === 0 && p.data) projects.value = p.data.items
  } catch (e) {
    message.error(`加载失败: ${e instanceof Error ? e.message : e}`)
  } finally {
    loading.value = false
  }
}

function pickProject(name: string) {
  curProject.value = name ?? ''
  void refresh()
}

// ---- 预览弹窗 ----
const viewVisible = ref(false)
const viewing = ref<memoryView | null>(null)
const viewHtml = computed(() => viewing.value ? md.render(viewing.value.content) : '')

async function openView(key: string) {
  const r = await client.memorySvc.get({ key })
  if (r.status !== 0 || !r.data) {
    message.error(r.msg || '读取失败')
    return
  }
  viewing.value = r.data
  viewVisible.value = true
}

// ---- 编辑抽屉 ----
const editVisible = ref(false)
const editBusy = ref(false)
const editForm = ref({ key: '', content: '', project: '', tags: '' })
const editIsNew = ref(false)

async function openEdit(key: string) {
  const r = await client.memorySvc.get({ key })
  if (r.status !== 0 || !r.data) {
    message.error(r.msg || '读取失败')
    return
  }
  fillEdit(r.data)
}

function fillEdit(m: memoryView) {
  editIsNew.value = false
  editForm.value = { key: m.key, content: m.content, project: m.project, tags: m.tags.join(', ') }
  viewVisible.value = false
  editVisible.value = true
}

function editViewing() {
  if (viewing.value) fillEdit(viewing.value)
}

async function save() {
  const f = editForm.value
  if (!f.content.trim()) {
    message.warning('内容不能为空')
    return
  }
  editBusy.value = true
  const r = await client.memorySvc.save({
    key: f.key.trim(), content: f.content, project: f.project.trim(),
    tags: f.tags.split(',').map(t => t.trim()).filter(Boolean)
  })
  editBusy.value = false
  if (r.status === 0 && r.data) {
    message.success(`已保存 · rev ${r.data.revision}`)
    editVisible.value = false
    void refresh()
  } else {
    message.error(r.msg || '保存失败')
  }
}

async function remove(m: memoryMetaView) {
  viewVisible.value = false
  const r = await client.memorySvc.delete({ key: m.key })
  if (r.status === 0) {
    message.success('已删除')
    void refresh()
  } else {
    message.error(r.msg || '删除失败')
  }
}

// ---- 新建项目 ----
const projVisible = ref(false)
const projBusy = ref(false)
const projName = ref('')

function openNewProject() {
  projName.value = ''
  projVisible.value = true
}

async function createProject() {
  const name = projName.value.trim()
  if (!name) {
    message.warning('填个项目名')
    return
  }
  if (projects.value.some(p => p.name === name)) {
    message.warning('项目已存在')
    return
  }
  projBusy.value = true
  const r = await client.memorySvc.save({
    key: `${name}/overview`, content: `# ${name}\n\n`, project: name, tags: ['project']
  })
  projBusy.value = false
  if (r.status === 0) {
    message.success('项目已创建')
    projVisible.value = false
    curProject.value = name
    void refresh()
  } else {
    message.error(r.msg || '创建失败')
  }
}

// ---- MCP 接入信息 ----
const mcpVisible = ref(false)
const mcpInfo = ref<memoryMcpView | null>(null)
const mcpBusy = ref(false)
// 阅读链接基址：后端按请求 Host 推导，代理/域名场景都正确
const mcpBase = ref('')

async function openMcp() {
  mcpVisible.value = true
  const r = await client.memorySvc.mcpInfo()
  if (r.status === 0 && r.data) {
    mcpInfo.value = r.data
    mcpBase.value = r.data.url.replace(/\/mcp$/, '')
  } else {
    message.error(r.msg || '获取接入信息失败')
  }
}

async function resetToken() {
  mcpBusy.value = true
  const r = await client.memorySvc.resetToken()
  mcpBusy.value = false
  if (r.status === 0 && r.data) {
    mcpInfo.value = r.data
    message.success('已重置令牌，旧令牌即刻失效')
  } else {
    message.error(r.msg || '重置失败')
  }
}

const mcpConfigJson = computed(() => JSON.stringify({
  mcpServers: {
    memory: {
      url: mcpInfo.value?.url ?? '',
      transport: 'http',
      headers: { Authorization: `Bearer ${mcpInfo.value?.token ?? ''}` }
    }
  }
}, null, 2))

function fmtTime(ts: number) {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

onMounted(() => {
  void refresh()
  void client.memorySvc.mcpInfo().then(r => {
    if (r.status === 0 && r.data) mcpBase.value = r.data.url.replace(/\/mcp$/, '')
  })
})
</script>

<template>
  <div class="mem-page">
    <div class="stat-grid">
      <div class="stat-card" style="background:var(--cb-mint)">
        <div class="s-num">{{ totalProjects }}</div><div class="s-lb">记忆总数</div>
      </div>
      <div class="stat-card">
        <div class="s-num">{{ projects.length }}</div><div class="s-lb">项目数</div>
      </div>
    </div>

    <div class="mem-layout">
      <!-- 记忆列表 -->
      <div class="panel main">
        <div class="panel-title">
          <n-icon style="vertical-align:-2px"><BulbOutline /></n-icon> 记忆库
          <n-select
            :value="curProject" :options="projectOptions"
            size="small" style="width:180px" @update:value="pickProject"
          />
          <span class="hint">{{ items.length }} 条</span>
          <div class="bar-right">
            <n-input
              v-model:value="query" size="small" placeholder="搜 key / 内容…" clearable
              style="width:200px" @keyup.enter="refresh" @clear="refresh"
            />
            <n-button size="small" @click="refresh">
              <template #icon><n-icon><SearchOutline /></n-icon></template>
            </n-button>
            <n-button size="small" @click="openMcp">
              <template #icon><n-icon><FlashOutline /></n-icon></template>接入 MCP
            </n-button>
            <n-button size="small" @click="openNewProject">
              <template #icon><n-icon><AddOutline /></n-icon></template>新建项目
            </n-button>

          </div>
        </div>

        <n-spin :show="loading" size="small">
          <div v-if="items.length" class="mem-table">
            <div class="mem-head">
              <span>Key</span><span>标签</span><span>更新</span><span></span>
            </div>
            <div v-for="m in items" :key="m.key" class="mem-row" @click="openView(m.key)">
              <span class="mono mem-key">{{ m.key }}</span>
              <span class="mem-meta">
                <n-tag v-for="t in m.tags || []" :key="t" size="tiny" :bordered="false">{{ t }}</n-tag>
                <span v-if="!(m.tags || []).length" class="dim">—</span>
              </span>
              <span class="dim">{{ fmtTime(m.updatedAt) }}</span>
              <span class="mem-ops">
                <n-popconfirm @positive-click="remove(m)">
                  <template #trigger>
                    <n-button size="tiny" quaternary @click.stop>
                      <template #icon><n-icon><TrashOutline /></n-icon></template>
                    </n-button>
                  </template>
                  删除记忆 {{ m.key }}？
                </n-popconfirm>
              </span>
            </div>
          </div>
          <n-empty v-else-if="!loading" description="还没有记忆" style="padding:60px 0">
            <template #extra><n-button size="small" type="primary" @click="openNewProject">新建项目</n-button></template>
          </n-empty>
        </n-spin>

        <div class="usage">
          <b>说明</b>
          <div class="dim">
            这是所有 AI 会话共用的记忆库——Devin / Codex / Claude 存的都在这里。
            点「接入 MCP」拿端点和令牌配到客户端；<span class="mono">/m/&lt;key&gt;</span> 是公开阅读链接。
          </div>
        </div>
      </div>
    </div>

    <!-- 新建项目弹窗 -->
    <n-modal v-model:show="projVisible" preset="card" title="新建项目" style="width: 400px">
      <div class="fld">
        <label>项目名 · 会同时生成 项目名/overview 记忆</label>
        <n-input v-model:value="projName" placeholder="如 vivid" autofocus @keyup.enter="createProject" />
      </div>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:8px">
          <n-button @click="projVisible = false">取消</n-button>
          <n-button type="primary" :loading="projBusy" @click="createProject">创建</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 预览弹窗 -->
    <n-modal v-model:show="viewVisible" preset="card" style="width: 680px" :bordered="false">
      <template #header>
        <span class="mono" style="font-size:14px">{{ viewing?.key }}</span>
      </template>
      <div v-if="viewing" class="mem-view">
        <div class="view-meta">
          <n-tag v-if="viewing.project" size="tiny" :bordered="false" type="success">{{ viewing.project }}</n-tag>
          <n-tag v-for="t in viewing.tags" :key="t" size="tiny" :bordered="false">{{ t }}</n-tag>
          <span class="dim">{{ viewing.writtenBy || '—' }} · rev {{ viewing.revision }} · {{ fmtTime(viewing.updatedAt) }}</span>
        </div>
        <div class="md-view" v-html="viewHtml"></div>
      </div>
      <template #footer>
        <div v-if="viewing" style="display:flex;justify-content:space-between;align-items:center">
          <n-popconfirm @positive-click="remove(viewing)">
            <template #trigger>
              <n-button size="small" quaternary type="error">
                <template #icon><n-icon><TrashOutline /></n-icon></template>删除
              </n-button>
            </template>
            删除该记忆？
          </n-popconfirm>
          <div style="display:flex;gap:8px">
            <n-button size="small" @click="copyText(viewing.url).then(() => message.success('签名链接已复制'))">
              <template #icon><n-icon><CopyOutline /></n-icon></template>阅读链接
            </n-button>
            <n-button size="small" type="primary" @click="editViewing">
              <template #icon><n-icon><PencilOutline /></n-icon></template>编辑
            </n-button>
          </div>
        </div>
      </template>
    </n-modal>

    <!-- 编辑抽屉 -->
    <n-drawer v-model:show="editVisible" :width="560" placement="right">
      <n-drawer-content :title="editIsNew ? '新建记忆' : '编辑记忆'" closable>
        <div class="fld">
          <label>KEY · 必填，形如 project/topic</label>
          <n-input v-model:value="editForm.key" :readonly="!editIsNew" placeholder="cyi-box/stack" />
        </div>
        <div class="fld">
          <label>PROJECT · 分组名</label>
          <n-input v-model:value="editForm.project" placeholder="如 cyi-box" list="mem-projects" />
          <datalist id="mem-projects">
            <option v-for="p in projects" :key="p.name" :value="p.name" />
          </datalist>
        </div>
        <div class="fld">
          <label>TAGS · 逗号分隔</label>
          <n-input v-model:value="editForm.tags" placeholder="server, gotcha, command" />
        </div>
        <div class="fld">
          <label>CONTENT · Markdown</label>
          <n-input
            v-model:value="editForm.content" type="textarea" :autosize="{ minRows: 14 }"
            placeholder="# 事实不是进度&#10;写持久性信息：架构、服务器、命令、坑…"
            class="mono-area"
          />
        </div>
        <template #footer>
          <div style="display:flex;justify-content:flex-end;gap:8px">
            <n-button @click="editVisible = false">取消</n-button>
            <n-button type="primary" :loading="editBusy" @click="save">
              <template #icon><n-icon><PencilOutline /></n-icon></template>保存
            </n-button>
          </div>
        </template>
      </n-drawer-content>
    </n-drawer>

    <!-- MCP 接入弹窗 -->
    <n-modal v-model:show="mcpVisible" preset="card" title="接入 MCP · agent-memory" style="width: 560px">
      <div v-if="mcpInfo">
        <div class="fld">
          <label>端点</label>
          <div class="kv mono" @click="copyText(mcpInfo.url).then(() => message.success('已复制'))">
            {{ mcpInfo.url }} <n-icon><CopyOutline /></n-icon>
          </div>
        </div>
        <div class="fld">
          <label>令牌（Bearer）</label>
          <div class="kv mono" @click="copyText(mcpInfo.token).then(() => message.success('已复制'))">
            {{ mcpInfo.token }} <n-icon><CopyOutline /></n-icon>
          </div>
        </div>
        <div class="fld">
          <label>客户端配置 · mcp_config.json</label>
          <pre class="cfg mono">{{ mcpConfigJson }}</pre>
          <n-button size="small" @click="copyText(mcpConfigJson).then(() => message.success('已复制'))">
            <template #icon><n-icon><CopyOutline /></n-icon></template>复制配置
          </n-button>
        </div>
        <div class="dim" style="font-size:12px;margin-top:8px">
          记忆阅读链接 {{ mcpInfo.url.replace('/mcp', '/m/') }}&lt;key&gt; 无需令牌。
        </div>
      </div>
      <template #footer>
        <div v-if="mcpInfo" style="display:flex;justify-content:space-between;align-items:center">
          <n-popconfirm @positive-click="resetToken">
            <template #trigger>
              <n-button size="small" secondary type="warning" :loading="mcpBusy">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>重置令牌
              </n-button>
            </template>
            重置后旧令牌即刻失效，所有客户端要换配置。继续？
          </n-popconfirm>
          <n-button @click="mcpVisible = false">关闭</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.mem-page { display: flex; flex-direction: column; gap: 14px; }
.stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.stat-card {
  border: var(--cb-border); border-radius: 14px; padding: 14px 18px;
  background: #fff; box-shadow: var(--cb-shadow-sm);
}
.s-num { font-size: 26px; font-weight: 900; font-family: Consolas, monospace; }
.s-lb { font-size: 12px; font-weight: 800; color: var(--cb-ink-3); margin-top: 2px; }
.mem-layout { display: grid; grid-template-columns: 1fr; gap: 14px; align-items: start; }
.panel {
  border: var(--cb-border); border-radius: 14px; background: #fff;
  box-shadow: var(--cb-shadow-sm); padding: 16px 18px;
}
.panel-title {
  display: flex; align-items: center; gap: 8px;
  font-weight: 900; font-size: 14px; margin-bottom: 12px;
}
.hint { font-size: 12px; color: var(--cb-ink-3); font-weight: 600; }
.bar-right { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.mem-table { display: flex; flex-direction: column; }
.mem-head, .mem-row {
  display: grid; grid-template-columns: minmax(0, 2.4fr) minmax(0, 1.6fr) 140px 40px;
  gap: 8px; align-items: center; padding: 8px 10px; font-size: 12.5px;
}
.mem-head { font-weight: 800; color: var(--cb-ink-3); font-size: 11px; border-bottom: 2px solid var(--cb-ink); }
.mem-row { border-bottom: 1px dashed var(--cb-cream-2, #e5e0d5); cursor: pointer; }
.mem-row:hover { background: var(--cb-cream, #f7f4ec); }
.mem-key { font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mem-meta { display: flex; gap: 4px; flex-wrap: wrap; }
.mem-ops { text-align: right; }
.mem-view .view-meta {
  display: flex; align-items: center; gap: 6px; flex-wrap: wrap;
  font-size: 12px; margin-bottom: 12px;
}
.md-view { line-height: 1.8; font-size: 14px; }
.md-view :deep(h1) { font-size: 24px; font-weight: 900; border-bottom: 3px solid var(--cb-ink); padding-bottom: 6px; }
.md-view :deep(h2) { font-size: 19px; font-weight: 900; margin-top: 20px; }
.md-view :deep(h3) { font-size: 16px; font-weight: 800; }
.md-view :deep(a) { color: var(--cb-primary); font-weight: 700; }
.md-view :deep(code) {
  background: var(--cb-yellow); border: 1.5px solid var(--cb-ink);
  border-radius: 5px; padding: 1px 6px; font-family: Consolas, monospace; font-size: 12.5px;
}
.md-view :deep(pre) {
  background: #1a1a1a; color: #f8f6f6; border-radius: 10px;
  padding: 14px 16px; overflow: auto;
}
.md-view :deep(pre code) { background: none; border: none; padding: 0; }
.md-view :deep(blockquote) {
  margin: 8px 0; padding: 4px 14px; border-left: 3px solid var(--cb-ink); color: var(--cb-ink-3);
}
.mono { font-family: Consolas, 'SF Mono', monospace; }
.dim { color: var(--cb-ink-3); }
.fld { display: flex; flex-direction: column; gap: 6px; margin-bottom: 14px; }
.fld label { font-size: 11px; font-weight: 800; color: var(--cb-ink-3); letter-spacing: 1px; }
.kv {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  border: var(--cb-border); border-radius: 10px; padding: 9px 12px;
  background: var(--cb-cream, #faf9f5); cursor: pointer; font-size: 12px;
  word-break: break-all;
}
.kv:hover { background: var(--cb-yellow); }
.cfg {
  border: var(--cb-border); border-radius: 10px; padding: 12px;
  background: var(--cb-cream, #faf9f5); font-size: 12px; line-height: 1.6;
  overflow: auto; margin: 0 0 8px;
}
.mono-area :deep(textarea) { font-family: Consolas, 'SF Mono', monospace; font-size: 13px; line-height: 1.7; }
.usage {
  margin-top: 16px; border: 2px dashed var(--cb-ink); border-radius: 12px;
  padding: 12px 14px; background: var(--cb-cream-2, #f1eee8); font-size: 13px;
  display: flex; flex-direction: column; gap: 6px;
}
@media (max-width: 900px) { .mem-layout { grid-template-columns: 1fr; } }
</style>

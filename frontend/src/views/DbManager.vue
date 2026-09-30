<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  NButton, NDataTable, NDropdown, NEmpty, NForm, NFormItem, NIcon, NInput,
  NInputNumber, NModal, NRadioGroup, NRadioButton, NSelect, NSpin, NTag,
  NText, useDialog, useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  AddOutline, CubeOutline, DownloadOutline,
  PlayOutline, RefreshOutline, GridOutline,
  TrashOutline, PencilOutline, EyeOutline, FlashOutline,
  CloudUploadOutline
} from '@vicons/ionicons5'
import { EditorView, basicSetup } from 'codemirror'
import { sql as sqlLang, MySQL, PostgreSQL, SQLite } from '@codemirror/lang-sql'
import { keymap } from '@codemirror/view'
import { Compartment, Prec } from '@codemirror/state'
import { client, readToken } from '../lib/api'
import type dbconnView from '../api/dbconnView'
import type dbQueryView from '../api/dbQueryView'

const message = useMessage()
const dialog = useDialog()

// ============ 连接树 ============
interface ConnNode { conn: dbconnView; dbs: string[]; open: boolean; loading: boolean }
interface TableRow { name: string; type: string }

const conns = ref<ConnNode[]>([])
const curConn = ref<dbconnView | null>(null)
const curDb = ref('')
const tables = ref<TableRow[]>([])
const tablesLoading = ref(false)
const tablesFilter = ref('')

const filteredTables = computed(() => {
  const k = tablesFilter.value.trim().toLowerCase()
  if (!k) return tables.value
  return tables.value.filter(t => t.name.toLowerCase().includes(k))
})

async function loadConns() {
  const r = await client.dbMgrSvc.list()
  if (r.status === 0 && r.data) {
    conns.value = r.data.map(c => ({ conn: c, dbs: [], open: false, loading: false }))
  }
}

async function toggleConn(node: ConnNode) {
  node.open = !node.open
  if (node.open && node.dbs.length === 0) {
    node.loading = true
    const r = await client.dbMgrSvc.databases({ id: node.conn.id })
    node.loading = false
    if (r.status === 0 && r.data) {
      node.dbs = r.data.names
      if (r.data.names.length === 1) void pickDb(node.conn, r.data.names[0])
    } else {
      message.error(r.msg || '获取库列表失败')
    }
  }
}

// table → columns 补全映射
const schemaCols = ref<Record<string, string[]>>({})

async function pickDb(conn: dbconnView, db: string) {
  curConn.value = conn
  curDb.value = db
  tablesLoading.value = true
  const r = await client.dbMgrSvc.tables({ id: conn.id, database: db })
  tablesLoading.value = false
  if (r.status === 0 && r.data) {
    tables.value = r.data.tables
    const m: Record<string, string[]> = {}
    for (const c of r.data.columns) {
      (m[c.table] ||= []).push(c.name)
    }
    schemaCols.value = m
    reconfigureEditor()
  } else {
    message.error(r.msg || '获取表结构失败')
  }
}

// ============ SQL 编辑器（CodeMirror） ============
const editorEl = ref<HTMLElement | null>(null)
let view: EditorView | null = null
const langConf = new Compartment()

const engineDialect = computed(() => {
  switch (curConn.value?.engine) {
    case 'mysql': return MySQL
    case 'postgres': return PostgreSQL
    default: return SQLite
  }
})

function reconfigureEditor() {
  view?.dispatch({
    effects: langConf.reconfigure(sqlLang({
      schema: schemaCols.value,
      dialect: engineDialect.value,
      upperCaseKeywords: true
    }))
  })
}

onMounted(() => {
  void loadConns()
  view = new EditorView({
    doc: 'SELECT * FROM ',
    parent: editorEl.value!,
    extensions: [
      basicSetup,
      langConf.of(sqlLang({ schema: {}, dialect: SQLite, upperCaseKeywords: true })),
      Prec.highest(keymap.of([{
        key: 'Ctrl-Enter',
        run: () => { void runQuery(); return true }
      }, {
        key: 'Cmd-Enter',
        run: () => { void runQuery(); return true }
      }]))
    ]
  })
})
onBeforeUnmount(() => view?.destroy())

function currentSql(): string {
  if (!view) return ''
  const sel = view.state.selection.main
  // 有选中执行选中片段，否则整篇
  const text = sel.empty ? view.state.doc.toString() : view.state.sliceDoc(sel.from, sel.to)
  return text.trim()
}
function setSql(text: string) {
  view?.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } })
}

// ============ 执行 ============
const running = ref(false)
const result = ref<dbQueryView | null>(null)
const resultRows = ref<Record<string, unknown>[]>([])
const runErr = ref('')

const resultColumns = computed<DataTableColumns>(() => {
  if (!result.value) return []
  return result.value.columns.map(c => ({
    title: c, key: c,
    render: (row: Record<string, unknown>) => {
      const v = row[c]
      if (v === null || v === undefined) return h(NText, { depth: 3, style: 'font-style:italic' }, { default: () => 'NULL' })
      return h('span', { style: 'font-family:monospace;font-size:12px' }, String(v))
    },
    ellipsis: { tooltip: true }
  }))
})

async function runQuery() {
  if (!curConn.value) {
    message.warning('先在左侧选一个连接')
    return
  }
  const sqlText = currentSql()
  if (!sqlText) {
    message.warning('SQL 不能为空')
    return
  }
  running.value = true
  runErr.value = ''
  try {
    const r = await client.dbMgrSvc.query({
      id: curConn.value.id,
      database: curDb.value || null,
      sql: sqlText,
      maxRows: 500
    })
    if (r.status === 0 && r.data) {
      result.value = r.data
      if (r.data.isSelect) {
        const raw = JSON.parse(r.data.rowsJson || '[]') as unknown[][]
        resultRows.value = raw.map(arr => {
          const obj: Record<string, unknown> = {}
          r.data!.columns.forEach((c, i) => { obj[c] = arr[i] })
          return obj
        })
      } else {
        resultRows.value = []
      }
    } else {
      runErr.value = r.msg || '执行失败'
      result.value = null
      resultRows.value = []
    }
  } finally {
    running.value = false
  }
}

function browseTable(t: TableRow) {
  setSql(`SELECT * FROM ${t.name} LIMIT 200;`)
  void runQuery()
}

// ============ 连接编辑 ============
const connVisible = ref(false)
const connSaving = ref(false)
const connForm = reactive<{
  id: number | null; name: string; engine: string; host: string; port: number
  username: string; password: string; database: string; params: string
}>({ id: null, name: '', engine: 'mysql', host: '', port: 3306, username: 'root', password: '', database: '', params: '' })

const enginePorts: Record<string, number> = { mysql: 3306, postgres: 5432, sqlite: 0 }
function onEngineChange(e: string) { connForm.port = enginePorts[e] ?? 0 }

function openConnCreate() {
  Object.assign(connForm, { id: null, name: '', engine: 'mysql', host: '', port: 3306, username: 'root', password: '', database: '', params: '' })
  connVisible.value = true
}
function openConnEdit(c: dbconnView, ev: Event) {
  ev.stopPropagation()
  Object.assign(connForm, {
    id: c.id, name: c.name, engine: c.engine, host: c.host, port: c.port,
    username: c.username, password: '', database: c.database, params: ''
  })
  connVisible.value = true
}

async function saveConn() {
  if (!connForm.name.trim()) { message.warning('名称必填'); return }
  if (connForm.engine !== 'sqlite' && !connForm.host.trim()) { message.warning('主机必填'); return }
  if (connForm.engine === 'sqlite' && !connForm.database.trim()) { message.warning('sqlite 需填文件路径'); return }
  connSaving.value = true
  try {
    const r = await client.dbMgrSvc.save({
      id: connForm.id,
      name: connForm.name.trim(),
      engine: connForm.engine,
      host: connForm.host.trim() || null,
      port: connForm.port || null,
      username: connForm.username.trim() || null,
      password: connForm.password || null,
      database: connForm.database.trim() || null,
      params: connForm.params.trim() || null
    })
    if (r.status === 0) {
      message.success('已保存')
      connVisible.value = false
      void loadConns()
    } else {
      message.error(r.msg || '保存失败')
    }
  } finally {
    connSaving.value = false
  }
}

function removeConn(c: dbconnView, ev: Event) {
  ev.stopPropagation()
  dialog.warning({
    title: '删除连接',
    content: `确定删除连接「${c.name}」吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.dbMgrSvc.delete({ id: c.id })
      if (r.status === 0) {
        message.success('已删除')
        void loadConns()
      } else {
        message.error(r.msg || '删除失败')
      }
    }
  })
}

async function testConn(c: dbconnView, ev: Event) {
  ev.stopPropagation()
  const r = await client.dbMgrSvc.test({ id: c.id })
  if (r.status === 0) message.success('连接成功')
  else message.error(r.msg || '连接失败')
}

// ============ 导出 ============
function onExport(format: string) {
  if (!curConn.value) { message.warning('先选一个连接'); return }
  const sqlText = currentSql()
  if (!sqlText) { message.warning('先写一条要导出的 SQL'); return }
  const body = new URLSearchParams({
    id: String(curConn.value.id),
    db: curDb.value,
    sql: sqlText,
    format
  })
  fetch(`/api/dbm/export?token=${encodeURIComponent(readToken())}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body
  }).then(async resp => {
    if (!resp.ok) {
      message.error('导出失败：' + await resp.text())
      return
    }
    const blob = await resp.blob()
    const dispo = resp.headers.get('Content-Disposition') || ''
    const m = dispo.match(/filename="?([^";]+)/)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = m?.[1] || `export.${format}`
    a.click()
    URL.revokeObjectURL(a.href)
  }).catch(e => message.error('导出失败：' + String(e)))
}

const exportOptions = [
  { label: '导出 JSON', key: 'json' },
  { label: '导出 CSV', key: 'csv' },
  { label: '导出 Excel (.xlsx)', key: 'xlsx' }
]

// ============ 导入 ============
const importVisible = ref(false)
const importTable = ref('')
const importFile = ref<File | null>(null)
const importing = ref(false)
const importFileInput = ref<HTMLInputElement | null>(null)

const importTableOptions = computed(() =>
  tables.value.filter(t => t.type === 'table').map(t => ({ label: t.name, value: t.name })))

function openImport() {
  if (!curConn.value) { message.warning('先选一个连接和库'); return }
  importTable.value = ''
  importFile.value = null
  importVisible.value = true
}

function pickImportFile() { importFileInput.value?.click() }
function onImportFile(ev: Event) {
  importFile.value = (ev.target as HTMLInputElement).files?.[0] ?? null
}

async function doImport() {
  if (!importTable.value || !importFile.value || !curConn.value) {
    message.warning('选表和文件')
    return
  }
  importing.value = true
  try {
    const fd = new FormData()
    fd.append('file', importFile.value)
    const q = new URLSearchParams({
      id: String(curConn.value.id), db: curDb.value,
      table: importTable.value, token: readToken()
    })
    const resp = await fetch(`/api/dbm/import?${q}`, { method: 'POST', body: fd })
    const j = await resp.json().catch(() => null)
    if (j?.status === 0) {
      message.success(`导入完成：${j.inserted} 行${j.skipped ? `，跳过 ${j.skipped} 行` : ''}`)
      importVisible.value = false
    } else {
      message.error(j?.msg || '导入失败')
    }
  } finally {
    importing.value = false
    if (importFileInput.value) importFileInput.value.value = ''
  }
}

const engineTag: Record<string, string> = { mysql: '#bae6fd', postgres: '#a7f3d0', sqlite: '#fef08a' }
</script>

<template>
  <div class="dbm-wrap">
    <!-- 左：连接 → 库 → 表 -->
    <div class="panel dbm-side">
      <div class="dbm-side-head">
        <span style="font-weight:900">连接</span>
        <div style="flex:1"></div>
        <n-button size="tiny" type="primary" @click="openConnCreate">
          <template #icon><n-icon><AddOutline /></n-icon></template>
        </n-button>
      </div>
      <n-empty v-if="!conns.length" description="还没有连接" size="small" style="padding:30px 0" />
      <div v-for="node in conns" :key="node.conn.id" class="conn-node">
        <div class="conn-row" :class="{ active: curConn?.id === node.conn.id }" @click="toggleConn(node)">
          <span class="engine-dot" :style="{ background: engineTag[node.conn.engine] || '#ddd' }"></span>
          <span class="conn-name">{{ node.conn.name }}</span>
          <span class="conn-host">{{ node.conn.engine === 'sqlite' ? 'sqlite' : node.conn.host }}</span>
          <span class="conn-ops">
            <n-button size="tiny" quaternary @click="testConn(node.conn, $event)">
              <template #icon><n-icon :size="13"><FlashOutline /></n-icon></template>
            </n-button>
            <n-button size="tiny" quaternary @click="openConnEdit(node.conn, $event)">
              <template #icon><n-icon :size="13"><PencilOutline /></n-icon></template>
            </n-button>
            <n-button size="tiny" quaternary type="error" @click="removeConn(node.conn, $event)">
              <template #icon><n-icon :size="13"><TrashOutline /></n-icon></template>
            </n-button>
          </span>
        </div>
        <div v-if="node.open" class="conn-dbs">
          <div v-if="node.loading" style="padding:4px 12px;color:var(--cb-ink-2);font-size:12px">加载中…</div>
          <div v-for="db in node.dbs" :key="db" class="db-row"
            :class="{ active: curConn?.id === node.conn.id && curDb === db }"
            @click="pickDb(node.conn, db)">
            <n-icon :size="13"><CubeOutline /></n-icon>{{ db }}
          </div>
        </div>
      </div>

      <div v-if="curConn" class="dbm-tables">
        <div class="dbm-tables-head">
          <span style="font-weight:900;font-size:12px">表 · {{ curDb || '默认库' }}</span>
          <div style="flex:1"></div>
          <n-button size="tiny" quaternary @click="pickDb(curConn, curDb)">
            <template #icon><n-icon :size="13"><RefreshOutline /></n-icon></template>
          </n-button>
        </div>
        <n-input v-model:value="tablesFilter" size="small" placeholder="过滤表名" clearable style="margin-bottom:8px" />
        <n-spin :show="tablesLoading" size="small">
          <div class="table-list">
            <div v-for="t in filteredTables" :key="t.name" class="table-row" @dblclick="browseTable(t)">
              <n-icon :size="13" :color="t.type === 'view' ? '#8a90a6' : '#ec5b13'">
                <component :is="t.type === 'view' ? EyeOutline : GridOutline" />
              </n-icon>
              <span>{{ t.name }}</span>
              <span class="table-ops">
                <n-button size="tiny" quaternary @click="browseTable(t)">
                  <template #icon><n-icon :size="12"><PlayOutline /></n-icon></template>
                </n-button>
              </span>
            </div>
            <div v-if="!filteredTables.length && !tablesLoading" style="padding:10px;font-size:12px;color:var(--cb-ink-2)">无表</div>
          </div>
        </n-spin>
      </div>
    </div>

    <!-- 右：编辑器 + 结果 -->
    <div class="dbm-main">
      <div class="panel dbm-toolbar">
        <n-tag size="small" :bordered="true" style="font-weight:700">
          {{ curConn ? `${curConn.name} · ${curDb || curConn.database || '默认库'}` : '未选择连接' }}
        </n-tag>
        <div style="flex:1"></div>
        <n-button size="small" type="primary" :loading="running" @click="runQuery">
          <template #icon><n-icon><PlayOutline /></n-icon></template>
          运行 ⌃⏎
        </n-button>
        <n-dropdown trigger="click" :options="exportOptions" @select="onExport">
          <n-button size="small">
            <template #icon><n-icon><DownloadOutline /></n-icon></template>
            导出
          </n-button>
        </n-dropdown>
        <n-button size="small" @click="openImport">
          <template #icon><n-icon><CloudUploadOutline /></n-icon></template>
          导入
        </n-button>
      </div>

      <div class="panel dbm-editor-wrap">
        <div ref="editorEl" class="dbm-editor"></div>
      </div>

      <div v-if="runErr" class="panel dbm-err">{{ runErr }}</div>
      <div v-else-if="result" class="panel dbm-result">
        <div class="dbm-status">
          <template v-if="result.isSelect">
            {{ result.rowCount }} 行 · {{ result.elapsedMs }}ms
            <n-tag v-if="result.truncated" size="tiny" type="warning" style="margin-left:6px">已截断(500)</n-tag>
          </template>
          <template v-else>
            影响 {{ result.affectedRows }} 行 · {{ result.elapsedMs }}ms
          </template>
        </div>
        <n-data-table v-if="result.isSelect" :columns="resultColumns" :data="resultRows"
          size="small" :bordered="false" :single-line="false" :max-height="420" />
      </div>
      <div v-else class="panel dbm-hint">
        <n-empty description="写 SQL 后 Ctrl+Enter 执行；双击左边表名快速 SELECT" />
      </div>
    </div>
  </div>

  <!-- 连接编辑弹窗 -->
  <n-modal v-model:show="connVisible" preset="card" :title="connForm.id === null ? '新建连接' : '编辑连接'" style="width: 500px">
    <n-form label-placement="top">
      <div style="display:flex;gap:16px">
        <n-form-item label="名称" style="flex:1">
          <n-input v-model:value="connForm.name" placeholder="比如 生产 MySQL" />
        </n-form-item>
        <n-form-item label="引擎" style="width:220px">
          <n-radio-group v-model:value="connForm.engine" @update:value="onEngineChange">
            <n-radio-button value="mysql">MySQL</n-radio-button>
            <n-radio-button value="postgres">PG</n-radio-button>
            <n-radio-button value="sqlite">SQLite</n-radio-button>
          </n-radio-group>
        </n-form-item>
      </div>
      <div v-if="connForm.engine !== 'sqlite'" style="display:flex;gap:16px">
        <n-form-item label="主机" style="flex:1">
          <n-input v-model:value="connForm.host" placeholder="127.0.0.1" />
        </n-form-item>
        <n-form-item label="端口" style="width:110px">
          <n-input-number v-model:value="connForm.port" :min="1" :max="65535" style="width:100%" />
        </n-form-item>
      </div>
      <div v-if="connForm.engine !== 'sqlite'" style="display:flex;gap:16px">
        <n-form-item label="用户名" style="flex:1">
          <n-input v-model:value="connForm.username" />
        </n-form-item>
        <n-form-item :label="connForm.id === null ? '密码' : '密码（留空不变）'" style="flex:1">
          <n-input v-model:value="connForm.password" type="password" show-password-on="click" />
        </n-form-item>
      </div>
      <n-form-item :label="connForm.engine === 'sqlite' ? '数据库文件路径' : '默认数据库（可选）'">
        <n-input v-model:value="connForm.database"
          :placeholder="connForm.engine === 'sqlite' ? '/path/to/db.sqlite' : '登录后默认进入的库'" />
      </n-form-item>
      <n-form-item label="连接参数（可选）">
        <n-input v-model:value="connForm.params" placeholder="如 sslmode=require 或 charset=utf8mb4" />
      </n-form-item>
    </n-form>
    <template #footer>
      <div style="display:flex;justify-content:flex-end;gap:10px">
        <n-button @click="connVisible = false">取消</n-button>
        <n-button type="primary" :loading="connSaving" @click="saveConn">保存</n-button>
      </div>
    </template>
  </n-modal>

  <!-- 导入弹窗 -->
  <n-modal v-model:show="importVisible" preset="card" title="导入数据" style="width: 440px">
    <n-form label-placement="top">
      <n-form-item label="目标表">
        <n-select v-model:value="importTable" :options="importTableOptions" placeholder="选择表" filterable />
      </n-form-item>
      <n-form-item label="文件（.json / .csv / .xlsx，首行或对象键为列名）">
        <n-button block @click="pickImportFile">
          {{ importFile ? importFile.name : '选择文件' }}
        </n-button>
        <input ref="importFileInput" type="file" accept=".json,.csv,.xlsx,.txt" style="display:none" @change="onImportFile" />
      </n-form-item>
      <div style="font-size:12px;color:var(--cb-ink-2)">列名与表列按名字交集匹配；单行失败会跳过并计数。</div>
    </n-form>
    <template #footer>
      <div style="display:flex;justify-content:flex-end;gap:10px">
        <n-button @click="importVisible = false">取消</n-button>
        <n-button type="primary" :loading="importing" @click="doImport">导入</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.dbm-wrap { display: flex; gap: 16px; align-items: flex-start; }
.dbm-side { width: 280px; flex-shrink: 0; padding: 12px; max-height: calc(100vh - 140px); overflow: auto; }
.dbm-side-head { display: flex; align-items: center; margin-bottom: 10px; }
.conn-node { margin-bottom: 4px; }
.conn-row {
  display: flex; align-items: center; gap: 7px; padding: 7px 8px;
  border: 2px solid transparent; border-radius: 8px; cursor: pointer; font-weight: 700;
}
.conn-row:hover { background: #f1efe9; }
.conn-row.active { border-color: var(--cb-ink); background: var(--cb-yellow); box-shadow: 3px 3px 0 var(--cb-ink); }
.engine-dot { width: 10px; height: 10px; border-radius: 50%; border: 2px solid var(--cb-ink); flex-shrink: 0; }
.conn-name { font-size: 13px; }
.conn-host { font-size: 11px; color: var(--cb-ink-2); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conn-ops { display: flex; opacity: 0; transition: opacity .12s; }
.conn-row:hover .conn-ops { opacity: 1; }
.conn-dbs { padding-left: 18px; margin: 2px 0 6px; }
.db-row {
  display: flex; align-items: center; gap: 6px; padding: 5px 8px; border-radius: 6px;
  font-size: 12px; font-weight: 600; cursor: pointer; color: var(--cb-ink-2);
}
.db-row:hover { background: #f1efe9; color: var(--cb-ink); }
.db-row.active { background: #dff3ff; color: var(--cb-ink); font-weight: 800; }
.dbm-tables { border-top: 2px dashed #ddd; margin-top: 10px; padding-top: 10px; }
.dbm-tables-head { display: flex; align-items: center; margin-bottom: 8px; }
.table-list { max-height: 320px; overflow: auto; }
.table-row {
  display: flex; align-items: center; gap: 6px; padding: 5px 8px; border-radius: 6px;
  font-size: 12px; font-weight: 600; cursor: pointer; font-family: monospace;
}
.table-row:hover { background: #f1efe9; }
.table-ops { margin-left: auto; opacity: 0; }
.table-row:hover .table-ops { opacity: 1; }

.dbm-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 14px; }
.dbm-toolbar { padding: 10px 14px; display: flex; align-items: center; gap: 10px; }
.dbm-editor-wrap { overflow: hidden; }
.dbm-editor { min-height: 170px; }
.dbm-editor :deep(.cm-editor) {
  font-size: 13px; border-radius: 0;
  background: #fffef9;
}
.dbm-editor :deep(.cm-editor.cm-focused) { outline: none; }
.dbm-editor :deep(.cm-gutters) { background: #fffef9; border-right: 2px solid var(--cb-ink); color: #9aa0b5; }
.dbm-editor :deep(.cm-activeLine) { background: #fff8d6; }
.dbm-editor :deep(.cm-tooltip) { border: 2px solid var(--cb-ink); border-radius: 8px; box-shadow: 4px 4px 0 var(--cb-ink); }
.dbm-err { padding: 14px 16px; color: #e5484d; font-weight: 700; font-size: 13px; }
.dbm-result { overflow: hidden; }
.dbm-status { padding: 9px 14px; font-size: 12px; font-weight: 700; color: var(--cb-ink-2); border-bottom: 2px solid var(--cb-ink); }
.dbm-hint { padding: 40px; }
</style>

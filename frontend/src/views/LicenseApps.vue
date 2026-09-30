<script setup lang="ts">
import { onActivated, onMounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NEmpty, NModal, NForm, NFormItem, NTag, NSwitch,
  NPopconfirm, useMessage
} from 'naive-ui'
import {
  AppsOutline, AddOutline, TrashOutline, CopyOutline, CreateOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type licenseAppView from '../api/licenseAppView'

const message = useMessage()

const apps = ref<licenseAppView[]>([])
const editVisible = ref(false)
const saving = ref(false)
const form = ref({ id: 0, name: '', enabled: 1 })

async function refresh() {
  const r = await client.licenseSvc.apps()
  if (r.status === 0 && r.data) apps.value = r.data.items
}
onMounted(refresh)
onActivated(refresh)

function openCreate() {
  form.value = { id: 0, name: '', enabled: 1 }
  editVisible.value = true
}
function openEdit(a: licenseAppView) {
  form.value = { id: a.id, name: a.name, enabled: a.enabled }
  editVisible.value = true
}

async function save() {
  saving.value = true
  try {
    const r = await client.licenseSvc.saveApp({
      id: form.value.id, name: form.value.name, enabled: form.value.enabled
    })
    if (r.status === 0) {
      message.success(form.value.id ? '已保存' : `项目已创建，appid：${r.data?.appid}`)
      editVisible.value = false
      void refresh()
    } else {
      message.error(r.msg || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(a: licenseAppView, v: boolean) {
  const r = await client.licenseSvc.saveApp({ id: a.id, name: a.name, enabled: v ? 1 : 0 })
  if (r.status === 0) { message.success(v ? '已启用' : '已停用'); void refresh() }
  else message.error(r.msg || '操作失败')
}

async function copy(t: string, what: string) {
  message.success(await copyText(t) ? `${what}已复制` : '复制失败')
}

async function removeApp(a: licenseAppView) {
  const r = await client.licenseSvc.deleteApp({ id: a.id })
  if (r.status === 0) { message.success('已删除'); void refresh() }
  else message.error(r.msg || '删除失败')
}

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
      <n-icon style="vertical-align:-2px"><AppsOutline /></n-icon> 项目管理
      <span class="hint">{{ apps.length }} 个 · 每个项目一个 appid · 校验接口按项目隔离卡密</span>
      <n-button size="small" type="primary" style="margin-left:auto" @click="openCreate">
        <template #icon><n-icon><AddOutline /></n-icon></template>添加项目
      </n-button>
    </div>

    <div v-if="apps.length" class="app-list">
      <div v-for="a in apps" :key="a.id" class="app-row">
        <div class="app-ic"><n-icon :size="18"><AppsOutline /></n-icon></div>
        <div class="app-meta">
          <div class="app-name">
            {{ a.name }}
            <n-tag v-if="!a.enabled" size="tiny" type="error" :bordered="false">已停用</n-tag>
            <span class="app-count">{{ a.cardCount }} 卡密</span>
          </div>
          <div class="app-ids">
            <span class="mono id-chip" @click="copy(a.appid, 'appid ')">appid: {{ a.appid }}</span>
          </div>
        </div>
        <div class="app-right">
          <n-switch size="small" :value="!!a.enabled" @update:value="v => toggleEnabled(a, v)" />
          <n-button size="small" quaternary @click="openEdit(a)">
            <template #icon><n-icon><CreateOutline /></n-icon></template>
          </n-button>
          <n-popconfirm @positive-click="removeApp(a)">
            <template #trigger>
              <n-button size="small" quaternary><template #icon><n-icon><TrashOutline /></n-icon></template></n-button>
            </template>
            删除项目「{{ a.name }}」？其下卡密一并删除
          </n-popconfirm>
        </div>
      </div>
    </div>
    <n-empty v-else description="还没有项目，点右上角添加" style="padding:60px 0" />

    <div class="usage">
      <b>校验接口</b>
      <div class="mono usage-line">GET /api/v1/license?appid=&lt;appid&gt;&amp;card=&lt;卡密&gt;&amp;domain=&lt;域名&gt;</div>
      <div class="dim">卡密首次校验即激活并绑定域名开始计时，之后仅限同域名通过。项目停用后其所有卡密立即失效。</div>
    </div>

    <n-modal v-model:show="editVisible" preset="card" :title="form.id ? '编辑项目' : '添加项目'" style="width: 420px">
      <n-form>
        <n-form-item label="项目名">
          <n-input v-model:value="form.name" placeholder="如：我的网站 / XX软件" />
        </n-form-item>
        <n-form-item v-if="form.id" label="启用">
          <n-switch :value="!!form.enabled" @update:value="v => form.enabled = v ? 1 : 0" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:8px">
          <n-button @click="editVisible = false">取消</n-button>
          <n-button type="primary" :loading="saving" @click="save">{{ form.id ? '保存' : '创建' }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.app-list { display: flex; flex-direction: column; gap: 8px; }
.app-row {
  display: flex; align-items: center; gap: 12px;
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  padding: 10px 14px; background: #fff;
}
.app-ic {
  width: 36px; height: 36px; border: 2.5px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-yellow); display: flex; align-items: center; justify-content: center; flex: none;
}
.app-meta { flex: 1; min-width: 0; }
.app-name { font-weight: 800; font-size: 14px; display: flex; align-items: center; gap: 8px; }
.app-count {
  font-size: 11px; font-weight: 800; border: 2px solid var(--cb-ink);
  border-radius: 6px; padding: 0 6px; background: var(--cb-mint);
}
.app-ids { display: flex; gap: 8px; margin-top: 4px; flex-wrap: wrap; }
.id-chip {
  font-size: 11px; color: var(--cb-ink-3); cursor: pointer;
  background: var(--cb-cream, #f7f4ec); border-radius: 6px; padding: 1px 7px;
}
.id-chip:hover { color: var(--cb-primary, #ec5b13); }
.app-right { display: flex; align-items: center; gap: 4px; flex: none; }
.mono { font-family: Consolas, monospace; }
.dim { color: var(--cb-ink-3); font-weight: 500; }
.usage {
  margin-top: 16px; border: 2px dashed var(--cb-ink); border-radius: 12px;
  padding: 12px 14px; background: var(--cb-cream-2, #f1eee8); font-size: 13px;
}
.usage-line { margin: 8px 0 4px; font-size: 13px; }
</style>

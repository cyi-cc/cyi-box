<script setup lang="ts">
import { onActivated, onMounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NInputNumber, NSelect, NEmpty, NModal, NForm, NFormItem,
  NTag, NPopconfirm, useMessage
} from 'naive-ui'
import {
  KeyOutline, AddOutline, TrashOutline, CopyOutline, SearchOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type licenseCardView from '../api/licenseCardView'
import type licenseAppView from '../api/licenseAppView'

const message = useMessage()

const apps = ref<licenseAppView[]>([])
const appId = ref<number>(0)
const list = ref<licenseCardView[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const kw = ref('')
const stats = ref({ total: 0, unused: 0, active: 0, expired: 0 })

const genVisible = ref(false)
const genBusy = ref(false)
const genForm = ref({ count: 10, hours: 720 })
const genResult = ref<string[]>([])

async function refresh() {
  if (apps.value.length === 0) {
    const ar = await client.licenseSvc.apps()
    if (ar.status === 0 && ar.data) {
      apps.value = ar.data.items
      if (!appId.value && apps.value.length) appId.value = apps.value[0].id
    }
    if (!appId.value) { list.value = []; total.value = 0; return }
  }
  const r = await client.licenseSvc.list({ appId: appId.value, page: page.value, pageSize, kw: kw.value })
  if (r.status === 0 && r.data) {
    list.value = r.data.items
    total.value = r.data.total
  }
  const s = await client.licenseSvc.stats({ appId: appId.value })
  if (s.status === 0 && s.data) stats.value = s.data
}
function switchApp(id: number) { appId.value = id; page.value = 1; void refresh() }
onMounted(refresh)
onActivated(refresh)

function search() { page.value = 1; void refresh() }

async function generate() {
  genBusy.value = true
  try {
    const r = await client.licenseSvc.generate({ appId: appId.value, count: genForm.value.count, hours: genForm.value.hours })
    if (r.status === 0 && r.data) {
      genResult.value = r.data.cards
      message.success(`已生成 ${r.data.cards.length} 张卡密`)
      void refresh()
    } else {
      message.error(r.msg || '生成失败')
    }
  } finally {
    genBusy.value = false
  }
}

async function copyCards() {
  message.success(await copyText(genResult.value.join('\n')) ? '已复制全部卡密' : '复制失败')
}

async function copyCard(c: string) {
  message.success(await copyText(c) ? '卡密已复制' : '复制失败')
}

async function remove(item: licenseCardView) {
  const r = await client.licenseSvc.delete({ id: item.id })
  if (r.status === 0) { message.success('已删除'); void refresh() }
  else message.error(r.msg || '删除失败')
}

const statusMap: Record<number, { label: string; type: 'default' | 'success' | 'error' }> = {
  0: { label: '未使用', type: 'default' },
  1: { label: '已激活', type: 'success' },
  2: { label: '已过期', type: 'error' }
}

function fmtTime(ts: number): string {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

function fmtHours(h: number): string {
  if (h === 0) return '永久'
  if (h % (24 * 365) === 0) return `${h / (24 * 365)} 年`
  if (h % 24 === 0) return `${h / 24} 天`
  return `${h} 小时`
}
</script>

<template>
  <div class="lic-page">
    <div class="stat-grid">
      <div class="stat-card" style="background:var(--cb-mint)">
        <div class="s-num">{{ stats.total }}</div><div class="s-lb">卡密总数</div>
      </div>
      <div class="stat-card">
        <div class="s-num">{{ stats.unused }}</div><div class="s-lb">未使用</div>
      </div>
      <div class="stat-card" style="background:var(--cb-yellow)">
        <div class="s-num">{{ stats.active }}</div><div class="s-lb">已激活</div>
      </div>
      <div class="stat-card">
        <div class="s-num">{{ stats.expired }}</div><div class="s-lb">已过期</div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">
        <n-icon style="vertical-align:-2px"><KeyOutline /></n-icon> 卡密管理
        <n-select
          :value="appId" :options="apps.map(a => ({ label: a.name, value: a.id }))"
          size="small" style="width:160px" @update:value="switchApp"
        />
        <span class="hint">{{ total }} 张</span>
        <div class="bar-right">
          <n-input
            v-model:value="kw" size="small" placeholder="搜卡密 / 域名" clearable
            style="width:180px" @keyup.enter="search" @clear="search"
          />
          <n-button size="small" @click="search">
            <template #icon><n-icon><SearchOutline /></n-icon></template>查询
          </n-button>
          <n-button size="small" type="primary" @click="genVisible = true; genResult = []">
            <template #icon><n-icon><AddOutline /></n-icon></template>生成卡密
          </n-button>
        </div>
      </div>

      <div v-if="list.length" class="lic-table">
        <div class="lic-head">
          <span>卡密</span><span>时长</span><span>状态</span><span>绑定域名</span>
          <span>到期时间</span><span>激活时间</span><span>创建</span><span></span>
        </div>
        <div v-for="c in list" :key="c.id" class="lic-row">
          <span class="mono lic-card" @click="copyCard(c.card)">{{ c.card }}</span>
          <span>{{ fmtHours(c.hours) }}</span>
          <span><n-tag size="small" :type="statusMap[c.status]?.type" :bordered="false">{{ statusMap[c.status]?.label }}</n-tag></span>
          <span class="mono dim">{{ c.domain || '—' }}</span>
          <span class="dim">{{ c.activatedAt ? (c.expiresAt ? fmtTime(c.expiresAt) : '永久') : '—' }}</span>
          <span class="dim">{{ fmtTime(c.activatedAt) }}</span>
          <span class="dim">{{ fmtTime(c.createdAt) }}</span>
          <n-popconfirm @positive-click="remove(c)">
            <template #trigger>
              <n-button size="tiny" quaternary><template #icon><n-icon><TrashOutline /></n-icon></template></n-button>
            </template>
            删除该卡密？
          </n-popconfirm>
        </div>
      </div>
      <n-empty v-else description="还没有卡密，点右上角生成" style="padding:60px 0" />

      <div v-if="total > pageSize" class="pager">
        <n-button size="small" :disabled="page <= 1" @click="page--; refresh()">上一页</n-button>
        <span class="dim">{{ page }} / {{ Math.ceil(total / pageSize) }} 页 · 共 {{ total }} 张</span>
        <n-button size="small" :disabled="page * pageSize >= total" @click="page++; refresh()">下一页</n-button>
      </div>

      <div class="usage">
        <b>接入方式</b>
        <div class="mono usage-line">GET /api/v1/license?appid=&lt;项目appid&gt;&amp;card=&lt;卡密&gt;&amp;domain=&lt;域名&gt;</div>
        <div class="dim">
          首次调用激活并绑定域名（开始计时）；之后仅同域名返回 <span class="mono">code=1</span>。
          换域名 → <span class="mono">卡密已绑定其他域名</span>；过期 → <span class="mono">卡密已过期</span>。
          建议项目启动时校验一次，运行中定时复查。
        </div>
      </div>
    </div>

    <n-modal v-model:show="genVisible" preset="card" title="生成卡密" style="width: 460px">
      <template v-if="!genResult.length">
        <n-form>
          <n-form-item label="数量">
            <n-input-number v-model:value="genForm.count" :min="1" :max="500" style="width:100%" />
          </n-form-item>
          <n-form-item label="有效时长（小时）· 0 = 永久，720 = 30 天">
            <n-input-number v-model:value="genForm.hours" :min="0" style="width:100%" />
          </n-form-item>
        </n-form>
        <div style="display:flex;justify-content:flex-end;gap:8px">
          <n-button @click="genVisible = false">取消</n-button>
          <n-button type="primary" :loading="genBusy" @click="generate">生成</n-button>
        </div>
      </template>
      <template v-else>
        <pre class="gen-list mono">{{ genResult.join('\n') }}</pre>
        <div style="display:flex;justify-content:flex-end;gap:8px;margin-top:12px">
          <n-button @click="genVisible = false">关闭</n-button>
          <n-button type="primary" @click="copyCards">
            <template #icon><n-icon><CopyOutline /></n-icon></template>复制全部
          </n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.lic-page { display: flex; flex-direction: column; gap: 14px; }
.stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.stat-card {
  border: var(--cb-border); border-radius: 14px; padding: 14px 18px;
  background: #fff; box-shadow: var(--cb-shadow-sm);
}
.s-num { font-size: 26px; font-weight: 900; font-family: Consolas, monospace; }
.s-lb { font-size: 12px; font-weight: 800; color: var(--cb-ink-3); margin-top: 2px; }

.bar-right { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.mono { font-family: Consolas, monospace; }
.dim { color: var(--cb-ink-3); font-weight: 500; }

.lic-table { display: flex; flex-direction: column; }
.lic-head, .lic-row {
  display: grid;
  grid-template-columns: 210px 70px 80px 1.2fr 110px 110px 110px 40px;
  gap: 8px; align-items: center; padding: 8px 10px; font-size: 12.5px;
}
.lic-head { font-weight: 800; color: var(--cb-ink-3); font-size: 11px; border-bottom: 2px solid var(--cb-ink); }
.lic-row { border-bottom: 1px dashed var(--cb-cream-2, #e5e0d5); }
.lic-row:hover { background: var(--cb-cream, #f7f4ec); }
.lic-card { font-size: 12.5px; font-weight: 700; cursor: pointer; }
.lic-card:hover { color: var(--cb-primary, #ec5b13); }

.pager { display: flex; align-items: center; justify-content: center; gap: 14px; margin-top: 14px; }
.usage {
  margin-top: 16px; border: 2px dashed var(--cb-ink); border-radius: 12px;
  padding: 12px 14px; background: var(--cb-cream-2, #f1eee8); font-size: 13px;
}
.usage-line { margin: 8px 0 4px; font-size: 13px; }
.gen-list {
  max-height: 300px; overflow: auto; background: var(--cb-cream, #f7f4ec);
  border-radius: 8px; padding: 10px; font-size: 12.5px; margin: 0; line-height: 1.9;
}
@media (max-width: 900px) { .stat-grid { grid-template-columns: 1fr 1fr; } .lic-head { display: none; } }
</style>

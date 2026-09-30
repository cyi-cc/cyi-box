<script setup lang="ts">
import { onMounted, onActivated, ref } from 'vue'
import {
  NButton, NIcon, NInput, NSelect, NEmpty, NPagination, useMessage
} from 'naive-ui'
import {
  SearchOutline, RefreshOutline, CheckmarkCircleOutline,
  TimeOutline, CloseCircleOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type payOrderView from '../api/payOrderView'

const message = useMessage()

// ---- 真实数据 ----
const orders = ref<payOrderView[]>([])
const total = ref(0)
const loading = ref(false)
const stats = ref({ todayMoney: 0, todayCount: 0, todayPaidCount: 0, totalCount: 0, totalMoney: 0, pendingCount: 0 })

const keyword = ref('')
const status = ref<number | null>(null)
const page = ref(1)
const pageSize = 15

async function load() {
  loading.value = true
  try {
    const [r, s] = await Promise.all([
      client.paySvc.list({ page: page.value, pageSize, status: status.value ?? undefined, kw: keyword.value.trim() || undefined }),
      client.paySvc.stats()
    ])
    if (r.status === 0 && r.data) {
      orders.value = r.data.items ?? []
      total.value = r.data.total
    }
    if (s.status === 0 && s.data) stats.value = s.data
  } finally {
    loading.value = false
  }
}
function search() { page.value = 1; void load() }
function refresh() { void load() }
onMounted(load)
onActivated(load)

const rate = () => stats.value.todayCount
  ? Math.round((stats.value.todayPaidCount / stats.value.todayCount) * 100) : 0

const statusMeta: Record<number, { label: string; cls: string }> = {
  1: { label: '已支付', cls: 'ok' },
  0: { label: '待支付', cls: 'wait' },
  3: { label: '已关闭', cls: 'off' }
}
const metaOf = (s: number) => statusMeta[s] ?? statusMeta[3]

function fen2yuan(fen: number): string { return (fen / 100).toFixed(2) }
function fmtTime(ts: number): string {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
async function copyNo(o: payOrderView) {
  message.success(await copyText(o.tradeNo) ? '订单号已复制' : '复制失败')
}
</script>

<template>
  <div class="pay-page">
    <div class="panel">
      <div class="panel-title">
        订单管理
        <span class="title-actions">
          <n-button size="small" :loading="loading" @click="refresh">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>刷新
          </n-button>
        </span>
      </div>
      <div class="stat-strip">
        <div class="stat-cell">
          <div class="stat-num">¥{{ fen2yuan(stats.todayMoney) }}</div>
          <div class="stat-lb">今日交易额</div>
        </div>
        <div class="stat-cell">
          <div class="stat-num">{{ stats.todayCount }}</div>
          <div class="stat-lb">今日订单</div>
        </div>
        <div class="stat-cell">
          <div class="stat-num">{{ rate() }}%</div>
          <div class="stat-lb">支付成功率</div>
        </div>
        <div class="stat-cell">
          <div class="stat-num">{{ stats.pendingCount }}</div>
          <div class="stat-lb">待支付</div>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="filters">
        <n-input
          v-model:value="keyword" size="small" placeholder="搜订单号 / 商户单号" style="width:220px"
          clearable @keyup.enter="search" @clear="search"
        >
          <template #prefix><n-icon><SearchOutline /></n-icon></template>
        </n-input>
        <n-select
          v-model:value="status" size="small" style="width:120px" placeholder="全部状态" clearable
          :options="[
            { label: '已支付', value: 1 },
            { label: '待支付', value: 0 },
            { label: '已关闭', value: 3 }
          ]"
          @update:value="search"
        />
        <span class="hint" style="margin-left:auto">共 {{ total }} 笔</span>
      </div>

      <div class="order-table">
        <div class="ot-head">
          <span class="c-no">订单号</span>
          <span class="c-name">商品</span>
          <span class="c-money">金额</span>
          <span class="c-ch">渠道</span>
          <span class="c-st">状态</span>
          <span class="c-time">创建时间</span>
          <span class="c-time">支付时间</span>
        </div>
        <div v-for="o in orders" :key="o.tradeNo" class="ot-row" @click="copyNo(o)">
          <span class="c-no mono">{{ o.tradeNo }}</span>
          <span class="c-name">{{ o.subject }}</span>
          <span class="c-money mono">¥{{ fen2yuan(o.money) }}</span>
          <span class="c-ch"><span class="ch-chip">支付宝</span></span>
          <span class="c-st">
            <span class="st-chip" :class="metaOf(o.status).cls">
              <n-icon :size="12">
                <CheckmarkCircleOutline v-if="o.status === 1" />
                <TimeOutline v-else-if="o.status === 0" />
                <CloseCircleOutline v-else />
              </n-icon>
              {{ metaOf(o.status).label }}
            </span>
          </span>
          <span class="c-time">{{ fmtTime(o.createdAt) }}</span>
          <span class="c-time">{{ fmtTime(o.paidAt) }}</span>
        </div>
        <n-empty v-if="!orders.length && !loading" description="暂无订单" style="padding:40px 0" />
      </div>

      <div class="pager">
        <n-pagination v-model:page="page" :item-count="total" :page-size="pageSize" @update:page="load" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.pay-page { display: flex; flex-direction: column; gap: 16px; }
.title-actions { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.stat-strip { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-top: 12px; }
.stat-cell {
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  background: var(--cb-cream-2, #f1eee8); padding: 10px 14px;
}
.stat-cell:nth-child(1) { background: var(--cb-mint); }
.stat-cell:nth-child(2) { background: var(--cb-yellow); }
.stat-num { font-size: 20px; font-weight: 900; font-family: Consolas, monospace; line-height: 1.2; }
.stat-lb { font-size: 11px; font-weight: 800; color: var(--cb-ink-3); margin-top: 2px; }

.filters { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.mono { font-family: Consolas, monospace; }
.hint { font-size: 12px; color: var(--cb-ink-3); font-weight: 500; }

.order-table { border: 2.5px solid var(--cb-ink); border-radius: 12px; overflow: hidden; }
.ot-head, .ot-row {
  display: grid; align-items: center;
  grid-template-columns: 220px 1fr 90px 70px 90px 110px 110px;
  gap: 8px; padding: 0 14px;
}
.ot-head {
  font-size: 12px; font-weight: 800; color: var(--cb-ink-3);
  background: var(--cb-cream-2, #f1eee8); height: 36px;
  border-bottom: 2.5px solid var(--cb-ink);
}
.ot-row {
  height: 44px; font-size: 13px; background: #fff; cursor: pointer;
  border-bottom: 1px solid var(--cb-cream-2, #f1eee8);
}
.ot-row:hover { background: var(--cb-cream-2, #f1eee8); }
.ot-row:last-child { border-bottom: none; }
.c-no { font-size: 12px; overflow: hidden; }
.c-money { font-weight: 800; }
.ch-chip {
  font-size: 11px; font-weight: 800; padding: 1px 8px;
  border: 2px solid var(--cb-ink); border-radius: 6px; background: var(--cb-mint);
}
.st-chip {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 11px; font-weight: 800; padding: 1px 8px;
  border: 2px solid var(--cb-ink); border-radius: 6px;
}
.st-chip.ok { background: var(--cb-mint); }
.st-chip.wait { background: var(--cb-yellow); }
.st-chip.off { background: var(--cb-cream-2, #f1eee8); color: var(--cb-ink-3); }
.c-time { font-size: 12px; color: var(--cb-ink-3); font-family: Consolas, monospace; }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { NIcon, NSpin } from 'naive-ui'
import { CheckmarkCircleOutline, TimeOutline, CloseCircleOutline } from '@vicons/ionicons5'
import QRCode from 'qrcode'
import { client } from '../lib/api'

const route = useRoute()
const tradeNo = String(route.params.tradeNo ?? '')

const info = ref<{
  tradeNo: string; outTradeNo: string; subject: string
  money: number; status: number; payurl: string; qrcode: string
  channel: string; cards: string; expiredAt: number
} | null>(null)
const qrDataUrl = ref('')
const notFound = ref(false)
const redirecting = ref(false)
const now = ref(Math.floor(Date.now() / 1000))
const abort = new AbortController()
let pollTimer: ReturnType<typeof setInterval> | null = null
let clockTimer: ReturnType<typeof setInterval> | null = null
function stopPoll() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

async function applyStatus(status: number, returnUrl?: string) {
  if (!info.value) return
  info.value.status = status
  if (status === 1) {
    const d = await client.paySvc.cashierInfo({ tradeNo })
    if (d.data) info.value.cards = d.data.cards
    if (returnUrl) { redirecting.value = true; setTimeout(() => { location.href = returnUrl }, 1500) }
  }
}

function startPoll() {
  stopPoll()
  pollTimer = setInterval(() => {
    if (!info.value || info.value.status !== 0) { stopPoll(); return }
    void client.paySvc.cashierStatus({ tradeNo }).then(r => {
      if (r.status === 0 && r.data) void applyStatus(r.data.status, r.data.returnUrl)
    })
  }, 3000)
}

const statusLabel = computed(() => {
  if (!info.value) return ''
  return { 0: '等待支付', 1: '支付成功', 3: '订单已关闭' }[info.value.status] ?? '订单已关闭'
})

const remainSec = computed(() => {
  if (!info.value || !info.value.expiredAt) return 0
  return Math.max(0, info.value.expiredAt - now.value)
})
const remainText = computed(() => {
  const m = Math.floor(remainSec.value / 60)
  const s = remainSec.value % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
})
const remainPct = computed(() => Math.min(100, (remainSec.value / 1800) * 100))

const chMeta = computed(() => info.value?.channel === 'wxpay'
  ? { name: '微信', badge: '微', color: '#07c160' }
  : { name: '支付宝', badge: '支', color: '#1677ff' })

async function load() {
  const r = await client.paySvc.cashierInfo({ tradeNo })
  if (r.status !== 0 || !r.data) { notFound.value = true; return }
  info.value = r.data
  if (r.data.qrcode) {
    qrDataUrl.value = await QRCode.toDataURL(r.data.qrcode, { width: 240, margin: 1 })
  }
  // SSE 推送订单状态（终态自动关闭流）；断流重连，流不可用时降级轮询
  if (info.value.status === 0) {
    try {
      void client.paySvc.watch({ tradeNo }, async msg => {
        await applyStatus(msg.status, msg.returnUrl)
      }, { signal: abort.signal, retry: {} }).then(r => {
        if (r.status !== 0 && !abort.signal.aborted && info.value?.status === 0) {
          console.warn('[pay] 状态流不可用，降级轮询:', r.msg)
          startPoll()
        }
      })
    } catch (e) {
      console.warn('[pay] 状态流不可用，降级轮询:', e)
      startPoll()
    }
  }
}

onMounted(() => {
  load()
  clockTimer = setInterval(() => { now.value = Math.floor(Date.now() / 1000) }, 1000)
})
onUnmounted(() => { abort.abort(); stopPoll(); if (clockTimer) clearInterval(clockTimer) })

function fen2yuan(fen: number): string { return (fen / 100).toFixed(2) }
</script>

<template>
  <div class="cashier-wrap">
    <div class="cashier-box">
      <div class="cash-head">
        <div class="cash-logo">池易支付</div>
        <div class="cash-sub">订单号 {{ tradeNo }}</div>
      </div>

      <n-spin v-if="!info && !notFound" size="large" style="margin:60px auto;display:block" />
      <div v-else-if="notFound" class="cash-state">
        <n-icon :size="48" color="#d03050"><CloseCircleOutline /></n-icon>
        <div>订单不存在</div>
      </div>

      <template v-else>
        <div class="cash-subject">{{ info!.subject || '商品订单' }}</div>
        <div class="cash-amount"><span class="cash-cny">¥</span>{{ fen2yuan(info!.money) }}</div>

        <div v-if="info!.status === 0" class="cash-qr-area">
          <div class="cash-qr-frame">
            <img v-if="qrDataUrl" :src="qrDataUrl" class="cash-qr" :alt="chMeta.name + '扫码'" />
            <div v-else class="cash-qr-empty">二维码生成中…</div>
            <div class="cash-qr-corner tl" /><div class="cash-qr-corner tr" />
            <div class="cash-qr-corner bl" /><div class="cash-qr-corner br" />
          </div>
          <div class="cash-qr-tip">
            <span class="cash-ch-badge" :style="{ background: chMeta.color }">{{ chMeta.badge }}</span>
            打开<b>{{ chMeta.name }}</b>扫一扫，向「池易」付款
          </div>
          <div class="cash-countdown">
            <n-icon><TimeOutline /></n-icon>
            支付剩余时间 {{ remainText }}
            <div class="cash-countdown-bar"><i :style="{ width: remainPct + '%' }" /></div>
          </div>
        </div>

        <div v-else-if="info!.status === 1" class="cash-state">
          <n-icon :size="56" color="#18a058"><CheckmarkCircleOutline /></n-icon>
          <div class="cash-ok">{{ statusLabel }}</div>
          <div v-if="redirecting" class="cash-redirect">正在跳回商家页面…</div>
          <pre v-if="info!.cards" class="cash-cards">{{ info!.cards }}</pre>
        </div>

        <div v-else class="cash-state">
          <n-icon :size="48" color="#909399"><CloseCircleOutline /></n-icon>
          <div>{{ statusLabel }}</div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.cashier-wrap {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  background: var(--cb-cream, #f7f4ec); padding: 20px;
}
.cashier-box {
  width: 380px; background: #fff; border: 3px solid var(--cb-ink, #1c1b1a);
  border-radius: 20px; padding: 28px; text-align: center;
  box-shadow: 6px 6px 0 var(--cb-ink, #1c1b1a);
}
.cash-head { border-bottom: 2px dashed var(--cb-cream-2, #e5e0d5); padding-bottom: 14px; margin-bottom: 16px; }
.cash-logo { font-size: 20px; font-weight: 900; }
.cash-sub { font-size: 11px; color: var(--cb-ink-3, #8a8577); font-family: Consolas, monospace; margin-top: 4px; }
.cash-subject { font-size: 14px; font-weight: 700; }
.cash-amount { font-size: 40px; font-weight: 900; font-family: Consolas, monospace; margin: 8px 0 18px; }
.cash-cny { font-size: 22px; vertical-align: 6px; margin-right: 2px; }
.cash-qr-area { display: flex; flex-direction: column; align-items: center; gap: 14px; }
.cash-qr-frame {
  position: relative; padding: 14px; background: #fff;
  border: 3px solid var(--cb-ink, #1c1b1a); border-radius: 16px;
  box-shadow: 5px 5px 0 var(--cb-ink, #1c1b1a);
}
.cash-qr { display: block; border-radius: 8px; }
.cash-qr-empty {
  width: 240px; height: 240px; display: flex; align-items: center; justify-content: center;
  color: var(--cb-ink-3, #8a8577); font-size: 13px;
}
.cash-qr-corner {
  position: absolute; width: 18px; height: 18px; border: 3px solid #1677ff;
}
.cash-qr-corner.tl { top: -6px; left: -6px; border-right: none; border-bottom: none; border-radius: 8px 0 0 0; }
.cash-qr-corner.tr { top: -6px; right: -6px; border-left: none; border-bottom: none; border-radius: 0 8px 0 0; }
.cash-qr-corner.bl { bottom: -6px; left: -6px; border-right: none; border-top: none; border-radius: 0 0 0 8px; }
.cash-qr-corner.br { bottom: -6px; right: -6px; border-left: none; border-top: none; border-radius: 0 0 8px 0; }
.cash-qr-tip {
  font-size: 14px; font-weight: 700; color: var(--cb-ink, #1c1b1a);
  display: flex; align-items: center; gap: 8px;
}
.cash-ch-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 24px; height: 24px; background: #1677ff; color: #fff;
  border-radius: 6px; font-size: 14px; font-weight: 900;
  border: 2px solid var(--cb-ink, #1c1b1a);
}
.cash-countdown {
  font-size: 12px; color: var(--cb-ink-3, #8a8577);
  display: flex; align-items: center; gap: 6px; width: 100%;
}
.cash-countdown-bar {
  flex: 1; height: 8px; background: var(--cb-cream-2, #e5e0d5);
  border: 2px solid var(--cb-ink, #1c1b1a); border-radius: 6px; overflow: hidden;
}
.cash-countdown-bar i {
  display: block; height: 100%; background: #1677ff; transition: width 1s linear;
}
.cash-state { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 30px 0; font-size: 15px; font-weight: 700; }
.cash-ok { color: #18a058; font-size: 18px; }
.cash-redirect { font-size: 12px; color: var(--cb-ink-3, #8a8577); font-weight: 400; }
.cash-cards {
  text-align: left; font-size: 12px; font-family: Consolas, monospace; max-height: 200px;
  overflow: auto; background: var(--cb-cream, #f7f4ec); border-radius: 8px; padding: 10px; margin: 0;
  white-space: pre-wrap; word-break: break-all; width: 100%;
}
</style>

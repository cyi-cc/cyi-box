<script setup lang="ts">
import { onMounted, onActivated, onUnmounted, ref } from 'vue'
import {
  NButton, NForm, NFormItem, NInput, NIcon, NInputNumber, NTag, NModal, useMessage
} from 'naive-ui'
import {
  KeyOutline, CopyOutline, RefreshOutline,
  LinkOutline, FlashOutline, CheckmarkCircleOutline, CloseCircleOutline
} from '@vicons/ionicons5'
import QRCode from 'qrcode'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type payUpstreamStatusView from '../api/payUpstreamStatusView'

const message = useMessage()

// ---- 商户凭证（pay_pid / pay_key 存 settings 表） ----
const cred = ref({ pid: '', key: '' })
const credVisible = ref(false)
const genSaving = ref(false)

function randKey(len: number): string {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  const buf = new Uint8Array(len)
  crypto.getRandomValues(buf)
  return [...buf].map(b => chars[b % chars.length]).join('')
}

async function loadCred() {
  const [p, k] = await Promise.all([
    client.settingSvc.get({ key: 'pay_pid', value: '' }),
    client.settingSvc.get({ key: 'pay_key', value: '' })
  ])
  cred.value.pid = p.data?.value ?? ''
  cred.value.key = k.data?.value ?? ''
  if (!cred.value.pid || !cred.value.key) await genCred(false)
}

async function genCred(notify = true) {
  genSaving.value = true
  try {
    const pid = String(1000 + Math.floor(Math.random() * 9000))
    const key = 'pk_live_' + randKey(24)
    await client.settingSvc.set({ key: 'pay_pid', value: pid })
    const r = await client.settingSvc.set({ key: 'pay_key', value: key })
    if (r.status === 0) {
      cred.value = { pid, key }
      credVisible.value = true
      if (notify) message.success('密钥已重置')
    } else if (notify) {
      message.error(r.msg || '生成失败')
    }
  } finally {
    genSaving.value = false
  }
}

async function copyCred(v: string) {
  message.success(await copyText(v) ? '已复制' : '复制失败')
}

// ---- 上游账号 + 状态 ----
const upstream = ref({ account: '', password: '' })
const saving = ref(false)
const upStatus = ref<payUpstreamStatusView | null>(null)

async function loadStatus() {
  const r = await client.paySvc.upstreamStatus()
  if (r.status === 0 && r.data) {
    upStatus.value = r.data
    upstream.value.account = r.data.username
    // 自检进行中时轮询一次
    if (r.data.ensuring === 1) setTimeout(loadStatus, 3000)
  }
}

async function saveCfg() {
  saving.value = true
  try {
    const r = await client.paySvc.saveUpstream({
      username: upstream.value.account.trim(),
      password: upstream.value.password
    })
    if (r.status === 0 && r.data) {
      upStatus.value = r.data
      upstream.value.password = ''
      message.success('已登录上游，商品自检/补库存后台进行中')
      setTimeout(loadStatus, 3000)
    } else {
      message.error(r.msg || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

onMounted(() => { void loadCred(); void loadStatus() })
onActivated(() => { void loadStatus() })

// ---- 测试支付（弹窗收银台 + SSE 状态推送） ----
const testFen = ref<number>(1)
const testing = ref(false)
const payModal = ref(false)
const payOrder = ref<{ tradeNo: string; money: number; status: number } | null>(null)
const payQrImg = ref('')
const payCards = ref('')
let payAbort: AbortController | null = null

async function testPay() {
  testing.value = true
  try {
    const money = (testFen.value / 100).toFixed(2)
    const r = await client.paySvc.testPay({ money })
    if (r.status === 0 && r.data) {
      payOrder.value = { tradeNo: r.data.tradeNo, money: r.data.money, status: 0 }
      payCards.value = ''
      if (r.data.qrcode) {
        payQrImg.value = await QRCode.toDataURL(r.data.qrcode, { width: 220, margin: 1 })
      } else {
        payQrImg.value = ''
      }
      payModal.value = true
      watchOrder(r.data.tradeNo)
    } else {
      message.error(r.msg || '下单失败')
    }
  } finally {
    testing.value = false
  }
}

// SSE：订单状态推送，断流自动重连；流不可用（旧缓存/异常）时降级 3s 轮询
let pollTimer: ReturnType<typeof setInterval> | null = null
function stopPoll() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

async function applyStatus(tradeNo: string, status: number) {
  if (!payOrder.value || payOrder.value.tradeNo !== tradeNo) return
  payOrder.value.status = status
  if (status === 1) {
    const d = await client.paySvc.cashierInfo({ tradeNo })
    if (d.data?.cards) payCards.value = d.data.cards
  }
}

function startPoll(tradeNo: string) {
  stopPoll()
  pollTimer = setInterval(() => {
    if (!payModal.value || !payOrder.value || payOrder.value.tradeNo !== tradeNo || payOrder.value.status !== 0) {
      stopPoll()
      return
    }
    void client.paySvc.cashierStatus({ tradeNo }).then(r => {
      if (r.status === 0 && r.data) void applyStatus(tradeNo, r.data.status)
    })
  }, 3000)
}

function watchOrder(tradeNo: string) {
  payAbort?.abort()
  payAbort = new AbortController()
  stopPoll()
  const fallback = (why: unknown) => {
    if (payOrder.value?.tradeNo === tradeNo && payOrder.value.status === 0 && payModal.value) {
      console.warn('[pay] 状态流不可用，降级轮询:', why)
      startPoll(tradeNo)
    }
  }
  try {
    void client.paySvc.watch(
      { tradeNo },
      async msg => { await applyStatus(tradeNo, msg.status) },
      { signal: payAbort.signal, retry: {} }
    ).then(r => { if (r.status !== 0 && !payAbort?.signal.aborted) fallback(r.msg) })
  } catch (e) {
    fallback(e)
  }
}

function closePayModal() {
  payModal.value = false
  payAbort?.abort()
  payAbort = null
  stopPoll()
  void loadStatus() // 顺带刷新库存显示
}

onUnmounted(() => { payAbort?.abort(); stopPoll() })

function fen2yuan(fen: number): string { return (fen / 100).toFixed(2) }
</script>

<template>
  <div class="pay-cfg-page">
    <!-- 商户凭证 -->
    <div class="panel cred-panel">
      <div class="cred-left">
        <div class="panel-title" style="margin-bottom:10px">
          <n-icon><KeyOutline /></n-icon> 商户接入凭证
        </div>
        <div class="cred-row">
          <span class="cred-label">商户 PID</span>
          <span class="cred-val mono">{{ cred.pid || '—' }}</span>
          <n-button v-if="cred.pid" size="tiny" quaternary @click="copyCred(cred.pid)">
            <template #icon><n-icon :size="13"><CopyOutline /></n-icon></template>
          </n-button>
        </div>
        <div class="cred-row">
          <span class="cred-label">易支付密钥</span>
          <span class="cred-val mono">{{ cred.key ? (credVisible ? cred.key : cred.key.slice(0, 12) + '••••••••') : '—' }}</span>
          <template v-if="cred.key">
            <n-button size="tiny" quaternary @click="credVisible = !credVisible">
              {{ credVisible ? '隐藏' : '显示' }}
            </n-button>
            <n-button size="tiny" quaternary @click="copyCred(cred.key)">
              <template #icon><n-icon :size="13"><CopyOutline /></n-icon></template>
            </n-button>
          </template>
        </div>
        <n-button size="small" quaternary :loading="genSaving" @click="genCred()">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>重置密钥
        </n-button>
      </div>
      <div class="cred-right">
        <div class="bal-label">对接协议</div>
        <div class="bal-num">易支付</div>
        <div class="bal-sub">submit.php · mapi.php · api.php</div>
      </div>
    </div>

    <div class="cfg-grid">
      <!-- 上游账号 -->
      <div class="panel">
        <div class="panel-title"><n-icon><LinkOutline /></n-icon> 上游平台账号（catfk.com）</div>
        <n-form label-placement="top">
          <n-form-item label="商户账号">
            <n-input v-model:value="upstream.account" placeholder="上游卡密平台登录账号" />
          </n-form-item>
          <n-form-item label="商户密码">
            <n-input v-model:value="upstream.password" type="password" show-password-on="click" placeholder="已保存可留空" />
          </n-form-item>
        </n-form>
        <n-button type="primary" :loading="saving" @click="saveCfg">保存并验证</n-button>

        <div v-if="upStatus?.configured === 1" class="up-status">
          <div class="up-row">
            <span class="up-lb">商户</span>
            <span>{{ upStatus.nickname || upStatus.username }}</span>
          </div>
          <div class="up-row">
            <span class="up-lb">商品</span>
            <span>{{ upStatus.goodsName || '—' }}
              <n-tag v-if="upStatus.goodsId" size="tiny" type="success">id={{ upStatus.goodsId }}</n-tag>
            </span>
          </div>
          <div class="up-row">
            <span class="up-lb">卡密库存</span>
            <span class="mono">
              {{ upStatus.stockCount }} 张（≈ ¥{{ fen2yuan(upStatus.stockCount * upStatus.unitPrice) }}）
            </span>
            <n-tag v-if="upStatus.ensuring === 1" size="tiny" type="warning">补库存中…</n-tag>
          </div>
          <div class="up-row" v-if="upStatus.walletOk === 1">
            <span class="up-lb">上游余额</span>
            <span class="mono">¥{{ fen2yuan(upStatus.walletAvail) }}（冻结 ¥{{ fen2yuan(upStatus.walletFrozen) }}）</span>
          </div>
          <div class="up-row err" v-if="upStatus.lastError">
            <span class="up-lb">自检错误</span>
            <span>{{ upStatus.lastError }}</span>
          </div>
        </div>
        <div v-else class="hint-line">保存后自动登录、自检分类「CDK」/商品「CDK0.01元」并补齐 ¥1000 库存。</div>
      </div>

      <!-- 测试支付 -->
      <div class="panel">
        <div class="panel-title"><n-icon><FlashOutline /></n-icon> 测试支付</div>
        <n-form label-placement="top">
          <n-form-item label="金额（分）">
            <n-input-number v-model:value="testFen" :min="1" :max="200000" style="width:100%" />
          </n-form-item>
        </n-form>
        <n-button type="primary" :loading="testing" @click="testPay">发起支付</n-button>
        <div class="hint-line">走真实上游下单链路，弹窗出支付宝付款码，实时推送支付结果。</div>
      </div>
    </div>

    <!-- 收银台弹窗（fast-pay 同款） -->
    <n-modal :show="payModal" preset="card" style="width:340px" :mask-closable="false" @close="closePayModal" @update:show="v => { if (!v) closePayModal() }">
      <div class="cash-modal" v-if="payOrder">
        <div class="cm-subject">测试支付</div>
        <div class="cm-amount">¥{{ fen2yuan(payOrder.money) }}</div>
        <div class="cm-no mono">{{ payOrder.tradeNo }}</div>

        <div v-if="payOrder.status === 0" class="cm-body">
          <img v-if="payQrImg" :src="payQrImg" class="cm-qr" alt="支付宝扫码" />
          <div class="cm-tip">请使用 <b>支付宝</b> 扫码支付</div>
        </div>
        <div v-else-if="payOrder.status === 1" class="cm-body">
          <n-icon :size="52" color="#18a058"><CheckmarkCircleOutline /></n-icon>
          <div class="cm-ok">支付成功</div>
          <pre v-if="payCards" class="cm-cards">{{ payCards }}</pre>
        </div>
        <div v-else class="cm-body">
          <n-icon :size="44" color="#909399"><CloseCircleOutline /></n-icon>
          <div class="cm-tip">订单已关闭</div>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<style scoped>
.pay-cfg-page { display: flex; flex-direction: column; gap: 16px; }

.cred-panel { display: grid; grid-template-columns: 1fr 240px; gap: 16px; align-items: center; }
.cred-left { min-width: 0; }
.cred-row {
  display: flex; align-items: center; gap: 8px; padding: 8px 0;
  border-bottom: 1px dashed var(--cb-cream-2, #e5e0d5);
}
.cred-label { width: 80px; font-size: 12px; font-weight: 800; color: var(--cb-ink-3); flex: none; }
.cred-val { font-size: 13px; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono { font-family: Consolas, monospace; }
.cred-right {
  border: 2.5px solid var(--cb-ink); border-radius: 14px; padding: 18px;
  background: var(--cb-mint); text-align: center;
}
.bal-label { font-size: 11px; font-weight: 800; color: var(--cb-ink-3); }
.bal-num { font-size: 26px; font-weight: 900; margin: 4px 0; }
.bal-sub { font-size: 11px; color: var(--cb-ink-3); font-weight: 600; }

.cfg-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; align-items: start; }
.hint-line { margin-top: 10px; font-size: 12px; color: var(--cb-ink-3); font-weight: 500; }

.up-status { margin-top: 14px; border-top: 1px dashed var(--cb-cream-2, #e5e0d5); padding-top: 10px; }
.up-row { display: flex; align-items: center; gap: 8px; font-size: 13px; padding: 3px 0; }
.up-lb { width: 64px; flex: none; font-size: 11px; font-weight: 800; color: var(--cb-ink-3); }
.up-row.err { color: #d03050; font-size: 12px; }

/* 收银台弹窗 */
.cash-modal { text-align: center; }
.cm-subject { font-size: 14px; font-weight: 700; }
.cm-amount { font-size: 32px; font-weight: 900; font-family: Consolas, monospace; margin: 6px 0 2px; }
.cm-no { font-size: 11px; color: var(--cb-ink-3); margin-bottom: 14px; }
.cm-body { display: flex; flex-direction: column; align-items: center; gap: 10px; padding-bottom: 4px; }
.cm-qr { border: 3px solid var(--cb-ink, #1c1b1a); border-radius: 12px; width: 220px; }
.cm-tip { font-size: 13px; color: var(--cb-ink-3); }
.cm-ok { font-size: 18px; font-weight: 800; color: #18a058; }
.cm-cards {
  text-align: left; font-size: 12px; font-family: Consolas, monospace; max-height: 180px;
  overflow: auto; background: var(--cb-cream, #f7f4ec); border-radius: 8px; padding: 10px;
  white-space: pre-wrap; word-break: break-all; width: 100%; margin: 0;
}
</style>

<script setup lang="ts">
import { onActivated, onMounted, ref } from 'vue'
import {
  NButton, NIcon, NInput, NEmpty, NModal, NForm, NFormItem, NTag, NSwitch,
  NPopconfirm, useMessage
} from 'naive-ui'
import {
  ShieldCheckmarkOutline, AddOutline, TrashOutline, CopyOutline, CreateOutline, CodeOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import { copyText } from '../lib/kit'
import type captchaKeyView from '../api/captchaKeyView'

const message = useMessage()

const keys = ref<captchaKeyView[]>([])
const editVisible = ref(false)
const saving = ref(false)
const form = ref({ id: 0, name: '', domains: '', enabled: 1 })
const snippetKey = ref<captchaKeyView | null>(null)

async function refresh() {
  const r = await client.captchaSvc.listKeys()
  if (r.status === 0 && r.data) keys.value = r.data
}
onMounted(refresh)
onActivated(refresh)

function openCreate() {
  form.value = { id: 0, name: '', domains: '', enabled: 1 }
  editVisible.value = true
}
function openEdit(k: captchaKeyView) {
  form.value = { id: k.id, name: k.name, domains: k.domains, enabled: k.enabled ? 1 : 0 }
  editVisible.value = true
}

async function save() {
  saving.value = true
  try {
    const r = await client.captchaSvc.saveKey({
      id: form.value.id, name: form.value.name,
      domains: form.value.domains, enabled: form.value.enabled
    })
    if (r.status === 0) {
      message.success(form.value.id ? '已保存' : '密钥已生成')
      editVisible.value = false
      void refresh()
    } else {
      message.error(r.msg || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(k: captchaKeyView, v: boolean) {
  const r = await client.captchaSvc.saveKey({ id: k.id, name: k.name, domains: k.domains, enabled: v ? 1 : 0 })
  if (r.status === 0) { message.success(v ? '已启用' : '已停用'); void refresh() }
  else message.error(r.msg || '操作失败')
}

async function copy(t: string, what: string) {
  message.success(await copyText(t) ? `${what}已复制` : '复制失败')
}

async function removeKey(k: captchaKeyView) {
  const r = await client.captchaSvc.deleteKey({ id: k.id })
  if (r.status === 0) { message.success('已删除'); void refresh() }
  else message.error(r.msg || '删除失败')
}

function embedSnippet(k: captchaKeyView): string {
  return `<iframe src="${location.origin}/captcha/widget?sitekey=${k.sitekey}"
        style="border:0;width:304px;height:222px;overflow:hidden" scrolling="no"></iframe>
<script>
addEventListener('message', e => {
  if (e.origin !== '${location.origin}') return
  if (e.data && e.data.type === 'cyi-captcha') {
    if (e.data.ok) {
      // e.data.token 随表单提交给你的后端
      console.log('captcha token:', e.data.token)
    } else {
      // 答错：widget 已自动换题，清掉旧 token 禁止提交
    }
  }
})
<\/script>`
}

function checkSnippet(k: captchaKeyView): string {
  return `GET ${location.origin}/v1/captcha/check?secret=${k.skey}&token=<token>`
}

</script>

<template>
  <div class="panel">
    <div class="panel-title">
      <n-icon style="vertical-align:-2px"><ShieldCheckmarkOutline /></n-icon> 滑块验证码
      <span class="hint">{{ keys.length }} 个密钥 · sitekey 嵌页面出题，secret 后端核销</span>
      <n-button size="small" type="primary" style="margin-left:auto" @click="openCreate">
        <template #icon><n-icon><AddOutline /></n-icon></template>添加接入方
      </n-button>
    </div>

    <div v-if="keys.length" class="key-list">
      <div v-for="k in keys" :key="k.id" class="key-row">
        <div class="key-ic"><n-icon :size="18"><ShieldCheckmarkOutline /></n-icon></div>
        <div class="key-meta">
          <div class="key-name">
            {{ k.name || '未命名' }}
            <n-tag v-if="!k.enabled" size="tiny" type="error" :bordered="false">已停用</n-tag>
            <span class="key-dom">{{ k.domains || '不限域名' }}</span>
          </div>
          <div class="key-ids">
            <span class="mono id-chip" @click="copy(k.sitekey, 'sitekey ')">{{ k.sitekey }}</span>
            <span class="mono id-chip" @click="copy(k.skey, 'secret ')">{{ k.skey.slice(0, 10) }}…</span>
            <span class="key-use">核销 {{ k.usedCount }} 次</span>
          </div>
        </div>
        <div class="key-right">
          <n-button size="small" @click="snippetKey = k">
            <template #icon><n-icon><CodeOutline /></n-icon></template>接入代码
          </n-button>
          <n-switch size="small" :value="k.enabled" @update:value="v => toggleEnabled(k, v)" />
          <n-button size="small" quaternary @click="openEdit(k)">
            <template #icon><n-icon><CreateOutline /></n-icon></template>
          </n-button>
          <n-popconfirm @positive-click="removeKey(k)">
            <template #trigger>
              <n-button size="small" quaternary><template #icon><n-icon><TrashOutline /></n-icon></template></n-button>
            </template>
            删除密钥「{{ k.name || k.sitekey }}」？该接入方将立即无法出题核销
          </n-popconfirm>
        </div>
      </div>
    </div>
    <n-empty v-else description="还没有接入方密钥" style="padding:60px 0" />

    <div class="usage">
      <b>接入方式（Turnstile 式）</b>
      <div class="dim">
        1. 对方页面嵌 iframe 指向 <span class="mono">/captcha/widget?sitekey=pk_xxx</span>，用户滑块通过后
        <span class="mono">postMessage</span> 回传 token（2 分钟有效，一次性）；
        2. 对方后端调 <span class="mono">/v1/captcha/check?secret=sk_xxx&amp;token=…</span> 校验并销毁。
        域名白名单命中 Referer + frame-ancestors，空名单不限制。
      </div>
    </div>

    <n-modal v-model:show="editVisible" preset="card" :title="form.id ? '编辑接入方' : '添加接入方'" style="width: 440px">
      <n-form>
        <n-form-item label="备注名">
          <n-input v-model:value="form.name" placeholder="如：主站登录 / 下载站" />
        </n-form-item>
        <n-form-item label="域名白名单">
          <n-input v-model:value="form.domains" placeholder="逗号分隔，如 a.com,*.b.com；留空=不限制" />
        </n-form-item>
        <n-form-item v-if="form.id" label="启用">
          <n-switch :value="!!form.enabled" @update:value="v => form.enabled = v ? 1 : 0" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:8px">
          <n-button @click="editVisible = false">取消</n-button>
          <n-button type="primary" :loading="saving" @click="save">{{ form.id ? '保存' : '生成密钥' }}</n-button>
        </div>
      </template>
    </n-modal>

    <n-modal :show="!!snippetKey" preset="card" title="接入代码" style="width: 560px" @update:show="v => { if (!v) snippetKey = null }">
      <div class="snip-sec"><b>前端嵌入</b></div>
      <pre v-if="snippetKey" class="snip">{{ embedSnippet(snippetKey) }}</pre>
      <div class="snip-sec"><b>后端核销</b></div>
      <pre v-if="snippetKey" class="snip">{{ checkSnippet(snippetKey) }}</pre>
      <div class="dim" style="margin-top:8px">返回 <span class="mono">{code:1}</span> 即通过；token 一次性且 2 分钟内有效。</div>
    </n-modal>
  </div>
</template>

<style scoped>
.key-list { display: flex; flex-direction: column; gap: 8px; }
.key-row {
  display: flex; align-items: center; gap: 12px;
  border: 2.5px solid var(--cb-ink); border-radius: 12px;
  padding: 10px 14px; background: #fff;
}
.key-ic {
  width: 36px; height: 36px; border: 2.5px solid var(--cb-ink); border-radius: 10px;
  background: var(--cb-mint); display: flex; align-items: center; justify-content: center; flex: none;
}
.key-meta { flex: 1; min-width: 0; }
.key-name { font-weight: 800; font-size: 14px; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.key-dom { font-size: 11px; font-weight: 700; color: var(--cb-ink-3); }
.key-ids { display: flex; gap: 8px; margin-top: 4px; flex-wrap: wrap; align-items: center; }
.id-chip {
  font-size: 11px; color: var(--cb-ink-3); cursor: pointer;
  background: var(--cb-cream, #f7f4ec); border-radius: 6px; padding: 1px 7px;
}
.id-chip:hover { color: var(--cb-primary, #ec5b13); }
.key-use { font-size: 11px; color: var(--cb-ink-3); font-weight: 700; }
.key-right { display: flex; align-items: center; gap: 6px; flex: none; }
.mono { font-family: Consolas, monospace; }
.dim { color: var(--cb-ink-3); font-weight: 500; }
.usage {
  margin-top: 16px; border: 2px dashed var(--cb-ink); border-radius: 12px;
  padding: 12px 14px; background: var(--cb-cream-2, #f1eee8); font-size: 13px; line-height: 1.8;
}
.snip-sec { font-size: 13px; margin: 8px 0 4px; }
.snip {
  background: var(--cb-cream, #f7f4ec); border: 2px solid var(--cb-ink); border-radius: 8px;
  padding: 10px; font-size: 11px; font-family: Consolas, monospace;
  white-space: pre-wrap; word-break: break-all; margin: 0 0 8px;
}
</style>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton, NEmpty, NForm, NFormItem, NIcon, NInput, NInputNumber,
  NModal, NRadioGroup, NRadioButton, useDialog, useMessage
} from 'naive-ui'
import { AddOutline, PencilOutline, TrashOutline, ServerOutline, KeyOutline, LockClosedOutline } from '@vicons/ionicons5'
import { client } from '../lib/api'
import { useServersStore } from '../stores/servers'
import { toolGradient } from '../lib/icons'
import type serverView from '../api/serverView'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const store = useServersStore()

onMounted(() => void store.refresh())

function open(s: serverView) {
  void router.push(`/servers/${s.id}`)
}

// ---- 新建/编辑 ----
const editVisible = ref(false)
const saving = ref(false)
const editing = reactive<{
  id: number | null
  name: string
  host: string
  port: number
  username: string
  authType: string
  secret: string
  note: string
}>({ id: null, name: '', host: '', port: 22, username: 'root', authType: 'password', secret: '', note: '' })

function openCreate() {
  Object.assign(editing, {
    id: null, name: '', host: '', port: 22, username: 'root',
    authType: 'password', secret: '', note: ''
  })
  editVisible.value = true
}

function openEdit(s: serverView, ev: Event) {
  ev.stopPropagation()
  Object.assign(editing, {
    id: s.id, name: s.name, host: s.host, port: s.port, username: s.username,
    authType: s.authType, secret: '', note: s.note || ''
  })
  editVisible.value = true
}

async function save() {
  if (!editing.name.trim() || !editing.host.trim() || !editing.username.trim()) {
    message.warning('名称、主机、用户名必填')
    return
  }
  if (editing.id === null && !editing.secret.trim()) {
    message.warning(editing.authType === 'key' ? '请粘贴私钥' : '请填写密码')
    return
  }
  saving.value = true
  try {
    const r = await client.serverSvc.save({
      id: editing.id,
      name: editing.name.trim(),
      host: editing.host.trim(),
      port: editing.port,
      username: editing.username.trim(),
      authType: editing.authType,
      secret: editing.secret || null,
      note: editing.note.trim() || null
    })
    if (r.status === 0) {
      message.success('已保存')
      editVisible.value = false
      void store.refresh()
    } else {
      message.error(r.msg || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

function remove(s: serverView, ev: Event) {
  ev.stopPropagation()
  dialog.warning({
    title: '删除服务器',
    content: `确定删除「${s.name}」吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.serverSvc.delete({ id: s.id })
      if (r.status === 0) {
        message.success('已删除')
        void store.refresh()
      } else {
        message.error(r.msg || '删除失败')
      }
    }
  })
}
</script>

<template>
  <div>
    <div style="display:flex;margin-bottom:18px;align-items:center">
      <div style="color:var(--cb-ink-2);font-size:13px;font-weight:500">
        点进服务器看负载、传文件、开终端；连接密钥 AES-GCM 加密存储
      </div>
      <div style="flex:1"></div>
      <n-button type="primary" @click="openCreate">
        <template #icon><n-icon><AddOutline /></n-icon></template>
        添加服务器
      </n-button>
    </div>

    <div class="tool-grid" v-if="store.items.length">
      <div v-for="s in store.items" :key="s.id" class="tool-card" @click="open(s)">
        <div class="sv-actions">
          <n-button size="tiny" quaternary @click="openEdit(s, $event)">
            <template #icon><n-icon><PencilOutline /></n-icon></template>
          </n-button>
          <n-button size="tiny" quaternary type="error" @click="remove(s, $event)">
            <template #icon><n-icon><TrashOutline /></n-icon></template>
          </n-button>
        </div>
        <div class="tool-icon" :style="{ background: toolGradient(s.id) }">
          <n-icon><ServerOutline /></n-icon>
        </div>
        <div class="tool-name">{{ s.name }}</div>
        <div class="tool-desc">{{ s.username }}@{{ s.host }}:{{ s.port }}</div>
        <div class="sv-badges">
          <n-icon :size="13"><component :is="s.authType === 'key' ? KeyOutline : LockClosedOutline" /></n-icon>
          {{ s.authType === 'key' ? '密钥登录' : '密码登录' }}
          <template v-if="s.note"> · {{ s.note }}</template>
        </div>
      </div>
    </div>
    <n-empty v-else description="还没有服务器，点右上角加一台吧" style="padding: 80px 0">
      <template #icon><n-icon :size="48"><ServerOutline /></n-icon></template>
    </n-empty>

    <n-modal v-model:show="editVisible" preset="card" :title="editing.id === null ? '添加服务器' : '编辑服务器'" style="width: 480px">
      <n-form label-placement="top">
        <n-form-item label="名称">
          <n-input v-model:value="editing.name" placeholder="比如 生产机-1" />
        </n-form-item>
        <div style="display:flex;gap:16px">
          <n-form-item label="主机" style="flex:1">
            <n-input v-model:value="editing.host" placeholder="IP 或域名" />
          </n-form-item>
          <n-form-item label="端口" style="width:110px">
            <n-input-number v-model:value="editing.port" :min="1" :max="65535" style="width:100%" />
          </n-form-item>
        </div>
        <n-form-item label="用户名">
          <n-input v-model:value="editing.username" placeholder="root" />
        </n-form-item>
        <n-form-item label="认证方式">
          <n-radio-group v-model:value="editing.authType">
            <n-radio-button value="password">密码</n-radio-button>
            <n-radio-button value="key">私钥</n-radio-button>
          </n-radio-group>
        </n-form-item>
        <n-form-item :label="editing.authType === 'key' ? '私钥（PEM）' : '密码'">
          <n-input
            v-model:value="editing.secret"
            :type="editing.authType === 'key' ? 'textarea' : 'password'"
            :show-password-on="editing.authType === 'key' ? undefined : 'click'"
            :autosize="editing.authType === 'key' ? { minRows: 4 } : undefined"
            :placeholder="editing.id === null ? '' : '留空表示不修改'"
          />
        </n-form-item>
        <n-form-item label="备注">
          <n-input v-model:value="editing.note" placeholder="可选" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:10px">
          <n-button @click="editVisible = false">取消</n-button>
          <n-button type="primary" :loading="saving" @click="save">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.sv-actions {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s ease;
}
.tool-card:hover .sv-actions { opacity: 1; }
.sv-badges {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--cb-ink-2);
  font-weight: 600;
}
</style>

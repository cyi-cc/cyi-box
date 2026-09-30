<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NIcon, NInput, NModal,
  NSpace, NText, NTooltip, useDialog, useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  CopyOutline, EyeOutline, EyeOffOutline, KeyOutline, PencilOutline,
  ShieldCheckmarkOutline, TrashOutline
} from '@vicons/ionicons5'
import { client } from '../lib/api'
import type vaultItemView from '../api/vaultItemView'

const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const rows = ref<vaultItemView[]>([])
const keyword = ref('')

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return rows.value
  return rows.value.filter(e =>
    e.title.toLowerCase().includes(k) ||
    e.username?.toLowerCase().includes(k) ||
    e.url?.toLowerCase().includes(k))
})

async function load() {
  loading.value = true
  try {
    const r = await client.vaultSvc.list()
    if (r.status === 0 && r.data) {
      rows.value = r.data
      void refreshAllTotp()
    } else {
      message.error(r.msg || '加载失败')
    }
  } finally {
    loading.value = false
  }
}

// ---- 密码查看/复制 ----
const revealed = reactive<Record<number, string>>({})

async function toggleReveal(id: number) {
  if (revealed[id]) {
    delete revealed[id]
    return
  }
  const r = await client.vaultSvc.reveal({ id })
  if (r.status === 0 && r.data) {
    revealed[id] = r.data.value
  } else {
    message.error(r.msg || '获取密码失败')
  }
}

async function copyText(text: string, what = '密码') {
  try {
    await navigator.clipboard.writeText(text)
    message.success(`${what}已复制`)
  } catch {
    message.error('复制失败')
  }
}

// ---- TOTP：本地倒计时，到期重新拉取 ----
// 填充层用 CSS transform 动画（duration=剩余秒数），不从 JS 每帧驱动；
// deadline 作 vnode key，换码时重建元素重启动画
const totps = reactive<Record<number, { code: string; left: number; deadline: number; duration: number }>>({})
let tick: ReturnType<typeof setInterval> | null = null

async function refreshTotp(id: number) {
  const r = await client.vaultSvc.totp({ id })
  if (r.status === 0 && r.data) {
    totps[id] = {
      code: r.data.code,
      left: r.data.secondsLeft,
      deadline: Date.now() + r.data.secondsLeft * 1000,
      duration: r.data.secondsLeft
    }
  }
}

async function refreshAllTotp() {
  await Promise.all(rows.value.filter(e => e.hasTotp).map(e => refreshTotp(e.id)))
}

onMounted(() => {
  void load()
  tick = setInterval(() => {
    for (const [id, t] of Object.entries(totps)) {
      t.left -= 1
      if (t.left <= 0) void refreshTotp(Number(id))
    }
  }, 1000)
})
onBeforeUnmount(() => { if (tick) clearInterval(tick) })

// ---- 编辑/新建 ----
const editVisible = ref(false)
const saving = ref(false)
const clearTotp = ref(false)
const editing = reactive<{
  id: number | null
  title: string
  username: string
  password: string
  url: string
  note: string
  totpSecret: string
}>({ id: null, title: '', username: '', password: '', url: '', note: '', totpSecret: '' })

const editingItem = ref<vaultItemView | null>(null)

function openCreate() {
  editingItem.value = null
  clearTotp.value = false
  Object.assign(editing, { id: null, title: '', username: '', password: '', url: '', note: '', totpSecret: '' })
  editVisible.value = true
}

function openEdit(e: vaultItemView) {
  editingItem.value = e
  clearTotp.value = false
  Object.assign(editing, {
    id: e.id, title: e.title, username: e.username ?? '',
    password: '', url: e.url ?? '', note: e.note ?? '', totpSecret: ''
  })
  editVisible.value = true
}

async function save() {
  if (!editing.title.trim()) {
    message.warning('请输入标题')
    return
  }
  saving.value = true
  try {
    const r = await client.vaultSvc.save({
      id: editing.id,
      title: editing.title.trim(),
      username: editing.username || null,
      url: editing.url || null,
      note: editing.note || null,
      password: editing.password !== '' ? editing.password : null,
      totpSecret: clearTotp.value ? '' : (editing.totpSecret.trim() !== '' ? editing.totpSecret.trim() : null)
    })
    if (r.status === 0) {
      message.success('已保存')
      editVisible.value = false
      void load()
    } else {
      message.error(r.msg || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

function remove(e: vaultItemView) {
  dialog.warning({
    title: '删除条目',
    content: `确定删除「${e.title}」吗？密码与 2FA 一并删除，不可恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.vaultSvc.delete({ id: e.id })
      if (r.status === 0) {
        message.success('已删除')
        void load()
      } else {
        message.error(r.msg || '删除失败')
      }
    }
  })
}

const columns: DataTableColumns<vaultItemView> = [
  {
    title: '标题', key: 'title', width: 200,
    render: row => h('div', [
      h('div', { style: 'font-weight:700' }, row.title),
      row.url ? h(NText, { depth: 3, style: 'font-size:12px' }, { default: () => row.url }) : null
    ])
  },
  {
    title: '用户名', key: 'username', width: 150,
    render: row => row.username
      ? h(NSpace, { align: 'center', size: 4 }, {
          default: () => [
            h('span', row.username || ''),
            h(NButton, { size: 'tiny', quaternary: true, onClick: () => copyText(row.username!, '用户名') },
              { icon: () => h(NIcon, { component: CopyOutline }) })
          ]
        })
      : h(NText, { depth: 3 }, { default: () => '-' })
  },
  {
    title: '密码', key: 'password', width: 200,
    render: row => {
      if (!row.hasPassword) return h(NText, { depth: 3 }, { default: () => '-' })
      const shown = revealed[row.id]
      return h(NSpace, { align: 'center', size: 6 }, {
        default: () => [
          h('code', { style: 'font-size:13px' }, shown ?? '••••••••'),
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => toggleReveal(row.id) },
            { icon: () => h(NIcon, { component: shown ? EyeOffOutline : EyeOutline }) }),
          shown ? h(NButton, { size: 'tiny', quaternary: true, onClick: () => copyText(shown) },
            { icon: () => h(NIcon, { component: CopyOutline }) }) : null
        ]
      })
    }
  },
  {
    title: '2FA', key: 'totp', width: 170,
    render: row => {
      if (!row.hasTotp) return h(NText, { depth: 3 }, { default: () => '-' })
      const t = totps[row.id]
      if (!t) return h(NText, { depth: 3 }, { default: () => '加载中…' })
      return h(NSpace, { align: 'center', size: 6 }, {
        default: () => [
          h('div', { class: 'totp-chip' + (t.left <= 5 ? ' totp-low' : '') }, [
            h('span', {
              class: 'totp-fill',
              key: t.deadline,
              style: `animation-duration:${t.duration}s`
            }),
            h('code', { class: 'totp-code' }, `${t.code.slice(0, 3)} ${t.code.slice(3)}`)
          ]),
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => copyText(t.code, '验证码') },
            { icon: () => h(NIcon, { component: CopyOutline }) })
        ]
      })
    }
  },
  { title: '备注', key: 'note', ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'actions', width: 120,
    render: row => h(NSpace, null, {
      default: () => [
        h(NTooltip, null, {
          trigger: () => h(NButton, { size: 'small', quaternary: true, onClick: () => openEdit(row) },
            { icon: () => h(NIcon, { component: PencilOutline }) }),
          default: () => '编辑'
        }),
        h(NTooltip, null, {
          trigger: () => h(NButton, { size: 'small', quaternary: true, type: 'error', onClick: () => remove(row) },
            { icon: () => h(NIcon, { component: TrashOutline }) }),
          default: () => '删除'
        })
      ]
    })
  }
]
</script>

<template>
  <n-card :bordered="false" style="border-radius: 16px">
    <div style="display:flex;gap:12px;margin-bottom:16px;align-items:center">
      <n-input v-model:value="keyword" placeholder="搜索标题 / 用户名 / 网址" clearable style="width: 280px">
        <template #prefix><n-icon><KeyOutline /></n-icon></template>
      </n-input>
      <div style="flex:1"></div>
      <n-button type="primary" @click="openCreate">新建条目</n-button>
    </div>

    <n-data-table :columns="columns" :data="filtered" :loading="loading" />
  </n-card>

  <n-modal v-model:show="editVisible" preset="card" :title="editing.id === null ? '新建条目' : '编辑条目'" style="width: 480px">
    <n-form label-placement="top">
      <n-form-item label="标题">
        <n-input v-model:value="editing.title" placeholder="如：GitHub / 服务器 root" />
      </n-form-item>
      <n-form-item label="用户名">
        <n-input v-model:value="editing.username" placeholder="登录账号" />
      </n-form-item>
      <n-form-item :label="editing.id === null ? '密码' : '密码（留空不变）'">
        <n-input v-model:value="editing.password" type="password" show-password-on="click" placeholder="AES-GCM 加密落库" />
      </n-form-item>
      <n-form-item label="2FA 密钥">
        <n-input
          v-model:value="editing.totpSecret"
          type="textarea"
          :rows="2"
          :placeholder="editingItem?.hasTotp ? '已设置，粘贴新密钥或 otpauth:// 链接可覆盖' : 'base32 密钥或 otpauth://totp/... 链接'"
        />
        <n-button v-if="editingItem?.hasTotp" size="tiny" tertiary style="margin-top:6px" @click="clearTotp = !clearTotp">
          {{ clearTotp ? '撤销清除 2FA' : '清除 2FA' }}
        </n-button>
      </n-form-item>
      <n-form-item label="网址">
        <n-input v-model:value="editing.url" placeholder="https://…" />
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
</template>

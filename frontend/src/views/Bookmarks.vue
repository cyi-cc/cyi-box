<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NEmpty, NForm, NFormItem, NIcon, NInput, NInputNumber,
  NModal, NSelect, useDialog, useMessage
} from 'naive-ui'
import { AddOutline, PencilOutline, TrashOutline, BookmarkOutline } from '@vicons/ionicons5'
import { client } from '../lib/api'
import { useBookmarksStore } from '../stores/bookmarks'
import { toolIcon, toolGradient } from '../lib/icons'
import type bookmarkView from '../api/bookmarkView'

const message = useMessage()
const dialog = useDialog()
const store = useBookmarksStore()

const iconOptions = [
  'bookmark-outline', 'star-outline', 'globe-outline', 'link-outline',
  'home-outline', 'code-outline', 'terminal-outline', 'logo-github',
  'cloud-outline', 'folder-outline', 'document-text-outline', 'newspaper-outline',
  'image-outline', 'videocam-outline', 'musical-notes-outline', 'cart-outline',
  'game-controller-outline', 'heart-outline', 'key-outline'
].map(v => ({ label: v, value: v }))

onMounted(() => void store.refresh())

function open(b: bookmarkView) {
  window.open(b.url, '_blank')
}

function host(url: string): string {
  try { return new URL(url).host } catch { return url }
}

// ---- 新建/编辑 ----
const editVisible = ref(false)
const saving = ref(false)
const editing = reactive<{
  id: number | null
  title: string
  url: string
  icon: string
  sort: number
}>({ id: null, title: '', url: '', icon: 'bookmark-outline', sort: 0 })

function openCreate() {
  Object.assign(editing, { id: null, title: '', url: '', icon: 'bookmark-outline', sort: 0 })
  editVisible.value = true
}

function openEdit(b: bookmarkView) {
  Object.assign(editing, { id: b.id, title: b.title, url: b.url, icon: b.icon || 'bookmark-outline', sort: b.sort })
  editVisible.value = true
}

async function save() {
  if (!editing.title.trim() || !editing.url.trim()) {
    message.warning('名称和网址必填')
    return
  }
  saving.value = true
  try {
    const r = await client.bookmarkSvc.save({
      id: editing.id,
      title: editing.title.trim(),
      url: editing.url.trim(),
      icon: editing.icon,
      sort: editing.sort
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

function remove(b: bookmarkView, ev: Event) {
  ev.stopPropagation()
  dialog.warning({
    title: '删除书签',
    content: `确定删除「${b.title}」吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const r = await client.bookmarkSvc.delete({ id: b.id })
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
        收藏的小网站会出现在左侧「我的书签」，点一下就进去了
      </div>
      <div style="flex:1"></div>
      <n-button type="primary" @click="openCreate">
        <template #icon><n-icon><AddOutline /></n-icon></template>
        添加书签
      </n-button>
    </div>

    <div class="tool-grid" v-if="store.items.length">
      <div v-for="b in store.items" :key="b.id" class="tool-card" @click="open(b)">
        <div class="bm-actions">
          <n-button size="tiny" quaternary @click.stop="openEdit(b)">
            <template #icon><n-icon><PencilOutline /></n-icon></template>
          </n-button>
          <n-button size="tiny" quaternary type="error" @click="remove(b, $event)">
            <template #icon><n-icon><TrashOutline /></n-icon></template>
          </n-button>
        </div>
        <div class="tool-icon" :style="{ background: toolGradient(b.id) }">
          <n-icon v-if="b.icon"><component :is="toolIcon(b.icon)" /></n-icon>
          <span v-else style="font-weight:900;font-size:20px">{{ b.title.slice(0, 1).toUpperCase() }}</span>
        </div>
        <div class="tool-name">{{ b.title }}</div>
        <div class="tool-desc">{{ host(b.url) }}</div>
      </div>
    </div>
    <n-empty v-else description="还没有书签，点右上角加一个吧" style="padding: 80px 0">
      <template #icon><n-icon :size="48"><BookmarkOutline /></n-icon></template>
    </n-empty>

    <n-modal v-model:show="editVisible" preset="card" :title="editing.id === null ? '添加书签' : '编辑书签'" style="width: 460px">
      <n-form label-placement="top">
        <n-form-item label="名称">
          <n-input v-model:value="editing.title" placeholder="展示名" />
        </n-form-item>
        <n-form-item label="网址">
          <n-input v-model:value="editing.url" placeholder="https://example.com（裸域名自动补 https://）" />
        </n-form-item>
        <div style="display:flex;gap:20px">
          <n-form-item label="图标" style="flex:1">
            <n-select v-model:value="editing.icon" :options="iconOptions" filterable />
          </n-form-item>
          <n-form-item label="排序（小的在前）" style="width:150px">
            <n-input-number v-model:value="editing.sort" style="width:100%" />
          </n-form-item>
        </div>
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
.bm-actions {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s ease;
}
.tool-card:hover .bm-actions { opacity: 1; }
</style>

<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NLayout, NLayoutSider, NLayoutContent, NMenu, NIcon, NButton
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import {
  SpeedometerOutline, ConstructOutline,
  LogOutOutline,
  KeyOutline, BookmarksOutline,
  ServerOutline, CubeOutline, CodeOutline, TimeOutline,
  DocumentTextOutline, SendOutline, ColorPaletteOutline, EyedropOutline, ColorWandOutline,
  CloudOutline, LinkOutline, GlobeOutline, SettingsOutline, ReceiptOutline, CardOutline, AppsOutline,
  BulbOutline
} from '@vicons/ionicons5'
import { useAuthStore } from '../stores/auth'


const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const collapsed = ref(false)

function icon(comp: any) {
  return () => h(NIcon, null, { default: () => h(comp) })
}

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = [
    { label: '仪表盘', key: '/dashboard', icon: icon(SpeedometerOutline) },
    { label: '密码管理', key: '/vault', icon: icon(KeyOutline) },
    { label: '书签导航', key: '/bookmarks', icon: icon(BookmarksOutline) },
    {
      type: 'group',
      label: '代理池',
      key: 'proxy-group',
      children: [
        { label: '代理列表', key: '/proxies', icon: icon(GlobeOutline) },
        { label: '提取链接', key: '/proxies/extract', icon: icon(KeyOutline) }
      ]
    },
    {
      type: 'group',
      label: '网盘',
      key: 'disk-group',
      children: [
        { label: '我的文件', key: '/disk', icon: icon(CloudOutline) },
        { label: '分享链接', key: '/disk/shares', icon: icon(LinkOutline) }
      ]
    },
    {
      type: 'group',
      label: '通用工具',
      key: 'util-group',
      children: [
        { label: 'JSON 工具', key: '/tools/json', icon: icon(CodeOutline) },
        { label: '时间戳转换', key: '/tools/timestamp', icon: icon(TimeOutline) },
        { label: 'Markdown', key: '/tools/markdown', icon: icon(DocumentTextOutline) },
        { label: '接口调试', key: '/tools/postman', icon: icon(SendOutline) },
        { label: '取色板', key: '/tools/color', icon: icon(ColorPaletteOutline) },
        { label: '图片拾色', key: '/tools/color-image', icon: icon(EyedropOutline) },
        { label: '渐变图鉴', key: '/tools/gradient', icon: icon(ColorWandOutline) }
      ]
    }
  ]
  if (auth.isAdmin) {
    options.push(
      {
        type: 'group',
        label: '支付',
        key: 'pay-group',
        children: [
          { label: '支付配置', key: '/pay/config', icon: icon(CardOutline) },
          { label: '订单管理', key: '/pay', icon: icon(ReceiptOutline) }
        ]
      },
      {
        type: 'group',
        label: '授权系统',
        key: 'license-group',
        children: [
          { label: '项目管理', key: '/licenses/apps', icon: icon(AppsOutline) },
          { label: '卡密管理', key: '/licenses', icon: icon(KeyOutline) }
        ]
      },
      {
        type: 'group',
        label: '系统管理',
        key: 'admin-group',
        children: [
          { label: '服务器', key: '/servers', icon: icon(ServerOutline) },
          { label: '数据库', key: '/dbm', icon: icon(CubeOutline) },
          { label: '记忆库', key: '/memories', icon: icon(BulbOutline) },
          { label: '系统设置', key: '/settings', icon: icon(SettingsOutline) },
        ]
      }
    )
  }
  return options
})

// 服务器详情归位到 /servers 菜单项
const activeKey = computed(() =>
  route.path.startsWith('/servers/') ? '/servers' : route.path
)

// ---- 页签栏：访问过的页面沉淀为标签，keep-alive 保活可切回 ----
type TabItem = { key: string; title: string }
const tabs = ref<TabItem[]>([])
const MAX_TABS = 12

watch(() => route.fullPath, p => {
  if (!tabs.value.some(t => t.key === p)) {
    tabs.value.push({ key: p, title: (route.meta.title as string) || p })
    if (tabs.value.length > MAX_TABS) tabs.value.shift()
  }
}, { immediate: true })

function goTab(key: string) {
  if (key !== route.fullPath) void router.push(key)
}

function closeTab(key: string, ev?: Event) {
  ev?.stopPropagation()
  const i = tabs.value.findIndex(t => t.key === key)
  if (i < 0) return
  tabs.value.splice(i, 1)
  if (route.fullPath === key) {
    const next = tabs.value[i] ?? tabs.value[i - 1]
    void router.push(next?.key ?? '/dashboard')
  }
}

function onMenuSelect(key: string) {
  void router.push(key)
}

function doLogout() {
  void auth.logout().then(() => router.replace('/login'))
}
</script>

<template>
  <n-layout has-sider style="height: 100vh">
    <n-layout-sider
      class="cb-sider"
      :width="224"
      :collapsed-width="64"
      :collapsed="collapsed"
      collapse-mode="width"
      show-trigger="bar"
      @update:collapsed="collapsed = $event"
    >
      <div class="cb-logo">
        <div class="brand-badge" style="width:36px;height:36px;font-size:19px">
          <n-icon><ConstructOutline /></n-icon>
        </div>
        <div v-if="!collapsed" class="cb-logo-text">
          池易工作箱
          <small>CYI BOX</small>
        </div>
      </div>

      <div class="cb-menu-wrap">
        <n-menu
          :value="activeKey"
          :options="menuOptions"
          :collapsed="collapsed"
          :collapsed-width="64"
          :indent="22"
          @update:value="onMenuSelect"
        />
      </div>

      <div v-if="!collapsed" class="cb-sider-foot">
        <n-button size="small" class="logout-btn" block @click="doLogout">
          <template #icon><n-icon :size="13"><LogOutOutline /></n-icon></template>
          退出登录
        </n-button>
      </div>
    </n-layout-sider>

    <n-layout>
      <div class="cb-tabs">
        <div
          v-for="t in tabs" :key="t.key" class="cb-tab"
          :class="{ on: t.key === route.fullPath }"
          @click="goTab(t.key)" @mousedown.middle.prevent="closeTab(t.key)"
        >
          <span>{{ t.title }}</span>
          <span class="x" title="关闭" @click="closeTab(t.key, $event)">×</span>
        </div>
      </div>

      <n-layout-content class="cb-content" :native-scrollbar="false">
        <router-view v-slot="{ Component }">
          <keep-alive :max="MAX_TABS">
            <component :is="Component" :key="route.fullPath" />
          </keep-alive>
        </router-view>
      </n-layout-content>
    </n-layout>

  </n-layout>
</template>

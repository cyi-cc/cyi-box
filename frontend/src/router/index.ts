import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/Login.vue'),
      meta: { title: '登录' }
    },
    {
      path: '/cashier/:tradeNo',
      name: 'cashier',
      component: () => import('../views/Cashier.vue'),
      meta: { title: '收银台' }
    },
    {
      path: '/',
      component: () => import('../layouts/AdminLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: '/dashboard' },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('../views/Dashboard.vue'),
          meta: { title: '仪表盘' }
        },
        {
          path: 'tools/json',
          name: 'tool-json',
          component: () => import('../views/tools/JsonTool.vue'),
          meta: { title: 'JSON 工具' }
        },
        {
          path: 'tools/timestamp',
          name: 'tool-timestamp',
          component: () => import('../views/tools/TimestampTool.vue'),
          meta: { title: '时间戳转换' }
        },
        {
          path: 'tools/markdown',
          name: 'tool-markdown',
          component: () => import('../views/tools/MarkdownTool.vue'),
          meta: { title: 'Markdown 编辑器' }
        },
        {
          path: 'tools/postman',
          name: 'tool-postman',
          component: () => import('../views/tools/PostmanTool.vue'),
          meta: { title: '接口调试' }
        },
        {
          path: 'tools/color',
          name: 'tool-color',
          component: () => import('../views/tools/ColorTool.vue'),
          meta: { title: '取色板' }
        },
        {
          path: 'tools/color-image',
          name: 'tool-color-image',
          component: () => import('../views/tools/ColorImage.vue'),
          meta: { title: '图片拾色' }
        },
        {
          path: 'tools/gradient',
          name: 'tool-gradient',
          component: () => import('../views/tools/GradientTool.vue'),
          meta: { title: '渐变图鉴' }
        },
        {
          path: 'vault',
          name: 'vault',
          component: () => import('../views/Vault.vue'),
          meta: { title: '密码管理' }
        },
        {
          path: 'bookmarks',
          name: 'bookmarks',
          component: () => import('../views/Bookmarks.vue'),
          meta: { title: '书签导航' }
        },
        {
          path: 'disk',
          name: 'disk',
          component: () => import('../views/Disk.vue'),
          meta: { title: '我的文件' }
        },
        {
          path: 'disk/shares',
          name: 'disk-shares',
          component: () => import('../views/DiskShares.vue'),
          meta: { title: '分享链接' }
        },
        {
          path: 'proxies',
          name: 'proxies',
          component: () => import('../views/Proxies.vue'),
          meta: { title: '代理列表' }
        },
        {
          path: 'proxies/extract',
          name: 'proxy-extract',
          component: () => import('../views/ProxyExtract.vue'),
          meta: { title: '提取链接', requiresAdmin: true }
        },
        {
          path: 'servers',
          name: 'servers',
          component: () => import('../views/Servers.vue'),
          meta: { title: '服务器管理', requiresAdmin: true }
        },
        {
          path: 'servers/:id(\\d+)',
          name: 'server-detail',
          component: () => import('../views/ServerDetail.vue'),
          meta: { title: '服务器详情', requiresAdmin: true }
        },
        {
          path: 'dbm',
          name: 'dbm',
          component: () => import('../views/DbManager.vue'),
          meta: { title: '数据库', requiresAdmin: true }
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('../views/Settings.vue'),
          meta: { title: '系统设置', requiresAdmin: true }
        },
        {
          path: 'pay',
          name: 'pay-orders',
          component: () => import('../views/PayOrders.vue'),
          meta: { title: '订单管理', requiresAdmin: true }
        },
        {
          path: 'pay/config',
          name: 'pay-config',
          component: () => import('../views/PayConfig.vue'),
          meta: { title: '支付配置', requiresAdmin: true }
        },
        {
          path: 'licenses',
          name: 'licenses',
          component: () => import('../views/Licenses.vue'),
          meta: { title: '卡密管理', requiresAdmin: true }
        },
        {
          path: 'memories',
          name: 'memories',
          component: () => import('../views/Memories.vue'),
          meta: { title: '记忆库', requiresAdmin: true }
        },
        {
          path: 'licenses/apps',
          name: 'license-apps',
          component: () => import('../views/LicenseApps.vue'),
          meta: { title: '项目管理', requiresAdmin: true }
        },
      ]
    },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
})

router.beforeEach(async to => {
  const auth = useAuthStore()
  if (!auth.ready) await auth.fetchMe()
  if (to.meta.requiresAuth && !auth.user) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { path: '/' }
  }
  if (to.name === 'login' && auth.user) {
    return { path: '/' }
  }
})

router.afterEach(to => {
  document.title = to.meta.title ? `${to.meta.title} · 池易工作箱` : '池易工作箱'
})

export default router

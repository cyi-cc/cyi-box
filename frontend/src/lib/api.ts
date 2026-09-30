import api from '../api/fun'
import { createDiscreteApi } from 'naive-ui'

const TOKEN_KEY = 'cyibox_token'

export const readToken = () => localStorage.getItem(TOKEN_KEY) ?? ''
export const writeToken = (t: string) => localStorage.setItem(TOKEN_KEY, t)
export const clearToken = () => localStorage.removeItem(TOKEN_KEY)

// 后端统一走 /api（vite 代理剥前缀）；生产部署时前端同源反代即可
export const client = api.create('/api')

// 组件外使用的离散 message/dialog（拦截器、store 里提示用）
export const { message, dialog } = createDiscreteApi(['message', 'dialog'])

// token 走请求 state 透传，后端 Guard 从 ctx.State["token"] 取
client.addRequestInterceptor((_svc, _method, state) => {
  const token = readToken()
  if (token) state.token = token
})

let redirecting = false
client.addResponseInterceptor(async (_svc, _method, result) => {
  // 4011 = authErrCode：登录态失效，清 token + 清 store 用户态，跳登录页
  if (result?.code === 4011) {
    clearToken()
    // user 不清的话守卫会把 /login 又弹回 /，死循环
    const { useAuthStore } = await import('../stores/auth')
    useAuthStore().user = null
    if (!redirecting && !location.pathname.startsWith('/login')) {
      redirecting = true
      message.warning(result?.msg || '登录已失效，请重新登录')
      const { default: router } = await import('../router')
      await router.replace({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } })
      redirecting = false
    }
  }
})

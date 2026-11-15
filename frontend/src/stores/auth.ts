import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { client, readToken, writeToken, clearToken } from '../lib/api'
import type userView from '../api/userView'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<userView | null>(null)
  const ready = ref(false) // 首次 Me 探测是否完成（路由守卫用）

  const isAdmin = computed(() => user.value?.role === 'admin')

  async function fetchMe() {
    if (!readToken()) {
      user.value = null
      ready.value = true
      return
    }
    try {
      const r = await client.authSvc.me()
      user.value = r.status === 0 && r.data?.user ? r.data.user : null
    } catch {
      user.value = null
    }
    ready.value = true
  }

  async function login(name: string, password: string): Promise<string | null> {
    const r = await client.authSvc.login({ name, password })
    if (r.status !== 0 || !r.data?.token || !r.data.user) {
      return r.msg || '登录失败'
    }
    writeToken(r.data.token)
    user.value = r.data.user
    return null
  }

  async function logout() {
    try { await client.authSvc.logout() } catch { /* 忽略 */ }
    clearToken()
    user.value = null
  }

  return { user, ready, isAdmin, fetchMe, login, logout }
})

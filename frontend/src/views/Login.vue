<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NInput, NButton, NIcon } from 'naive-ui'
import {
  ConstructOutline, LockClosedOutline,
  FlashOutline, ShieldCheckmarkOutline, CubeOutline
} from '@vicons/ionicons5'
import { useAuthStore } from '../stores/auth'
import { useBookmarksStore } from '../stores/bookmarks'
import { takeRedirect } from '../lib/api'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const bookmarksStore = useBookmarksStore()

const form = reactive({ password: '' })
const loading = ref(false)
const errorMsg = ref('')

async function submit() {
  if (!form.password) {
    errorMsg.value = '请输入密码'
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    const err = await auth.login(form.password)
    if (err) {
      errorMsg.value = err
      return
    }
    void bookmarksStore.refresh()
    // 兼容旧 ?redirect= 链接，优先取；隐式回跳存 sessionStorage
    const q = route.query.redirect
    const redirect = (typeof q === 'string' && q.startsWith('/') ? q : takeRedirect()) || '/'
    void router.replace(redirect)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <!-- Memphis 漂浮几何装饰 -->
    <div class="deco deco-circle"></div>
    <div class="deco deco-triangle"></div>
    <div class="deco deco-cross"></div>
    <div class="deco deco-zigzag"></div>
    <div class="deco deco-semicircle"></div>

    <div class="login-card">
      <div class="login-brand">
        <div class="login-brand-top">
          <div class="brand-badge"><n-icon><ConstructOutline /></n-icon></div>
          <span class="brand-name">池易工作箱</span>
          <span class="brand-tag">TOOLBOX</span>
        </div>

        <div class="login-brand-mid">
          <h2>把顺手的工具<br />都装进<em>一个箱子</em></h2>
          <p>统一入口 · 统一账号 · 统一审美。<br />小工具不用再到处散落，这里就是你的工作台。</p>
        </div>

        <div class="login-brand-bottom">
          <span class="brand-chip"><n-icon><FlashOutline /></n-icon> fasthttp 驱动</span>
          <span class="brand-chip"><n-icon><ShieldCheckmarkOutline /></n-icon> 会话级鉴权</span>
          <span class="brand-chip"><n-icon><CubeOutline /></n-icon> SQLite 单文件存储</span>
        </div>
      </div>

      <div class="login-form-side">
        <div class="login-form">
          <h1>欢迎回来</h1>
          <p class="sub">登录你的账户继续使用</p>

          <n-input
            v-model:value="form.password"
            autofocus
            size="large"
            type="password"
            show-password-on="click"
            placeholder="密码"
            @keyup.enter="submit"
          >
            <template #prefix><n-icon><LockClosedOutline /></n-icon></template>
          </n-input>

          <div class="login-err">{{ errorMsg }}</div>

          <n-button type="primary" block class="login-btn" :loading="loading" @click="submit">
            登 录
          </n-button>

          <div class="login-foot">池易工作箱 · cyi-box</div>
        </div>
      </div>
    </div>
  </div>
</template>

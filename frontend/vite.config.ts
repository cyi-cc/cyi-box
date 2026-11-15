import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发期 /api 代理到 fun 后端并剥掉前缀：/api/cell → /cell
export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8890',
        changeOrigin: true,
        ws: true,
        rewrite: p => p.replace(/^\/api/, '')
      },
      // 滑块验证码 iframe 页及其内部 verify 调用（不走 /api 前缀，接入方调试用）
      '/captcha': { target: 'http://localhost:8890', changeOrigin: true },
      '/v1': { target: 'http://localhost:8890', changeOrigin: true }
    }
  }
})

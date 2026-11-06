import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 收银台只与 pos-service 通信；出票助手（cpagent）由服务端调用，浏览器永远碰不到它。
// 开发期用代理解决同源：/api -> http://127.0.0.1:8080
export default defineConfig({
  plugins: [vue()],
  // 开发期用代理解决同源：/pos -> http://127.0.0.1:8080（契约单机版 server 即 /pos/v1）
  define: {
    __POS_MODE__: JSON.stringify(process.env.POS_MODE || 'standalone')
  },
  server: {
    // 必须显式绑 IPv4：Vite 默认 localhost 在本机会解析成 ::1，其它进程连不上。
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/pos': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true
      },
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true
      }
    }
  }
})
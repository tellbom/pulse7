import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// 目标环境：Windows 7 SP1 + Chrome 109（192.168.140.128 回测机）。
// base './' 保证产物可被 pulse7 serve 嵌入（agent/web_embed.go + api_server.go 直接读 web/index.html）。
export default defineConfig({
  base: './',
  plugins: [vue()],
  build: {
    target: 'chrome109',
    cssTarget: 'chrome109',
    outDir: 'dist',
    chunkSizeWarningLimit: 1600
  },
  server: {
    host: '0.0.0.0',
    port: 5173
  },
  preview: {
    host: '0.0.0.0',
    port: 4173
  }
});

// 单文件构建：产物为一个独立 HTML（JS/CSS 全内联），可直接 file:// 打开。
// 用于无 HTTP 服务的环境（如 VM 双击演示）；正式接入仍走 pulse7 serve（vite.config.js）。
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { viteSingleFile } from 'vite-plugin-singlefile';

export default defineConfig({
  base: './',
  plugins: [vue(), viteSingleFile()],
  build: {
    target: 'chrome109',
    cssTarget: 'chrome109',
    outDir: 'dist-single',
    chunkSizeWarningLimit: 2000
  }
});

import { defineConfig } from 'vite';
export default defineConfig({ base: './', resolve: { alias: { vue: 'vue/dist/vue.esm-bundler.js' } }, build: { target: 'chrome102', outDir: '../agent/web', emptyOutDir: true } });

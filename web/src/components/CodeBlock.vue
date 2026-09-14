<script setup>
// 代码块 — 等宽机器原文 + 复制（信息设计：机器输出全部走 Consolas）。
import CopyBtn from './CopyBtn.vue';

defineProps({ lang: { type: String, default: '' }, code: { type: String, default: '' } });
</script>

<template>
  <div class="cb">
    <div class="cb__bar">
      <span class="mono cb__lang">{{ lang }}</span>
      <CopyBtn :text="code" />
    </div>
    <div class="cb__body">
      <pre class="mono cb__pre">{{ code }}</pre>
    </div>
  </div>
</template>

<style scoped>
.cb {
  margin: 12px 0 8px;
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--g200);
}
.cb__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px;
  background: var(--code-bg);
  border-bottom: 1px solid var(--g200);
}
.cb__lang {
  font-size: 12px;
  color: var(--g500);
}
.cb__body {
  padding: 10px 16px;
  background: var(--code-bg);
}
.cb__pre {
  font-size: 13px;
  color: var(--green700);
  white-space: pre-wrap;
  line-height: 1.6;
  overflow-x: auto;
}
</style>

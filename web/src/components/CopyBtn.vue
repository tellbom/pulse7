<script setup>
// 复制按钮 — 「低成本事实校验」：路径/命令/结果一键可复制。
import { ref } from 'vue';

const props = defineProps({ text: { type: String, default: '' } });
const ok = ref(false);

function copy() {
  const done = () => {
    ok.value = true;
    setTimeout(() => (ok.value = false), 1400);
  };
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(props.text).then(done, done);
  } else {
    // Chrome 109 在 file:// 下可能无 clipboard 权限，退回 execCommand。
    const ta = document.createElement('textarea');
    ta.value = props.text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand('copy');
    } catch (e) {
      /* 忽略 */
    }
    document.body.removeChild(ta);
    done();
  }
}
</script>

<template>
  <button class="cp" :class="{ 'cp--ok': ok }" title="复制" @click.stop="copy">
    {{ ok ? '✓' : '⎘' }}
  </button>
</template>

<style scoped>
.cp {
  font-size: 12px;
  color: var(--g400);
  padding: 1px 4px;
  border-radius: var(--radius-sm);
  transition: color 0.12s, background-color 0.12s;
  flex-shrink: 0;
}
.cp:hover {
  color: var(--g700);
  background: var(--g200);
}
.cp--ok {
  color: var(--green600);
}
</style>

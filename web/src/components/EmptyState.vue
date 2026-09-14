<script setup>
// 空态 — 引导，不是道歉：写清下一步能做什么。
import { store, actions } from '../store/store.js';
import OrbAvatar from './OrbAvatar.vue';

const SUGGESTIONS = [
  { icon: '✎', label: '查看最近代码变更' },
  { icon: '△', label: '运行测试套件' },
  { icon: '⚠', label: '审查安全漏洞' },
  { icon: '⌕', label: '搜索接口定义' },
  { icon: '⊞', label: '分析性能瓶颈' },
  { icon: '⊕', label: '生成单元测试' }
];

function suggest(label) {
  store.emptyMode = false;
  actions.send(label);
}
</script>

<template>
  <div class="es">
    <OrbAvatar :size="96" big class="es__orb" />
    <h1 class="es__h">准备好了，输入你的第一个任务</h1>
    <p class="es__sub">
      工作区
      <span class="mono es__ws">{{ (store.workspace || '').split('\\').pop() || '（未选择）' }}</span
      >，从下方选择或直接输入
    </p>
    <div class="es__chips">
      <button v-for="s in SUGGESTIONS" :key="s.label" class="es__chip" @click="suggest(s.label)">
        <span class="es__ico">{{ s.icon }}</span>
        {{ s.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.es {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 32px;
  overflow-y: auto;
}
.es__orb {
  margin-bottom: 32px;
}
.es__h {
  font-size: var(--fs-xl);
  font-weight: 600;
  color: var(--g900);
  margin-bottom: 12px;
  text-align: center;
  line-height: 1.3;
}
.es__sub {
  font-size: 16px;
  color: var(--g500);
  margin-bottom: 40px;
  text-align: center;
}
.es__ws {
  color: var(--g700);
  background: var(--g100);
  padding: 1px 8px;
  border-radius: var(--radius-xs);
  font-size: 14px;
}
.es__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: center;
  max-width: 600px;
}
.es__chip {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--g200);
  border-radius: 999px;
  padding: 8px 16px;
  font-size: 14px;
  color: var(--g600);
  transition: background-color 0.12s, border-color 0.12s, color 0.12s;
}
.es__chip:hover {
  background: var(--g50);
  border-color: var(--g300);
  color: var(--g900);
}
.es__ico {
  color: var(--g400);
  font-size: 13px;
}
</style>

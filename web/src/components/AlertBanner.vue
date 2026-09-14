<script setup>
// 常驻告警横幅：受限模式（process_mode restricted）/ 事件流断开。降级必须持续可见（原则：强可见）。
import { store } from '../store/store.js';
import { actions } from '../store/store.js';

function dismiss() {
  store.alertRestricted = false;
}
</script>

<template>
  <div v-if="store.alertRestricted" class="al al--restricted">
    <span class="al__text"
      >受限模式 — {{ store.overview.mode && store.overview.mode.reason ? store.overview.mode.reason : 'Session Job 分配失败' }}（cleanupGuaranteed=false）。仅会话 Job
      成员受确定性清理，第三方后代可自行脱离。</span
    >
    <button class="al__close" aria-label="关闭告警" @click="dismiss">✕</button>
  </div>
  <div v-else-if="store.alertEndpointDown" class="al al--down">
    <span class="al__text"
      >事件流已断开 — {{ store.streamError || '连接中断' }}。断开期间产生的事件不回放；检查 pulse7 serve 是否仍在运行。</span
    >
    <button class="al__reconnect" @click="actions.connectStream()">重新连接</button>
  </div>
</template>

<style scoped>
.al {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 20px;
  border-bottom: 1px solid;
  font-size: 13px;
  flex-shrink: 0;
}
.al--restricted {
  border-color: var(--amber200);
  background: var(--amber50);
  color: var(--amber700);
}
.al--down {
  border-color: var(--red200);
  background: var(--red50);
  color: var(--red600);
}
.al__text {
  flex: 1;
}
.al__close {
  opacity: 0.5;
  color: inherit;
}
.al__close:hover {
  opacity: 1;
}
.al__reconnect {
  border: 1px solid currentColor;
  border-radius: var(--radius-xs);
  padding: 3px 10px;
  font-size: 12px;
  color: inherit;
}
</style>

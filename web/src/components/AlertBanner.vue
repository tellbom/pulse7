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
  <div v-if="store.streamNotice" class="al al--notice">
    <span class="al__text">{{ store.streamNotice }}</span>
    <button class="al__close" aria-label="关闭同步提示" @click="store.streamNotice = ''">✕</button>
  </div>
  <div v-if="store.connectionState === 'syncing'" class="al al--sync">
    <span class="al__text">连接中断，正在重新同步当前执行状态与实时事件；最后已知的任务状态会保留。</span>
  </div>
  <div v-else-if="store.alertEndpointDown" class="al al--down">
    <span class="al__text">事件流已断开 — {{ store.streamError || '连接中断' }}。任务状态尚未被判定为结束。</span>
    <button class="al__reconnect" @click="actions.reconnectStream()">立即重新同步</button>
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
.al--sync {
  border-color: var(--blue200);
  background: var(--blue50);
  color: var(--blue700);
}
.al--notice {
  border-color: var(--amber200);
  background: var(--amber50);
  color: var(--amber700);
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

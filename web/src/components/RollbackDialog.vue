<script setup>
// 回滚确认（屏 H）：列出检查点；执行前回显目标（序号+时间+commit+kind）；勾选核对后才能执行。
// dirtyFiles 语义固定为「工作区相对 HEAD 的变更文件数」，旧记录无计数明确显示“未记录”。
import { ref, computed } from 'vue';
import { store, actions } from '../store/store.js';

const selected = ref(null);
const confirmed = ref(false);

const target = computed(() => store.checkpoints.find((c) => c.seq === selected.value) || null);

function close() {
  store.rollbackOpen = false;
}
async function exec() {
  if (!target.value || !confirmed.value) return;
  const seq = target.value.seq;
  close();
  await actions.rollback(seq);
}
function esc(e) {
  if (e.key === 'Escape') close();
}
</script>

<template>
  <div class="rd-mask" @click.self="close" @keydown="esc" tabindex="-1">
    <div class="rd" role="dialog" aria-modal="true" aria-label="回滚到检查点">
      <div class="rd__head">
        <h2 class="rd__t">回滚到检查点</h2>
        <button class="rd__x" aria-label="关闭" @click="close">✕</button>
      </div>
      <div class="rd__body">
        <p class="rd__hint">回滚前请核对目标序号、时间与 commit 哈希。回滚只恢复工作区文件，不改变分支 HEAD。</p>
        <div class="rd__list">
          <label
            v-for="cp in store.checkpoints"
            :key="cp.seq"
            class="rd-item"
            :class="{ 'rd-item--sel': selected === cp.seq }"
          >
            <input
              type="radio"
              name="cp"
              class="rd-item__radio"
              :checked="selected === cp.seq"
              @change="selected = cp.seq; confirmed = false"
            />
            <div>
              <div class="rd-item__l1">
                <span class="mono rd-item__seq">#{{ cp.seq }}</span>
                <span class="mono rd-item__time">{{ cp.createdAt }}</span>
                <span class="mono rd-item__commit">{{ cp.commit }}</span>
                <span class="rd-item__kind">{{ cp.kind === 'auto' ? '自动' : '模型主动' }}</span>
              </div>
              <div class="rd-item__l2">
                {{ cp.dirtyFiles !== null && cp.dirtyFiles !== undefined ? `工作区相对 HEAD 变更 ${cp.dirtyFiles} 个文件` : '变更文件数未记录（旧元数据）' }}
              </div>
            </div>
          </label>
        </div>

        <div v-if="target" class="rd__confirm">
          <div class="mono rd__target">目标：#{{ target.seq }} · {{ target.createdAt }} · {{ target.commit }}</div>
          <label class="rd__ck">
            <input v-model="confirmed" type="checkbox" />
            <span>已核对目标，确认执行回滚</span>
          </label>
        </div>

        <div class="rd__acts">
          <button class="rd__cancel" @click="close">取消</button>
          <button class="rd__go" :disabled="!target || !confirmed" @click="exec">回滚</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rd-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.3);
}
.rd {
  width: 480px;
  background: #fff;
  border-radius: var(--radius-xl);
  border: 1px solid var(--g200);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18);
  overflow: hidden;
}
.rd__head {
  padding: 20px 24px;
  border-bottom: 1px solid var(--g100);
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.rd__t {
  font-size: 15px;
  font-weight: 600;
  color: var(--g900);
}
.rd__x {
  color: var(--g400);
  font-size: 18px;
  line-height: 1;
}
.rd__x:hover {
  color: var(--g700);
}
.rd__body {
  padding: 20px 24px;
}
.rd__hint {
  font-size: 13px;
  color: var(--g500);
  margin-bottom: 16px;
}
.rd__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 20px;
}
.rd-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--g100);
  cursor: pointer;
  transition: border-color 0.12s, background-color 0.12s;
}
.rd-item:hover {
  border-color: var(--g200);
  background: var(--g50);
}
.rd-item--sel {
  border-color: var(--blue400);
  background: var(--blue50);
}
.rd-item__radio {
  margin-top: 3px;
}
.rd-item__l1 {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  flex-wrap: wrap;
}
.rd-item__seq {
  font-weight: 600;
  color: var(--g800);
}
.rd-item__time {
  color: var(--g500);
}
.rd-item__commit {
  color: var(--blue600);
}
.rd-item__kind {
  color: var(--g400);
}
.rd-item__l2 {
  font-size: 12px;
  color: var(--g400);
  margin-top: 2px;
}
.rd__confirm {
  margin-bottom: 20px;
  padding: 12px;
  background: var(--red50);
  border: 1px solid var(--red100);
  border-radius: var(--radius-lg);
  font-size: 13px;
}
.rd__target {
  color: var(--g800);
  margin-bottom: 8px;
}
.rd__ck {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: var(--g700);
}
.rd__acts {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.rd__cancel {
  padding: 8px 16px;
  font-size: 13px;
  color: var(--g600);
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
}
.rd__cancel:hover {
  border-color: var(--g300);
}
.rd__go {
  padding: 8px 16px;
  font-size: 13px;
  background: var(--red500);
  color: #fff;
  border-radius: var(--radius-lg);
}
.rd__go:hover:not(:disabled) {
  background: var(--red600);
}
.rd__go:disabled {
  opacity: 0.4;
}
.rd__go:not(:disabled):active {
  transform: scale(0.97);
}
</style>

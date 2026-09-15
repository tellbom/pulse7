<script setup>
// 计划问题卡（plan-decision-frontend-handoff）：
// 展示完整问题原文（不推断）、关联答复入口（decisionId 绑定）、已答/取消/回显态、用户退出按钮。
// 发送失败保留草稿并刷新状态；成功后等 SSE，不乐观把问题显示为已解决。
import { ref, computed } from 'vue';
import { store, actions } from '../store/store.js';

const draft = ref('');
const sending = ref(false);

const d = computed(() => (store.planState && store.planState.decision) || null);
const planActive = computed(() => !!(store.planState && store.planState.active));
const awaiting = computed(() => !!(d.value && d.value.awaitingReply));
const answered = computed(() => d.value && !d.value.awaitingReply && d.value.replyUuid);
const cancelled = computed(() => d.value && d.value.cancelled);

async function submit() {
  const text = draft.value.trim();
  if (!text || sending.value) return;
  sending.value = true;
  const r = await actions.answerPlanningDecision(text);
  sending.value = false;
  if (r.ok) draft.value = '';
  // 失败：refreshPlan 已在 action 内处理，草稿保留
}
</script>

<template>
  <div v-if="planActive || awaiting" class="pd">
    <div class="pd__head">
      <span class="pd__tag">计划模式</span>
      <span v-if="store.planState && store.planState.path" class="mono pd__path" :title="store.planState.path">{{
        store.planState.path.split(/[\\/]/).pop()
      }}</span>
      <span class="pd__sp" />
      <button v-if="planActive && !awaiting" class="pd__exit" title="解除计划阶段限制（不批准计划内容）" @click="actions.exitPlanMode()">退出计划模式</button>
    </div>

    <div v-if="awaiting" class="pd__q" role="alertdialog" aria-label="模型提出的计划问题">
      <div class="pd__qt">模型需要你的决定</div>
      <div class="pd__qbody">{{ d.question }}</div>
      <div class="pd__row">
        <input
          v-model="draft"
          class="pd__input"
          type="text"
          placeholder="输入回答（可以说“不知道”，后续由模型判断是否还需澄清）…"
          :disabled="sending"
          @keyup.enter="submit"
        />
        <button class="pd__send" :disabled="sending || !draft.trim()" @click="submit">{{ sending ? '发送中…' : '回答' }}</button>
      </div>
      <div class="pd__meta mono">decisionId: {{ d.id }}</div>
    </div>

    <div v-else-if="answered" class="pd__done">
      <span>已收到回复</span>
      <span v-if="d.replyPreview" class="mono pd__preview">{{ d.replyPreview.slice(0, 120) }}<template v-if="d.replyPreview.length > 120">…</template></span>
      <span class="pd__note">（回复不等于问题已解决，是否还需澄清由后续模型判断）</span>
    </div>

    <div v-else-if="cancelled" class="pd__done">
      <span>等待已被取消（用户退出了计划模式）</span>
      <span class="pd__note">此问题未获回答，不冒充已解决</span>
    </div>

    <div v-else class="pd__idle">
      计划阶段进行中——模型正在探索并整理计划文件；需要你决策时会在这里提问。
    </div>
  </div>
</template>

<style scoped>
.pd {
  margin: 6px 16px;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  background: #fff;
  flex-shrink: 0;
}
.pd__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--g100);
}
.pd__tag {
  font-size: 11px;
  font-weight: 600;
  color: var(--bar-fg);
  background: var(--paper-deep);
  border-radius: 999px;
  padding: 1px 8px;
}
.pd__path {
  font-size: 11px;
  color: var(--bar-fg-dim);
}
.pd__sp {
  flex: 1;
}
.pd__exit {
  font-size: 11px;
  color: var(--bar-fg-dim);
  border: 1px solid var(--g200);
  border-radius: var(--radius-xs);
  padding: 2px 10px;
}
.pd__exit:hover {
  color: var(--bar-fg);
  border-color: var(--g300);
}
.pd__q {
  padding: 12px 14px;
}
.pd__qt {
  font-size: 12px;
  font-weight: 600;
  color: var(--amber700);
  margin-bottom: 6px;
}
.pd__qbody {
  font-size: 14px;
  color: var(--ink);
  line-height: 1.6;
  margin-bottom: 10px;
  white-space: pre-wrap;
}
.pd__row {
  display: flex;
  gap: 8px;
}
.pd__input {
  flex: 1;
  border: 1px solid var(--amber200);
  border-radius: var(--radius-sm);
  padding: 7px 10px;
  font-size: 13px;
  outline: none;
}
.pd__input:focus {
  border-color: var(--amber400);
}
.pd__send {
  flex-shrink: 0;
  font-size: 13px;
  background: var(--ink);
  color: #fff;
  border-radius: var(--radius-sm);
  padding: 7px 16px;
}
.pd__send:disabled {
  opacity: 0.4;
}
.pd__meta {
  font-size: 10px;
  color: var(--g300);
  margin-top: 6px;
}
.pd__done {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
  padding: 10px 14px;
  font-size: 13px;
  color: var(--bar-fg-dim);
}
.pd__preview {
  font-size: 11px;
  color: var(--bar-fg);
  background: var(--paper-deep);
  border-radius: var(--radius-xs);
  padding: 1px 6px;
}
.pd__note {
  font-size: 11px;
  color: var(--g400);
}
.pd__idle {
  padding: 10px 14px;
  font-size: 12px;
  color: var(--bar-fg-dim);
}
</style>

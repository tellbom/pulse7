<script setup>
// 单栏时间轴 — 按事件到达顺序渲染各记录（800px 居中）。
import { watch, ref, nextTick } from 'vue';
import { store, getters, actions } from '../store/store.js';
import UserMsg from './UserMsg.vue';
import AssistantMsg from './AssistantMsg.vue';
import ToolRecord from './ToolRecord.vue';
import InlineRecord from './InlineRecord.vue';
import TurnResultBlock from './TurnResultBlock.vue';

const el = ref(null);

watch(
  () => store.scrollTick,
  async () => {
    await nextTick();
    if (el.value) el.value.scrollTop = el.value.scrollHeight;
  },
  { flush: 'post' }
);

async function loadEarlier() {
  const before = el.value ? el.value.scrollHeight : 0;
  await actions.loadEarlierHistory();
  await nextTick();
  if (el.value) el.value.scrollTop += el.value.scrollHeight - before;
}
</script>

<template>
  <div ref="el" class="ct">
    <div class="ct__inner">
      <div class="ct__init">
        <span class="ct__initok">{{ getters.historyPreview ? '历史只读预览' : '当前执行会话' }}</span>
        <span class="mono ct__initws">{{ getters.viewedWorkspace }}</span>
        <span v-if="getters.viewingActive && store.context.budget"
          >上下文预算 <span class="mono ct__initb">{{ store.context.budget.toLocaleString() }}</span> token（估算）</span
        >
      </div>

      <div v-if="store.hasEarlierHistory || store.historyLoading" class="ct__older">
        <button :disabled="store.historyLoading" @click="loadEarlier">
          {{ store.historyLoading ? '正在读取历史…' : `加载更早记录（当前从第 ${store.historyStartOffset + 1} 条开始）` }}
        </button>
      </div>

      <template v-for="item in store.timeline" :key="item.id">
        <UserMsg v-if="item.type === 'user'" :text="item.text" :attachment="item.attachment" />
        <AssistantMsg v-else-if="item.type === 'assistant'" :text="item.text" :reasoning="item.reasoning" :streaming="item.streaming" :model="item.model" :text-preview="item.textPreview" :reasoning-preview="item.reasoningPreview" :lc="item.lc" />
        <div v-else-if="item.type === 'tool'" class="ct__toolwrap">
          <ToolRecord :item="item" />
        </div>
        <InlineRecord v-else-if="['waiting', 'skill_catalog', 'skill', 'outside_write', 'compaction', 'process_warning', 'process', 'permission'].includes(item.type)" :item="item" />
        <TurnResultBlock v-else-if="item.type === 'turn_result'" :item="item" />
      </template>
      <div class="ct__pad" />
    </div>
  </div>
</template>

<style scoped>
.ct {
  flex: 1;
  overflow-y: auto;
}
.ct__inner {
  max-width: 800px;
  margin: 0 auto;
}
.ct__init {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 24px;
  border-bottom: 1px solid var(--g100);
  font-size: 12px;
  color: var(--g400);
  flex-wrap: wrap;
}
.ct__initok {
  color: var(--green600);
  font-weight: 500;
  font-size: 13px;
}
.ct__older {
  display: flex;
  justify-content: center;
  padding: 10px 24px 2px;
}
.ct__older button {
  border: 1px solid var(--g200);
  border-radius: 999px;
  padding: 4px 12px;
  color: var(--g500);
  background: #fff;
  font-size: 11px;
}
.ct__older button:hover {
  color: var(--blue600);
  border-color: var(--blue200);
}
.ct__older button:disabled {
  opacity: 0.55;
}
.ct__toolwrap {
  padding: 4px 24px;
}
.ct__pad {
  height: 24px;
}
</style>

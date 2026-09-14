<script setup>
// 单栏时间轴 — 按事件到达顺序渲染各记录（800px 居中）。
import { watch, ref, nextTick } from 'vue';
import { store } from '../store/store.js';
import UserMsg from './UserMsg.vue';
import AssistantMsg from './AssistantMsg.vue';
import ToolRecord from './ToolRecord.vue';
import InlineRecord from './InlineRecord.vue';
import TurnResultBlock from './TurnResultBlock.vue';

const el = ref(null);

watch(
  () => store.scrollTick + store.timeline.length,
  async () => {
    await nextTick();
    if (el.value) el.value.scrollTop = el.value.scrollHeight;
  },
  { flush: 'post' }
);
</script>

<template>
  <div ref="el" class="ct">
    <div class="ct__inner">
      <div class="ct__init">
        <span class="ct__initok">会话已建立</span>
        <span class="mono ct__initws">{{ store.workspace }}</span>
        <span v-if="store.context.budget"
          >上下文预算 <span class="mono ct__initb">{{ store.context.budget.toLocaleString() }}</span> token（估算）</span
        >
      </div>

      <template v-for="item in store.timeline" :key="item.id">
        <UserMsg v-if="item.type === 'user'" :text="item.text" :attachment="item.attachment" />
        <AssistantMsg v-else-if="item.type === 'assistant'" :text="item.text" :reasoning="item.reasoning" :streaming="item.streaming" :model="item.model" :text-preview="item.textPreview" :reasoning-preview="item.reasoningPreview" :lc="item.lc" />
        <div v-else-if="item.type === 'tool'" class="ct__toolwrap">
          <ToolRecord :item="item" />
        </div>
        <InlineRecord v-else-if="['waiting', 'skill', 'outside_write', 'compaction', 'process_warning', 'process', 'permission'].includes(item.type)" :item="item" />
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
.ct__toolwrap {
  padding: 4px 24px;
}
.ct__pad {
  height: 24px;
}
</style>

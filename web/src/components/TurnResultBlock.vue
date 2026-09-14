<script setup>
// 任务结束摘要（屏 D）：结束形态（五种，长得不同）+ 耗时 + 命令清单 + 工作区外写入汇总 + 后台进程 + 回滚入口。
import { ref } from 'vue';
import { store, actions } from '../store/store.js';
import { formatMs } from '../lib/format.js';
import CopyBtn from './CopyBtn.vue';

const props = defineProps({ item: { type: Object, required: true } });
const answer = ref('');

const cfg = {
  success: { label: '已完成', color: 'ok', icon: '✓' },
  need_answer: { label: '等待回答', color: 'warn', icon: '?' },
  max_rounds: { label: '触顶中止', color: 'err', icon: '⊤' },
  error: { label: '出错中止', color: 'err', icon: '✕' },
  interrupted: { label: '已中断', color: 'err', icon: '■' }
}[props.item.status] || { label: props.item.status, color: 'err', icon: '·' };

function submitAnswer() {
  const text = answer.value.trim();
  if (!text) return;
  answer.value = '';
  props.item.answered = true;
  actions.send(text);
}
</script>

<template>
  <div class="tbk">
    <div class="tbk__card" :class="'tbk__card--' + cfg.color">
      <div class="tbk__head">
        <span class="tbk__label">{{ cfg.label }}</span>
        <span class="mono tbk__meta">{{ item.elapsedMs ? formatMs(item.elapsedMs) : '' }}</span>
      </div>
      <div v-if="item.error" class="tbk__err">{{ item.error }}</div>
      <div class="tbk__body">
        <div v-if="item.commands.length">
          <div class="tbk__st">执行命令</div>
          <div v-for="(cmd, i) in item.commands" :key="i" class="mono tbk__cmd">
            <span class="tbk__prompt">$</span>
            <span class="tbk__cmdtxt">{{ cmd }}</span>
            <CopyBtn :text="cmd" />
          </div>
        </div>
        <div v-if="item.outsideWrites.length" class="tbk__ow">
          <div class="tbk__owt">工作区外写入 ({{ item.outsideWrites.length }}) · 不可回滚</div>
          <div v-for="(w, i) in item.outsideWrites" :key="i" class="mono tbk__owpath">
            <span>{{ w.path }}</span>
            <CopyBtn :text="w.path" />
          </div>
        </div>
        <div v-if="item.bgProcesses.length">
          <div class="tbk__st">后台进程（pulse7 退出后按 detached 属性处理）</div>
          <div v-for="(p, i) in item.bgProcesses" :key="i" class="mono tbk__bg">
            <span class="tbk__bgtid">{{ p.taskId }}</span> {{ p.command }}
          </div>
        </div>
        <div class="tbk__acts">
          <button class="tbk__rb" @click="store.rollbackOpen = true">回滚到检查点</button>
          <template v-if="item.status === 'need_answer' && !item.answered">
            <input
              v-model="answer"
              class="tbk__ans"
              type="text"
              placeholder="输入回答后按 Enter 继续…"
              @keyup.enter="submitAnswer"
            />
          </template>
        </div>
        <div v-if="item.status === 'need_answer'" class="tbk__anshint">回答会追加为用户消息并继续本会话</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tbk {
  padding: 8px 16px;
}
.tbk__card {
  border-radius: var(--radius-lg);
  border: 1px solid;
  overflow: hidden;
}
.tbk__card--ok {
  border-color: var(--green200);
  background: var(--green50);
}
.tbk__card--warn {
  border-color: var(--amber200);
  background: var(--amber50);
}
.tbk__card--err {
  border-color: var(--red200);
  background: var(--red50);
}
.tbk__head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.04);
}
.tbk__label {
  font-weight: 600;
  font-size: 14px;
}
.tbk__card--ok .tbk__label {
  color: var(--green600);
}
.tbk__card--warn .tbk__label {
  color: var(--amber600);
}
.tbk__card--err .tbk__label {
  color: var(--red500);
}
.tbk__meta {
  font-size: 12px;
  color: var(--g400);
}
.tbk__err {
  padding: 8px 16px 0;
  font-size: 12px;
  color: var(--red500);
}
.tbk__body {
  padding: 12px 16px;
}
.tbk__st {
  font-size: 11px;
  color: var(--g500);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}
.tbk__cmd {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--g700);
}
.tbk__prompt {
  color: var(--g300);
}
.tbk__cmdtxt {
  word-break: break-all;
}
.tbk__ow {
  border-left: 2px solid var(--amber300);
  padding-left: 12px;
  margin: 10px 0;
}
.tbk__owt {
  font-size: 11px;
  color: var(--amber600);
  font-weight: 500;
  margin-bottom: 4px;
}
.tbk__owpath {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--g600);
}
.tbk__owpath span {
  word-break: break-all;
}
.tbk__bg {
  font-size: 12px;
  color: var(--g500);
}
.tbk__bgtid {
  color: var(--blue500);
}
.tbk__acts {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 2px;
}
.tbk__rb {
  font-size: 13px;
  color: var(--g600);
  border: 1px solid var(--g200);
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  background: #fff;
  transition: border-color 0.12s, color 0.12s;
}
.tbk__rb:hover {
  color: var(--g900);
  border-color: var(--g300);
}
.tbk__rb:active {
  transform: scale(0.97);
}
.tbk__ans {
  flex: 1;
  border: 1px solid var(--amber200);
  border-radius: var(--radius-sm);
  padding: 6px 12px;
  font-size: 13px;
  outline: none;
  background: #fff;
}
.tbk__ans:focus {
  border-color: var(--amber400);
}
.tbk__anshint {
  font-size: 11px;
  color: var(--g400);
  margin-top: 6px;
}
.tbk__body > div + div {
  margin-top: 10px;
}
</style>

<script setup>
// 输入卡 — 顶部上下文用量条（估算）；忙碌条 + 中断；自适应高度输入；模型切换；剩余上下文；发送。
// 大段粘贴自动折叠为附件，不撑爆输入框。
import { ref, computed, onBeforeUnmount } from 'vue';
import { ElMessage } from 'element-plus';
import { store, getters, actions } from '../store/store.js';

// 忙碌耗时：只在等待/执行态走秒（已发生的事实），不预测剩余时间。
const nowTick = ref(Date.now());
let timer = null;
const ensureTimer = () => {
  if (!timer && (getters.busy || store.phase === 'need_answer')) {
    timer = setInterval(() => (nowTick.value = Date.now()), 1000);
  } else if (timer && !getters.busy && store.phase !== 'need_answer') {
    clearInterval(timer);
    timer = null;
  }
};
const busyElapsed = computed(() => {
  ensureTimer();
  if (!store.turnStartedAt || (!getters.busy && store.phase !== 'need_answer')) return '';
  const s = Math.max(0, Math.round((nowTick.value - store.turnStartedAt) / 1000));
  return s >= 60 ? `${Math.floor(s / 60)}m${s % 60}s` : `${s}s`;
});
onBeforeUnmount(() => timer && clearInterval(timer));

const draft = ref('');
const attachment = ref(null); // { name, sizeText, ext, content }
const ta = ref(null);
const modelOpen = ref(false);

const busy = computed(() => getters.busy || store.switching);
const needAnswer = computed(() => store.phase === 'need_answer');
const ctxLevel = computed(() => getters.contextLevel);
const barCls = computed(() => ({ normal: 'ic__bar--n', warning: 'ic__bar--w', critical: 'ic__bar--c' })[ctxLevel.value]);
const ctxCls = computed(() => ({ normal: 'ic__ctx--n', warning: 'ic__ctx--w', critical: 'ic__ctx--c' })[ctxLevel.value]);
const placeholder = computed(() => {
  if (busy.value) return '任务进行中，中断后可继续输入…';
  if (needAnswer.value) return '输入回答，Enter 发送…';
  return '输入任务，或 @ 引用文件…';
});

const MODELS = ['gpt-4o-internal', 'gpt-4o-mini', 'claude-3-5-sonnet', 'deepseek-v3'];

function autosize() {
  const el = ta.value;
  if (!el) return;
  el.style.height = 'auto';
  el.style.height = Math.min(el.scrollHeight, 200) + 'px';
}

function onPaste(e) {
  const text = (e.clipboardData || window.clipboardData).getData('text');
  if (text && text.length > 2000) {
    e.preventDefault();
    attachment.value = {
      name: `粘贴内容 ${new Date().toLocaleTimeString()}.log`,
      sizeText: (text.length / 1024).toFixed(1) + 'KB',
      ext: 'LOG',
      content: text
    };
    ElMessage.success('大段内容已折叠为附件，随任务一并发送');
  }
}

function clearAttachment() {
  attachment.value = null;
}

async function submit() {
  const text = (draft.value || '').trim();
  if (!text && !attachment.value) return;
  if (busy.value) return;
  const full = attachment.value ? (attachment.value.content + (text ? '\n\n' + text : '')) : text;
  const att = attachment.value;
  draft.value = '';
  attachment.value = null;
  await nextTickFrame();
  if (ta.value) ta.value.style.height = 'auto';
  actions.send(full, att);
}
function nextTickFrame() {
  return new Promise((r) => requestAnimationFrame(() => r()));
}

function onKey(e) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    submit();
  }
}

async function pickModel(m) {
  modelOpen.value = false;
  if (m === store.config.model) return;
  await actions.saveModel(m);
}

function atHint() {
  ElMessage.info('输入 @ + 相对路径引用文件（自动补全需要后端支持，当前版本直接输入路径）');
}
function moreHint() {
  ElMessage.info('更多选项：当前版本提供粘贴折叠与 @ 引用');
}
</script>

<template>
  <div class="ic">
    <div class="ic__card">
      <div class="ic__track">
        <div class="ic__bar" :class="barCls" :style="{ width: getters.contextPct + '%' }" />
      </div>

      <div v-if="busy || needAnswer" class="ic__busy">
        <div class="ic__busyl">
          <span class="ic__busydot pulse-dot" :class="needAnswer ? 'ic__busydot--ans' : ''" />
          <span class="ic__busyt">
            {{ needAnswer ? '模型在等待你的回答' : store.phase === 'streaming' ? '模型输出中' : store.phase === 'tool_running' ? '工具执行中' : '等待模型响应' }}<span v-if="busyElapsed" class="mono ic__elapsed">{{ busyElapsed }}</span>
          </span>
        </div>
        <button v-if="busy" class="ic__stop" @click="actions.interrupt()">■ 中断</button>
      </div>

      <div v-if="attachment" class="ic__att">
        <span class="ic__attico mono">LOG</span>
        <span class="ic__attname">{{ attachment.name }}</span>
        <span class="mono ic__attsize">{{ attachment.sizeText }}</span>
        <button class="ic__attdel" aria-label="移除附件" @click="clearAttachment">✕</button>
      </div>

      <textarea
        ref="ta"
        v-model="draft"
        rows="3"
        :disabled="busy"
        :placeholder="placeholder"
        class="ic__ta"
        @input="autosize"
        @keydown="onKey"
        @paste="onPaste"
      />

      <div class="ic__tools">
        <div class="ic__model">
          <button class="ic__modelbtn" @click="modelOpen = !modelOpen">
            <span class="ic__modeldot" />
            {{ store.config.model || '（未配置模型）' }}
            <span class="ic__modelcaret">▾</span>
          </button>
          <template v-if="modelOpen">
            <div class="ic__overlay" @click="modelOpen = false" />
            <div class="ic__modelmenu">
              <button
                v-for="m in MODELS"
                :key="m"
                class="mono ic__modelitem"
                :class="{ 'ic__modelitem--cur': m === store.config.model }"
                @click="pickModel(m)"
              >
                {{ m }}
              </button>
              <div class="ic__modelnote">切换会写入全局配置并立即生效；活动轮中会被拒绝</div>
            </div>
          </template>
        </div>

        <button class="ic__ghost" title="引用文件" @click="atHint">@</button>
        <button class="ic__ghost ic__ghost--proc" title="后台进程（右上角打开）" aria-label="后台进程" @click="store.drawerOpen = true">
          <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.4">
            <rect x="1.8" y="3" width="12.4" height="10" rx="1.4" />
            <path d="M1.8 6.2h12.4M4 4.6h.01M5.8 4.6h.01" stroke-linecap="round" />
            <path d="M6 8.6h2.2M6 10.4h4" stroke-linecap="round" />
          </svg>
        </button>
        <button class="ic__ghost" title="更多选项" @click="moreHint">···</button>

        <div class="ic__sp" />

        <span class="mono ic__ctx" :class="ctxCls" :title="`已用 ${getters.contextPct}%（估算：序列化字节÷4，非端点 usage）`">
          剩余 {{ Math.max(0, 100 - getters.contextPct) }}%
        </span>

        <button class="ic__send" :disabled="busy || (!draft.trim() && !attachment)" :aria-label="needAnswer ? '发送回答' : '发送任务'" @click="submit">
          ↑
        </button>
      </div>
    </div>
    <p class="ic__disclaimer">AI 可能犯错，重要信息请自行核实</p>
  </div>
</template>

<style scoped>
.ic {
  padding: 8px 24px 16px;
  flex-shrink: 0;
  max-width: 800px;
  margin: 0 auto;
  width: 100%;
}
.ic__card {
  background: #fff;
  border-radius: var(--radius-xl);
  overflow: hidden;
  box-shadow: var(--shell-bshadow);
}
.ic__track {
  height: 2px;
  background: var(--g100);
  overflow: hidden;
}
.ic__bar {
  height: 100%;
  transition: width 0.4s;
}
.ic__bar--n {
  background: var(--g200);
}
.ic__bar--w {
  background: var(--amber400);
}
.ic__bar--c {
  background: var(--red400);
}
.ic__busy {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  border-bottom: 1px solid var(--g100);
  background: var(--amber50);
}
.ic__busyl {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--amber700);
}
.ic__busydot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--amber400);
}
.ic__busydot--ans {
  background: var(--blue400);
}
.ic__elapsed {
  margin-left: 8px;
  color: var(--g500);
  font-size: 12px;
}
.ic__stop {
  font-size: 12px;
  color: var(--red500);
  border: 1px solid var(--red200);
  border-radius: var(--radius-sm);
  padding: 3px 10px;
  transition: color 0.12s, border-color 0.12s;
}
.ic__stop:hover {
  color: var(--red700);
  border-color: var(--red300, var(--red400));
}
.ic__att {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 12px 16px 0;
  border: 1px solid var(--g200);
  border-radius: var(--radius-md);
  padding: 8px 12px;
  background: var(--g50);
}
.ic__attico {
  font-size: 9px;
  font-weight: 700;
  color: var(--blue600);
  border: 1px solid var(--blue100);
  background: var(--blue50);
  border-radius: var(--radius-xs);
  padding: 3px 6px;
}
.ic__attname {
  flex: 1;
  font-size: 12px;
  color: var(--g700);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ic__attsize {
  font-size: 11px;
  color: var(--g400);
}
.ic__attdel {
  color: var(--g400);
  padding: 2px 6px;
  border-radius: var(--radius-xs);
}
.ic__attdel:hover {
  color: var(--g700);
  background: var(--g200);
}
.ic__ta {
  display: block;
  width: 100%;
  padding: 16px 16px 8px;
  font-size: 15px;
  color: var(--g900);
  outline: none;
  background: transparent;
  line-height: 1.6;
  border: none;
}
.ic__ta::placeholder {
  color: var(--g400);
}
.ic__ta:disabled {
  opacity: 0.5;
}
.ic__tools {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px 12px;
}
.ic__model {
  position: relative;
}
.ic__modelbtn {
  display: flex;
  align-items: center;
  gap: 6px;
  border-radius: 999px;
  border: 1px solid var(--g200);
  background: #fff;
  padding: 6px 12px;
  font-size: 13px;
  color: var(--g700);
  transition: background-color 0.12s;
}
.ic__modelbtn:hover {
  background: var(--g50);
}
.ic__modeldot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--green500);
  flex-shrink: 0;
}
.ic__modelcaret {
  color: var(--g400);
  font-size: 10px;
}
.ic__overlay {
  position: fixed;
  inset: 0;
  z-index: 10;
}
.ic__modelmenu {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 0;
  width: 220px;
  background: #fff;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  z-index: 11;
  padding: 6px 0;
}
.ic__modelitem {
  width: 100%;
  text-align: left;
  padding: 8px 16px;
  font-size: 13px;
  color: var(--g600);
}
.ic__modelitem:hover {
  background: var(--g50);
}
.ic__modelitem--cur {
  color: var(--g900);
  font-weight: 600;
}
.ic__modelnote {
  font-size: 11px;
  color: var(--g300);
  padding: 6px 16px 4px;
  border-top: 1px solid var(--g100);
  margin-top: 4px;
}
.ic__ghost {
  color: var(--g400);
  border-radius: 50%;
  padding: 8px;
  font-size: 15px;
  transition: color 0.12s, background-color 0.12s;
}
.ic__ghost:hover {
  color: var(--g700);
  background: var(--g100);
}
.ic__sp {
  flex: 1;
}
.ic__ctx {
  font-size: 12px;
  margin-right: 8px;
}
.ic__ctx--n {
  color: var(--g400);
}
.ic__ctx--w {
  color: var(--amber500);
}
.ic__ctx--c {
  color: var(--red500);
}
.ic__send {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: var(--g900);
  color: #fff;
  font-size: 17px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: background-color 0.12s, transform 0.1s;
}
.ic__send:hover {
  background: var(--g700);
}
.ic__send:disabled {
  opacity: 0.3;
}
.ic__send:not(:disabled):active {
  transform: scale(0.94);
}
.ic__disclaimer {
  text-align: center;
  font-size: 12px;
  color: var(--g400);
  margin-top: 10px;
}
</style>

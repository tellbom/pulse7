<script setup>
// 助手消息 — 左对齐 + 球体头像 + pulse7·模型 标签；流式光标；推理区（端点提供的 reasoning_content，
// 独立可折叠、不并入最终回答）；``` 代码块渲染；操作按钮（复制可用）。
import { ref, computed, reactive } from 'vue';
import { ElMessage } from 'element-plus';
import OrbAvatar from './OrbAvatar.vue';
import CodeBlock from './CodeBlock.vue';
import { mdToHtml } from '../lib/md.js';
import { actions } from '../store/store.js';

const props = defineProps({
  text: { type: String, default: '' },
  reasoning: { type: String, default: '' },
  streaming: { type: Boolean, default: false },
  model: { type: String, default: '' },
  textPreview: { type: Boolean, default: false },
  reasoningPreview: { type: Boolean, default: false },
  lc: { type: Object, default: null }
});

const md = (t) => mdToHtml(t);

// 大内容预览：按需经 lc1 引用分页拉取原文（服务端 nextOffset），不自动全量加载。
const full = reactive({ text: '', textMore: false, textNext: 0, reasoning: '', reasoningMore: false, reasoningNext: 0 });
const loadingKey = ref('');

async function loadFull(kind) {
  const refKey = kind === 'reasoning' ? 'reasoning' : 'content';
  const refValue = props.lc && props.lc[refKey];
  if (!refValue) return;
  loadingKey.value = kind;
  const offset = kind === 'reasoning' ? (full.reasoningMore ? full.reasoningNext : 0) : full.textMore ? full.textNext : 0;
  const r = await actions.fetchResultPage(refValue, offset);
  if (r) {
    if (kind === 'reasoning') {
      full.reasoning += r.result || '';
      full.reasoningNext = r.nextOffset || 0;
      full.reasoningMore = !!r.hasMore;
    } else {
      full.text += r.result || '';
      full.textNext = r.nextOffset || 0;
      full.textMore = !!r.hasMore;
    }
  }
  loadingKey.value = '';
}

const reasoningOpen = ref(false);

// 轻量渲染：``` 围栏代码块 + 行内 `code`，其余按段落原样（不做富文本）。
const segments = computed(() => {
  const parts = [];
  const re = /```([\w+-]*)\n?([\s\S]*?)(?:```|$)/g;
  let last = 0;
  let m;
  while ((m = re.exec(props.text)) !== null) {
    if (m.index > last) parts.push({ kind: 'text', text: props.text.slice(last, m.index) });
    parts.push({ kind: 'code', lang: m[1] || '', code: m[2] });
    last = re.lastIndex;
  }
  if (last < props.text.length) parts.push({ kind: 'text', text: props.text.slice(last) });
  return parts;
});

function info(msg) {
  ElMessage.info(msg);
}

function copyAll() {
  const ta = document.createElement('textarea');
  ta.value = props.text;
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try {
    document.execCommand('copy');
    ElMessage.success('已复制');
  } catch (e) {
    /* 忽略 */
  }
  document.body.removeChild(ta);
}
</script>

<template>
  <div class="am">
    <OrbAvatar :size="32" class="am__orb" />
    <div class="am__main">
      <div class="am__label">pulse7 · <span class="mono am__model">{{ model }}</span></div>
      <div v-if="reasoning" class="am__reasoning">
        <button class="am__rtoggle" :aria-expanded="reasoningOpen" @click="reasoningOpen = !reasoningOpen">
          <span class="am__rico">◇</span>
          <span>推理过程（端点提供，与最终回答分离）</span>
          <span v-if="reasoningPreview && !full.reasoning" class="am__pv">预览</span>
          <span class="am__rcaret">{{ reasoningOpen ? '▲' : '▼' }}</span>
        </button>
        <div v-if="reasoningOpen" class="am__rbody">
          <div>{{ full.reasoning || reasoning }}</div>
          <div v-if="reasoningPreview" class="am__fullrow">
            <button class="am__fullbtn" :disabled="loadingKey === 'reasoning'" @click="loadFull('reasoning')">
              {{ loadingKey === 'reasoning' ? '读取中…' : full.reasoning ? (full.reasoningMore ? '继续加载原文' : '原文已全部加载') : '展开完整原文' }}
            </button>
          </div>
        </div>
      </div>
      <div class="am__body">
        <template v-if="textPreview && !full.text">
          <div class="am__md" v-html="md(text)"></div>
          <div class="am__fullrow">
            <span class="am__pv">正文为预览（原文件过大）</span>
            <button class="am__fullbtn" :disabled="loadingKey === 'text'" @click="loadFull('text')">{{ loadingKey === 'text' ? '读取中…' : '展开完整原文' }}</button>
          </div>
        </template>
        <div v-else-if="full.text" class="am__md" v-html="md(full.text)"></div>
        <template v-if="!textPreview || full.text" v-for="(seg, i) in segments" :key="i">
          <div v-if="seg.kind === 'text'" class="am__md" v-html="md(seg.text)"></div>
          <CodeBlock v-else :lang="seg.lang" :code="seg.code" />
        </template>
        <span v-if="streaming" class="cursor-blink am__cursor">▍</span>
      </div>
      <div v-if="full.text && full.textMore" class="am__fullrow">
        <button class="am__fullbtn" :disabled="loadingKey === 'text'" @click="loadFull('text')">继续加载原文</button>
      </div>
      <div v-if="!streaming" class="am__acts">
        <button class="am__act" title="复制" @click="copyAll">⎘</button>
        <button class="am__act" title="重试（当前版本不支持重新生成）" @click="info('当前版本不支持重新生成上一条回答')">↺</button>
        <button class="am__act" title="更多" @click="info('暂无更多操作')">···</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.am {
  display: flex;
  gap: 12px;
  padding: 16px 24px;
}
.am__orb {
  margin-top: 2px;
}
.am__main {
  flex: 1;
  min-width: 0;
}
.am__label {
  font-size: 13px;
  color: var(--g500);
  margin-bottom: 8px;
  font-weight: 500;
}
.am__reasoning {
  margin-bottom: 10px;
  border: 1px solid var(--g150);
  border-left: 2px solid var(--g300);
  border-radius: var(--radius-md);
  background: var(--g50);
  overflow: hidden;
}
.am__rtoggle {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  text-align: left;
  padding: 7px 12px;
  font-size: 12px;
  color: var(--g500);
  transition: color 0.12s;
}
.am__rtoggle:hover {
  color: var(--g700);
}
.am__rico {
  font-size: 11px;
  color: var(--g400);
}
.am__rcaret {
  margin-left: auto;
  font-size: 10px;
  color: var(--g300);
}
.am__rbody {
  padding: 4px 14px 10px;
  font-size: 13px;
  color: var(--g600);
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  border-top: 1px dashed var(--g150);
}
.am__model {
  font-weight: 400;
  font-size: 12px;
}
.am__body {
  font-size: 15px;
  color: var(--g800);
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: break-word;
}
.am__text {
  white-space: pre-wrap;
}
.am__cursor {
  color: var(--blue400);
}
/* Markdown 渲染排版 */
.am__md :deep(h2),
.am__md :deep(h3),
.am__md :deep(h4),
.am__md :deep(h5) {
  margin: 14px 0 6px;
  line-height: 1.4;
  color: var(--g900);
}
.am__md :deep(h2) {
  font-size: 17px;
}
.am__md :deep(h3) {
  font-size: 16px;
}
.am__md :deep(h4),
.am__md :deep(h5) {
  font-size: 15px;
}
.am__md :deep(p) {
  margin: 4px 0;
}
.am__md :deep(ul),
.am__md :deep(ol) {
  margin: 6px 0;
  padding-left: 22px;
}
.am__md :deep(li) {
  margin: 3px 0;
}
.am__md :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: var(--g100);
  border-radius: var(--radius-xs);
  padding: 1px 5px;
  color: var(--g800);
}
.am__md :deep(blockquote) {
  margin: 6px 0;
  padding: 2px 12px;
  border-left: 3px solid var(--g300);
  color: var(--g600);
}
.am__md :deep(hr) {
  border: none;
  border-top: 1px solid var(--g200);
  margin: 12px 0;
}
.am__md :deep(a) {
  color: var(--blue600);
}
.am__pv {
  font-size: 10px;
  color: var(--amber600);
  background: var(--amber50);
  border: 1px solid var(--amber100);
  border-radius: 999px;
  padding: 0 6px;
  margin-left: 6px;
}
.am__fullrow {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}
.am__fullbtn {
  font-size: 12px;
  color: var(--blue600);
  border: 1px solid var(--blue200);
  border-radius: var(--radius-xs);
  padding: 3px 12px;
}
.am__fullbtn:hover:not(:disabled) {
  background: var(--blue50);
}
.am__fullbtn:disabled {
  opacity: 0.5;
}
.am__acts {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-top: 12px;
  margin-left: -6px;
}
.am__act {
  padding: 6px;
  color: var(--g300);
  border-radius: var(--radius-sm);
  font-size: 15px;
  transition: color 0.12s, background-color 0.12s;
}
.am__act:hover {
  color: var(--g600);
  background: var(--g100);
}
</style>

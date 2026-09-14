<script setup>
// 工具记录条（事件 F）：折叠一行（工具名 + 参数摘要 + 耗时 + 状态）；展开参数 JSON 与完整结果。
// 状态：running 蓝 / success 绿 / error 红 / denied 权限不足 / hard_link 硬链接能力拒绝（琥珀）。
// 完整结果不在事件里，展开时经 GET /api/tool-result?ref= 拉取。
import { ref, computed, reactive } from 'vue';
import { store, actions } from '../store/store.js';
import { formatMs } from '../lib/format.js';

const props = defineProps({ item: { type: Object, required: true } });

const open = ref(false);
const loading = ref(false);
const tick = ref(Date.now());
// 分页状态：字节偏移全部来自服务端 nextOffset，不用 JS 字符数推算（large-content handoff §6）。
const pages = reactive({ text: '', nextOffset: 0, totalBytes: 0, hasMore: false });

let timer = null;
// 运行中的工具显示走时时长（真实节奏，不装进度）。
if (props.item.status === 'running') {
  timer = setInterval(() => {
    tick.value = Date.now();
    if (props.item.status !== 'running') {
      clearInterval(timer);
      timer = null;
    }
  }, 500);
}

const elapsedText = computed(() => {
  if (props.item.status === 'running') return formatMs(tick.value - (props.item.startedAt || tick.value));
  if (props.item.status === 'unknown') return '';
  return props.item.elapsedMs > 0 ? formatMs(props.item.elapsedMs) : '';
});

const dotCls = computed(
  () =>
    ({
      running: 'tr__dot--run pulse-dot',
      success: 'tr__dot--ok',
      error: 'tr__dot--err',
      denied: 'tr__dot--err',
      hard_link: 'tr__dot--warn',
      unknown: 'tr__dot--unknown'
    })[props.item.status] || 'tr__dot--run'
);

const statusText = computed(
  () =>
    ({
      running: '进行中',
      success: '成功',
      error: '失败',
      denied: '权限不足',
      hard_link: '拒绝',
      unknown: '状态未知'
    })[props.item.status] || ''
);

const argsPreview = computed(() => {
  const vals = Object.values(props.item.args || {}).slice(0, 2);
  return vals
    .map((v) => (typeof v === 'string' ? (v.split('\\').pop() || v) : String(v)))
    .join(' · ');
});

async function toggle() {
  open.value = !open.value;
  const ref = props.item.lcRef || props.item.resultRef;
  if (open.value && ref && !pages.totalBytes && !loading.value) {
    await loadMore(true);
  }
}

async function loadMore(first) {
  loading.value = true;
  const ref = props.item.lcRef || props.item.resultRef;
  const r = await actions.fetchResultPage(ref, first ? 0 : pages.nextOffset);
  if (r) {
    pages.text += r.result || '';
    pages.nextOffset = r.nextOffset || 0;
    pages.totalBytes = r.totalBytes || 0;
    pages.hasMore = !!r.hasMore;
    props.item.expandedResult = pages.text;
  }
  loading.value = false;
}

const progressText = computed(() => {
  if (!pages.totalBytes) return '';
  const kb = (n) => (n < 1024 ? n + 'B' : (n / 1024).toFixed(1) + 'KB');
  return `${kb(pages.text.length)} / ${kb(pages.totalBytes)}`;
});
</script>

<template>
  <div class="tr">
    <button class="tr__row" :aria-expanded="open" @click="toggle">
      <span class="tr__dot" :class="dotCls" />
      <span class="mono tr__name">{{ item.name }}</span>
      <span class="tr__args">{{ argsPreview }}</span>
      <span
        class="tr__status"
        :class="{
          'tr__status--run': item.status === 'running',
          'tr__status--ok': item.status === 'success',
          'tr__status--err': item.status === 'error' || item.status === 'denied',
          'tr__status--warn': item.status === 'hard_link',
          'tr__status--unknown': item.status === 'unknown'
        }"
      >
        {{ statusText }}
      </span>
      <span v-if="item.status !== 'running' && elapsedText" class="mono tr__ms">{{ elapsedText }}</span>
      <span class="tr__caret">{{ open ? '▲' : '▼' }}</span>
    </button>

    <div v-if="open" class="tr__panel mono">
      <div class="tr__sect">
        <div class="tr__sectt">参数</div>
        <pre class="tr__pre">{{ JSON.stringify(item.args, null, 2) }}</pre>
      </div>
      <div class="tr__sect">
        <div class="tr__sectt">
          {{ item.status === 'hard_link' ? '拒绝原因' : item.status === 'running' ? '结果（尚未返回）' : '结果' }}
          <span v-if="item.resultPreview && !pages.text" class="tr__pv">预览（原文过大，展开后按需加载）</span>
        </div>
        <div v-if="item.status === 'hard_link'" class="tr__hl">
          hard_link_impact_unknown — 文件存在多个硬链接，系统无法确定写入影响范围，已阻止操作。请先确认链接状态后重试。不能通过提权或切换 open 档位绕过。
        </div>
        <pre v-else-if="item.status === 'denied'" class="tr__pre tr__pre--err">{{ item.summary || '权限规则拒绝了本次调用' }}</pre>
        <template v-else-if="item.status === 'unknown'">
          <div v-if="item.expandedResult || item.resultFull" class="tr__unk-note">历史记录无结果元数据，成败未知：</div>
          <pre v-if="item.expandedResult || item.resultFull" class="tr__pre tr__scroll">{{ item.expandedResult || item.resultFull }}</pre>
          <div v-else class="tr__unk-note">历史记录未保存结果元数据，本次调用成败未知（不按成功显示）。</div>
        </template>
        <pre v-else class="tr__pre tr__scroll">{{ loading && !pages.text ? '读取完整结果…' : item.expandedResult || item.resultFull || item.summary || '（无结果文本）' }}</pre>
        <div v-if="pages.hasMore" class="tr__more">
          <span class="mono tr__progress">{{ progressText }}</span>
          <button class="tr__morebtn" :disabled="loading" @click="loadMore(false)">{{ loading ? '读取中…' : '加载更多' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tr {
  padding: 2px 0;
}
.tr__row {
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  width: 100%;
  border-radius: var(--radius-lg);
  padding: 8px 12px;
  transition: background-color 0.12s;
}
.tr__row:hover {
  background: var(--g50);
}
.tr__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.tr__dot--run {
  background: var(--blue400);
}
.tr__dot--ok {
  background: var(--green500);
}
.tr__dot--err {
  background: var(--red400);
}
.tr__dot--warn {
  background: var(--amber500);
}
.tr__dot--unknown {
  background: var(--g300);
}
.tr__name {
  font-size: 12px;
  color: var(--g600);
  font-weight: 500;
  flex-shrink: 0;
}
.tr__args {
  font-size: 12px;
  color: var(--g400);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tr__status {
  font-size: 12px;
  flex-shrink: 0;
}
.tr__status--run {
  color: var(--blue500);
}
.tr__status--ok {
  color: var(--green600);
}
.tr__status--err {
  color: var(--red500);
}
.tr__status--warn {
  color: var(--amber600);
}
.tr__status--unknown {
  color: var(--g400);
}
.tr__ms {
  font-size: 11px;
  color: var(--g300);
  flex-shrink: 0;
}
.tr__caret {
  color: var(--g300);
  font-size: 10px;
  flex-shrink: 0;
}
.tr__panel {
  margin: 4px 8px;
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--g200);
  background: var(--code-bg);
  font-size: 12px;
}
.tr__sect {
  padding: 12px 16px;
}
.tr__sect + .tr__sect {
  border-top: 1px solid var(--g200);
}
.tr__sectt {
  font-size: 10px;
  color: var(--g400);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: 8px;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 8px;
}
.tr__pv {
  font-size: 10px;
  color: var(--amber700);
  background: var(--amber50);
  border: 1px solid var(--amber100);
  border-radius: 999px;
  padding: 0 6px;
  text-transform: none;
  letter-spacing: 0;
}
.tr__pre {
  color: var(--g600);
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.6;
}
.tr__pre--err {
  color: var(--red500);
}
.tr__scroll {
  max-height: 168px;
  overflow-y: auto;
}
.tr__hl {
  color: var(--amber700);
  line-height: 1.6;
}
.tr__unk-note {
  color: var(--g500);
  font-size: 12px;
  line-height: 1.6;
  font-family: var(--font-sans);
}
.tr__more {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 8px;
}
.tr__progress {
  font-size: 11px;
  color: var(--g400);
}
.tr__morebtn {
  font-size: 11px;
  color: var(--blue600);
  border: 1px solid var(--blue200);
  border-radius: var(--radius-xs);
  padding: 2px 10px;
}
.tr__morebtn:hover:not(:disabled) {
  background: var(--blue50);
}
.tr__morebtn:disabled {
  opacity: 0.5;
}
</style>

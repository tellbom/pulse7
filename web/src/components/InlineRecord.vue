<script setup>
// 任务流中的轻量系统记录 — 每个契约事件在界面上有归宿（设计任务书第 5 节）。
// waiting / skill_catalog / skill / outside_write / compaction / process_warning / process / permission
import { ref, computed } from 'vue';
import { store, actions } from '../store/store.js';
import { formatBytes } from '../lib/format.js';
import CopyBtn from './CopyBtn.vue';

const props = defineProps({ item: { type: Object, required: true } });
const it = props.item;
const cpOpen = ref(false);

const kindText = computed(() => {
  const map = {
    background_count: '后台任务并发',
    background_duration: '任务时长',
    process_count: '进程数'
  };
  return map[it.kind] || it.kind;
});

const procText = computed(() => {
  const p = it.process || {};
  const name = (p.imagePath || p.command || '').split('\\').pop() || `pid ${p.pid}`;
  return `pid ${p.pid || '?'} · ${name}`;
});

const procAct = computed(() => {
  const map = { created: '创建', ended: '结束', terminated: '被终止' };
  return map[it.action] || it.action;
});

const formatCatalogKiB = (bytes) => {
  const value = Math.max(0, Number(bytes) || 0) / 1024;
  return `${Number.isInteger(value) ? value : value.toFixed(1)} KiB`;
};

const catalogModeText = computed(() => {
  const map = {
    full: '完整元数据',
    shortened: '描述已缩短',
    names: '描述已省略',
    index: '改为索引按需查阅',
    empty: '未发现技能'
  };
  return map[it.mode] || it.mode || '未知模式';
});
</script>

<template>
  <!-- 等待态：区分等首字 / 工具执行中 / 已中断，长得必须不一样 -->
  <div v-if="it.type === 'waiting'" class="ir-wait">
    <span class="ir-wait__dot pulse-dot" />
    <span class="ir-wait__t">等待模型响应 <span class="mono ir-wait__s">{{ it.seconds }}s</span></span>
    <span class="ir-wait__hint">内网端点慢是常态；这是心跳提示，不代表卡死</span>
  </div>

  <!-- skill_catalog：本次任务可发现的目录投影，不代表任何技能正文已读取 -->
  <div v-else-if="it.type === 'skill_catalog'" class="ir-catalog" :class="`ir-catalog--${it.mode}`">
    <div class="ir-catalog__row">
      <span class="ir-catalog__title">技能目录已构建</span>
      <span>{{ it.count }} 个技能</span>
      <span class="mono">{{ formatCatalogKiB(it.listingBytes) }} / {{ formatCatalogKiB(it.budgetBytes) }}</span>
      <span class="ir-catalog__mode">{{ catalogModeText }}</span>
    </div>
    <div v-if="it.indexPath" class="mono ir-catalog__index">完整元数据索引：{{ it.indexPath }}</div>
    <div v-for="warning in it.warnings" :key="warning" class="ir-catalog__warning">⚠ {{ warning }}</div>
  </div>

  <!-- skill_loaded -->
  <div v-else-if="it.type === 'skill'" class="ir-skill">
    skill 正文已读取 <span class="mono ir-skill__name">{{ it.name }}</span>
  </div>

  <!-- outside_workspace_write -->
  <div v-else-if="it.type === 'outside_write'" class="ir-ow">
    <span class="ir-ow__ico">⚠</span>
    <div class="ir-ow__main">
      <span class="ir-ow__t">工作区外写入</span>
      <span class="mono ir-ow__tool">[{{ it.tool }}]</span>
      <div class="mono ir-ow__path">{{ it.resolvedPath || it.requestedPath }}</div>
      <div class="ir-ow__note">不在 checkpoint 覆盖范围，无法回滚</div>
    </div>
    <CopyBtn :text="it.resolvedPath || it.requestedPath" />
  </div>

  <!-- compaction：压缩/截断留痕，可展开明细 -->
  <div v-else-if="it.type === 'compaction'" class="ir-cp">
    <button class="ir-cp__row" @click="cpOpen = !cpOpen">
      <span class="ir-cp__ico">◇</span>
      <span class="ir-cp__t">{{ it.method === 'micro' ? '本地微压缩' : it.method === 'truncate' ? '上下文截断' : '上下文压缩' }}{{ it.emergency ? '（紧急）' : '' }}</span>
      <span class="mono ir-cp__num">{{ it.beforeTokens?.toLocaleString() }} → {{ it.afterTokens?.toLocaleString() }} token</span>
      <span v-if="it.method === 'truncate' && it.discardedBytes" class="ir-cp__disc">丢弃 {{ formatBytes(it.discardedBytes) }}</span>
      <span class="ir-cp__caret">{{ cpOpen ? '▲' : '▼' }}</span>
    </button>
    <div v-if="cpOpen" class="ir-cp__detail">
      <div>{{ it.method === 'micro' ? '旧工具结果已外置保存（lc1 引用可按需取回），近期结果与调用结构保留在上下文中。' : '部分历史消息已从上下文移除；已外置内容可经引用取回，未外置部分不可恢复。' }}token 数为估算值（序列化字节 ÷4）。</div>
      <div v-if="it.summary" class="mono ir-cp__summary">{{ it.summary }}</div>
      <div v-if="it.discardedToolCallIds && it.discardedToolCallIds.length" class="ir-cp__ids">
        丢弃的工具调用：<span class="mono">{{ it.discardedToolCallIds.join('、') }}</span>
      </div>
    </div>
  </div>

  <!-- process_warning：琥珀告警，不拦截 -->
  <div v-else-if="it.type === 'process_warning'" class="ir-pw">
    <span class="ir-pw__dot" />
    <span class="ir-pw__t">{{ kindText }} <span class="mono">{{ it.current }}/{{ it.threshold }}</span> 已超阈值，告警不拦截</span>
  </div>

  <!-- process：进程创建/结束/终止轻记录 -->
  <div v-else-if="it.type === 'process'" class="ir-proc mono">
    <span class="ir-proc__act">{{ procAct }}</span> {{ procText }}
  </div>

  <!-- permission：等待确认的权限请求卡（请求与响应均留痕） -->
  <div v-else-if="it.type === 'permission'" class="ir-perm" :class="{ 'ir-perm--done': it.decision }">
    <div class="ir-perm__head">
      <span class="ir-perm__t">权限确认</span>
      <span class="mono ir-perm__tool">{{ it.tool }}</span>
      <span class="mono ir-perm__target">{{ it.target }}</span>
    </div>
    <pre class="mono ir-perm__args">{{ JSON.stringify(it.args, null, 2) }}</pre>
    <div v-if="!it.decision" class="ir-perm__acts">
      <button class="ir-perm__allow" @click="actions.confirmPermission('allow')">允许</button>
      <button class="ir-perm__deny" @click="actions.confirmPermission('deny')">拒绝 / 取消</button>
    </div>
    <div v-else class="ir-perm__done">已{{ it.decision === 'allow' ? '允许' : '拒绝' }}，结果已入审计</div>
  </div>
</template>

<style scoped>
/* waiting */
.ir-wait {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  font-size: 13px;
}
.ir-wait__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--amber400);
  flex-shrink: 0;
}
.ir-wait__t {
  color: var(--g400);
}
.ir-wait__s {
  color: var(--g300);
  margin-left: 4px;
}
.ir-wait__hint {
  font-size: 11px;
  color: var(--g300);
}

/* skill catalog / skill_loaded */
.ir-catalog {
  margin: 4px 12px;
  padding: 7px 10px;
  border: 1px solid var(--blue100);
  background: var(--blue50);
  border-radius: var(--radius-lg);
  color: var(--g500);
  font-size: 11px;
}
.ir-catalog--shortened,
.ir-catalog--names,
.ir-catalog--index {
  border-color: var(--amber200);
  background: var(--amber50);
}
.ir-catalog__row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.ir-catalog__title {
  color: var(--g700);
  font-weight: 600;
}
.ir-catalog__mode {
  color: var(--blue700);
}
.ir-catalog--shortened .ir-catalog__mode,
.ir-catalog--names .ir-catalog__mode,
.ir-catalog--index .ir-catalog__mode,
.ir-catalog__warning {
  color: var(--amber700);
}
.ir-catalog__index,
.ir-catalog__warning {
  margin-top: 4px;
  word-break: break-all;
}
.ir-skill {
  padding: 6px 20px;
  font-size: 12px;
  color: var(--g400);
}
.ir-skill__name {
  color: var(--g500);
  background: var(--g100);
  padding: 1px 6px;
  border-radius: var(--radius-xs);
}

/* outside write */
.ir-ow {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 12px;
  margin: 4px 8px;
  border-radius: var(--radius-lg);
  background: var(--amber50);
  border: 1px solid var(--amber100);
  font-size: 13px;
}
.ir-ow__ico {
  color: var(--amber500);
  margin-top: 2px;
  flex-shrink: 0;
}
.ir-ow__main {
  flex: 1;
  min-width: 0;
}
.ir-ow__t {
  color: var(--amber700);
  font-weight: 500;
}
.ir-ow__tool {
  color: var(--g500);
  font-size: 11px;
  margin-left: 8px;
}
.ir-ow__path {
  color: var(--g600);
  font-size: 12px;
  margin-top: 2px;
  word-break: break-all;
}
.ir-ow__note {
  font-size: 11px;
  color: var(--g400);
  margin-top: 2px;
}

/* compaction */
.ir-cp {
  padding: 4px 8px;
}
.ir-cp__row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--g400);
  width: 100%;
  text-align: left;
  padding: 6px 8px;
  border-radius: var(--radius-lg);
  transition: color 0.12s, background-color 0.12s;
}
.ir-cp__row:hover {
  color: var(--g600);
  background: var(--g50);
}
.ir-cp__ico {
  font-size: 11px;
}
.ir-cp__num {
  color: var(--g300);
  font-size: 11px;
}
.ir-cp__disc {
  color: var(--amber500);
  font-size: 12px;
}
.ir-cp__caret {
  margin-left: auto;
  font-size: 10px;
  color: var(--g300);
}
.ir-cp__detail {
  font-size: 12px;
  color: var(--g400);
  padding: 8px 16px;
  background: var(--g50);
  border-radius: var(--radius-lg);
  margin: 4px 8px 0;
  border: 1px solid var(--g100);
  line-height: 1.6;
}
.ir-cp__summary {
  margin-top: 4px;
  color: var(--g500);
}
.ir-cp__ids {
  margin-top: 4px;
}

/* process warning */
.ir-pw {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 20px;
  font-size: 13px;
}
.ir-pw__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--amber400);
  flex-shrink: 0;
}
.ir-pw__t {
  color: var(--amber700);
}

/* process */
.ir-proc {
  padding: 3px 20px;
  font-size: 11px;
  color: var(--g300);
}
.ir-proc__act {
  color: var(--g400);
  margin-right: 4px;
}

/* permission */
.ir-perm {
  margin: 6px 12px;
  border: 1px solid var(--blue200);
  background: var(--blue50);
  border-radius: var(--radius-lg);
  padding: 12px 16px;
}
.ir-perm__head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}
.ir-perm__t {
  font-size: 13px;
  font-weight: 600;
  color: var(--g800);
}
.ir-perm__tool {
  font-size: 12px;
  color: var(--g600);
}
.ir-perm__target {
  font-size: 12px;
  color: var(--g500);
  word-break: break-all;
}
.ir-perm__args {
  font-size: 12px;
  color: var(--g600);
  margin: 8px 0;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 160px;
  overflow-y: auto;
}
.ir-perm__acts {
  display: flex;
  gap: 8px;
}
.ir-perm__allow {
  border: 1px solid var(--green200);
  color: var(--green700);
  background: #fff;
  border-radius: var(--radius-sm);
  padding: 5px 14px;
  font-size: 13px;
}
.ir-perm__allow:hover {
  background: var(--green50);
}
.ir-perm__deny {
  border: 1px solid var(--red200);
  color: var(--red500);
  background: #fff;
  border-radius: var(--radius-sm);
  padding: 5px 14px;
  font-size: 13px;
}
.ir-perm__deny:hover {
  background: var(--red50);
}
.ir-perm__done {
  font-size: 12px;
  color: var(--g500);
}
.ir-perm--done {
  border-color: var(--g200);
  background: var(--g50);
}
</style>

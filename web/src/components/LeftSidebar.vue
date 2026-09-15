<script setup>
// 左侧栏（252px，可折叠至 52px）：新建/搜索/工具入口 + 历史会话（按日期分组）+ 检查点常驻区。
import { ref, computed } from 'vue';
import { store, actions } from '../store/store.js';
import { dateGroup } from '../lib/format.js';
import OrbAvatar from './OrbAvatar.vue';

const cpOpen = ref(true);

const groups = computed(() => {
  const buckets = { today: [], yesterday: [], earlier: [] };
  store.sessions.forEach((s) => buckets[dateGroup(s.updatedAt)].push(s));
  return [
    { key: 'today', label: '今天', items: buckets.today },
    { key: 'yesterday', label: '昨天', items: buckets.yesterday },
    { key: 'earlier', label: '更早', items: buckets.earlier }
  ].filter((g) => g.items.length);
});

const maxSeq = computed(() => {
  const seqs = store.checkpoints.map((c) => c.seq);
  return seqs.length ? Math.max(...seqs) : null;
});

const wsName = computed(() => {
  const w = store.workspace || '';
  return w.split('\\').pop() || w;
});

function select(s) {
  if (s.sessionId === store.sessionId && !store.emptyMode) return;
  actions.resumeSession(s.sessionId);
}
</script>

<template>
  <!-- 折叠态 -->
  <div v-if="store.leftCollapsed" class="sb-mini">
    <OrbAvatar :size="32" />
    <button class="sb-mini__b" title="展开侧栏" @click="store.leftCollapsed = false">▶</button>
    <button class="sb-mini__b" title="新建会话" @click="actions.newSession()">✎</button>
  </div>

  <!-- 展开态 -->
  <div v-else class="sb">
    <div class="sb-top">
      <div class="sb-top__row">
        <span class="sb-top__label">会话</span>
        <button class="sb-top__fold" title="折叠侧栏" @click="store.leftCollapsed = true">⊟</button>
      </div>
      <button class="sb-top__new" @click="actions.newSession()">
        <span class="sb-top__ico">✎</span>
        新建会话
      </button>
      <button class="sb-top__item" @click="store.settingsOpen = true">
        <span class="sb-top__ico">⚙</span>
        工具与集成
      </button>
    </div>

    <div class="sb-list">
      <div class="sb-list__head">
        <span class="sb-list__t">会话</span>
        <button class="sb-list__refresh" title="刷新会话列表" aria-label="刷新会话列表" @click="actions.refreshSessions()">
          <svg viewBox="0 0 14 14" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.4">
            <path d="M12 7a5 5 0 1 1-1.5-3.5M12 1v3h-3" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
      </div>
      <!-- 坏会话逐项警告：健康会话仍可选，不掩盖也不放大成整表失败 -->
      <div v-if="store.sessionErrors.length" class="sb-errs">
        <div class="sb-errs__t">{{ store.sessionErrors.length }} 个会话文件读取失败（已跳过）</div>
        <div v-for="(e, i) in store.sessionErrors" :key="i" class="sb-errs__item mono" :title="e.message">
          {{ e.fileName || e.sessionId }} · {{ e.code }}（{{ e.stage }}）
        </div>
      </div>
      <div v-for="g in groups" :key="g.key" class="sb-group">
        <div class="sb-group__t">{{ g.label }}</div>
        <button
          v-for="s in g.items"
          :key="s.sessionId"
          class="sb-item"
          :class="{ 'sb-item--cur': s.sessionId === store.sessionId && !store.emptyMode }"
          @click="select(s)"
        >
          <span class="sb-item__txt">{{ s.firstUser || s.sessionId }}</span>
          <span v-if="s.sessionId === store.sessionId && !store.emptyMode" class="sb-item__cur">当前</span>
        </button>
      </div>
      <div v-if="!groups.length" class="sb-empty">暂无历史会话</div>
    </div>

    <div class="sb-cp">
      <button class="sb-cp__head" @click="cpOpen = !cpOpen">
        <span class="sb-cp__t">检查点 · {{ wsName }}</span>
        <span class="sb-cp__caret">{{ cpOpen ? '▲' : '▼' }}</span>
      </button>
      <div v-if="cpOpen" class="sb-cp__body">
        <div v-for="cp in store.checkpoints" :key="cp.seq" class="sb-cprow">
          <span class="mono sb-cprow__seq">#{{ cp.seq }}</span>
          <span class="mono sb-cprow__time">{{ String(cp.createdAt || '').slice(0, 5) }}</span>
          <span class="sb-cprow__kind" :class="cp.kind === 'auto' ? 'sb-cprow__kind--auto' : 'sb-cprow__kind--model'">
            {{ cp.kind === 'auto' ? 'auto' : 'model' }}
          </span>
          <span class="mono sb-cprow__commit">{{ cp.commit }}</span>
          <span v-if="cp.dirtyFiles !== null && cp.dirtyFiles > 0" class="mono sb-cprow__dirty">±{{ cp.dirtyFiles }}</span>
          <span class="sb-cprow__sp" />
          <span v-if="cp.seq === maxSeq" class="sb-cprow__now">当前</span>
          <button class="sb-cprow__rb" @click="store.rollbackOpen = true">回滚</button>
        </div>
        <div v-if="!store.checkpoints.length" class="sb-cp__none">当前会话暂无检查点</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sb-mini {
  width: 52px;
  flex-shrink: 0;
  border-right: 1px solid var(--g200);
  background: #fff;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  gap: 12px;
}
.sb-mini__b {
  color: var(--bar-fg-dim);
  padding: 6px;
  border-radius: var(--radius-lg);
  font-size: 13px;
  transition: color 0.12s, background-color 0.12s;
}
.sb-mini__b:hover {
  color: var(--bar-fg);
  background: var(--g100);
}

.sb-list__refresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: var(--radius-xs);
  color: var(--bar-fg-dim);
  transition: color 0.12s, background-color 0.12s;
}
.sb-list__refresh:hover {
  color: var(--accent);
  background: var(--blue50);
}

.sb {
  width: 252px;
  flex-shrink: 0;
  border-right: 1px solid var(--bar-line);
  /* 侧栏与顶栏同色 #F8F8F7 */
  background: var(--side-bg);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.sb-top {
  padding: 8px 12px 12px;
  flex-shrink: 0;
}
.sb-top__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.sb-top__label {
  font-size: 11px;
  color: var(--bar-fg-dim);
  padding: 0 12px;
}
.sb-top__fold {
  color: var(--bar-fg-dim);
  padding: 6px;
  border-radius: var(--radius-lg);
  font-size: 12px;
}
.sb-top__fold:hover {
  color: var(--bar-fg);
  background: var(--g100);
}
.sb-top__new {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 12px;
  border-radius: var(--radius-lg);
  background: var(--g50);
  font-size: 14px;
  color: var(--bar-fg);
  font-weight: 500;
  transition: background-color 0.12s;
}
.sb-top__new:hover {
  background: var(--g100);
}
.sb-top__item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 12px;
  border-radius: var(--radius-lg);
  font-size: 14px;
  color: var(--bar-fg-dim);
  transition: background-color 0.12s, color 0.12s;
}
.sb-top__item:hover {
  background: var(--g50);
  color: var(--g800);
}
.sb-top__ico {
  color: var(--bar-fg-dim);
  font-size: 14px;
}

.sb-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 12px;
}
.sb-list__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
}
.sb-list__t {
  font-size: 11px;
  color: var(--bar-fg-dim);
  font-weight: 500;
}
.sb-list__h {
  font-size: 10px;
  color: var(--bar-fg-dim);
}
.sb-group {
  margin-bottom: 4px;
}
.sb-group__t {
  font-size: 11px;
  color: var(--bar-fg-dim);
  padding: 4px 12px;
  font-weight: 500;
}
.sb-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 12px;
  border-radius: var(--radius-lg);
  font-size: 14px;
  color: var(--bar-fg-dim);
  text-align: left;
  transition: background-color 0.12s, color 0.12s;
}
.sb-item:hover {
  background: var(--g50);
  color: var(--g900);
}
.sb-item--cur {
  background: transparent;
  color: var(--ink);
  font-weight: 600;
  box-shadow: inset 3px 0 0 #17171c;
}
.sb-item__txt {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-right: 6px;
  line-height: 1.35;
}
.sb-item__cur {
  font-size: 10px;
  color: #ffffff;
  background: #17171c;
  border-radius: 999px;
  padding: 1px 7px;
  font-weight: 600;
  font-weight: 500;
  flex-shrink: 0;
}
.sb-empty {
  font-size: 12px;
  color: var(--bar-fg-dim);
  padding: 12px;
}
.sb-errs {
  margin: 0 4px 8px;
  border: 1px solid var(--amber200);
  background: var(--amber50);
  border-radius: var(--radius-md);
  padding: 8px 10px;
}
.sb-errs__t {
  font-size: 12px;
  color: var(--amber400);
  margin-bottom: 4px;
}
.sb-errs__item {
  font-size: 11px;
  color: var(--bar-fg-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.7;
}

.sb-cp {
  border-top: 1px solid var(--g100);
  flex-shrink: 0;
}
.sb-cp__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 10px 16px;
}
.sb-cp__head:hover {
  background: var(--g50);
}
.sb-cp__t {
  font-size: 12px;
  color: var(--bar-fg-dim);
  font-weight: 500;
}
.sb-cp__caret {
  font-size: 10px;
  color: var(--bar-fg-dim);
}
.sb-cp__body {
  padding: 0 8px 8px;
}
.sb-cprow {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: var(--radius-lg);
  font-size: 12px;
}
.sb-cprow:hover {
  background: var(--g50);
}
.sb-cprow__seq {
  color: var(--bar-fg-dim);
}
.sb-cprow__time {
  color: var(--bar-fg-dim);
}
.sb-cprow__kind {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 999px;
  flex-shrink: 0;
}
.sb-cprow__kind--auto {
  background: var(--g100);
  color: var(--bar-fg-dim);
}
.sb-cprow__kind--model {
  background: var(--blue50);
  color: var(--blue600);
}
.sb-cprow__commit {
  color: var(--blue600);
}
.sb-cprow__dirty {
  color: var(--amber400);
}
.sb-cprow__sp {
  flex: 1;
}
.sb-cprow__now {
  font-size: 10px;
  color: var(--bar-fg-dim);
}
.sb-cprow__rb {
  display: none;
  font-size: 11px;
  color: var(--blue500);
  border: 1px solid var(--blue200);
  border-radius: 999px;
  padding: 1px 8px;
}
.sb-cprow:hover .sb-cprow__rb {
  display: inline-block;
}
.sb-cprow__rb:hover {
  color: var(--blue700);
}
.sb-cp__none {
  font-size: 12px;
  color: var(--bar-fg-dim);
  padding: 8px;
}
</style>

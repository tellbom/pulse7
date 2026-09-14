<script setup>
// 顶部栏（40px）：品牌区｜工作区切换｜连接状态 + 权限档位 + 向导/设置。
import { ref, computed } from 'vue';
import { store, getters, actions } from '../store/store.js';
import OrbAvatar from './OrbAvatar.vue';

const wsOpen = ref(false);
const wsInputOpen = ref(false);
const wsDraft = ref('');

const recentWorkspaces = computed(() => {
  const set = new Set(store.sessions.map((s) => s.cwd).filter(Boolean));
  set.delete(store.workspace);
  return [...set].slice(0, 5);
});

function pickWorkspace(w) {
  wsOpen.value = false;
  actions.switchWorkspace(w);
}
function applyManual() {
  const v = wsDraft.value.trim();
  wsInputOpen.value = false;
  wsOpen.value = false;
  wsDraft.value = '';
  if (v) actions.switchWorkspace(v);
}
const dirtyCount = computed(() => {
  const cp = store.checkpoints[0];
  return cp && cp.dirtyFiles != null ? cp.dirtyFiles : null;
});
</script>

<template>
  <header class="tb">
    <div class="tb-brand" :style="{ width: store.leftCollapsed ? '52px' : '252px' }">
      <OrbAvatar :size="20" class="tb-brand__orb" />
      <template v-if="!store.leftCollapsed">
        <span class="tb-brand__name">pulse7 Agent</span>
        <span class="mono tb-brand__ver">· {{ store.version }}</span>
      </template>
    </div>

    <div class="tb-ws">
      <div class="tb-ws__wrap">
        <button class="tb-ws__btn mono" @click="wsOpen = !wsOpen">
          <span class="tb-ws__path">{{ store.workspace || '（未选择工作区）' }}</span>
          <span v-if="dirtyCount !== null && dirtyCount > 0" class="tb-ws__dirty">● {{ dirtyCount }}</span>
          <span class="tb-ws__caret">▾</span>
        </button>
        <template v-if="wsOpen">
          <div class="tb-overlay" @click="wsOpen = false" />
          <div class="tb-ws__menu">
            <div class="tb-ws__label">当前</div>
            <div class="tb-ws__item mono tb-ws__item--cur">{{ store.workspace }}</div>
            <template v-if="recentWorkspaces.length">
              <div class="tb-ws__label">最近（来自历史会话）</div>
              <button v-for="w in recentWorkspaces" :key="w" class="tb-ws__item mono" @click="pickWorkspace(w)">
                <span class="tb-ws__dot" />{{ w }}
              </button>
            </template>
            <div class="tb-ws__label">其他路径</div>
            <div v-if="!wsInputOpen" class="tb-ws__item tb-ws__item--act" @click="wsInputOpen = true">输入其他路径…</div>
            <div v-else class="tb-ws__manual">
              <input
                v-model="wsDraft"
                class="tb-ws__input mono"
                placeholder="E:\projects\my-project"
                @keyup.enter="applyManual"
                @keyup.esc="wsInputOpen = false"
              />
              <button class="tb-ws__go" @click="applyManual">切换</button>
            </div>
            <div class="tb-ws__hint">活动轮或有后台任务运行时切换会被拒绝</div>
          </div>
        </template>
      </div>
    </div>

    <div class="tb-right">
      <div class="tb-conn mono">
        <span class="tb-conn__dot" :class="store.connected ? 'tb-conn__dot--on' : 'tb-conn__dot--off'" />
        <span :class="store.connected ? 'tb-conn__ok' : 'tb-conn__bad'">
          {{ store.connected ? `${store.listener.address}${store.listener.port ? ':' + store.listener.port : ''} · 已连接` : '事件流已断开' }}
        </span>
      </div>
      <div class="tb-sep" />
      <div class="tb-perm mono" role="group" aria-label="权限档位">
        <button
          v-for="t in [['strict', '严格'], ['standard', '标准'], ['open', '开放']]"
          :key="t[0]"
          class="tb-perm__b"
          :class="{ 'tb-perm__b--on': store.permissions.profile === t[0] }"
          :title="t[0]"
          @click="actions.setProfile(t[0])"
        >
          {{ t[1] }}
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.tb {
  display: flex;
  align-items: center;
  height: 40px;
  border-bottom: 1px solid var(--bar-line);
  background: var(--bar-bg);
  flex-shrink: 0;
}
.tb-brand {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-right: 1px solid var(--bar-line);
  height: 100%;
  flex-shrink: 0;
  overflow: hidden;
  white-space: nowrap;
  transition: width 0.15s ease-out;
}
.tb-brand__name {
  font-weight: 600;
  font-size: 15px;
  letter-spacing: 0.4px;
  color: var(--bar-fg);
}
.tb-brand__ver {
  font-size: 11px;
  color: var(--bar-fg-dim);
  margin-left: 2px;
}
.tb-ws {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  padding: 0 16px;
  height: 100%;
}
.tb-ws__wrap {
  position: relative;
}
.tb-ws__btn {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--bar-fg);
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  max-width: 420px;
  transition: background-color 0.12s;
}
.tb-ws__btn:hover {
  background: rgba(0, 0, 0, 0.05);
  color: var(--bar-fg);
}
.tb-ws__path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tb-ws__dirty {
  color: var(--amber400);
  font-size: 11px;
  flex-shrink: 0;
}
.tb-ws__caret {
  color: var(--bar-fg-dim);
  font-size: 10px;
  flex-shrink: 0;
}
.tb-overlay {
  position: fixed;
  inset: 0;
  z-index: 20;
}
.tb-ws__menu {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  width: 380px;
  background: #fff;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  z-index: 21;
  padding: 8px 0;
}
.tb-ws__label {
  font-size: 11px;
  color: var(--g400);
  padding: 8px 16px 4px;
}
.tb-ws__item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  font-size: 12px;
  color: var(--g500);
  padding: 9px 16px;
  overflow: hidden;
}
.tb-ws__item:hover {
  background: var(--g50);
  color: var(--g900);
}
.tb-ws__item--cur {
  color: var(--g900);
  font-weight: 600;
}
.tb-ws__item--act {
  color: var(--blue600);
}
.tb-ws__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.tb-ws__manual {
  display: flex;
  gap: 6px;
  padding: 4px 16px 6px;
}
.tb-ws__input {
  flex: 1;
  border: 1px solid var(--g200);
  border-radius: var(--radius-sm);
  padding: 5px 8px;
  font-size: 12px;
  outline: none;
}
.tb-ws__input:focus {
  border-color: var(--blue400);
}
.tb-ws__go {
  border: 1px solid var(--g200);
  border-radius: var(--radius-sm);
  padding: 5px 10px;
  font-size: 12px;
  color: var(--g600);
}
.tb-ws__go:hover {
  background: var(--g50);
}
.tb-ws__hint {
  font-size: 11px;
  color: var(--g300);
  padding: 6px 16px 4px;
}
.tb-right {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  height: 100%;
  flex-shrink: 0;
}
.tb-conn {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
}
.tb-conn__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.tb-conn__dot--on {
  background: var(--green400);
}
.tb-conn__dot--off {
  background: var(--red400);
}
.tb-conn__ok {
  color: var(--bar-fg-dim);
}
.tb-conn__bad {
  color: var(--red400);
}
.tb-sep {
  width: 1px;
  height: 16px;
  background: var(--bar-line);
  flex-shrink: 0;
}
.tb-perm {
  display: flex;
  border: 1px solid var(--bar-line);
  border-radius: var(--radius-xs);
  overflow: hidden;
  font-size: 11px;
  flex-shrink: 0;
}
.tb-perm__b {
  padding: 4px 10px;
  color: var(--bar-fg-dim);
  transition: background-color 0.12s, color 0.12s;
}
.tb-perm__b + .tb-perm__b {
  border-left: 1px solid var(--bar-line);
}
.tb-perm__b:hover {
  background: rgba(0, 0, 0, 0.05);
  color: var(--bar-fg);
}
.tb-perm__b--on {
  background: var(--ink);
  color: #ffffff;
  font-weight: 500;
}
.tb-chip {
  font-size: 11px;
  color: var(--g500);
  border: 1px solid var(--g200);
  border-radius: var(--radius-sm);
  padding: 4px 10px;
  flex-shrink: 0;
  transition: color 0.12s, border-color 0.12s;
}
.tb-chip:hover {
  color: var(--g900);
  border-color: var(--g300);
}
</style>

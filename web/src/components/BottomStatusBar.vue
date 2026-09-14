<script setup>
// 底部状态栏（28px 常驻）：后台任务入口（含告警态）+ 上下文用量（估算）。
import { store, getters } from '../store/store.js';

function taskText() {
  const n = getters.runningTasks.length;
  if (store.overview.exceeded) return `进程 ${store.overview.count}/${store.overview.threshold}`;
  return n > 0 ? String(n) : '';
}
</script>

<template>
  <div class="bs">
    <button
      class="bs__tasks mono"
      :class="{ 'bs__tasks--warn': store.overview.exceeded, 'bs__tasks--some': !store.overview.exceeded && getters.runningTasks.length > 0 }"
      @click="store.drawerOpen = !store.drawerOpen"
      :title="store.overview.exceeded ? '进程数超阈值（告警不拦截）' : '后台任务与进程概况'"
    >
      <span>{{ store.overview.exceeded ? '⚠' : '⚙' }}</span>
      <span v-if="getters.runningTasks.length" class="bs__count">{{ getters.runningTasks.length }}</span>
      <span v-if="store.overview.exceeded" class="bs__warn">{{ taskText() }}</span>
    </button>
    <div class="bs__sp" />
    <div class="bs__ctx">
      <div class="bs__track">
        <div
          class="bs__bar"
          :class="{ 'bs__bar--n': getters.contextLevel === 'normal', 'bs__bar--w': getters.contextLevel === 'warning', 'bs__bar--c': getters.contextLevel === 'critical' }"
          :style="{ width: getters.contextPct + '%' }"
        />
      </div>
      <span class="mono tabular-nums bs__pct" :class="'bs__pct--' + getters.contextLevel">{{ getters.contextPct }}%</span>
    </div>
  </div>
</template>

<style scoped>
.bs {
  height: 28px;
  border-top: 1px solid var(--paper-deep);
  background: var(--paper-deep);
  color: var(--ink);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 16px;
  flex-shrink: 0;
}
.bs__tasks {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #55554d;
  transition: color 0.12s;
}
.bs__tasks:hover {
  color: var(--ink);
}
.bs__tasks--some {
  color: #3d3d36;
}
.bs__tasks--warn {
  color: var(--amber600);
}
.bs__count {
  font-size: 10px;
}
.bs__warn {
  font-size: 10px;
  margin-left: 2px;
}
.bs__sp {
  flex: 1;
}
.bs__ctx {
  display: flex;
  align-items: center;
  gap: 8px;
}
.bs__track {
  width: 64px;
  height: 2px;
  background: var(--g100);
  border-radius: 999px;
  overflow: hidden;
}
.bs__bar {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s;
}
.bs__bar--n {
  background: var(--g300);
}
.bs__bar--w {
  background: var(--amber400);
}
.bs__bar--c {
  background: var(--red400);
}
.bs__pct {
  font-size: 10px;
}
.bs__pct--normal {
  color: #55554d;
}
.bs__pct--warning {
  color: var(--amber500);
}
.bs__pct--critical {
  color: var(--red500);
}
</style>

<script setup>
// 后台任务抽屉（屏 G + 常驻区 2/3）：任务卡片（拉输出/终止）+ 进程概况（含受限模式）+ 遗留 detached（未经核实）。
import { store, getters, actions } from '../store/store.js';
import { formatRuntime, formatBytes } from '../lib/format.js';

function output(taskId) {
  const st = store.taskOutputs[taskId];
  return st ? st.text : '';
}
function outputServed(taskId) {
  return store.taskOutputs[taskId] ? store.taskOutputs[taskId].text.length : 0;
}
function esc() {
  store.drawerOpen = false;
}
</script>

<template>
  <div>
    <div class="dw-mask" @click="esc" />
    <div class="dw">
      <div class="dw__head">
        <span class="dw__t">后台进程</span>
        <button class="dw__close" aria-label="关闭" @click="esc">✕</button>
      </div>
      <div class="dw__grid">
        <div>
          <div class="dw__st">
            后台任务
            <template v-if="getters.runningTasks.length">（{{ getters.runningTasks.length }} 运行中）</template>
          </div>
          <div class="dw__tasks">
            <div v-for="t in store.tasks" :key="t.taskId" class="dw-card">
              <div class="dw-card__top">
                <div class="dw-card__idr">
                  <span
                    class="dw-card__dot"
                    :class="{ 'dw-card__dot--run pulse-dot': t.status === 'running' || t.status === 'output_truncated', 'dw-card__dot--ok': t.status === 'exited', 'dw-card__dot--err': t.status === 'killed' || t.status === 'failed' }"
                  />
                  <span class="mono dw-card__id">{{ t.taskId }}</span>
                  <span v-if="t.detached" class="dw-card__det">detached · 将在 pulse7 退出后继续运行</span>
                </div>
                <span class="mono dw-card__rt">{{ formatRuntime(t.runtimeMs || 0) }}</span>
              </div>
              <div class="mono dw-card__cmd">{{ t.command }}</div>
              <div class="dw-card__meta">
                <span class="mono dw-card__bytes">{{ formatBytes(t.outputBytes || 0) }}</span>
                <span v-if="t.outputTruncated" class="dw-card__trunc">输出截断 · 进程仍在运行（不是结束）</span>
                <span v-if="t.exitCode !== null && t.exitCode !== undefined" class="mono dw-card__exit">exit {{ t.exitCode }}</span>
                <span v-if="t.outputError" class="dw-card__trunc">{{ t.outputError }}</span>
              </div>
              <div class="dw-card__acts">
                <button class="dw-card__btn dw-card__btn--pull" @click="actions.pullTaskOutput(t.taskId, true)">拉取输出</button>
                <button
                  v-if="t.status === 'running' || t.status === 'output_truncated'"
                  class="dw-card__btn dw-card__btn--kill"
                  @click="actions.killTask(t.taskId)"
                >
                  终止
                </button>
              </div>
              <pre v-if="output(t.taskId)" class="mono dw-card__out">{{ output(t.taskId) }}</pre>
              <div v-else-if="outputServed(t.taskId) === 0 && t.outputBytes > 0" class="dw-card__outhint">
                已存储 {{ formatBytes(t.outputBytes) }}，点「拉取输出」读取
              </div>
            </div>
            <div v-if="!store.tasks.length" class="dw__none">无后台任务</div>
          </div>
        </div>

        <div class="dw-side">
          <div>
            <div class="dw__st">进程概况</div>
            <div class="dw-kv">
              <div class="dw-kv__row">
                <span class="dw-kv__k">会话进程数</span>
                <span v-if="store.overview.error" class="dw-kv__err">计数失败（不用 0 冒充）</span>
                <span v-else class="mono dw-kv__v" :class="{ 'dw-kv__v--warn': store.overview.exceeded }">
                  {{ store.overview.count !== null && store.overview.count !== undefined ? `${store.overview.count} / ${store.overview.threshold}` : '未知' }}
                </span>
              </div>
              <div class="dw-kv__row">
                <span class="dw-kv__k">清理模式</span>
                <span class="mono dw-kv__v" :class="{ 'dw-kv__v--warn': store.overview.mode && store.overview.mode.restricted }">
                  {{ store.overview.mode ? (store.overview.mode.mode || '未知') + (store.overview.mode.restricted ? '（受限）' : '') : '未知' }}
                </span>
              </div>
              <div class="dw-kv__note">
                {{
                  store.overview.mode && store.overview.mode.restricted
                    ? 'Session Job 分配失败，退出时进程清理无法保证；仅 Job 成员受确定性清理，第三方后代可自行脱离'
                    : '会话 Job 内成员在 pulse7 退出时确定性收割；第三方后代允许脱离，不视为异常'
                }}
              </div>
            </div>
          </div>

          <div class="dw-side__sec">
            <div class="dw__st">上次遗留（未核实）</div>
            <div v-for="(d, i) in store.detachedHistory" :key="i" class="mono dw-side__det">{{ d.notice }}</div>
            <div v-if="!store.detachedHistory.length" class="dw-side__det dw-side__det--none">无遗留记录</div>
          </div>

          <div class="dw-side__sec">
            <div class="dw__st">登记进程（非完整 OS 进程表）</div>
            <div v-for="(p, i) in store.processes" :key="i" class="mono dw-side__proc">
              pid {{ p.pid }} · {{ (p.imagePath || '').split('\\').pop() || p.command }}
            </div>
            <div v-if="!store.processes.length" class="dw-side__det dw-side__det--none">当前无登记进程</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dw-mask {
  position: fixed;
  inset: 0;
  z-index: 20;
}
.dw {
  position: fixed;
  top: 52px;
  right: 16px;
  width: 480px;
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 90px);
  z-index: 30;
  background: #fff;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.16);
  overflow-y: auto;
  animation: dw-drop 0.14s ease-out;
}
@keyframes dw-drop {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.dw__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 24px;
  border-bottom: 1px solid var(--g100);
  position: sticky;
  top: 0;
  background: #fff;
  z-index: 1;
}
.dw__t {
  font-size: 13px;
  font-weight: 600;
  color: var(--g700);
}
.dw__close {
  color: var(--g400);
}
.dw__close:hover {
  color: var(--g700);
}
.dw__grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
  padding: 12px 20px 16px;
}
.dw__st {
  font-size: 11px;
  color: var(--g400);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 500;
  margin-bottom: 16px;
}
.dw__tasks {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.dw__none {
  font-size: 13px;
  color: var(--g300);
}
/* 任务卡：Manus 浅色主题（白卡细边，运行态 accent） */
.dw-card {
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  padding: 14px 16px;
  background: #ffffff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}
.dw-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.dw-card__idr {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.dw-card__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dw-card__dot--run {
  background: var(--accent);
}
.dw-card__dot--ok {
  background: var(--green400);
}
.dw-card__dot--err {
  background: var(--red400);
}
.dw-card__id {
  font-size: 12px;
  color: var(--blue600);
  font-weight: 500;
}
.dw-card__det {
  font-size: 11px;
  color: var(--g400);
}
.dw-card__rt {
  font-size: 12px;
  color: var(--g400);
}
.dw-card__cmd {
  font-size: 13px;
  color: var(--ink);
  word-break: break-all;
  margin-bottom: 6px;
}
.dw-card__meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  flex-wrap: wrap;
}
.dw-card__bytes {
  color: var(--g400);
}
.dw-card__trunc {
  color: var(--amber600);
}
.dw-card__exit {
  color: var(--g500);
}
.dw-card__acts {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}
.dw-card__btn {
  font-size: 12px;
  border-radius: var(--radius-sm);
  padding: 4px 10px;
  transition: border-color 0.12s, color 0.12s;
}
.dw-card__btn--pull {
  color: var(--blue500);
  border: 1px solid var(--blue100);
}
.dw-card__btn--pull:hover {
  color: var(--blue700);
  border-color: var(--blue200);
}
.dw-card__btn--kill {
  color: var(--red400);
  border: 1px solid var(--red100);
}
.dw-card__btn--kill:hover {
  color: var(--red600);
  border-color: var(--red200);
}
.dw-card__out {
  margin-top: 10px;
  font-size: 12px;
  color: var(--g600);
  background: var(--code-bg);
  border: 1px solid var(--g100);
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  max-height: 180px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
.dw-card__outhint {
  margin-top: 10px;
  font-size: 11px;
  color: var(--g400);
}
.dw-side {
  border-top: 1px solid var(--g100);
  padding-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.dw-side__sec {
  padding-top: 8px;
}
.dw-kv__row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  margin-bottom: 10px;
}
.dw-kv__k {
  color: var(--g500);
}
.dw-kv__v {
  color: var(--g700);
}
.dw-kv__v--warn {
  color: var(--amber600);
  font-weight: 600;
}
.dw-kv__err {
  color: var(--red400);
}
.dw-kv__note {
  font-size: 12px;
  color: var(--g400);
  line-height: 1.6;
}
.dw-side__det {
  font-size: 12px;
  color: var(--g400);
  line-height: 1.7;
  word-break: break-all;
}
.dw-side__det--none {
  color: var(--g300);
}
.dw-side__proc {
  font-size: 12px;
  color: var(--g500);
  line-height: 1.7;
}
</style>

<script setup>
// pulse7 主工作台：顶栏 / 告警 / 左侧栏 / 中间任务流（空态↔对话）/ 输入卡 / 底部状态栏 / 抽屉与弹窗。
import { onMounted } from 'vue';
import { store, actions, getters } from './store/store.js';
import TopBar from './components/TopBar.vue';
import AlertBanner from './components/AlertBanner.vue';
import LeftSidebar from './components/LeftSidebar.vue';
import EmptyState from './components/EmptyState.vue';
import ChatTimeline from './components/ChatTimeline.vue';
import InputCard from './components/InputCard.vue';
import BottomStatusBar from './components/BottomStatusBar.vue';
import TaskDrawer from './components/TaskDrawer.vue';
import SettingsDialog from './components/SettingsDialog.vue';
import FirstRunWizard from './components/FirstRunWizard.vue';
import RollbackDialog from './components/RollbackDialog.vue';
import PlanDecisionCard from './components/PlanDecisionCard.vue';

onMounted(() => {
  actions.boot();
});

async function interruptAndUnlock() {
  store.busyGuard = false;
  await actions.interrupt();
}
</script>

<template>
  <FirstRunWizard v-if="store.wizardOpen" />

  <div v-else class="app">
    <TopBar />
    <AlertBanner />

    <div class="app__main">
      <LeftSidebar />

      <div class="app__content">
        <!-- 忙碌切换守卫：活动轮（可中断）或后台任务仍在运行（不可中断）时如实说明 -->
        <div v-if="store.busyGuard" class="app__guard">
          <span class="app__guarddot pulse-dot" />
          <span class="app__guardt">{{
            store.guardMode === 'background_running'
              ? '后台任务仍在运行，停止任务后才能切换工作区或恢复会话'
              : '当前会话有任务运行中，中断或完成后再切换'
          }}</span>
          <button v-if="store.guardMode !== 'background_running'" class="app__guardstop" @click="interruptAndUnlock">■ 中断当前任务</button>
          <button v-else class="app__guardstop" @click="store.busyGuard = false; store.drawerOpen = true">⚙ 查看后台任务</button>
          <button class="app__guardx" aria-label="关闭提示" @click="store.busyGuard = false">✕</button>
        </div>

        <PlanDecisionCard v-if="store.planState" />
        <div class="app__chat">
          <div class="app__scroll">
            <EmptyState v-if="store.emptyMode" />
            <ChatTimeline v-else />
          </div>
          <InputCard />
        </div>
      </div>
    </div>

    <BottomStatusBar />

    <TaskDrawer v-if="store.drawerOpen" />
    <SettingsDialog v-if="store.settingsOpen" />
    <RollbackDialog v-if="store.rollbackOpen" />
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--paper);
  overflow: hidden;
  color: var(--ink);
}
.app__main {
  display: flex;
  flex: 1;
  overflow: hidden;
}
.app__content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}
.app__guard {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  border-bottom: 1px solid var(--amber200);
  background: var(--amber50);
  font-size: 13px;
  color: var(--amber700);
  flex-shrink: 0;
}
.app__guarddot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--amber500);
  flex-shrink: 0;
}
.app__guardt {
  flex: 1;
}
.app__guardstop {
  font-size: 12px;
  color: var(--red500);
  border: 1px solid var(--red200);
  border-radius: var(--radius-sm);
  padding: 3px 10px;
  flex-shrink: 0;
}
.app__guardstop:hover {
  color: var(--red700);
}
.app__guardx {
  color: var(--g400);
}
.app__guardx:hover {
  color: var(--g700);
}
.app__chat {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.app__scroll {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
</style>

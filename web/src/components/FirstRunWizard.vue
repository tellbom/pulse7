<script setup>
// 首次运行向导（屏 A，三步）：连接配置（含连接测试）→ 选择工作区 → 确认并开始。
// 首屏写清：双击 exe 仍是 CLI，桌面快捷方式（pulse7 serve）才是本页面。跳过是出口，不是默认路径。
import { ref, computed } from 'vue';
import { ElMessage } from 'element-plus';
import { store, actions } from '../store/store.js';
import OrbAvatar from './OrbAvatar.vue';

const step = ref(1);
const endpoint = ref('');
const modelName = ref('');
const apiKey = ref('');
const workspace = ref('');
const testing = ref(false);
const testMsg = ref('');
const testOk = ref(false);

const recent = computed(() => {
  const set = new Set(store.sessions.map((s) => s.cwd).filter(Boolean));
  return [...set].slice(0, 4);
});

async function test() {
  testing.value = true;
  testMsg.value = '';
  try {
    const fields = { base_url: endpoint.value, model: modelName.value };
    if (apiKey.value) fields.api_key = apiKey.value;
    const r = await actions.connectionTest(fields);
    testOk.value = true;
    testMsg.value = `连接成功 · ${r.model} · ${r.elapsedMs}ms`;
  } catch (e) {
    testOk.value = false;
    testMsg.value = e.message || '连接失败';
  }
  testing.value = false;
}

async function finish() {
  try {
    const fields = {};
    if (endpoint.value) fields.base_url = endpoint.value;
    if (modelName.value) fields.model = modelName.value;
    if (apiKey.value) fields.api_key = apiKey.value;
    if (Object.keys(fields).length) await actions.saveConfig(fields);
    if (workspace.value) await actions.switchWorkspace(workspace.value);
    store.wizardOpen = false;
    ElMessage.success('配置完成，可以发出第一个任务了');
  } catch (e) {
    ElMessage.error(e.message);
  }
}
</script>

<template>
  <div class="wz">
    <div class="wz__box">
      <div class="wz__brand">
        <OrbAvatar :size="56" big-radius class="wz__orb" />
        <div class="wz__name">pulse7</div>
        <div class="wz__sub">首次运行配置向导 · {{ store.version }}</div>
        <div class="wz__hint">
          双击桌面快捷方式（pulse7 serve）进入本页面；直接双击 exe 仍是命令行界面，这不是错误。
        </div>
      </div>

      <div class="wz__progress">
        <div v-for="n in 3" :key="n" class="wz__seg" :class="{ 'wz__seg--on': n <= step }" />
        <span class="mono wz__step">{{ step }}/3</span>
      </div>

      <div class="wz__card">
        <div class="wz__body">
          <template v-if="step === 1">
            <h2 class="wz__t">连接配置</h2>
            <div class="wz-field">
              <label class="wz-field__l" for="wz-ep">端点地址</label>
              <input id="wz-ep" v-model="endpoint" type="url" class="mono wz-field__i" placeholder="http://192.168.1.100:8080/v1" />
            </div>
            <div class="wz-field">
              <label class="wz-field__l" for="wz-mn">模型名称</label>
              <input id="wz-mn" v-model="modelName" type="text" class="mono wz-field__i" placeholder="gpt-4o 或内网模型名" />
            </div>
            <div class="wz-field">
              <label class="wz-field__l" for="wz-key">API Key <span class="wz-field__note">保存后不回显</span></label>
              <input id="wz-key" v-model="apiKey" type="password" class="mono wz-field__i" placeholder="sk-…" autocomplete="off" />
            </div>
            <div class="wz-test">
              <button class="wz-test__btn" :disabled="testing" @click="test">{{ testing ? '测试中…' : '测试连接' }}</button>
              <span v-if="testMsg" class="wz-test__msg" :class="testOk ? 'wz-test__msg--ok' : 'wz-test__msg--fail'">
                {{ testOk ? '✓ ' : '✕ ' }}{{ testMsg }}
              </span>
            </div>
          </template>

          <template v-else-if="step === 2">
            <h2 class="wz__t">选择工作区</h2>
            <div class="wz-field">
              <label class="wz-field__l" for="wz-ws">工作区路径</label>
              <input id="wz-ws" v-model="workspace" type="text" class="mono wz-field__i" placeholder="E:\projects\my-project" />
            </div>
            <div class="wz-recent">
              <div class="wz-recent__t">最近使用</div>
              <button v-for="p in recent" :key="p" class="mono wz-recent__item" @click="workspace = p">{{ p }}</button>
              <div v-if="!recent.length" class="wz-recent__none">暂无最近记录</div>
            </div>
          </template>

          <template v-else>
            <h2 class="wz__t">确认配置</h2>
            <div class="wz-summary">
              <div v-for="row in [['端点', endpoint || '(未设置)'], ['模型', modelName || '(未设置)'], ['密钥', apiKey ? '(已输入，不回显)' : '(未设置)'], ['工作区', workspace || '(未设置)']]" :key="row[0]" class="wz-summary__row">
                <span class="wz-summary__k">{{ row[0] }}</span>
                <span class="mono wz-summary__v">{{ row[1] }}</span>
              </div>
            </div>
            <div class="wz-tip">配置写入用户全局层；项目级 .pulse7 的优先级仍按现有规则合并（项目层不存放密钥）。</div>
          </template>
        </div>

        <div class="wz__foot">
          <button class="wz__skip" @click="store.wizardOpen = false">跳过，我自己改配置文件</button>
          <div class="wz__btns">
            <button v-if="step > 1" class="wz__prev" @click="step--">上一步</button>
            <button v-if="step < 3" class="wz__next" @click="step++">下一步</button>
            <button v-else class="wz__next" @click="finish">开始使用</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.wz {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  overflow-y: auto;
}
.wz__box {
  width: 520px;
  padding: 24px 0;
}
.wz__brand {
  text-align: center;
  margin-bottom: 32px;
}
.wz__orb {
  margin: 0 auto 16px;
  border-radius: 20%;
}
.wz__name {
  font-size: 24px;
  font-weight: 600;
  color: var(--g900);
  margin-bottom: 4px;
}
.wz__sub {
  font-size: 14px;
  color: var(--g500);
}
.wz__hint {
  font-size: 12px;
  color: var(--g400);
  margin: 10px auto 0;
  max-width: 400px;
  line-height: 1.6;
}
.wz__progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 24px;
  padding: 0 8px;
}
.wz__seg {
  height: 2px;
  flex: 1;
  border-radius: 999px;
  background: var(--g200);
  transition: background-color 0.15s;
}
.wz__seg--on {
  background: var(--g900);
}
.wz__step {
  font-size: 12px;
  color: var(--g400);
  flex-shrink: 0;
}
.wz__card {
  border: 1px solid var(--g200);
  border-radius: var(--radius-xl);
  overflow: hidden;
  background: #fff;
}
.wz__body {
  padding: 20px 24px;
}
.wz__t {
  font-size: 16px;
  font-weight: 600;
  color: var(--g900);
  margin-bottom: 16px;
}
.wz-field {
  margin-bottom: 16px;
}
.wz-field__l {
  display: block;
  font-size: 13px;
  color: var(--g600);
  margin-bottom: 6px;
  font-weight: 500;
}
.wz-field__note {
  color: var(--g400);
  font-weight: 400;
}
.wz-field__i {
  width: 100%;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  padding: 10px 12px;
  font-size: 13px;
  color: var(--g900);
  outline: none;
  transition: border-color 0.12s;
}
.wz-field__i::placeholder {
  color: var(--g300);
}
.wz-field__i:focus {
  border-color: var(--blue400);
}
.wz-test {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.wz-test__btn {
  font-size: 13px;
  border: 1px solid var(--g200);
  padding: 8px 16px;
  border-radius: var(--radius-lg);
}
.wz-test__btn:hover:not(:disabled) {
  background: var(--g50);
  border-color: var(--g300);
}
.wz-test__btn:disabled {
  opacity: 0.5;
}
.wz-test__msg {
  font-size: 13px;
}
.wz-test__msg--ok {
  color: var(--green600);
}
.wz-test__msg--fail {
  color: var(--red500);
}
.wz-recent__t {
  font-size: 12px;
  color: var(--g400);
  margin-bottom: 8px;
}
.wz-recent__item {
  display: block;
  font-size: 13px;
  color: var(--blue500);
  padding: 2px 0;
}
.wz-recent__item:hover {
  color: var(--blue700);
}
.wz-recent__none {
  font-size: 12px;
  color: var(--g300);
}
.wz-summary {
  border: 1px solid var(--g100);
  border-radius: var(--radius-lg);
  padding: 16px;
  background: var(--g50);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.wz-summary__row {
  display: flex;
  gap: 12px;
  font-size: 13px;
}
.wz-summary__k {
  color: var(--g400);
  width: 56px;
  flex-shrink: 0;
}
.wz-summary__v {
  color: var(--g700);
  word-break: break-all;
}
.wz-tip {
  font-size: 12px;
  color: var(--g400);
  margin-top: 12px;
  line-height: 1.6;
}
.wz__foot {
  padding: 16px 24px;
  border-top: 1px solid var(--g100);
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--g50);
}
.wz__skip {
  font-size: 13px;
  color: var(--g400);
}
.wz__skip:hover {
  color: var(--g600);
}
.wz__btns {
  display: flex;
  gap: 8px;
}
.wz__prev {
  padding: 8px 16px;
  font-size: 13px;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  color: var(--g600);
  background: #fff;
}
.wz__prev:hover {
  background: var(--g50);
}
.wz__next {
  padding: 8px 16px;
  font-size: 13px;
  background: var(--g900);
  color: #fff;
  border-radius: var(--radius-lg);
}
.wz__next:hover {
  background: var(--g700);
}
.wz__next:active {
  transform: scale(0.97);
}
</style>

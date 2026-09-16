<script setup>
// 设置 — 四页签：连接配置 / 运行参数 / Skills / 工作区。
// 参数页按 config-controls-and-session-isolation.md 开放全量可写字段：
// 默认值回显当前运行值（GET /api/config），保存走 PUT /api/config（全局层），
// 响应的 saved/restartRequiredFields 用于"已保存，重启后生效"提示；不自动重启。
import { ref, computed, onMounted, reactive } from 'vue';
import { ElMessage } from 'element-plus';
import { store, actions } from '../store/store.js';

const tab = ref('connection');
const endpoint = ref('');
const modelName = ref('');
const apiKey = ref('');
const testState = ref('idle');
const testMsg = ref('');
const wsDraft = ref(store.workspace);
const savedNotice = ref('');
const saving = ref(false);

// 字段定义：key=API 字段，def=默认值，restart=保存后需重启生效，min/max=输入保护
const FIELDS = [
  { group: '任务与上下文', items: [
    { key: 'max_ctx', label: '上下文上限（字节）', def: 256000, min: 16000, max: 32000000, hint: '约 /4 = 估算 token；默认 256000 ≈ 6.4 万 token' },
    { key: 'max_rounds', label: '单任务最大轮次', def: 100, min: 1, max: 10000, hint: '触顶即中止，仍可中断' },
    {
      key: 'skill_catalog_budget_bytes',
      label: '技能目录预算（KiB）',
      def: 8192,
      min: 1,
      max: 1024,
      unit: 'kib',
      hint: '控制技能名称、简介与定位信息占用的上下文。默认 8 KiB；超限按 full → shortened → names → index 降级。技能正文仅在模型读取时进入上下文；此值不扩大总上下文窗口。'
    }
  ] },
  { group: '模型请求（超时与重试）', items: [
    { key: 'llm_first_chunk_timeout_sec', label: '首块超时（秒）', def: 300, min: 5, max: 3600, hint: '等待模型第一段的时长，内网慢端点可调大' },
    { key: 'llm_idle_timeout_sec', label: '流空闲超时（秒）', def: 120, min: 5, max: 3600, hint: '相邻数据块之间的最大间隔' },
    { key: 'llm_compress_timeout_sec', label: '压缩调用超时（秒）', def: 180, min: 5, max: 3600 },
    { key: 'llm_max_retries', label: '可重试失败的重试次数', def: 2, min: 0, max: 10, hint: '0 = 关闭重试' }
  ] },
  { group: '进程与沙箱（重启生效）', items: [
    { key: 'shell_timeout_sec', label: '前台命令超时（秒）', def: 120, min: 5, max: 86400, restart: true },
    { key: 'memory_limit_mb', label: '子进程内存上限（MB）', def: 2048, min: 64, max: 4095, restart: true, hint: '过小会导致子进程启动失败' },
    { key: 'process_warn_threshold', label: '进程数告警阈值', def: 50, min: 0, max: 100000, restart: true, hint: '0 = 关闭告警；只提醒不拦截' },
    { key: 'sandbox_preference', label: '沙箱模式', def: 'auto', type: 'enum', options: ['auto', 'jobobject', 'sandboxie'], restart: true }
  ] },
  { group: '后台任务（重启生效）', items: [
    { key: 'background_task_max_output_mb', label: '单任务输出上限（MB）', def: 50, min: 1, max: 4096, restart: true, hint: '超限截断并标注，进程继续' },
    { key: 'background_task_warn_count', label: '并发告警数', def: 5, min: 0, max: 1000, restart: true, hint: '0 = 关闭' },
    { key: 'background_task_warn_sec', label: '时长告警（秒）', def: 3600, min: 0, max: 864000, restart: true, hint: '0 = 关闭；只提醒不终止' }
  ] },
  { group: '行为开关（重启生效）', items: [
    { key: 'read_only', label: '只读模式', def: false, type: 'bool', restart: true, hint: '开启后所有写入类工具被拒绝' },
    { key: 'cleanup_on_exit', label: '退出时清理子进程', def: true, type: 'bool', restart: true }
  ] }
];
const FIELD_KEYS = FIELDS.flatMap((g) => g.items.map((i) => i.key));
const FIELD_DEF = Object.fromEntries(FIELDS.flatMap((g) => g.items.map((i) => [i.key, i])));

const form = reactive({});
const toFormValue = (field, value) => (field.unit === 'kib' ? Number(value) / 1024 : value);
const toApiValue = (field, value) => (field.unit === 'kib' ? Number(value) * 1024 : value);
const formatKiB = (bytes) => {
  const value = Math.max(0, Number(bytes) || 0) / 1024;
  return `${Number.isInteger(value) ? value : value.toFixed(1)} KiB`;
};
const fieldCurrentText = (field) => {
  const value = store.config[field.key];
  if (value === undefined) return '—';
  return field.unit === 'kib' ? formatKiB(value) : value;
};
const fieldDefaultText = (field) => (field.unit === 'kib' ? formatKiB(field.def) : field.def);
const isDirty = (key) => {
  const d = FIELD_DEF[key];
  const cur = store.config[key];
  if (d.type === 'enum' || d.type === 'bool') return String(form[key]) !== String(cur);
  return form[key] !== undefined && form[key] !== '' && Number(toApiValue(d, form[key])) !== Number(cur);
};

onMounted(() => {
  endpoint.value = store.config.base_url || '';
  modelName.value = store.config.model || '';
  wsDraft.value = store.workspace || '';
  FIELD_KEYS.forEach((k) => {
    const d = FIELD_DEF[k];
    const cur = store.config[k];
    form[k] = toFormValue(d, cur !== undefined ? cur : d.def);
  });
});

const skills = computed(() => store.skillsAvailable);
const usedSkills = computed(() => store.skillsUsed);
const catalogModeText = computed(() => {
  const mode = store.skillCatalog && store.skillCatalog.mode;
  return {
    full: '完整元数据',
    shortened: '描述已缩短',
    names: '描述已省略',
    index: '按需查阅完整目录',
    empty: '未发现技能'
  }[mode] || mode || '';
});
const catalogUsageText = computed(() => {
  const catalog = store.skillCatalog;
  if (!catalog) return '';
  return `${formatKiB(catalog.listingBytes)} / ${formatKiB(catalog.budgetBytes)}`;
});
const recentWorkspaces = computed(() => {
  const set = new Set(store.sessions.map((s) => s.cwd).filter(Boolean));
  set.delete(store.workspace);
  return [...set].slice(0, 5);
});

function close() {
  store.settingsOpen = false;
}

async function test() {
  testState.value = 'testing';
  testMsg.value = '';
  try {
    const fields = { base_url: endpoint.value, model: modelName.value };
    if (apiKey.value) fields.api_key = apiKey.value;
    const r = await actions.connectionTest(fields);
    testState.value = 'ok';
    testMsg.value = `连接成功 · ${r.model} · ${r.elapsedMs}ms（未保存配置）`;
  } catch (e) {
    testState.value = 'fail';
    testMsg.value = e.message || '连接失败';
  }
}

async function save() {
  saving.value = true;
  savedNotice.value = '';
  try {
    const fields = {};
    if (endpoint.value !== (store.config.base_url || '')) fields.base_url = endpoint.value;
    if (modelName.value !== (store.config.model || '')) fields.model = modelName.value;
    if (apiKey.value) fields.api_key = apiKey.value;
    FIELD_KEYS.forEach((k) => {
      if (!isDirty(k)) return;
      const d = FIELD_DEF[k];
      const value = Number(form[k]);
      if (!['enum', 'bool'].includes(d.type || '') && (!Number.isFinite(value) || !Number.isInteger(value) || value < d.min || value > d.max)) {
        throw new Error(`${d.label}必须是 ${d.min}–${d.max} 之间的整数`);
      }
      fields[k] = toApiValue(d, form[k]);
    });
    if (!Object.keys(fields).length) {
      close();
      return;
    }
    const r = await actions.saveConfig(fields);
    apiKey.value = '';
    const restart = (r.restartRequiredFields || []).filter((f) => FIELD_DEF[f]);
    const catalogChanged = fields.skill_catalog_budget_bytes !== undefined;
    if (restart.length) {
      savedNotice.value =
        '已保存到用户全局配置；' +
        (catalogChanged ? '技能目录预算从下一次任务开始生效；' : '') +
        '以下项需重启 pulse7 后生效：' +
        restart.map((f) => FIELD_DEF[f].label).join('、');
      ElMessage.warning('部分配置重启后生效');
    } else if (catalogChanged) {
      savedNotice.value = '已保存到用户全局配置；技能目录预算从下一次任务开始生效，无需重启。';
      ElMessage.success('技能目录预算已保存，将在下一次任务生效');
    } else {
      savedNotice.value = '已保存到用户全局配置并即时生效（不写项目层）';
      ElMessage.success('配置已保存');
    }
    testState.value = 'idle';
    FIELD_KEYS.forEach((k) => {
      const d = FIELD_DEF[k];
      form[k] = r[k] !== undefined ? toFormValue(d, r[k]) : form[k];
    });
  } catch (e) {
    ElMessage.error(e.message);
  }
  saving.value = false;
}

async function applyWorkspace() {
  const v = wsDraft.value.trim();
  if (!v) return;
  await actions.switchWorkspace(v);
  close();
}
</script>

<template>
  <div class="sd-mask" @click.self="close">
    <div class="sd" role="dialog" aria-modal="true" aria-label="设置">
      <div class="sd__corner" aria-hidden="true" />
      <div class="sd__head">
        <div>
          <h2 class="sd__t">设置</h2>
          <div class="mono sd__ver">pulse7 · {{ store.version }} · 全局配置层（%USERPROFILE%\.pulse7\config.json）</div>
        </div>
        <button class="sd__x" aria-label="关闭" @click="close">✕</button>
      </div>
      <div class="sd__tabs">
        <button
          v-for="t in [['connection', '连接配置'], ['params', '运行参数'], ['skills', 'Skills'], ['workspace', '工作区']]"
          :key="t[0]"
          class="sd-tab"
          :class="{ 'sd-tab--on': tab === t[0] }"
          @click="tab = t[0]"
        >
          {{ t[1] }}
        </button>
      </div>

      <div class="sd__body">
        <!-- 连接配置 -->
        <template v-if="tab === 'connection'">
          <div class="sd-grid">
            <div class="sd-field sd-field--wide">
              <label class="sd-field__l" for="st-ep">端点地址</label>
              <input id="st-ep" v-model="endpoint" type="url" class="mono sd-field__i" placeholder="http://192.168.1.100:8080/v1" />
            </div>
            <div class="sd-field">
              <label class="sd-field__l" for="st-mn">模型名称</label>
              <input id="st-mn" v-model="modelName" type="text" class="mono sd-field__i" placeholder="gpt-4o 或内网模型名" />
            </div>
            <div class="sd-field">
              <label class="sd-field__l" for="st-key">API Key <span class="sd-field__note">保存后不回显；只写全局层</span></label>
              <input id="st-key" v-model="apiKey" type="password" class="mono sd-field__i" placeholder="sk-…" autocomplete="off" />
            </div>
          </div>
          <div class="sd-field__status mono" v-if="store.config.apiKeyConfigured">已配置密钥（值不回显）</div>
          <div class="sd-test">
            <button class="sd-test__btn" :disabled="testState === 'testing'" @click="test">
              {{ testState === 'testing' ? '测试中…' : '测试连接' }}
            </button>
            <span v-if="testState === 'ok'" class="sd-test__ok">✓ {{ testMsg }}</span>
            <span v-if="testState === 'fail'" class="sd-test__fail">✕ {{ testMsg }}</span>
          </div>
        </template>

        <!-- 运行参数 -->
        <template v-else-if="tab === 'params'">
          <div v-for="g in FIELDS" :key="g.group" class="sd-pg">
            <div class="sd-pg__t">{{ g.group }}</div>
            <div class="sd-grid">
              <div v-for="f in g.items" :key="f.key" class="sd-field">
                <label class="sd-field__l" :for="'pf-' + f.key">
                  {{ f.label }}
                  <span v-if="f.restart" class="sd-rb" title="保存后需重启 pulse7 生效">重启生效</span>
                </label>
                <select v-if="f.type === 'enum'" :id="'pf-' + f.key" v-model="form[f.key]" class="mono sd-field__i">
                  <option v-for="o in f.options" :key="o" :value="o">{{ o }}</option>
                </select>
                <label v-else-if="f.type === 'bool'" class="sd-sw">
                  <input v-model="form[f.key]" type="checkbox" />
                  <span>{{ form[f.key] ? '已启用' : '已关闭' }}（默认 {{ f.def ? '开' : '关' }}）</span>
                </label>
                <input
                  v-else
                  :id="'pf-' + f.key"
                  v-model.number="form[f.key]"
                  type="number"
                  class="mono sd-field__i"
                  :min="f.min"
                  :max="f.max"
                  :step="1"
                />
                <div class="sd-field__meta">
                  <span>当前 {{ fieldCurrentText(f) }} · 默认 {{ fieldDefaultText(f) }}</span>
                  <span v-if="isDirty(f.key)" class="sd-dirty">已修改</span>
                </div>
                <div v-if="f.hint" class="sd-field__hint">{{ f.hint }}</div>
              </div>
            </div>
          </div>
          <div class="sd-note">
            范围为页面输入保护；高值不等于已在低配 Win7 上压力验收。保存写入用户全局层，优先级仍为
            flag &gt; 项目配置 &gt; 全局配置 &gt; 默认值。
          </div>
        </template>

        <!-- Skills -->
        <template v-else-if="tab === 'skills'">
          <div class="sd-skills__hint">
            Skills 只来自工作区与当前运行用户的 .pulse7/skills/&lt;目录&gt;/SKILL.md。外部安装、更新或删除会在<b>下一次任务开始</b>时重新扫描，无需新建会话；当前没有 watcher 或实时刷新通知。目录可见不等于正文已进入上下文。
          </div>
          <div class="sd-catalog" :class="store.skillCatalog ? `sd-catalog--${store.skillCatalog.mode}` : ''">
            <template v-if="store.skillCatalogPending">
              <div class="sd-catalog__title">正在构建本次任务的技能目录…</div>
              <div class="sd-catalog__note">扫描发生在任务边界，不会在任务内的多轮工具循环中重复执行。</div>
            </template>
            <template v-else-if="store.skillCatalog">
              <div class="sd-catalog__head">
                <div>
                  <div class="sd-catalog__title">本次任务发现 {{ store.skillCatalog.count }} 个技能</div>
                  <div class="sd-catalog__usage">目录占用 <span class="mono">{{ catalogUsageText }}</span></div>
                </div>
                <span class="sd-catalog__mode">{{ catalogModeText }}</span>
              </div>
              <div class="sd-catalog__note">
                统计为目录 system 消息 JSON 序列化后的 UTF-8 字节，不是实际 token；预算占用既有 max_ctx，不会扩大总上下文窗口。
              </div>
              <div v-if="store.skillCatalog.indexPath" class="sd-catalog__index">
                完整元数据索引：<span class="mono">{{ store.skillCatalog.indexPath }}</span>
              </div>
              <div v-for="warning in store.skillCatalog.warnings" :key="warning" class="sd-catalog__warning">⚠ {{ warning }}</div>
            </template>
            <template v-else>
              <div class="sd-catalog__title">尚无本次任务的技能目录事件</div>
              <div class="sd-catalog__note">发送下一条用户任务后会重新扫描并显示目录模式；这里不声称已实时检测外部文件变化。</div>
            </template>
          </div>
          <div class="sd-skills__legend">
            <span><b>目录状态</b>表示模型本次可发现的元数据投影</span>
            <span><b>本轮已读取正文</b>只来自 skill_loaded 事件</span>
          </div>
          <div v-for="s in skills" :key="s.name" class="sd-skill">
            <div class="sd-skill__l">
              <span class="sd-skill__dot" :class="{ 'sd-skill__dot--on': usedSkills.includes(s.name) }" />
              <div class="sd-skill__main">
                <span class="mono sd-skill__name">{{ s.name }}</span>
                <span v-if="s.description" class="sd-skill__desc">{{ s.description }}</span>
                <span v-if="s.path" class="mono sd-skill__path">{{ s.path }}</span>
              </div>
            </div>
            <div class="sd-skill__r">
              <span v-if="usedSkills.includes(s.name)" class="sd-skill__used">本轮已读取正文</span>
              <span v-if="s.scope || s.source" class="sd-skill__src">{{ s.scope || s.source }}</span>
            </div>
          </div>
          <div v-if="!skills.length" class="sd-skills__hint">尚未收到 session_init.skills 发现元数据；下一次任务开始时会重新扫描。</div>
        </template>

        <!-- 工作区 -->
        <template v-else>
          <div class="sd-field">
            <label class="sd-field__l" for="st-ws">当前工作区（以实际存在的目录绝对路径为准）</label>
            <div class="sd-ws">
              <input id="st-ws" v-model="wsDraft" type="text" class="mono sd-field__i sd-ws__input" placeholder="E:\projects\my-project" />
              <button class="sd-ws__apply" @click="applyWorkspace">应用</button>
            </div>
            <div class="sd-field__hint">活动轮或后台任务运行时切换会被拒绝（409）；切换会关闭当前会话。</div>
          </div>
          <div class="sd-recent">
            <div class="sd-recent__t">最近使用（点击填入）</div>
            <button v-for="w in recentWorkspaces" :key="w" class="mono sd-recent__item" @click="wsDraft = w">{{ w }}</button>
            <div v-if="!recentWorkspaces.length" class="sd-field__hint">暂无历史记录</div>
          </div>
        </template>
      </div>

      <div class="sd__foot">
        <div class="sd__saved">{{ savedNotice }}</div>
        <div class="sd__btns">
          <button class="sd__cancel" @click="close">取消</button>
          <button class="sd__save" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sd-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.2);
}
.sd {
  width: 720px;
  max-width: 94vw;
  max-height: 84vh;
  background: #fff;
  border: 1px solid var(--g200);
  border-radius: var(--radius-xl);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.16);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.sd__head {
  padding: 20px 28px;
  border-bottom: 1px solid var(--g100);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}
.sd__t {
  font-size: var(--fs-lg);
  font-weight: 600;
  color: var(--g900);
}
.sd__ver {
  font-size: 11px;
  color: var(--g400);
  margin-top: 3px;
}
.sd__x {
  color: var(--g400);
  font-size: 18px;
}
.sd__x:hover {
  color: var(--g700);
}
.sd__tabs {
  display: flex;
  border-bottom: 1px solid var(--g100);
  padding: 0 28px;
  gap: 4px;
  flex-shrink: 0;
}
.sd-tab {
  padding: 12px 16px;
  font-size: 13px;
  border-bottom: 2px solid transparent;
  color: var(--g400);
  margin-bottom: -1px;
  transition: color 0.12s, border-color 0.12s;
}
.sd-tab:hover {
  color: var(--g700);
}
.sd-tab--on {
  border-color: var(--accent);
  color: var(--g900);
  font-weight: 500;
}
.sd__body {
  padding: 20px 28px;
  overflow-y: auto;
  flex: 1;
}
.sd-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 18px;
}
.sd-field--wide {
  grid-column: 1 / -1;
}
.sd-field {
  display: flex;
  flex-direction: column;
}
.sd-field__l {
  font-size: 12px;
  color: var(--g600);
  margin-bottom: 6px;
  font-weight: 500;
}
.sd-field__note {
  color: var(--g400);
  font-weight: 400;
}
.sd-field__i {
  width: 100%;
  border: 1px solid var(--g200);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 13px;
  color: var(--g900);
  outline: none;
  transition: border-color 0.12s;
  background: #fff;
}
.sd-field__i:focus {
  border-color: var(--blue400);
}
.sd-field__i::placeholder {
  color: var(--g300);
}
.sd-field__meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--g400);
  margin-top: 4px;
}
.sd-dirty {
  color: var(--amber600);
}
.sd-field__hint {
  font-size: 11px;
  color: var(--g400);
  margin-top: 2px;
  line-height: 1.5;
}
.sd-field__status {
  font-size: 11px;
  color: var(--g400);
  margin-top: 10px;
}
.sd-rb {
  font-size: 10px;
  color: var(--amber700);
  background: var(--amber50);
  border: 1px solid var(--amber200);
  border-radius: 999px;
  padding: 0 6px;
  margin-left: 6px;
  font-weight: 400;
}
.sd-sw {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--g200);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 13px;
  color: var(--g700);
  cursor: pointer;
}
.sd-pg {
  margin-bottom: 20px;
}
.sd-pg__t {
  font-size: 12px;
  font-weight: 600;
  color: var(--g700);
  padding-bottom: 6px;
  border-bottom: 1px solid var(--g100);
  margin-bottom: 12px;
}
.sd-note {
  font-size: 11px;
  color: var(--g400);
  line-height: 1.6;
  border-top: 1px solid var(--g100);
  padding-top: 10px;
}
.sd-test {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 14px;
}
.sd-test__btn {
  font-size: 13px;
  border: 1px solid var(--g200);
  padding: 8px 16px;
  border-radius: var(--radius-sm);
}
.sd-test__btn:hover:not(:disabled) {
  border-color: var(--g300);
  background: var(--g50);
}
.sd-test__btn:disabled {
  opacity: 0.5;
}
.sd-test__ok {
  color: var(--green600);
  font-size: 13px;
}
.sd-test__fail {
  color: var(--red500);
  font-size: 13px;
  word-break: break-all;
}
.sd-skills__hint {
  font-size: 12px;
  color: var(--g500);
  margin-bottom: 14px;
  line-height: 1.7;
}
.sd-catalog {
  border: 1px solid var(--g200);
  background: var(--g50);
  border-radius: var(--radius-lg);
  padding: 14px 16px;
  margin-bottom: 12px;
}
.sd-catalog--shortened,
.sd-catalog--names,
.sd-catalog--index {
  border-color: var(--amber200);
  background: var(--amber50);
}
.sd-catalog__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.sd-catalog__title {
  color: var(--g800);
  font-size: 13px;
  font-weight: 600;
}
.sd-catalog__usage,
.sd-catalog__note,
.sd-catalog__index,
.sd-catalog__warning {
  margin-top: 5px;
  color: var(--g500);
  font-size: 11px;
  line-height: 1.55;
}
.sd-catalog__mode {
  flex-shrink: 0;
  color: var(--blue700);
  background: var(--blue50);
  border: 1px solid var(--blue100);
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 11px;
}
.sd-catalog--shortened .sd-catalog__mode,
.sd-catalog--names .sd-catalog__mode,
.sd-catalog--index .sd-catalog__mode {
  color: var(--amber700);
  background: #fff;
  border-color: var(--amber200);
}
.sd-catalog__index {
  word-break: break-all;
}
.sd-catalog__warning {
  color: var(--amber700);
}
.sd-skills__legend {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  color: var(--g400);
  font-size: 11px;
  padding: 0 2px 8px;
}
.sd-skill {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--g100);
}
.sd-skill__l {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}
.sd-skill__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--g200);
  margin-top: 6px;
  flex-shrink: 0;
}
.sd-skill__dot--on {
  background: var(--green400);
}
.sd-skill__name {
  font-size: 14px;
  color: var(--g800);
}
.sd-skill__main {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.sd-skill__desc {
  color: var(--g500);
  font-size: 11px;
  margin-top: 2px;
}
.sd-skill__path {
  color: var(--g300);
  font-size: 10px;
  margin-top: 2px;
  word-break: break-all;
}
.sd-skill__r {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.sd-skill__used {
  font-size: 11px;
  color: var(--green500);
}
.sd-skill__src {
  font-size: 11px;
  color: var(--g400);
}
.sd-ws {
  display: flex;
  gap: 10px;
  align-items: stretch;
}
.sd-ws__input {
  flex: 1;
  min-width: 0;
}
.sd-ws__apply {
  flex-shrink: 0;
  white-space: nowrap;
  padding: 8px 22px;
  font-size: 13px;
  border: 1px solid var(--g300);
  border-radius: var(--radius-sm);
  color: var(--g700);
  background: #fff;
}
.sd-ws__apply:hover {
  background: var(--g50);
  border-color: var(--g400);
}
.sd-recent {
  margin-top: 14px;
}
.sd-recent__t {
  font-size: 12px;
  color: var(--g400);
  margin-bottom: 6px;
}
.sd-recent__item {
  display: block;
  text-align: left;
  font-size: 12px;
  color: var(--blue600);
  padding: 3px 0;
}
.sd-recent__item:hover {
  color: var(--blue700);
}
.sd__foot {
  padding: 14px 28px;
  border-top: 1px solid var(--g100);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-shrink: 0;
}
.sd__saved {
  font-size: 12px;
  color: var(--g600);
  line-height: 1.5;
  flex: 1;
  min-width: 0;
}
.sd__btns {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
.sd__cancel {
  padding: 8px 18px;
  font-size: 13px;
  color: var(--g600);
  border: 1px solid var(--g200);
  border-radius: var(--radius-sm);
}
.sd__cancel:hover {
  border-color: var(--g300);
}
.sd__save {
  padding: 8px 22px;
  font-size: 13px;
  background: var(--g900);
  color: #fff;
  border-radius: var(--radius-sm);
}
.sd__save:hover:not(:disabled) {
  background: var(--g700);
}
.sd__save:disabled {
  opacity: 0.5;
}
</style>

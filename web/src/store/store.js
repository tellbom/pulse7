// 中央状态仓库 — 消费契约事件（C1.3），驱动任务流/常驻区/弹窗。
// 关键规则：
//  · F2 attempt：start 开缓冲、delta 追加对应 attempt、complete 才有效、discard 撤掉已展示文本；
//  · 不装作知道：估算值标估算、心跳只说明等待时长、rounds 未知就不显示；
//  · busy = 轮次进行中；切工作区/恢复会话在后台任务仍运行时会被后端 409，如实呈现。
import { reactive } from 'vue';
import { ElMessage } from 'element-plus';
import { api, openStream, liveMode } from '../api/client.js';
import { createMockBackend } from '../api/mock.js';

let mock = null;
let streamCtrl = null;
let mockSub = null;
let itemSeq = 0;
let switchGen = 0; // 工作区/会话切换代次：旧异步响应不得覆盖新页面（handoff §3）
const nid = (p) => `${p}-${++itemSeq}`;

export const store = reactive({
  booted: false,
  liveMode,
  version: 'rc-0.14',

  // 连接
  listener: { address: '127.0.0.1', port: null, listening: false, tokenValid: false },
  connected: false,
  streamError: '',

  // 配置与权限
  config: {},
  permissions: { profile: 'standard', rules: [], readOnly: false },
  workspace: '',

  // 会话
  sessions: [],
  sessionErrors: [], // GET /api/sessions 的 errors[]：坏文件逐项警告，不掩盖健康会话
  sessionId: null,
  hasMoreHistory: false,
  skillsAvailable: [],
  skillsUsed: [],

  // 任务流
  timeline: [], // 见 consumeEvent 内各工厂
  scrollTick: 0,

  // 当前轮
  phase: 'idle', // idle | waiting_first | streaming | tool_running | need_answer
  turnStartedAt: 0,
  waitedSeconds: 0,
  attempts: {}, // attempt -> { text, status }
  pendingPermission: null,
  turnCommands: [],
  turnOutsideWrites: [],
  turnBgProcesses: [],
  // 计划模式：{active,path,decision?}——decision={id,question,awaitingReply,replyUuid?,replyPreview?,cancelled?}
  planState: null,

  // 常驻状态
  context: { usedTokens: 0, budget: 0, percentLeft: 100, warningLevel: 'normal' },
  tasks: [],
  processes: [],
  detachedHistory: [],
  overview: { count: null, threshold: 50, exceeded: null, error: '', mode: null },
  checkpoints: [],
  taskOutputs: {}, // taskId -> { text, nextOffset, totalBytes }

  // 告警横幅
  alertRestricted: false,
  alertEndpointDown: false,

  // UI 开关
  leftCollapsed: false,
  drawerOpen: false,
  settingsOpen: false,
  wizardOpen: false,
  rollbackOpen: false,
  busyGuard: false,
  guardMode: 'busy', // busy | background_running：忙碌守卫横幅的两种成因
  switching: false, // 工作区切换进行中：禁用重复切换与发送
  emptyMode: true
});

export const getters = {
  get busy() {
    return ['waiting_first', 'streaming', 'tool_running'].includes(store.phase);
  },
  get runningTasks() {
    return store.tasks.filter((t) => t.status === 'running' || t.status === 'output_truncated');
  },
  get blockSwitch() {
    return getters.busy || getters.runningTasks.length > 0;
  },
  get contextPct() {
    const c = store.context || {};
    if (!c.budget) return 0;
    const used = Math.max(0, Math.min(100, Math.round(((c.budget * (100 - (c.percentLeft ?? 100))) / 100 / c.budget) * 100)));
    return c.percentLeft !== undefined ? Math.round(100 - c.percentLeft) : used;
  },
  get contextLevel() {
    // 优先使用后端 context_state 的 warningLevel（与压缩阈值一致，65% 起预警）；缺失时按占比推算。
    const w = store.context && store.context.warningLevel;
    if (w === 'normal' || w === 'warning' || w === 'critical') return w;
    const p = getters.contextPct;
    return p >= 90 ? 'critical' : p >= 65 ? 'warning' : 'normal';
  }
};

// ─── 时间线条目工厂 ───────────────────────────────────────────
const push = (item) => {
  store.timeline.push(item);
  store.scrollTick++;
};

const removeWhere = (fn) => {
  store.timeline = store.timeline.filter((x) => !fn(x));
};

// ─── 会话/工作区状态全量清理（handoff §3.3） ──────────────────
function resetSessionState() {
  store.timeline = [];
  store.attempts = {};
  store.sessionId = null;
  store.emptyMode = true;
  store.phase = 'idle';
  store.pendingPermission = null;
  store.waitedSeconds = 0;
  store.turnStartedAt = 0;
  store.turnCommands = [];
  store.turnOutsideWrites = [];
  store.turnBgProcesses = [];
  store.context = { usedTokens: 0, budget: 0, percentLeft: 100, warningLevel: 'normal' };
  store.skillsAvailable = [];
  store.skillsUsed = [];
  store.checkpoints = [];
  store.taskOutputs = {};
}

export const actions = {
  // ─── 启动 ───
  async boot() {
    if (store.booted) return;
    store.booted = true;
    if (!liveMode) mock = createMockBackend();
    const call = mock ? mock.api : api;
    try {
      store.config = await call('GET', '/api/config');
      store.workspace = store.config.workspace || '';
      store.listener = await call('GET', '/api/listener');
      store.permissions = await call('GET', '/api/permissions');
      await actions.refreshTasks();
      await actions.refreshSessions();
      await actions.loadCheckpoints();
    } catch (e) {
      ElMessage.error('初始化失败：' + e.message);
    }
    actions.connectStream();
    actions.refreshPlan();
    if (!store.config.base_url || !store.config.model) store.wizardOpen = true;
  },

  call(method, path, body) {
    return (mock ? mock.api : api)(method, path, body);
  },

  connectStream() {
    if (streamCtrl) streamCtrl.abort();
    if (mockSub) {
      mockSub.close();
      mockSub = null;
    }
    store.streamError = '';
    if (mock) {
      mockSub = mock.connect((evt) => consumeEvent(evt));
      store.connected = true;
      return;
    }
    streamCtrl = openStream({
      open() {
        store.connected = true;
        store.alertEndpointDown = false;
      },
      event(evt) {
        consumeEvent(evt);
      },
      error(e) {
        store.connected = false;
        store.streamError = e.message || '事件流断开';
        store.alertEndpointDown = true;
        // SSE 断开时不得永久卡 busy：本地复位轮次态（会话与历史保留）
        if (getters.busy) {
          store.phase = 'idle';
          removeWhere((x) => x.type === 'waiting');
        }
      }
    });
  },

  // ─── 列表刷新 ───
  async refreshTasks() {
    try {
      const r = await actions.call('GET', '/api/tasks');
      store.tasks = r.tasks || [];
      store.processes = r.processes || [];
      store.detachedHistory = r.detachedHistory || [];
      if (r.overview) store.overview = { ...store.overview, ...r.overview };
    } catch (e) {
      /* 常驻区尽力刷新 */
    }
  },
  async refreshSessions() {
    try {
      const r = await actions.call('GET', '/api/sessions');
      store.sessions = r.sessions || [];
      store.sessionErrors = r.errors || [];
    } catch (e) {
      /* 列表失败不伪装为空 */
    }
  },
  async loadCheckpoints() {
    try {
      const r = await actions.call('GET', '/api/checkpoints');
      store.checkpoints = r.checkpoints || [];
    } catch (e) {
      /* 无检查点 */
    }
  },

  // ─── 发送 / 中断 / 回答 ───
  async send(prompt, attachment) {
    prompt = (prompt || '').trim();
    if (!prompt) return;
    if (getters.busy || store.switching) {
      ElMessage.warning(store.switching ? '工作区切换进行中' : '任务进行中，中断后可继续输入');
      return;
    }
    push({ type: 'user', id: nid('u'), text: prompt, attachment: attachment || null });
    store.turnCommands = [];
    store.turnOutsideWrites = [];
    store.turnBgProcesses = [];
    store.turnStartedAt = Date.now();
    store.waitedSeconds = 0;
    store.emptyMode = false;
    const isAnswer = store.phase === 'need_answer';
    try {
      const r = isAnswer
        ? await actions.call('POST', '/api/answer', { sessionId: store.sessionId, answer: prompt })
        : await actions.call('POST', '/api/turns', { prompt });
      store.sessionId = r.sessionId;
      store.phase = 'waiting_first';
      actions.pushWaiting('waiting_first');
    } catch (e) {
      if (e.code === 'plan_decision_pending') {
        // 存在待答计划问题：普通输入不猜成回答，提示用关联入口
        await actions.refreshPlan();
        ElMessage.warning('模型在等待计划问题的回复，请在问题卡片中作答');
      } else {
        ElMessage.error(e.message);
      }
    }
  },

  async interrupt() {
    try {
      await actions.call('POST', '/api/interrupt', {});
      ElMessage.info('已请求中断，等待结束事件确认');
    } catch (e) {
      // 后端明确"没有活动轮"= 前端状态机卡住：就地复位，避免永远点不掉
      if (e.status === 409) {
        store.phase = 'idle';
        store.pendingPermission = null;
        store.waitedSeconds = 0;
        removeWhere((x) => x.type === 'waiting');
        ElMessage.warning('后端无活动任务，界面状态已复位');
      } else {
        ElMessage.error(e.message);
      }
    }
  },

  // ─── 权限确认 ───
  async confirmPermission(decision) {
    const p = store.pendingPermission;
    if (!p) return;
    try {
      await actions.call('POST', '/api/permission', { requestId: p.requestId, decision });
    } catch (e) {
      ElMessage.error(e.message);
    }
  },

  async setProfile(profile) {
    const prev = store.permissions.profile;
    store.permissions.profile = profile;
    try {
      const r = await actions.call('PUT', '/api/permissions', { profile });
      store.permissions = r;
    } catch (e) {
      store.permissions.profile = prev;
      ElMessage.error(e.message);
    }
  },

  async saveModel(model) {
    try {
      const r = await actions.call('PUT', '/api/config', { model });
      store.config = r;
      ElMessage.success('模型已保存，后续请求生效');
    } catch (e) {
      ElMessage.error(e.message);
    }
  },

  // ─── 工作区 / 会话（串行 + 代次校验；409 区分 busy 与 background_running） ───
  async switchWorkspace(path) {
    if (!path || store.switching) return;
    const gen = ++switchGen;
    store.switching = true;
    try {
      const r = await actions.call('PUT', '/api/workspace', { path });
      if (gen !== switchGen) return;
      resetSessionState();
      store.workspace = r.workspace;
      ElMessage.success('工作区已切换：' + r.workspace);
    } catch (e) {
      if (gen !== switchGen) return;
      if (e.status === 409) {
        store.guardMode = e.code === 'background_running' ? 'background_running' : 'busy';
        store.busyGuard = true;
        ElMessage.warning(e.code === 'background_running' ? '后台任务运行中，先停止任务才能切换' : '当前任务运行中，中断或完成后再切换');
      } else {
        ElMessage.error(e.message);
      }
    } finally {
      if (gen === switchGen) store.switching = false;
    }
    if (gen !== switchGen) return;
    await Promise.all([actions.refreshSessions(), actions.loadCheckpoints(), actions.refreshTasks(), actions.reloadConfig()]);
  },

  async reloadConfig() {
    try {
      store.config = await actions.call('GET', '/api/config');
    } catch (e) {
      /* 配置读取失败不覆盖现有展示 */
    }
  },

  async resumeSession(id) {
    if (store.switching) return;
    const gen = ++switchGen;
    store.switching = true;
    try {
      const r = await actions.call('POST', '/api/sessions/resume', { sessionId: id });
      if (gen !== switchGen) return;
      resetSessionState();
      store.sessionId = r.sessionId;
      store.workspace = r.cwd || store.workspace;
      store.emptyMode = false;
      await actions.loadHistory();
      await Promise.all([actions.loadCheckpoints(), actions.refreshTasks(), actions.refreshPlan()]);
      ElMessage.success('会话已恢复，历史已载入');
    } catch (e) {
      if (gen !== switchGen) return;
      if (e.status === 409 && e.code === 'workspace_mismatch') {
        // 会话属于别的工作区：按契约自动切回原工作区后重试恢复（不谎报成任务运行中）
        try {
          const sess = store.sessions.find((x) => x.sessionId === id);
          const cwd = (sess && sess.cwd) || '';
          if (!cwd) throw new Error('该会话未记录工作区，无法自动切换');
          await actions.call('PUT', '/api/workspace', { path: cwd });
          const r = await actions.call('POST', '/api/sessions/resume', { sessionId: id });
          if (gen !== switchGen) return;
          resetSessionState();
          store.sessionId = r.sessionId;
          store.workspace = r.cwd || cwd;
          store.emptyMode = false;
          await actions.loadHistory();
          await Promise.all([actions.loadCheckpoints(), actions.refreshTasks(), actions.refreshPlan()]);
          ElMessage.success('已切回原工作区并恢复会话');
          return;
        } catch (e2) {
          ElMessage.error('恢复失败：' + (e2.message || '请先手动切换到该会话的工作区'));
        }
      } else if (e.status === 409) {
        store.guardMode = e.code === 'background_running' ? 'background_running' : 'busy';
        store.busyGuard = true;
        ElMessage.warning(e.code === 'background_running' ? '后台任务运行中，先停止任务才能切换' : '当前任务运行中，中断或完成后再切换');
      } else {
        ElMessage.error(e.message);
      }
    } finally {
      if (gen === switchGen) store.switching = false;
    }
  },

  async loadHistory() {
    if (!store.sessionId) return;
    let offset = 0;
    try {
      for (;;) {
        const r = await actions.call('GET', `/api/sessions/${encodeURIComponent(store.sessionId)}/messages?offset=${offset}&limit=100`);
        renderHistory(r.messages || []);
        store.hasMoreHistory = !!r.hasMore;
        if (!r.hasMore) break;
        offset = r.nextOffset;
      }
    } catch (e) {
      ElMessage.error('历史读取失败：' + e.message);
    }
  },

  // 新建会话：先通知后端真正关闭当前会话（创建新 sessionId 的前提），成功后才清屏。
  async newSession() {
    if (store.switching) return;
    try {
      await actions.call('POST', '/api/sessions/new', {});
      resetSessionState();
    } catch (e) {
      if (e.status === 409) {
        store.guardMode = e.code === 'background_running' ? 'background_running' : 'busy';
        store.busyGuard = true;
        ElMessage.warning(e.code === 'background_running' ? '后台任务运行中，先停止任务再新建' : '当前任务运行中，中断或完成后再新建');
      } else {
        ElMessage.error('新建会话失败：' + e.message);
      }
    }
  },

  // ─── 计划模式（plan-decision-frontend-handoff） ───
  async refreshPlan() {
    try {
      const r = await actions.call('GET', '/api/plan');
      if (r && (r.sessionId === store.sessionId || !store.sessionId)) {
        store.planState = r.state || null;
        if (r.sessionId) store.sessionId = store.sessionId || r.sessionId;
      }
    } catch (e) {
      /* 运行中 409：沿用已有状态与事件，不覆盖 */
    }
  },

  // 回复计划问题：必须带 decisionId 关联；成功后等 SSE 渲染下一轮，不乐观清问题
  async answerPlanningDecision(answer) {
    const d = store.planState && store.planState.decision;
    if (!d || !d.awaitingReply) {
      ElMessage.warning('没有等待中的计划问题');
      return { ok: false, draft: answer };
    }
    try {
      await actions.call('POST', '/api/answer', { sessionId: store.sessionId, decisionId: d.id, answer });
      store.phase = 'streaming';
      return { ok: true };
    } catch (e) {
      if (e.code === 'decision_mismatch' || e.code === 'session_mismatch') {
        await actions.refreshPlan();
        ElMessage.warning('该问题已不再等待回复，状态已刷新');
      } else {
        ElMessage.error('回复失败：' + e.message + '（草稿已保留）');
      }
      return { ok: false, draft: answer };
    }
  },

  // 用户直接退出计划模式：只解除阶段限制与待答问题，不批准计划、不代判
  async exitPlanMode() {
    try {
      await actions.call('POST', '/api/plan/exit', { sessionId: store.sessionId });
      await actions.refreshPlan();
      ElMessage.success('已退出计划模式（不改变计划文件内容）');
    } catch (e) {
      if (e.code === 'plan_mode') {
        ElMessage.warning('退出未生效：' + e.message);
      } else if (e.status === 409) {
        ElMessage.warning('任务运行中，请先中断本轮再退出计划模式');
      } else {
        ElMessage.error(e.message);
      }
    }
  },

  // ─── 后台任务 ───
  async pullTaskOutput(taskId, all) {
    const st = store.taskOutputs[taskId] || { text: '', nextOffset: 0, totalBytes: 0 };
    try {
      for (let i = 0; i < (all ? 32 : 4); i++) {
        const r = await actions.call('GET', `/api/tasks/${encodeURIComponent(taskId)}/output?offset=${st.nextOffset}&limit=262144`);
        if (r.nextOffset <= st.nextOffset) break;
        st.text += r.content || '';
        st.nextOffset = r.nextOffset;
        st.totalBytes = r.totalBytes || st.totalBytes;
        if (!all) break;
      }
      store.taskOutputs[taskId] = st;
    } catch (e) {
      ElMessage.error(e.message);
    }
  },

  async killTask(taskId) {
    try {
      const r = await actions.call('POST', '/api/tasks/kill', { task_id: taskId });
      ElMessage.success(r.result || '终止请求已发出');
      await actions.refreshTasks();
    } catch (e) {
      ElMessage.error(e.message);
    }
  },

  // ─── 检查点 / 回滚 ───
  async rollback(to) {
    try {
      const r = await actions.call('POST', '/api/rollback', { to });
      ElMessage.success(r.result || '已回滚');
      await actions.loadCheckpoints();
    } catch (e) {
      ElMessage.error(e.message);
    }
  },

  // 工具结果/大内容分页读取：字节 offset，翻页必须用服务端 nextOffset（large-content handoff）。
  async fetchResultPage(ref, offset = 0, limit = 32768) {
    const sid = store.sessionId ? `&sessionId=${encodeURIComponent(store.sessionId)}` : '';
    return actions.call('GET', `/api/tool-result?ref=${encodeURIComponent(ref)}&offset=${offset}&limit=${limit}${sid}`);
  },

  async expandToolResult(ref) {
    try {
      const r = await actions.fetchResultPage(ref);
      return r;
    } catch (e) {
      ElMessage.error(e.message);
      return null;
    }
  },

  // ─── 配置 ───
  async saveConfig(fields) {
    const r = await actions.call('PUT', '/api/config', fields);
    store.config = r;
    return r;
  },
  async connectionTest(fields) {
    return actions.call('POST', '/api/connection-test', fields);
  },

  // ─── 内部：等待提示条 ───
  pushWaiting(phaseArg) {
    removeWhere((x) => x.type === 'waiting');
    push({ type: 'waiting', id: nid('w'), phase: phaseArg, seconds: store.waitedSeconds });
  }
};

// ─── 历史渲染（会话消息 → 与实时一致的时间轴条目） ──────────────
// 工具卡片先按“状态未知”创建；role=tool 消息带 toolOutcome 时才落定成败；
// 缺失 toolOutcome（旧记录 / 中断合成消息）保持未知，不得按成功显示（handoff §1）。
function renderHistory(messages) {
  messages.forEach((m) => {
    if (m.role === 'user') {
      push({ type: 'user', id: nid('hu'), text: m.content || '' });
      return;
    }
    if (m.role === 'assistant') {
      if (m.tool_calls && m.tool_calls.length) {
        m.tool_calls.forEach((tc, i) => {
          let args = {};
          try {
            args = JSON.parse(tc.function.arguments || '{}');
          } catch (e) {
            /* arguments 是 JSON 字符串，解析失败按原样展示 */
          }
          push({ type: 'tool', id: nid('ht'), toolId: tc.id, name: tc.function.name, args, status: 'unknown', elapsedMs: 0, summary: '', resultFull: '', expandedResult: '', argPreview: !!(m.largeContent && m.largeContent['toolcall-' + i]) });
        });
      }
      if (m.content || m.reasoning_content) {
        push({
          type: 'assistant',
          id: nid('ha'),
          text: m.content || '',
          reasoning: m.reasoning_content || '',
          model: store.config.model || '',
          streaming: false,
          toolCount: 0,
          textPreview: !!(m.largeContent && m.largeContent.content),
          reasoningPreview: !!(m.largeContent && m.largeContent.reasoning),
          lc: m.largeContent || null
        });
      }
      return;
    }
    if (m.role === 'tool' && m.tool_call_id) {
      const rec = store.timeline.filter((x) => x.type === 'tool' && x.toolId === m.tool_call_id).pop();
      if (rec) {
        rec.resultFull = m.content || '';
        rec.expandedResult = m.content || '';
        if (m.largeContent && m.largeContent.content) {
          rec.lcRef = m.largeContent.content;
          rec.resultPreview = true;
        }
        const out = m.toolOutcome;
        if (out && typeof out.ok === 'boolean') {
          rec.status = out.errorCode === 'hard_link_impact_unknown' ? 'hard_link' : out.ok ? 'success' : 'error';
          rec.summary = out.summary || rec.summary;
        }
      }
    }
  });
}

// ─── 事件消费（契约 C1.3 全量映射） ────────────────────────────
function consumeEvent(evt) {
  const d = evt.data || {};
  switch (evt.type) {
    case 'session_init':
      store.sessionId = d.sessionId || store.sessionId;
      store.workspace = d.workspace || store.workspace;
      store.context.budget = d.contextBudget || store.context.budget;
      {
        const sk = Array.isArray(d.skills) ? d.skills : [];
        store.skillsAvailable = sk.map((s) => (typeof s === 'string' ? { name: s, source: '' } : s));
      }
      push({ type: 'session_init', id: nid('si'), workspace: d.workspace || store.workspace, budget: d.contextBudget, model: d.model, tools: d.tools, skills: Array.isArray(d.skills) ? d.skills.length : d.skills });
      break;

    case 'assistant_attempt': {
      const a = store.attempts[d.attempt] || (store.attempts[d.attempt] = { text: '', reasoning: '', status: 'start' });
      if (d.status === 'start') {
        a.text = '';
        a.reasoning = '';
        a.status = 'start';
        removeWhere((x) => x.type === 'waiting' || (x.type === 'assistant' && x.attempt === d.attempt));
        push({ type: 'assistant', id: nid('a'), attempt: d.attempt, text: '', reasoning: '', model: store.config.model || '', streaming: true, toolCount: 0 });
      } else if (d.status === 'complete') {
        a.status = 'complete';
        removeWhere((x) => x.type === 'assistant' && x.attempt === d.attempt);
        if (a.text || a.reasoning) push({ type: 'assistant', id: nid('a'), attempt: d.attempt, text: a.text, reasoning: a.reasoning, model: store.config.model || '', streaming: false, toolCount: 0 });
      } else if (d.status === 'discard') {
        a.status = 'discard';
        a.text = '';
        a.reasoning = '';
        removeWhere((x) => x.type === 'assistant' && x.attempt === d.attempt);
      }
      if (d.error && a) a.error = d.error;
      break;
    }

    case 'assistant_reasoning_delta': {
      // 端点真实提供的 reasoning_content；与正文按 attempt 分别累积，discard 一并移除（handoff §2）。
      const a = store.attempts[d.attempt];
      if (a) a.reasoning = (a.reasoning || '') + (d.delta || '');
      let item = store.timeline.find((x) => x.type === 'assistant' && x.attempt === d.attempt);
      if (!item) {
        removeWhere((x) => x.type === 'waiting');
        item = { type: 'assistant', id: nid('a'), attempt: d.attempt, text: '', reasoning: '', model: store.config.model || '', streaming: true, toolCount: 0 };
        push(item);
      }
      item.reasoning = a ? a.reasoning : (item.reasoning || '') + (d.delta || '');
      if (store.phase === 'waiting_first') {
        store.phase = 'streaming';
        removeWhere((x) => x.type === 'waiting');
      }
      break;
    }

    case 'assistant_delta': {
      const a = store.attempts[d.attempt];
      if (a) a.text += d.delta || '';
      let item = store.timeline.find((x) => x.type === 'assistant' && x.attempt === d.attempt);
      if (!item) {
        removeWhere((x) => x.type === 'waiting');
        item = { type: 'assistant', id: nid('a'), attempt: d.attempt, text: '', reasoning: '', model: store.config.model || '', streaming: true, toolCount: 0 };
        push(item);
      }
      item.text = a ? a.text : (item.text || '') + (d.delta || '');
      if (store.phase === 'waiting_first') {
        store.phase = 'streaming';
        removeWhere((x) => x.type === 'waiting');
      }
      break;
    }

    case 'tool_call': {
      store.phase = 'tool_running';
      // API 直发/外部下发时页面可能仍停在空态：首个工具事件自动进入会话视图
      if (store.emptyMode) store.emptyMode = false;
      removeWhere((x) => x.type === 'waiting');
      const item = {
        type: 'tool',
        id: nid('t'),
        toolId: d.id,
        name: d.name,
        args: d.args || {},
        status: 'running',
        startedAt: Date.now(),
        elapsedMs: 0,
        summary: '',
        resultRef: null,
        resultFull: '',
        expandedResult: ''
      };
      push(item);
      if (d.name === 'shell' && d.args && d.args.command && !d.args.background) {
        store.turnCommands.push(d.args.command);
      }
      break;
    }

    case 'tool_result': {
      const rec = store.timeline.filter((x) => x.type === 'tool' && x.toolId === d.id).pop();
      if (rec) {
        rec.elapsedMs = Date.now() - (rec.startedAt || Date.now());
        rec.summary = d.summary || '';
        rec.resultRef = d.resultRef || null;
        if (d.errorCode === 'hard_link_impact_unknown') rec.status = 'hard_link';
        else if (d.errorCode === 'permission_denied' || d.errorCode === 'denied') rec.status = 'denied';
        else rec.status = d.ok ? 'success' : 'error';
        if (!d.ok && !d.errorCode && d.summary) rec.resultFull = d.summary;
      }
      if (store.phase === 'tool_running') store.phase = 'streaming';
      break;
    }

    case 'permission_request':
      store.pendingPermission = { ...d };
      push({ type: 'permission', id: nid('p'), requestId: d.requestId || '', tool: d.tool, args: d.args || {}, target: d.target || '', decision: null });
      break;

    case 'permission_response': {
      store.pendingPermission = null;
      const card = store.timeline.filter((x) => x.type === 'permission').pop();
      if (card) card.decision = d.decision || 'deny';
      break;
    }

    case 'context_state':
      store.context = { ...store.context, ...d };
      break;

    case 'compaction':
      push({
        type: 'compaction',
        id: nid('c'),
        method: d.method || 'summary',
        reason: d.reason || '',
        beforeTokens: d.beforeTokens,
        afterTokens: d.afterTokens,
        emergency: !!d.emergency,
        summary: d.summary || '',
        discardedBytes: d.discardedBytes,
        keptToolCallIds: d.keptToolCallIds || [],
        discardedToolCallIds: d.discardedToolCallIds || []
      });
      break;

    case 'planning_decision': {
      // 模型的计划问题：按 sessionId 隔离，展示完整问题原文；不推断
      if (d.sessionId === store.sessionId || !store.sessionId) {
        store.planState = store.planState || {};
        store.planState = { ...(store.planState||{}), active: true, decision: d.decision };
        store.phase = 'need_answer';
        removeWhere((x) => x.type === 'waiting');
      }
      break;
    }

    case 'plan_state': {
      if (d.sessionId === store.sessionId || !store.sessionId) store.planState = d.state || null;
      break;
    }

    case 'skill_loaded':
      if (!store.skillsUsed.includes(d.name)) store.skillsUsed.push(d.name);
      push({ type: 'skill', id: nid('sk'), name: d.name });
      break;

    case 'background_task': {
      const t = store.tasks.find((x) => x.taskId === d.taskId);
      if (d.action === 'start') {
        const nt = {
          taskId: d.taskId,
          command: d.command || (t && t.command) || '',
          pid: d.pid,
          detached: !!d.detached,
          startedAt: new Date().toISOString(),
          runtimeMs: 0,
          outputBytes: 0,
          status: d.status || 'running',
          exitCode: null,
          outputTruncated: false
        };
        const idx = store.tasks.findIndex((x) => x.taskId === d.taskId);
        if (idx >= 0) store.tasks.splice(idx, 1, nt);
        else store.tasks.unshift(nt);
        store.turnBgProcesses.push({ taskId: d.taskId, command: nt.command });
      } else if (t) {
        if (d.status) t.status = d.status;
        if (d.exitCode !== undefined) t.exitCode = d.exitCode;
        if (d.action === 'output_limit') {
          t.outputTruncated = true;
          if (t.status === 'running') t.status = 'output_truncated';
        }
        if (d.action === 'output_error') t.outputError = d.error || '观察停止，任务未终止';
      }
      // 事件声明的字节区间内主动拉取（不定时轮询）
      if (d.action === 'output' && typeof d.nextOffset === 'number') {
        const st = store.taskOutputs[d.taskId] || (store.taskOutputs[d.taskId] = { text: '', nextOffset: d.offset || 0, totalBytes: 0 });
        const target = d.nextOffset;
        const pull = async () => {
          try {
            for (let guard = 0; guard < 64 && st.nextOffset < target; guard++) {
              const r = await actions.call('GET', `/api/tasks/${encodeURIComponent(d.taskId)}/output?offset=${st.nextOffset}&limit=262144`);
              if (r.nextOffset <= st.nextOffset) break;
              st.text += r.content || '';
              st.nextOffset = r.nextOffset;
            }
          } catch (e) {
            /* 下次事件再补 */
          }
        };
        pull();
      }
      actions.refreshTasks();
      break;
    }

    case 'heartbeat':
      store.waitedSeconds = d.waitedSeconds || 0;
      if (store.phase === 'waiting_first' || store.phase === 'streaming') {
        const w = store.timeline.find((x) => x.type === 'waiting' && x.phase === 'waiting_first');
        if (w) w.seconds = store.waitedSeconds;
        else actions.pushWaiting('waiting_first');
      }
      break;

    case 'turn_result': {
      const elapsedMs = store.turnStartedAt ? Date.now() - store.turnStartedAt : 0;
      removeWhere((x) => x.type === 'waiting');
      push({
        type: 'turn_result',
        id: nid('r'),
        status: d.status,
        error: d.error || '',
        elapsedMs,
        commands: [...store.turnCommands],
        outsideWrites: [...store.turnOutsideWrites],
        bgProcesses: [...store.turnBgProcesses]
      });
      store.phase = d.status === 'need_answer' ? 'need_answer' : 'idle';
      store.pendingPermission = null;
      store.waitedSeconds = 0;
      actions.refreshTasks();
      actions.loadCheckpoints();
      actions.refreshPlan();
      break;
    }

    case 'outside_workspace_write': {
      push({ type: 'outside_write', id: nid('ow'), tool: d.tool, requestedPath: d.requestedPath, resolvedPath: d.resolvedPath });
      store.turnOutsideWrites.push({ path: d.resolvedPath || d.requestedPath, tool: d.tool });
      break;
    }

    case 'process':
      push({ type: 'process', id: nid('pr'), action: d.action, process: d.process || {} });
      if (d.action === 'created' || d.action === 'ended' || d.action === 'terminated') actions.refreshTasks();
      break;

    case 'process_mode':
      store.overview.mode = d;
      store.alertRestricted = !!d.restricted && !d.cleanupGuaranteed;
      actions.refreshTasks();
      break;

    case 'process_warning': {
      push({ type: 'process_warning', id: nid('pw'), kind: d.kind, current: d.current, threshold: d.threshold, taskId: d.taskId });
      if (d.kind === 'process_count') {
        store.overview.count = d.current;
        store.overview.threshold = d.threshold;
        store.overview.exceeded = d.current > d.threshold;
      }
      break;
    }

    case 'process_count_error':
      store.overview.error = d.error || '进程计数失败';
      break;

    case 'detached_history':
      store.detachedHistory = [{ verified: false, notice: d.notice || '' }, ...(store.detachedHistory || [])];
      break;

    default:
      break;
  }
}

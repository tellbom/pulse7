// 演示后端 — 当页面未被 pulse7 serve 提供（无有效 token）时启用。
// 严格按 artifacts/api-contract.md 的 DTO/事件包络产出数据，脚本化一轮完整任务，
// 覆盖定稿原型的全部界面状态：流式输出、attempt 丢弃、工具成功/硬链接拒绝、
// 工作区外写入、上下文截断、进程告警、心跳等待、后台任务、need_answer、中断、权限确认。

const delay = (ms) => new Promise((r) => setTimeout(r, ms));
const now = () => Date.now();
const pad = (n) => String(n).padStart(2, '0');
const hms = (d) => `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
const todayAt = (h, m) => {
  const d = new Date();
  d.setHours(h, m, 0, 0);
  return d.toISOString();
};
const daysAgoAt = (days, h, m) => {
  const d = new Date();
  d.setDate(d.getDate() - days);
  d.setHours(h, m, 0, 0);
  return d.toISOString();
};

const WORKSPACES = [
  'E:\\projects\\auth-service',
  'E:\\projects\\data-pipeline',
  'E:\\projects\\legacy-api',
  'E:\\projects\\frontend'
];

const S_ = {
  config: {
    base_url: 'http://192.168.1.100:8080/v1',
    model: 'gpt-4o-internal',
    apiKeyConfigured: true,
    configSource: '用户全局配置（%USERPROFILE%\\.pulse7\\config.json）',
    workspace: 'E:\\projects\\auth-service',
    max_ctx: 256000,
    read_only: false,
    max_rounds: 100,
    shell_timeout_sec: 120,
    memory_limit_mb: 2048,
    process_warn_threshold: 50,
    background_task_max_output_mb: 50,
    background_task_warn_count: 5,
    background_task_warn_sec: 3600,
    cleanup_on_exit: true,
    llm_first_chunk_timeout_sec: 300,
    llm_idle_timeout_sec: 120,
    llm_max_retries: 2,
    llm_compress_timeout_sec: 180,
    sandbox_preference: 'auto'
  },
  listener: { address: '127.0.0.1', port: 8080, listening: true, tokenValid: true },
  permissions: {
    profile: 'standard',
    rules: [
      { tool: 'write_file', pattern: 'E:\\projects\\**', action: 'allow' },
      { tool: 'shell', pattern: '*', action: 'ask' }
    ],
    readOnly: false
  },
  sessions: [
    { sessionId: 's-001', cwd: 'E:\\projects\\auth-service', updatedAt: todayAt(9, 41), messageCount: 14, firstUser: '检查 handler.go 里的 JWT 验证逻辑，看看有没有问题' },
    { sessionId: 's-002', cwd: 'E:\\projects\\data-pipeline', updatedAt: todayAt(8, 12), messageCount: 8, firstUser: '优化 Kafka 消费者并发模型，当前吞吐量约 3k/s 不达标' },
    { sessionId: 's-003', cwd: 'E:\\projects\\legacy-api', updatedAt: daysAgoAt(1, 17, 5), messageCount: 21, firstUser: 'legacy-api 里有几个接口响应超 2s，帮我找出瓶颈' },
    { sessionId: 's-004', cwd: 'E:\\projects\\frontend', updatedAt: daysAgoAt(2, 11, 30), messageCount: 6, firstUser: '把登录页的表单校验改成 zod schema 验证，并加 toast 提示' },
    { sessionId: 's-005', cwd: 'E:\\projects\\infra', updatedAt: daysAgoAt(4, 9, 15), messageCount: 33, firstUser: '写一份 Terraform 模块把现有 EC2 迁移到 ECS Fargate' }
  ],
  checkpoints: [
    { taskId: 'demo', seq: 3, createdAt: hms(new Date(new Date().setHours(9, 40, 12))), commit: 'a3f8d2c', kind: 'auto', dirtyFiles: 2, dirtyFilesBasis: '工作区相对 HEAD 的变更文件数' },
    { taskId: 'demo', seq: 2, createdAt: hms(new Date(new Date().setHours(9, 33, 5))), commit: '91be44a', kind: 'model', dirtyFiles: 0, dirtyFilesBasis: '工作区相对 HEAD 的变更文件数' },
    { taskId: 'demo', seq: 1, createdAt: hms(new Date(new Date().setHours(9, 18, 50))), commit: '7c2a189', kind: 'auto', dirtyFiles: null, dirtyFilesBasis: '' }
  ],
  tasks: [
    { taskId: 'LUID-0x1a4', command: 'go test ./... -v -run TestAuth', pid: 8214, detached: false, startedAt: new Date(now() - 183000).toISOString(), runtimeMs: 183200, outputBytes: 48291, status: 'running', exitCode: null, outputTruncated: false },
    { taskId: 'LUID-0x1a1', command: 'npm run build:prod', pid: 6102, detached: false, startedAt: new Date(now() - 941000).toISOString(), runtimeMs: 941000, outputBytes: 52428800, status: 'output_truncated', exitCode: null, outputTruncated: true }
  ],
  processes: [
    { pid: 8214, creationTime: '13340128741991234', imagePath: 'C:\\Go\\bin\\go.exe', command: 'go test ./... -v -run TestAuth', startedAt: new Date(now() - 183000).toISOString(), status: 'running', source: 'task', taskId: 'LUID-0x1a4', detached: false, sessionManaged: true },
    { pid: 8230, creationTime: '13340128751004567', imagePath: 'C:\\Go\\pkg\\tool\\windows_amd64\\compile.exe', command: 'compile -p ./src', startedAt: new Date(now() - 180000).toISOString(), status: 'running', source: 'task', taskId: 'LUID-0x1a4', detached: false, sessionManaged: true, parentPid: 8214 }
  ],
  detachedHistory: [
    { verified: false, notice: 'pid 7432 · cmd.exe /c run-nightly-sync.bat（将在 pulse7 退出后继续运行；历史记录，未经核实）' },
    { verified: false, notice: 'pid 7891 · node.exe server.js（历史记录，未经核实，可能已退出）' }
  ],
  overview: {
    count: 52,
    threshold: 50,
    exceeded: true,
    error: '',
    mode: {
      cleanupScope: 'session_job',
      descendantsMayEscape: true,
      mode: 'jobobject',
      restricted: true,
      cleanupGuaranteed: false,
      reason: 'Session Job 分配失败（Win7 受限模式）',
      notice: '已进入受限模式：仅显式 detached 与 Job 成员可见，第三方后代可自行脱离。'
    }
  },
  skillsUsed: ['git-ops'],
  toolsCount: 18,
  skillsAvailable: [
    { name: 'git-ops', source: '项目级', active: true },
    { name: 'code-search', source: '个人级', active: true },
    { name: 'file-diff', source: '项目级', active: false }
  ]
};

const TASK_OUTPUT_TEXT = [
  '=== RUN   TestAuthHandler_ValidToken\n--- PASS: TestAuthHandler_ValidToken (0.01s)\n',
  '=== RUN   TestAuthHandler_ExpiredToken\n    handler_test.go:84: expired token accepted, want 401\n--- FAIL: TestAuthHandler_ExpiredToken (0.00s)\n',
  '=== RUN   TestRateLimiter\n--- PASS: TestRateLimiter (2.31s)\n',
  'FAIL\nFAIL\tgithub.com/internal/auth-service/src\t3.882s\nFAIL\n'
];

const FULL_RESULTS = {
  'tc-001': 'package handler\n\nimport "github.com/gin-gonic/gin"\n\nfunc AuthHandler(c *gin.Context) {\n    token := c.GetHeader("Authorization")\n    // ……（共 312 行）',
  'tc-002': '$ go vet ./src/...\n# 无输出（退出码 0）',
  'tc-004': '已写入 middleware.go (87 行)',
  'tc-run': '$ go test ./src/... -v\nPASS ok  github.com/internal/auth-service/src 3.882s'
};

const HISTORY_S001 = buildHistory();

function buildHistory() {
  const t = (h, m, s) => {
    const d = new Date();
    d.setHours(h, m, s);
    return d.toISOString();
  };
  const msgs = [
    { role: 'user', content: '检查 handler.go 里的 JWT 验证逻辑，看看有没有问题，顺便在 middleware.go 里补一个限流中间件', timestamp: t(9, 38, 2) },
    {
      role: 'assistant',
      content: '',
      reasoning_content: '用户要求审查 JWT 验证并补限流。先读 handler.go 了解现状，再决定是否需要修改。',
      tool_calls: [
        { id: 'tc-001', type: 'function', function: { name: 'read_file', arguments: '{"path":"E:\\\\projects\\\\auth-service\\\\src\\\\handler.go"}' } },
        { id: 'tc-002', type: 'function', function: { name: 'shell', arguments: '{"command":"go vet ./src/..."}' } },
        { id: 'tc-old', type: 'function', function: { name: 'grep', arguments: '{"pattern":"jwt"}' } }
      ],
      timestamp: t(9, 38, 5)
    },
    { role: 'tool', tool_call_id: 'tc-001', content: FULL_RESULTS['tc-001'], toolOutcome: { ok: true, summary: '读取 312 行' }, timestamp: t(9, 38, 5) },
    { role: 'tool', tool_call_id: 'tc-002', content: FULL_RESULTS['tc-002'], toolOutcome: { ok: false, summary: 'exit 1：vet 报告 2 处问题' }, timestamp: t(9, 38, 8) },
    {
      role: 'assistant',
      content: 'handler.go 的 JWT 验证有两个问题：\n\n1. 没有验证 `exp` 字段——过期 token 仍然有效\n2. 错误时返回 HTTP 200（应为 401）\n\n我来给 middleware.go 写限流中间件，同时修正这两个问题。',
      reasoning_content: 'vet 的输出与直接读码一致。问题明确：exp 未校验、错误码误用 200。下一步写 middleware。',
      timestamp: t(9, 39, 12)
    },
    {
      role: 'assistant',
      content: '限流中间件已写入 middleware.go，采用滑动窗口算法，每 IP 每分钟 60 次请求。JWT 验证已修正 `exp` 检查和状态码。',
      timestamp: t(9, 40, 40)
    }
  ];
  // 注意：tc-old 没有对应 role=tool 消息（旧记录缺结果）→ 前端按“状态未知”显示。
  return msgs.map((m, i) => ({
    uuid: `hist-${i}`,
    parentUuid: i ? `hist-${i - 1}` : null,
    timestamp: m.timestamp,
    sessionId: 's-001',
    cwd: 'E:\\projects\\auth-service',
    version: 1,
    ...m
  }));
}

function err(status, code, message) {
  return Promise.reject({ status, code, message });
}

export function createMockBackend() {
  const listeners = new Set();
  let script = null; // { aborted, interrupt, permissionGate }
  let nextSessionId = 's-006';
  let scriptsRun = 0;
  let pendingPermission = null;

  const emit = (type, data) => {
    listeners.forEach((cb) => cb({ type, data }));
  };

  const busy = () => script && !script.aborted;

  function makeScript() {
    const s = {
      aborted: false,
      interrupted: false,
      permissionGate: null,
      wait: async function (ms) {
        const step = 80;
        let left = ms;
        while (left > 0) {
          if (this.aborted) throw new Error('aborted');
          await delay(Math.min(step, left));
          left -= step;
        }
        if (this.aborted) throw new Error('aborted');
      }
    };
    s.interrupt = () => {
      s.interrupted = true;
      s.aborted = true;
      if (s.permissionGate) s.permissionGate.rejectDeny && s.permissionGate.rejectDeny('interrupted');
    };
    return s;
  }

  async function streamDeltas(s, attempt, text, chunk = 3) {
    for (let i = 0; i < text.length; i += chunk) {
      await s.wait(24);
      emit('assistant_delta', { delta: text.slice(i, i + chunk), attempt });
    }
  }

  async function streamReasoning(s, attempt, text, chunk = 6) {
    for (let i = 0; i < text.length; i += chunk) {
      await s.wait(30);
      emit('assistant_reasoning_delta', { delta: text.slice(i, i + chunk), attempt });
    }
  }

  // 主演示脚本：完整复现定稿原型中的对话流。
  async function runShowcase(s, sessionId) {
    emit('session_init', {
      sessionId,
      workspace: S_.config.workspace,
      model: S_.config.model,
      tools: S_.toolsCount,
      skills: S_.skillsAvailable.map((x) => ({ name: x.name, source: x.source })),
      contextBudget: 64000
    });
    emit('context_state', { usedTokens: 21000, budget: 64000, percentLeft: 67.2, warningLevel: 'normal' });
    await s.wait(300);

    emit('assistant_attempt', { attempt: 1, status: 'start' });
    await streamDeltas(s, 1, '好的，我先读取 handler.go 看看当前的 JWT 验证实现，分析是否有问题，再在 middleware.go 里添加限流中间件。');
    emit('assistant_attempt', { attempt: 1, status: 'complete' });
    await s.wait(220);

    emit('skill_loaded', { name: 'git-ops', path: 'E:\\projects\\auth-service\\.pulse7\\skills\\git-ops' });

    emit('tool_call', { id: 'tc-001', name: 'read_file', args: { path: 'E:\\projects\\auth-service\\src\\handler.go' } });
    await s.wait(420);
    emit('tool_result', { id: 'tc-001', ok: true, summary: '读取 312 行', resultRef: `tool:tc-001` });

    emit('tool_call', { id: 'tc-002', name: 'shell', args: { command: 'go vet ./src/...' } });
    emit('process', { action: 'created', process: { pid: 9102, creationTime: '13340129120099123', imagePath: 'C:\\Go\\bin\\go.exe', command: 'go vet ./src/...', startedAt: new Date().toISOString(), status: 'running', source: 'task', taskId: 'fg', detached: false, sessionManaged: true } });
    await s.wait(900);
    emit('tool_result', { id: 'tc-002', ok: true, summary: '退出码 0', resultRef: 'tool:tc-002' });
    emit('process', { action: 'ended', process: { pid: 9102, creationTime: '13340129120099123', imagePath: 'C:\\Go\\bin\\go.exe', status: 'exited' } });

    await s.wait(260);
    emit('assistant_attempt', { attempt: 2, status: 'start' });
    // 演示 attempt 丢弃（F2）：第 2 次尝试先输出推理再输出正文片段，discard 后两者都必须从界面撤除。
    await streamReasoning(s, 2, '先看 middleware.go 的现有结构，评估插入点。');
    await streamDeltas(s, 2, '我先看一下 middle');
    emit('assistant_attempt', { attempt: 2, status: 'discard' });
    await s.wait(300);
    emit('assistant_attempt', { attempt: 3, status: 'start' });
    await streamReasoning(s, 3, 'vet 结果与读码一致：exp 未校验、错误码误用 200。先陈述问题，再写限流中间件。');
    await streamDeltas(s, 3, 'handler.go 的 JWT 验证有两个问题：\n\n1. 没有验证 `exp` 字段——过期 token 仍然有效\n2. 错误时返回 HTTP 200（应为 401）\n\n我来给 middleware.go 写限流中间件，同时修正这两个问题。');
    emit('assistant_attempt', { attempt: 3, status: 'complete' });
    await s.wait(240);

    emit('tool_call', { id: 'tc-003', name: 'write_file', args: { path: 'E:\\projects\\auth-service\\src\\middleware.go', content: '……' } });
    await s.wait(360);
    emit('tool_result', { id: 'tc-003', ok: false, summary: 'hard_link_impact_unknown', resultRef: 'tool:tc-003', errorCode: 'hard_link_impact_unknown' });

    await s.wait(200);
    emit('assistant_attempt', { attempt: 4, status: 'start' });
    await streamDeltas(s, 4, 'middleware.go 存在硬链接，直接写入会影响未知路径。我先检查链接数量再决定操作方式。');
    emit('assistant_attempt', { attempt: 4, status: 'complete' });

    emit('tool_call', { id: 'tc-004', name: 'write_file', args: { path: 'E:\\projects\\auth-service\\src\\middleware.go', content: '……87 行……' } });
    await s.wait(380);
    emit('tool_result', { id: 'tc-004', ok: true, summary: '已写入 middleware.go (87 行)', resultRef: 'tool:tc-004' });
    emit('outside_workspace_write', { tool: 'write_file', requestedPath: 'C:\\Users\\admin\\AppData\\Local\\auth-service\\rate-limit.log', resolvedPath: 'C:\\Users\\admin\\AppData\\Local\\auth-service\\rate-limit.log', checkpointCovered: false, rollbackCovered: false });

    emit('compaction', { reason: 'threshold', beforeTokens: 63200, afterTokens: 39600, emergency: true, summary: 'original_bytes=252800 discarded_bytes=146400', method: 'truncate', removedMessages: 9 });
    emit('context_state', { usedTokens: 39600, budget: 64000, percentLeft: 38.1, warningLevel: 'normal' });
    emit('process_warning', { kind: 'process_count', current: 52, threshold: 50, blocked: false });
    await s.wait(260);

    emit('assistant_attempt', { attempt: 5, status: 'start' });
    await streamReasoning(s, 5, '写入已完成，测试仍在后台跑；总结改动即可收尾。');
    await streamDeltas(s, 5, '限流中间件已写入 middleware.go，采用滑动窗口算法，每 IP 每分钟 60 次请求。JWT 验证已修正 `exp` 检查和状态码。');
    emit('assistant_attempt', { attempt: 5, status: 'complete' });

    // 后台任务：启动 go test，随后产生可读输出区间（已存在则不重复登记）。
    if (!S_.tasks.some((t) => t.taskId === 'LUID-0x1a4')) {
      S_.tasks.unshift({ taskId: 'LUID-0x1a4', command: 'go test ./... -v -run TestAuth', pid: 8214, detached: false, startedAt: new Date().toISOString(), runtimeMs: 0, outputBytes: 0, status: 'running', exitCode: null, outputTruncated: false });
    }
    emit('background_task', { action: 'start', taskId: 'LUID-0x1a4', pid: 8214, command: 'go test ./... -v -run TestAuth', detached: false, status: 'running' });
    await s.wait(400);
    let off = 0;
    for (let i = 1; i < 4; i++) {
      await s.wait(420);
      const chunkText = TASK_OUTPUT_TEXT[i];
      taskText['LUID-0x1a4'] = (taskText['LUID-0x1a4'] || '') + chunkText;
      off = taskText['LUID-0x1a4'].length;
      S_.tasks.forEach((t) => {
        if (t.taskId === 'LUID-0x1a4') t.outputBytes = off;
      });
      emit('background_task', { action: 'output', taskId: 'LUID-0x1a4', offset: off - chunkText.length, nextOffset: off });
    }

    emit('turn_result', { status: 'success' });
  }

  // 次轮脚本：后台 shell + 心跳等待 + need_answer 展示。
  async function runSecondTurn(s, sessionId) {
    emit('tool_call', { id: 'tc-run', name: 'shell', args: { command: 'go test ./src/... -v', background: true } });
    emit('background_task', { action: 'start', taskId: 'LUID-0x1b2', pid: 9340, command: 'go test ./src/... -v', detached: false, status: 'running' });
    if (!S_.tasks.some((t) => t.taskId === 'LUID-0x1b2')) {
      S_.tasks.unshift({ taskId: 'LUID-0x1b2', command: 'go test ./src/... -v', pid: 9340, detached: false, startedAt: new Date().toISOString(), runtimeMs: 0, outputBytes: 0, status: 'running', exitCode: null, outputTruncated: false });
    }
    for (let i = 0; i < 3; i++) {
      await s.wait(5000);
      emit('heartbeat', { waitedSeconds: 15 * (i + 1) });
    }
    // 心跳后进入 need_answer：等待用户回答（或中断）。
    emit('assistant_attempt', { attempt: 1, status: 'start' });
    await streamDeltas(s, 1, '测试仍在后台运行。在等待期间我发现 middleware.go 的限流配置需要确认：配额按 IP 还是按用户维度统计？');
    emit('assistant_attempt', { attempt: 1, status: 'complete' });
    emit('turn_result', { status: 'need_answer' });
  }

  async function runAnswer(s, sessionId) {
    emit('assistant_attempt', { attempt: 1, status: 'start' });
    await streamDeltas(s, 1, '收到，按 IP 维度统计。我把 NewRateLimiter 的 key 函数改为 c.ClientIP()，测试继续在后台跑，结果出来我会汇总。');
    emit('assistant_attempt', { attempt: 1, status: 'complete' });
    emit('turn_result', { status: 'success' });
  }

  async function startTurn(prompt, isAnswer) {
    const sessionId = S_.currentSessionId || (S_.currentSessionId = nextSessionId);
    const s = makeScript();
    script = s;
    const finish = () => {
      script = null;
    };
    try {
      if (isAnswer) {
        await runAnswer(s, sessionId);
      } else if (scriptsRun === 0) {
        scriptsRun = 1;
        await runShowcase(s, sessionId);
      } else if (scriptsRun === 1) {
        scriptsRun = 2;
        await runSecondTurn(s, sessionId);
      } else {
        // 常规脚本：短回复 + 一个前台工具 + 结束。
        emit('assistant_attempt', { attempt: 1, status: 'start' });
        await s.wait(1800);
        emit('heartbeat', { waitedSeconds: 15 });
        await streamDeltas(s, 1, `已收到任务：「${prompt.slice(0, 40)}${prompt.length > 40 ? '…' : ''}」。` + (S_.permissions.profile !== 'open' && /写入|修改|删除/.test(prompt) ? '这个操作涉及工作区写入，需要你确认。' : '我先看一下相关文件再继续。'));
        emit('assistant_attempt', { attempt: 1, status: 'complete' });
        if (S_.permissions.profile !== 'open' && /写入|修改|删除/.test(prompt)) {
          const requestId = 'pr-' + Date.now();
          pendingPermission = { requestId, resolve: null };
          const gate = new Promise((resolve) => {
            pendingPermission.resolve = resolve;
          });
          s.permissionGate = pendingPermission;
          emit('permission_request', { requestId, tool: 'write_file', args: { path: 'E:\\projects\\auth-service\\src\\middleware.go' }, target: 'E:\\projects\\auth-service\\src\\middleware.go' });
          const decision = await gate;
          if (decision === 'allow') {
            emit('permission_response', { tool: 'write_file', decision: 'allow', source: 'user', requested: 'E:\\projects\\auth-service\\src\\middleware.go', target: 'E:\\projects\\auth-service\\src\\middleware.go' });
            emit('tool_call', { id: 'tc-w1', name: 'write_file', args: { path: 'E:\\projects\\auth-service\\src\\middleware.go' } });
            await s.wait(420);
            emit('tool_result', { id: 'tc-w1', ok: true, summary: '已写入 middleware.go', resultRef: 'tool:tc-w1' });
          } else {
            emit('permission_response', { tool: 'write_file', decision: 'deny', source: 'user', requested: 'E:\\projects\\auth-service\\src\\middleware.go', target: 'E:\\projects\\auth-service\\src\\middleware.go' });
            emit('tool_call', { id: 'tc-w1', name: 'write_file', args: { path: 'E:\\projects\\auth-service\\src\\middleware.go' } });
            await s.wait(200);
            emit('tool_result', { id: 'tc-w1', ok: false, summary: '用户拒绝了本次写入', resultRef: 'tool:tc-w1' });
          }
          pendingPermission = null;
        }
        emit('turn_result', { status: 'success' });
      }
    } catch (e) {
      if (s.interrupted) {
        emit('turn_result', { status: 'interrupted' });
      } else {
        emit('turn_result', { status: 'error', error: String(e && e.message ? e.message : e) });
      }
    }
    finish();
    return sessionId;
  }

  const taskText = {
    'LUID-0x1a4': TASK_OUTPUT_TEXT[0],
    'LUID-0x1a1': '> vite build v5.0.0\ntransforming...\n' + '\u2713 1427 modules transformed.\ndist/assets/index-9f2c1a.js  927.86 kB | gzip: 308.13 kB\n'
  };

  const api = async (method, path, body) => {
    await delay(60);
    const url = new URL(path, 'http://mock.local');
    const seg = url.pathname.split('/').filter(Boolean); // ['api', ...]
    if (seg[0] !== 'api') return err(404, 'not_found', '未知接口');
    const rest = seg.slice(1);

    if (method === 'GET' && rest[0] === 'config') return { ...S_.config };
    if (method === 'PUT' && rest[0] === 'config') {
      if (busy()) return err(409, 'busy', '当前有任务执行中，配置暂不可修改');
      const RUNTIME_KEYS = ['max_ctx', 'max_rounds', 'shell_timeout_sec', 'memory_limit_mb', 'process_warn_threshold', 'background_task_max_output_mb', 'background_task_warn_count', 'background_task_warn_sec', 'cleanup_on_exit', 'read_only', 'llm_first_chunk_timeout_sec', 'llm_idle_timeout_sec', 'llm_max_retries', 'llm_compress_timeout_sec', 'sandbox_preference'];
      const RESTART_KEYS = ['shell_timeout_sec', 'memory_limit_mb', 'process_warn_threshold', 'background_task_max_output_mb', 'background_task_warn_count', 'background_task_warn_sec', 'read_only', 'cleanup_on_exit', 'sandbox_preference'];
      const saved = {};
      ['base_url', 'model', ...RUNTIME_KEYS].forEach((k) => {
        if (body && body[k] !== undefined) {
          if (k === 'api_key') return;
          if (k === 'base_url' || k === 'model') S_.config[k] = body[k];
          saved[k] = body[k];
        }
      });
      if (body && body.api_key !== undefined) {
        S_.config.apiKeyConfigured = true;
      }
      RUNTIME_KEYS.forEach((k) => {
        if (saved[k] !== undefined && k !== 'base_url' && k !== 'model') S_.config[k] = saved[k];
      });
      const restartRequiredFields = RESTART_KEYS.filter((k) => saved[k] !== undefined);
      return { ...S_.config, saved: { ...saved }, restartRequired: restartRequiredFields.length > 0, restartRequiredFields };
    }
    if (method === 'POST' && rest[0] === 'connection-test') {
      await delay(1000);
      const baseUrl = (body && body.base_url) || S_.config.base_url;
      const model = (body && body.model) || S_.config.model;
      if (!baseUrl || !/^https?:\/\//.test(baseUrl)) return err(502, 'endpoint_unreachable', '连接失败：端点地址无效');
      return { ok: true, model, elapsedMs: 412 };
    }
    if (method === 'GET' && rest[0] === 'listener') return { ...S_.listener };
    if (method === 'PUT' && rest[0] === 'workspace') {
      if (S_.tasks.some((t) => t.status === 'running' || t.status === 'output_truncated')) return err(409, 'background_running', '后台任务仍在运行，先停止任务再切换工作区');
      if (busy()) return err(409, 'busy', '当前有任务执行中，不能切换工作区');
      if (!body || !body.path) return err(400, 'invalid', '缺少 path');
      S_.config.workspace = body.path;
      S_.currentSessionId = '';
      return { workspace: body.path };
    }
    if (method === 'GET' && rest[0] === 'tasks' && rest[2] === 'output') {
      const task = S_.tasks.find((t) => t.taskId === rest[1]);
      if (!task) return err(404, 'not_found', '任务不存在');
      const offset = Number(url.searchParams.get('offset') || 0);
      const limit = Number(url.searchParams.get('limit') || 4096);
      const text = taskText[task.taskId] || '';
      if (!(offset >= 0) || offset > text.length) return err(400, 'invalid', 'offset 越界');
      const nextOffset = Math.min(text.length, offset + limit);
      return { taskId: task.taskId, status: task.status, exitCode: task.exitCode, offset, nextOffset, totalBytes: Math.max(task.outputBytes, text.length), content: text.slice(offset, nextOffset) };
    }
    if (method === 'GET' && rest[0] === 'tasks') {
      const tasks = S_.tasks.map((t) => ({
        ...t,
        runtimeMs: t.status === 'running' || t.status === 'output_truncated' ? Math.max(t.runtimeMs, now() - new Date(t.startedAt).getTime()) : t.runtimeMs
      }));
      return { tasks, processes: S_.processes, detachedHistory: S_.detachedHistory, overview: S_.overview };
    }
    if (method === 'POST' && rest[0] === 'tasks' && rest[1] === 'kill') {
      const task = S_.tasks.find((t) => t.taskId === (body && body.task_id));
      if (!task) return err(404, 'not_found', '任务不存在');
      task.status = 'killed';
      return { result: `已请求终止 ${task.taskId}（终止请求已接收，不等于进程已消失）` };
    }
    if (method === 'GET' && rest[0] === 'checkpoints') return { checkpoints: S_.checkpoints.map((c) => ({ ...c })) };
    if (method === 'POST' && rest[0] === 'rollback') {
      const target = S_.checkpoints.find((c) => c.seq === (body && Number(body.to)));
      if (!target) return err(404, 'not_found', '检查点不存在');
      return { result: `已回滚到 #${target.seq} · ${target.commit}（工作区文件已恢复，分支 HEAD 未改变）` };
    }
    if (method === 'GET' && rest[0] === 'permissions') return JSON.parse(JSON.stringify(S_.permissions));
    if (method === 'PUT' && rest[0] === 'permissions') {
      if (!['strict', 'standard', 'open'].includes(body && body.profile)) return err(400, 'invalid', '无效档位');
      S_.permissions.profile = body.profile;
      return JSON.parse(JSON.stringify(S_.permissions));
    }
    if (method === 'GET' && rest[0] === 'sessions' && rest[2] === 'messages') {
      if (rest[1] !== 's-001') return { sessionId: rest[1], messages: [], offset: 0, nextOffset: 0, hasMore: false };
      const offset = Number(url.searchParams.get('offset') || 0);
      const limit = Number(url.searchParams.get('limit') || 100);
      const page = HISTORY_S001.slice(offset, offset + limit);
      return { sessionId: rest[1], messages: page, offset, nextOffset: offset + page.length, hasMore: offset + page.length < HISTORY_S001.length };
    }
    if (method === 'GET' && rest[0] === 'sessions') {
      return { sessions: S_.sessions.filter((s) => s.sessionId !== S_.currentSessionId || true), errors: [] };
    }
    if (method === 'POST' && rest[0] === 'sessions' && rest[1] === 'resume') {
      if (busy()) return err(409, 'busy', '当前有任务执行中，先中断或等待完成');
      const s = S_.sessions.find((x) => x.sessionId === (body && body.sessionId));
      if (!s) return err(404, 'not_found', '会话不存在');
      S_.currentSessionId = s.sessionId;
      S_.config.workspace = s.cwd;
      return { sessionId: s.sessionId, cwd: s.cwd };
    }
    if (method === 'POST' && rest[0] === 'turns') {
      if (busy()) return err(409, 'busy', '同一时刻仅一个执行上下文');
      if (!body || !body.prompt || !body.prompt.trim()) return err(400, 'invalid', '空 prompt');
      const sid = S_.currentSessionId || nextSessionId;
      S_.currentSessionId = sid;
      startTurn(body.prompt, false);
      return { sessionId: sid };
    }
    if (method === 'POST' && rest[0] === 'answer') {
      if (busy()) return err(409, 'busy', '当前有任务执行中');
      if (!body || !body.answer || !body.answer.trim()) return err(400, 'invalid', '空回答');
      startTurn(body.answer, true);
      return { sessionId: body.sessionId || S_.currentSessionId };
    }
    if (method === 'POST' && rest[0] === 'interrupt') {
      if (!busy()) return err(409, 'no_active_turn', '没有活动轮次');
      script.interrupt();
      return { requested: true };
    }
    if (method === 'POST' && rest[0] === 'permission') {
      if (!pendingPermission || (body && body.requestId && body.requestId !== pendingPermission.requestId)) {
        return err(409, 'conflict', '没有等待中的确认，或 requestId 不匹配');
      }
      const decision = body && body.decision === 'allow' ? 'allow' : 'deny';
      pendingPermission.resolve(decision);
      return { accepted: true };
    }
    if (method === 'GET' && rest[0] === 'tool-result') {
      const ref = url.searchParams.get('ref') || '';
      const m = ref.match(/^(?:([^#]+)#)?tool:(.+)$/);
      if (!m) return err(404, 'not_found', '无法解析 ref');
      const result = FULL_RESULTS[m[2]];
      if (!result) return err(404, 'not_found', '结果未持久化');
      const offset = Number(url.searchParams.get('offset') || 0);
      const limit = Math.min(Number(url.searchParams.get('limit') || 32768), 262144);
      const nextOffset = Math.min(result.length, offset + limit);
      return { id: m[2], result: result.slice(offset, nextOffset), offset, nextOffset, totalBytes: result.length, hasMore: nextOffset < result.length, contentRef: '' };
    }
    return err(404, 'not_found', `未实现 ${method} ${path}`);
  };

  const connect = (cb) => {
    listeners.add(cb);
    // 初始常驻状态：受限模式横幅 + 历史遗留 detached 清单 + 首个 context_state。
    delay(200).then(() => {
      emit('process_mode', S_.overview.mode);
      emit('detached_history', { verified: false, notice: '上次会话遗留 2 个 detached 进程（未经核实）：pid 7432 · cmd.exe；pid 7891 · node.exe' });
      emit('context_state', { usedTokens: 49000, budget: 64000, percentLeft: 23.4, warningLevel: 'warning' });
    });
    return {
      close() {
        listeners.delete(cb);
      }
    };
  };

  return { api, connect, get state() { return S_; } };
}

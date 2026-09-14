import { useState, useRef, useEffect } from "react";

// ─── Types ────────────────────────────────────────────────────────────────────

type TurnStatus = "running" | "streaming" | "tool_running" | "waiting" | "done" | "error" | "interrupted" | "need_answer";
type ToolStatus = "running" | "success" | "error" | "denied" | "hard_link";
type PermTier = "strict" | "standard" | "open";

interface ToolCallItem {
  id: string;
  name: string;
  args: Record<string, unknown>;
  status: ToolStatus;
  elapsedMs: number;
  resultFull: string;
}

interface BgTask {
  taskId: string;
  command: string;
  runtimeMs: number;
  outputBytes: number;
  status: "running" | "exited" | "killed" | "output_truncated";
  exitCode: number | null;
  outputTruncated: boolean;
}

interface Checkpoint {
  seq: number;
  createdAt: string;
  commit: string;
  kind: "auto" | "model";
  dirtyFiles: number | null;
}

interface Session {
  id: string;
  cwd: string;
  label: string;
  firstMsg: string;
  msgCount: number;
  updatedAt: string;
  group: "today" | "yesterday" | "earlier";
}

// ─── Mock Data ────────────────────────────────────────────────────────────────

const MOCK_SESSIONS: Session[] = [
  { id: "s-001", cwd: "E:\\projects\\auth-service",  label: "auth-service",  firstMsg: "检查 handler.go 里的 JWT 验证逻辑，看看有没有问题", msgCount: 14, updatedAt: "09:41", group: "today" },
  { id: "s-002", cwd: "E:\\projects\\data-pipeline", label: "data-pipeline", firstMsg: "优化 Kafka 消费者并发模型，当前吞吐量约 3k/s 不达标", msgCount: 8,  updatedAt: "08:12", group: "today" },
  { id: "s-003", cwd: "E:\\projects\\legacy-api",    label: "legacy-api",    firstMsg: "legacy-api 里有几个接口响应超 2s，帮我找出瓶颈", msgCount: 21, updatedAt: "昨天",  group: "yesterday" },
  { id: "s-004", cwd: "E:\\projects\\frontend",      label: "frontend",      firstMsg: "把登录页的表单校验改成 zod schema 验证，并加 toast 提示", msgCount: 6,  updatedAt: "09-10", group: "earlier" },
  { id: "s-005", cwd: "E:\\projects\\infra",         label: "infra",         firstMsg: "写一份 Terraform 模块把现有 EC2 迁移到 ECS Fargate", msgCount: 33, updatedAt: "09-08", group: "earlier" },
];

const MOCK_BG_TASKS: BgTask[] = [
  { taskId: "LUID-0x1a4", command: "go test ./... -v -run TestAuth", runtimeMs: 183200,  outputBytes: 48291,    status: "running",          exitCode: null, outputTruncated: false },
  { taskId: "LUID-0x1a1", command: "npm run build:prod",             runtimeMs: 941000,  outputBytes: 52428800, status: "output_truncated", exitCode: null, outputTruncated: true  },
];

const MOCK_CHECKPOINTS: Checkpoint[] = [
  { seq: 3, createdAt: "09:40:12", commit: "a3f8d2c", kind: "auto",  dirtyFiles: 2    },
  { seq: 2, createdAt: "09:33:05", commit: "91be44a", kind: "model", dirtyFiles: 0    },
  { seq: 1, createdAt: "09:18:50", commit: "7c2a189", kind: "auto",  dirtyFiles: null },
];

const MOCK_TOOLS: ToolCallItem[] = [
  { id: "tc-001", name: "read_file",  args: { path: "E:\\projects\\auth-service\\src\\handler.go" },   status: "success",   elapsedMs: 48,   resultFull: `package handler\n\nimport "github.com/gin-gonic/gin"\n\nfunc AuthHandler(c *gin.Context) {\n    token := c.GetHeader("Authorization")\n    // ... (共 312 行)` },
  { id: "tc-002", name: "shell",      args: { command: "go vet ./src/..." },                           status: "success",   elapsedMs: 2340, resultFull: `$ go vet ./src/...\n# 无输出（退出码 0）` },
  { id: "tc-003", name: "write_file", args: { path: "E:\\projects\\auth-service\\src\\middleware.go" }, status: "hard_link", elapsedMs: 12,   resultFull: "" },
  { id: "tc-004", name: "write_file", args: { path: "E:\\projects\\auth-service\\src\\middleware.go" }, status: "success",   elapsedMs: 31,   resultFull: `已写入 middleware.go (87 行)` },
];

const WORKSPACES = ["E:\\projects\\auth-service", "E:\\projects\\data-pipeline", "E:\\projects\\legacy-api", "E:\\projects\\frontend"];
const MODELS = ["gpt-4o-internal", "gpt-4o-mini", "claude-3-5-sonnet", "deepseek-v3"];

const SUGGESTIONS = [
  { icon: "✎", label: "查看最近代码变更" },
  { icon: "△", label: "运行测试套件" },
  { icon: "⚠", label: "审查安全漏洞" },
  { icon: "⌕", label: "搜索接口定义" },
  { icon: "⊞", label: "分析性能瓶颈" },
  { icon: "⊕", label: "生成单元测试" },
];

// ─── Utility ──────────────────────────────────────────────────────────────────

function formatMs(ms: number) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}
function formatBytes(b: number) {
  if (b < 1024) return `${b}B`;
  if (b < 1048576) return `${(b / 1024).toFixed(1)}KB`;
  return `${(b / 1048576).toFixed(1)}MB`;
}
function formatRuntime(ms: number) {
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  return m === 0 ? `${s}s` : `${m}m${s % 60}s`;
}

// ─── Orb Avatar ───────────────────────────────────────────────────────────────

const ORB_STYLE: React.CSSProperties = {
  background: "radial-gradient(circle at 35% 30%, #f0e6ff 0%, #c7d2fe 35%, #bfdbfe 55%, #a5f3fc 75%, #fce7f3 100%)",
  boxShadow: "0 2px 8px rgba(139,92,246,0.18), inset 0 1px 2px rgba(255,255,255,0.5)",
};

// ─── CopyBtn ──────────────────────────────────────────────────────────────────

function CopyBtn({ text, className = "" }: { text: string; className?: string }) {
  const [ok, setOk] = useState(false);
  return (
    <button
      onClick={() => { navigator.clipboard.writeText(text).catch(() => {}); setOk(true); setTimeout(() => setOk(false), 1400); }}
      className={`text-[12px] text-gray-400 hover:text-gray-700 transition-colors p-1 rounded-lg hover:bg-gray-200 ${className}`}
      title="复制"
    >
      {ok ? "✓" : "⎘"}
    </button>
  );
}

// ─── CodeBlock ────────────────────────────────────────────────────────────────

function CodeBlock({ lang, code }: { lang: string; code: string }) {
  return (
    <div className="mt-3 mb-2 rounded-xl overflow-hidden border border-gray-200">
      <div className="flex items-center justify-between px-4 py-2 bg-[#f8f8f8] border-b border-gray-200">
        <span className="text-[12px] text-gray-500 font-mono">{lang}</span>
        <CopyBtn text={code} />
      </div>
      <div className="px-4 py-3 bg-[#f8f8f8]">
        <pre className="font-mono text-[13px] text-green-700 whitespace-pre-wrap leading-relaxed overflow-x-auto">{code}</pre>
      </div>
    </div>
  );
}

// ─── ToolRecord (no timeline rail) ───────────────────────────────────────────

function ToolRecord({ tool }: { tool: ToolCallItem }) {
  const [open, setOpen] = useState(false);

  const dotClass =
    tool.status === "running"   ? "bg-blue-400 pulse-dot" :
    tool.status === "success"   ? "bg-green-500" :
    tool.status === "hard_link" ? "bg-amber-500" : "bg-red-400";

  const statusEl =
    tool.status === "running"   ? <span className="text-blue-500 text-[12px]">进行中</span> :
    tool.status === "success"   ? <span className="text-green-600 text-[12px]">成功</span> :
    tool.status === "hard_link" ? <span className="text-amber-600 text-[12px]">拒绝</span> :
    tool.status === "denied"    ? <span className="text-red-500 text-[12px]">权限不足</span> :
                                   <span className="text-red-500 text-[12px]">失败</span>;

  const argsPreview = Object.entries(tool.args).slice(0, 2)
    .map(([, v]) => typeof v === "string" ? v.split("\\").pop() ?? v : String(v))
    .join(" · ");

  return (
    <div className="py-0.5">
      <button
        onClick={() => setOpen(o => !o)}
        aria-expanded={open}
        className="flex items-center gap-2 text-left w-full rounded-xl px-3 py-2 hover:bg-gray-50 transition-colors group"
      >
        <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${dotClass}`} />
        <span className="font-mono text-[12px] text-gray-600 font-medium flex-shrink-0">{tool.name}</span>
        <span className="text-[12px] text-gray-400 truncate flex-1 min-w-0">{argsPreview}</span>
        <span className="flex-shrink-0">{statusEl}</span>
        {tool.status !== "running" && (
          <span className="font-mono text-[11px] text-gray-300 flex-shrink-0">{formatMs(tool.elapsedMs)}</span>
        )}
        <span className="text-gray-300 text-[10px] flex-shrink-0">{open ? "▲" : "▼"}</span>
      </button>

      {open && (
        <div className="mt-1 mx-2 rounded-xl overflow-hidden border border-gray-200 bg-[#f8f8f8] text-[12px]">
          <div className="px-4 py-3 border-b border-gray-200">
            <div className="text-[10px] text-gray-400 uppercase tracking-wider mb-2 font-medium">参数</div>
            <pre className="font-mono text-gray-600 whitespace-pre-wrap break-all leading-relaxed">{JSON.stringify(tool.args, null, 2)}</pre>
          </div>
          <div className="px-4 py-3">
            <div className="text-[10px] text-gray-400 uppercase tracking-wider mb-2 font-medium">
              {tool.status === "hard_link" ? "拒绝原因" : "结果"}
            </div>
            {tool.status === "hard_link" ? (
              <div className="text-amber-700 leading-relaxed">
                hard_link_impact_unknown — 文件存在多个硬链接，系统无法确定写入影响范围，已阻止操作。请先确认链接状态后重试。
              </div>
            ) : (
              <pre className="font-mono text-gray-600 whitespace-pre-wrap break-all leading-relaxed max-h-36 overflow-y-auto">{tool.resultFull}</pre>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Inline event records ─────────────────────────────────────────────────────

function WaitingRecord({ seconds, phase }: {
  seconds: number; phase: "waiting_first_token" | "tool_running" | "interrupted";
}) {
  return (
    <div className="flex items-center gap-2 py-2 px-3 text-[13px]">
      <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${phase === "interrupted" ? "bg-red-400" : "bg-amber-400 pulse-dot"}`} />
      {phase === "interrupted"
        ? <span className="text-red-500">已中断</span>
        : <span className="text-gray-400">{phase === "waiting_first_token" ? "等待模型响应" : "工具执行中"} <span className="font-mono text-gray-300 ml-1">{seconds}s</span></span>
      }
    </div>
  );
}

function OutsideWriteRecord({ tool, requestedPath }: { tool: string; requestedPath: string }) {
  return (
    <div className="flex items-start gap-2 py-2 px-3 rounded-xl bg-amber-50 border border-amber-100 mx-2 my-1 text-[13px]">
      <span className="text-amber-500 flex-shrink-0 mt-0.5">⚠</span>
      <div>
        <span className="text-amber-700 font-medium">工作区外写入</span>
        <span className="text-gray-500 font-mono text-[11px] ml-2">[{tool}]</span>
        <div className="font-mono text-[12px] text-gray-600 mt-0.5 break-all">{requestedPath}</div>
        <div className="text-[11px] text-gray-400 mt-0.5">不在 checkpoint 覆盖范围，无法回滚</div>
      </div>
    </div>
  );
}

function CompactionRecord({ method = "truncate", beforeTokens, afterTokens, discardedBytes, emergency }: {
  method?: "summary" | "truncate"; beforeTokens: number; afterTokens: number;
  discardedBytes?: number; emergency: boolean;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="py-1 px-2">
      <button onClick={() => setOpen(o => !o)}
        className="flex items-center gap-2 text-[13px] text-gray-400 hover:text-gray-600 transition-colors w-full text-left px-2 py-1.5 rounded-xl hover:bg-gray-50"
      >
        <span className="text-[11px]">◇</span>
        <span>{method === "truncate" ? "上下文截断" : "上下文压缩"}{emergency ? "（紧急）" : ""}</span>
        <span className="font-mono text-gray-300 text-[11px]">{beforeTokens.toLocaleString()} → {afterTokens.toLocaleString()} token</span>
        {method === "truncate" && discardedBytes && (
          <span className="text-amber-500 text-[12px]">丢弃 {formatBytes(discardedBytes)}</span>
        )}
        <span className="text-[10px] ml-auto text-gray-300">{open ? "▲" : "▼"}</span>
      </button>
      {open && (
        <div className="text-[12px] text-gray-400 px-4 py-2 bg-gray-50 rounded-xl mx-2 mt-1 border border-gray-100">
          部分历史消息已从上下文移除，已丢弃内容无法恢复。
        </div>
      )}
    </div>
  );
}

function SkillLoadedRecord({ name }: { name: string }) {
  return (
    <div className="py-1.5 px-5 text-[12px] text-gray-400">
      skill 已加载 <span className="font-mono text-gray-500 bg-gray-100 px-1.5 py-0.5 rounded-md">{name}</span>
    </div>
  );
}

function ProcessWarning({ current, threshold }: { current: number; threshold: number }) {
  return (
    <div className="flex items-center gap-2 py-1.5 px-5 text-[13px]">
      <span className="w-1.5 h-1.5 rounded-full bg-amber-400 flex-shrink-0" />
      <span className="text-amber-700">进程数 <span className="font-mono">{current}/{threshold}</span> 已超阈值，不拦截</span>
    </div>
  );
}

function TurnResultBlock({ status, rounds, elapsedMs, commands, outsideWrites, bgProcesses }: {
  status: "success" | "need_answer" | "max_rounds" | "error" | "interrupted";
  rounds: number; elapsedMs: number; commands: string[];
  outsideWrites: { path: string; tool: string }[]; bgProcesses: { taskId: string; command: string }[];
}) {
  const [showRollback, setShowRollback] = useState(false);
  const cfg = {
    success:     { label: "已完成",   color: "text-green-600", bg: "bg-green-50",  border: "border-green-200" },
    need_answer: { label: "等待回答", color: "text-amber-600", bg: "bg-amber-50",  border: "border-amber-200" },
    max_rounds:  { label: "触顶中止", color: "text-red-500",   bg: "bg-red-50",    border: "border-red-200"   },
    error:       { label: "出错中止", color: "text-red-500",   bg: "bg-red-50",    border: "border-red-200"   },
    interrupted: { label: "已中断",   color: "text-red-500",   bg: "bg-red-50",    border: "border-red-200"   },
  }[status];

  return (
    <div className="py-2 px-4">
      <div className={`rounded-xl border ${cfg.border} ${cfg.bg} overflow-hidden`}>
        <div className="px-4 py-2.5 flex items-center gap-3 border-b border-gray-100/50">
          <span className={`font-semibold text-[14px] ${cfg.color}`}>{cfg.label}</span>
          <span className="text-[12px] text-gray-400 font-mono">{rounds} 轮 · {formatMs(elapsedMs)}</span>
        </div>
        <div className="px-4 py-3 space-y-2.5 text-[13px]">
          {commands.length > 0 && (
            <div>
              <div className="text-[11px] text-gray-500 font-medium mb-1.5 uppercase tracking-wide">执行命令</div>
              {commands.map((cmd, i) => (
                <div key={i} className="flex items-center gap-1.5 font-mono text-[12px] text-gray-700">
                  <span className="text-gray-300">$</span>
                  <span className="break-all">{cmd}</span>
                  <CopyBtn text={cmd} />
                </div>
              ))}
            </div>
          )}
          {outsideWrites.length > 0 && (
            <div className="border-l-2 border-amber-300 pl-3">
              <div className="text-[11px] text-amber-600 font-medium mb-1">工作区外写入 ({outsideWrites.length}) · 不可回滚</div>
              {outsideWrites.map((w, i) => (
                <div key={i} className="font-mono text-[12px] text-gray-600 flex items-center gap-1">
                  <span className="break-all">{w.path}</span>
                  <CopyBtn text={w.path} />
                </div>
              ))}
            </div>
          )}
          {bgProcesses.length > 0 && (
            <div>
              <div className="text-[11px] text-gray-500 font-medium mb-1.5 uppercase tracking-wide">后台进程</div>
              {bgProcesses.map((p, i) => (
                <div key={i} className="font-mono text-[12px] text-gray-500">
                  <span className="text-blue-500">{p.taskId}</span> {p.command}
                </div>
              ))}
            </div>
          )}
          <div className="flex items-center gap-2 pt-0.5">
            <button onClick={() => setShowRollback(true)}
              className="text-[13px] text-gray-600 hover:text-gray-900 border border-gray-200 hover:border-gray-300 px-3 py-1.5 rounded-lg bg-white transition-colors active:scale-[0.97]"
            >回滚到检查点</button>
            {status === "need_answer" && (
              <input type="text" placeholder="输入回答后按 Enter 继续…"
                className="flex-1 border border-amber-200 rounded-lg px-3 py-1.5 text-[13px] outline-none focus:border-amber-400 transition-colors bg-white"
              />
            )}
          </div>
        </div>
      </div>
      {showRollback && <RollbackDialog checkpoints={MOCK_CHECKPOINTS} onClose={() => setShowRollback(false)} />}
    </div>
  );
}

// ─── UserMsg ─────────────────────────────────────────────────────────────────
// Right-aligned plain text, matching Figma (no heavy bubble)

function UserMsg({ text, attachment }: { text: string; attachment?: string }) {
  return (
    <div className="flex flex-col items-end gap-2 py-4 px-6">
      {attachment && (
        <div className="flex items-center gap-3 border border-gray-200 rounded-xl px-3 py-2.5 bg-white shadow-sm max-w-[60%]">
          <div className="w-9 h-9 rounded-lg bg-red-50 border border-red-100 flex items-center justify-center flex-shrink-0">
            <span className="font-mono text-[9px] font-bold text-red-500 tracking-tight">PDF</span>
          </div>
          <div className="min-w-0">
            <div className="text-[13px] text-gray-900 font-medium truncate">{attachment}</div>
            <div className="text-[11px] text-gray-400 mt-0.5">2.4 MB</div>
          </div>
        </div>
      )}
      <div className="max-w-[65%] text-[15px] text-gray-900 leading-relaxed text-right">
        {text}
      </div>
    </div>
  );
}

// ─── AssistantMsg ─────────────────────────────────────────────────────────────
// Left-aligned with orb avatar, label, disclosure, text, actions

function AssistantMsg({ text, streaming, model, toolCount, showCode }: {
  text: string; streaming?: boolean; model: string; toolCount?: number; showCode?: boolean;
}) {
  const [toolsOpen, setToolsOpen] = useState(false);

  return (
    <div className="flex gap-3 py-4 px-6">
      {/* Orb avatar */}
      <div className="w-8 h-8 rounded-full flex-shrink-0 mt-0.5" style={ORB_STYLE} />
      <div className="flex-1 min-w-0">
        {/* Label */}
        <div className="text-[13px] text-gray-500 mb-2 font-medium">
          pulse7 · <span className="font-mono font-normal text-[12px]">{model}</span>
        </div>
        {/* Tool disclosure */}
        {toolCount && toolCount > 0 && !streaming && (
          <button
            onClick={() => setToolsOpen(o => !o)}
            className="flex items-center gap-1.5 text-[13px] text-gray-400 hover:text-gray-600 mb-2.5 transition-colors"
          >
            <span className="text-[11px]">◇</span>
            <span>执行了 {toolCount} 个工具</span>
            <span className="text-[10px] ml-0.5">{toolsOpen ? "▲" : "▶"}</span>
          </button>
        )}
        {/* Response */}
        <div className="text-[15px] text-gray-800 leading-relaxed whitespace-pre-wrap">
          {text}{streaming && <span className="cursor-blink text-blue-400">▍</span>}
        </div>
        {/* Inline code block demo */}
        {showCode && (
          <CodeBlock lang="go"
            code={`func RateLimiter() gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Every(time.Minute), 60)
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.AbortWithStatus(http.StatusTooManyRequests)
            return
        }
        c.Next()
    }
}`} />
        )}
        {/* Action buttons */}
        {!streaming && (
          <div className="flex items-center gap-0.5 mt-3 -ml-1.5">
            {["⎘", "↺", "···"].map((icon, i) => (
              <button key={i} title={["复制", "重试", "更多"][i]}
                className="p-1.5 text-gray-300 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors text-[16px]"
              >{icon}</button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── EmptyState ───────────────────────────────────────────────────────────────

function EmptyState({ workspace, onSuggest }: { workspace: string; onSuggest: (s: string) => void }) {
  return (
    <div className="flex-1 flex flex-col items-center justify-center px-8 py-12">
      {/* Orb */}
      <div className="w-24 h-24 rounded-full mb-8 flex-shrink-0" style={{
        ...ORB_STYLE,
        boxShadow: "0 12px 40px rgba(139,92,246,0.2), 0 4px 12px rgba(0,0,0,0.08), inset 0 1px 2px rgba(255,255,255,0.6)",
      }} />
      {/* Heading */}
      <h1 className="text-[32px] font-semibold text-gray-900 mb-3 text-center leading-tight">
        准备好了，输入你的第一个任务
      </h1>
      {/* Subtitle */}
      <p className="text-[16px] text-gray-500 mb-10 text-center">
        工作区 <span className="font-mono text-gray-700 bg-gray-100 px-2 py-0.5 rounded-md text-[14px]">{workspace.split("\\").pop()}</span>，从下方选择或直接输入
      </p>
      {/* Suggestion chips */}
      <div className="flex flex-wrap gap-2.5 justify-center max-w-[600px]">
        {SUGGESTIONS.map(s => (
          <button key={s.label}
            onClick={() => onSuggest(s.label)}
            className="flex items-center gap-2 border border-gray-200 rounded-full px-4 py-2 text-[14px] text-gray-600 hover:bg-gray-50 hover:border-gray-300 hover:text-gray-900 transition-colors"
          >
            <span className="text-gray-400 text-[13px]">{s.icon}</span>
            {s.label}
          </button>
        ))}
      </div>
    </div>
  );
}

// ─── SessionTabBar ────────────────────────────────────────────────────────────

function SessionTabBar({ sessions, active, onSelect, onNew }: {
  sessions: Session[]; active: string; onSelect: (id: string) => void; onNew: () => void;
}) {
  const openSessions = sessions.filter(s => s.group === "today").slice(0, 5);
  return (
    <div className="flex items-center border-b border-gray-200 bg-white flex-shrink-0 overflow-x-auto">
      {openSessions.map(s => (
        <button key={s.id}
          onClick={() => onSelect(s.id)}
          className={`flex items-center gap-2 px-4 py-2.5 text-[13px] border-b-2 whitespace-nowrap flex-shrink-0 transition-colors ${
            s.id === active
              ? "border-gray-900 text-gray-900 font-medium"
              : "border-transparent text-gray-500 hover:text-gray-800 hover:bg-gray-50"
          }`}
        >
          {s.label}
        </button>
      ))}
      <button onClick={onNew}
        className="px-3 py-2.5 text-gray-400 hover:text-gray-700 flex-shrink-0 text-[16px] border-b-2 border-transparent hover:bg-gray-50 transition-colors"
        title="新建会话"
      >+</button>
    </div>
  );
}

// ─── Rollback Dialog ──────────────────────────────────────────────────────────

function RollbackDialog({ checkpoints, onClose }: { checkpoints: Checkpoint[]; onClose: () => void }) {
  const [selected, setSelected] = useState<number | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [onClose]);
  const target = checkpoints.find(c => c.seq === selected);
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30"
      onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
      <div role="dialog" aria-modal="true" className="w-[460px] bg-white rounded-2xl border border-gray-200 shadow-2xl overflow-hidden">
        <div className="px-6 py-5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-[15px] font-semibold text-gray-900">回滚到检查点</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-700 transition-colors text-[18px] leading-none">✕</button>
        </div>
        <div className="px-6 py-5">
          <p className="text-[13px] text-gray-500 mb-4">回滚前请核对目标序号、时间与 commit 哈希。</p>
          <div className="space-y-2 mb-5">
            {checkpoints.map(cp => (
              <label key={cp.seq}
                className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-colors ${
                  selected === cp.seq ? "border-blue-400 bg-blue-50" : "border-gray-100 hover:border-gray-200 hover:bg-gray-50"
                }`}
              >
                <input type="radio" name="cp" checked={selected === cp.seq}
                  onChange={() => { setSelected(cp.seq); setConfirmed(false); }} className="mt-0.5" />
                <div>
                  <div className="flex items-center gap-2 text-[13px]">
                    <span className="font-mono font-semibold text-gray-800">#{cp.seq}</span>
                    <span className="font-mono text-gray-500">{cp.createdAt}</span>
                    <span className="font-mono text-blue-600">{cp.commit}</span>
                    <span className="text-gray-400">{cp.kind === "auto" ? "自动" : "模型主动"}</span>
                  </div>
                  <div className="text-[12px] text-gray-400 mt-0.5">
                    {cp.dirtyFiles !== null ? `相对 HEAD 变更 ${cp.dirtyFiles} 个文件` : "变更文件数未记录"}
                  </div>
                </div>
              </label>
            ))}
          </div>
          {target && (
            <div className="mb-5 p-3 bg-red-50 border border-red-100 rounded-xl text-[13px]">
              <div className="font-mono text-gray-800 mb-1.5">目标：#{target.seq} · {target.createdAt} · {target.commit}</div>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={confirmed} onChange={e => setConfirmed(e.target.checked)} />
                <span className="text-gray-700">已核对目标，确认执行回滚</span>
              </label>
            </div>
          )}
          <div className="flex justify-end gap-2">
            <button onClick={onClose} className="px-4 py-2 text-[13px] text-gray-600 border border-gray-200 rounded-xl hover:border-gray-300 transition-colors">取消</button>
            <button disabled={!selected || !confirmed} onClick={onClose}
              className="px-4 py-2 text-[13px] bg-red-500 text-white rounded-xl disabled:opacity-40 hover:bg-red-600 transition-colors active:scale-[0.97]"
            >回滚</button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Left Sidebar (Figma style) ───────────────────────────────────────────────

function LeftSidebar({ collapsed, onToggle, active, onSelect, onNew, isBusy, onBusySwitch }: {
  collapsed: boolean; onToggle: () => void;
  active: string; onSelect: (id: string) => void; onNew: () => void;
  isBusy: boolean; onBusySwitch: () => void;
}) {
  const [cpOpen, setCpOpen] = useState(true);
  const maxSeq = Math.max(...MOCK_CHECKPOINTS.map(c => c.seq));

  const groups: { key: Session["group"]; label: string }[] = [
    { key: "today",     label: "今天" },
    { key: "yesterday", label: "昨天" },
    { key: "earlier",   label: "更早" },
  ];

  if (collapsed) {
    return (
      <div className="w-[52px] flex-shrink-0 border-r border-gray-200 bg-white flex flex-col items-center py-3 gap-3">
        <div className="w-8 h-8 rounded-xl flex-shrink-0" style={ORB_STYLE} />
        <button onClick={onToggle} className="text-gray-400 hover:text-gray-700 p-1.5 rounded-xl hover:bg-gray-100 transition-colors text-[13px]">▶</button>
        <button onClick={onNew} className="text-gray-400 hover:text-gray-700 p-1.5 rounded-xl hover:bg-gray-100 transition-colors text-[16px]">✎</button>
      </div>
    );
  }

  return (
    <div className="w-[252px] flex-shrink-0 border-r border-gray-200 bg-white flex flex-col overflow-hidden">
      {/* Quick actions */}
      <div className="px-3 pt-2 pb-3 space-y-0.5 flex-shrink-0">
        <div className="flex items-center justify-between mb-1">
          <span className="text-[11px] text-gray-400 px-3">会话</span>
          <button onClick={onToggle} className="text-gray-400 hover:text-gray-700 p-1.5 rounded-xl hover:bg-gray-100 transition-colors text-[12px]" title="折叠侧栏">⊟</button>
        </div>
        <button onClick={onNew}
          className="flex items-center gap-2.5 w-full px-3 py-2 rounded-xl bg-gray-50 hover:bg-gray-100 transition-colors text-[14px] text-gray-700 font-medium"
        >
          <span className="text-gray-500 text-[14px]">✎</span>
          新建会话
        </button>
        <button className="flex items-center gap-2.5 w-full px-3 py-2 rounded-xl hover:bg-gray-50 transition-colors text-[14px] text-gray-500 hover:text-gray-800">
          <span className="text-gray-400 text-[14px]">⌕</span>
          搜索会话
        </button>
        <button className="flex items-center gap-2.5 w-full px-3 py-2 rounded-xl hover:bg-gray-50 transition-colors text-[14px] text-gray-500 hover:text-gray-800">
          <span className="text-gray-400 text-[14px]">⚙</span>
          工具与集成
        </button>
      </div>

      {/* Session list */}
      <div className="flex-1 overflow-y-auto px-3">
        <div className="flex items-center justify-between px-3 py-1.5">
          <span className="text-[11px] text-gray-400 font-medium">历史会话</span>
          <span className="text-[10px] text-gray-300">同一时刻仅一个活动会话</span>
        </div>
        {groups.map(g => {
          const items = MOCK_SESSIONS.filter(s => s.group === g.key);
          if (!items.length) return null;
          return (
            <div key={g.key} className="mb-1">
              <div className="text-[11px] text-gray-400 px-3 py-1 font-medium">{g.label}</div>
              {items.map(s => {
                const isCurrent = s.id === active;
                return (
                  <button key={s.id}
                    onClick={() => {
                      if (isCurrent) return;
                      if (isBusy) { onBusySwitch(); return; }
                      onSelect(s.id);
                    }}
                    className={`group flex items-center justify-between w-full px-3 py-2 rounded-xl text-[14px] transition-colors text-left ${
                      isCurrent
                        ? "bg-gray-100 text-gray-900 font-medium"
                        : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                    }`}
                  >
                    <span className="truncate flex-1 leading-snug min-w-0 mr-1.5">{s.firstMsg}</span>
                    {isCurrent ? (
                      <span className="text-[10px] text-blue-500 font-medium flex-shrink-0">当前</span>
                    ) : (
                      <span className="opacity-0 group-hover:opacity-100 text-[10px] text-gray-400 flex-shrink-0 transition-opacity">
                        {isBusy ? "忙碌" : "恢复"}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          );
        })}
      </div>

      {/* Checkpoints */}
      <div className="border-t border-gray-100 flex-shrink-0">
        <button onClick={() => setCpOpen(o => !o)}
          className="flex items-center justify-between w-full px-4 py-2.5 hover:bg-gray-50 transition-colors"
        >
          <span className="text-[12px] text-gray-500 font-medium">检查点 · auth-service</span>
          <span className="text-[10px] text-gray-300">{cpOpen ? "▲" : "▼"}</span>
        </button>
        {cpOpen && (
          <div className="pb-2 px-2">
            {MOCK_CHECKPOINTS.map(cp => (
              <div key={cp.seq}
                className="group flex items-center gap-2 px-2 py-1.5 rounded-xl hover:bg-gray-50 transition-colors text-[12px]"
              >
                <span className="font-mono text-gray-500 flex-shrink-0">#{cp.seq}</span>
                <span className="font-mono text-gray-400 flex-shrink-0">{cp.createdAt.slice(0, 5)}</span>
                <span className={`text-[10px] px-1.5 py-0.5 rounded-full flex-shrink-0 ${
                  cp.kind === "auto" ? "bg-gray-100 text-gray-500" : "bg-blue-50 text-blue-600"
                }`}>{cp.kind === "auto" ? "auto" : "model"}</span>
                <span className="font-mono text-blue-400 flex-shrink-0">{cp.commit}</span>
                {cp.dirtyFiles !== null && cp.dirtyFiles > 0 && (
                  <span className="text-amber-500 font-mono flex-shrink-0">±{cp.dirtyFiles}</span>
                )}
                <div className="flex-1 flex items-center justify-end gap-1.5">
                  {cp.seq === maxSeq && <span className="text-[10px] text-gray-300">当前</span>}
                  <button className="opacity-0 group-hover:opacity-100 text-[11px] text-blue-500 hover:text-blue-700 border border-blue-200 rounded-full px-2 py-0.5 transition-all">
                    回滚
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Input Card (Figma style) ─────────────────────────────────────────────────

function InputCard({ turnStatus, onInterrupt, contextPct, contextLevel, model, onModelChange }: {
  turnStatus: TurnStatus; onInterrupt: () => void;
  contextPct: number; contextLevel: "normal" | "warning" | "critical";
  model: string; onModelChange: (m: string) => void;
}) {
  const [value, setValue] = useState("");
  const [modelOpen, setModelOpen] = useState(false);
  const ref = useRef<HTMLTextAreaElement>(null);
  const busy = ["running", "streaming", "tool_running", "waiting"].includes(turnStatus);

  const barColor =
    contextLevel === "critical" ? "bg-red-400" :
    contextLevel === "warning"  ? "bg-amber-400" : "bg-gray-200";
  const ctxColor =
    contextLevel === "critical" ? "text-red-500" :
    contextLevel === "warning"  ? "text-amber-500" : "text-gray-400";

  return (
    <div className="px-6 pb-4 pt-2 flex-shrink-0 max-w-[800px] mx-auto w-full">
      <div className="bg-white rounded-2xl overflow-hidden"
        style={{ boxShadow: "0 0 0 1px rgba(0,0,0,0.08), 0 4px 24px rgba(0,0,0,0.06)" }}
      >
        {/* Context bar */}
        <div className="h-[2px] bg-gray-100 overflow-hidden">
          <div className={`h-full ${barColor} transition-[width] duration-500`} style={{ width: `${contextPct}%` }} />
        </div>

        {/* Busy state */}
        {busy && (
          <div className="flex items-center justify-between px-4 py-2 border-b border-gray-100 bg-amber-50">
            <div className="flex items-center gap-1.5 text-[13px] text-amber-700">
              <span className="w-1.5 h-1.5 rounded-full bg-amber-400 pulse-dot" />
              {turnStatus === "streaming" ? "模型输出中" : turnStatus === "tool_running" ? "工具执行中" : "等待模型响应"}
            </div>
            <button onClick={onInterrupt}
              className="text-[12px] text-red-500 hover:text-red-700 border border-red-200 hover:border-red-300 px-2.5 py-1 rounded-lg transition-colors"
            >■ 中断</button>
          </div>
        )}

        {/* Textarea */}
        <textarea
          ref={ref}
          value={value}
          disabled={busy}
          rows={3}
          placeholder={busy ? "任务进行中，中断后可继续输入…" : "输入任务，或 @ 引用文件…"}
          onChange={e => {
            setValue(e.target.value);
            e.target.style.height = "auto";
            e.target.style.height = Math.min(e.target.scrollHeight, 200) + "px";
          }}
          onKeyDown={e => {
            if (e.key === "Enter" && !e.shiftKey && !busy) {
              e.preventDefault();
              setValue("");
              if (ref.current) ref.current.style.height = "auto";
            }
          }}
          className="w-full px-4 pt-4 pb-2 text-[15px] text-gray-900 placeholder-gray-400 outline-none resize-none bg-transparent leading-relaxed disabled:opacity-50"
        />

        {/* Bottom toolbar */}
        <div className="flex items-center gap-2 px-4 py-3">
          {/* Model chip */}
          <div className="relative">
            <button
              onClick={() => setModelOpen(o => !o)}
              className="flex items-center gap-1.5 rounded-full border border-gray-200 bg-white hover:bg-gray-50 px-3 py-1.5 text-[13px] text-gray-700 transition-colors"
            >
              <span className="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" />
              {model}
              <span className="text-gray-400 text-[10px]">▾</span>
            </button>
            {modelOpen && (
              <>
                <div className="fixed inset-0 z-10" onClick={() => setModelOpen(false)} />
                <div className="absolute bottom-full left-0 mb-2 w-52 bg-white border border-gray-200 rounded-2xl shadow-lg z-20 py-1.5 overflow-hidden">
                  {MODELS.map(m => (
                    <button key={m}
                      onClick={() => { onModelChange(m); setModelOpen(false); }}
                      className={`w-full text-left px-4 py-2 text-[13px] font-mono hover:bg-gray-50 transition-colors ${m === model ? "text-gray-900 font-semibold" : "text-gray-600"}`}
                    >{m}</button>
                  ))}
                </div>
              </>
            )}
          </div>

          {/* @ attach */}
          <button className="text-gray-400 hover:text-gray-700 rounded-full p-2 hover:bg-gray-100 transition-colors text-[15px]" title="引用文件">
            @
          </button>

          {/* More */}
          <button className="text-gray-400 hover:text-gray-700 rounded-full p-2 hover:bg-gray-100 transition-colors text-[15px]" title="更多选项">
            ···
          </button>

          <div className="flex-1" />

          {/* Context indicator */}
          <span className={`text-[12px] font-mono mr-1 ${ctxColor}`}>{100 - contextPct}%</span>

          {/* Send button */}
          <button
            disabled={busy || !value.trim()}
            onClick={() => { setValue(""); if (ref.current) ref.current.style.height = "auto"; }}
            className="w-9 h-9 rounded-full bg-gray-900 disabled:opacity-30 hover:bg-gray-700 transition-colors flex items-center justify-center flex-shrink-0 active:scale-[0.94]"
          >
            <span className="text-white text-[17px] leading-none -mt-0.5">↑</span>
          </button>
        </div>
      </div>
      {/* Disclaimer */}
      <p className="text-center text-[12px] text-gray-400 mt-2.5">AI 可能犯错，重要信息请自行核实</p>
    </div>
  );
}

// ─── Task Drawer ──────────────────────────────────────────────────────────────

function TaskDrawer({ onClose, tasks, processCount, processThreshold, processExceeded }: {
  onClose: () => void; tasks: BgTask[];
  processCount: number | null; processThreshold: number; processExceeded: boolean;
}) {
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [onClose]);

  return (
    <>
      <div className="fixed inset-0 z-20" onClick={onClose} />
      <div className="fixed bottom-7 left-0 right-0 z-30 bg-white border-t border-gray-200 shadow-[0_-4px_24px_rgba(0,0,0,0.08)] max-h-[60vh] overflow-y-auto">
        <div className="flex items-center justify-between px-6 py-3 border-b border-gray-100 sticky top-0 bg-white">
          <span className="text-[13px] font-semibold text-gray-700">后台进程</span>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-700 transition-colors">✕</button>
        </div>
        <div className="px-6 py-4 grid grid-cols-[1fr_280px] gap-8">
          <div>
            <div className="text-[11px] text-gray-400 uppercase tracking-wider mb-4 font-medium">
              后台任务 {tasks.filter(t => t.status === "running" || t.status === "output_truncated").length > 0
                ? `(${tasks.filter(t => t.status === "running" || t.status === "output_truncated").length} 运行中)` : ""}
            </div>
            <div className="space-y-3">
              {tasks.map(task => (
                <div key={task.taskId} className="border border-gray-100 rounded-2xl p-4 space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full flex-shrink-0 ${
                        task.status === "running" || task.status === "output_truncated" ? "bg-blue-400 pulse-dot" :
                        task.status === "exited" ? "bg-green-400" : "bg-red-400"}`} />
                      <span className="font-mono text-[12px] text-blue-500">{task.taskId}</span>
                    </div>
                    <span className="font-mono text-[12px] text-gray-400">{formatRuntime(task.runtimeMs)}</span>
                  </div>
                  <div className="font-mono text-[13px] text-gray-700 break-all">{task.command}</div>
                  <div className="flex items-center gap-3 text-[12px]">
                    <span className="text-gray-400 font-mono">{formatBytes(task.outputBytes)}</span>
                    {task.outputTruncated && <span className="text-amber-600">输出截断 · 进程仍在运行</span>}
                  </div>
                  <div className="flex gap-2">
                    <button className="text-[12px] text-blue-500 hover:text-blue-700 border border-blue-100 hover:border-blue-200 px-2.5 py-1 rounded-lg transition-colors">拉取输出</button>
                    {(task.status === "running" || task.status === "output_truncated") && (
                      <button className="text-[12px] text-red-400 hover:text-red-600 border border-red-100 hover:border-red-200 px-2.5 py-1 rounded-lg transition-colors">终止</button>
                    )}
                  </div>
                </div>
              ))}
              {tasks.length === 0 && <div className="text-[13px] text-gray-300">无后台任务</div>}
            </div>
          </div>
          <div className="border-l border-gray-100 pl-8 space-y-6">
            <div>
              <div className="text-[11px] text-gray-400 uppercase tracking-wider mb-4 font-medium">进程概况</div>
              <div className="space-y-2.5 text-[13px]">
                <div className="flex justify-between">
                  <span className="text-gray-500">会话进程数</span>
                  <span className={`font-mono ${processExceeded ? "text-amber-600 font-semibold" : "text-gray-700"}`}>
                    {processCount !== null ? `${processCount} / ${processThreshold}` : <span className="text-red-400">计数失败</span>}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">受限模式</span>
                  <span className="font-mono text-amber-600">restricted</span>
                </div>
                <div className="text-[12px] text-gray-400 leading-relaxed">
                  Session Job 分配失败，退出时进程清理无法保证
                </div>
              </div>
            </div>
            <div className="border-t border-gray-100 pt-5">
              <div className="text-[11px] text-gray-400 uppercase tracking-wider mb-3 font-medium">上次遗留（未核实）</div>
              <div className="font-mono text-[12px] text-gray-400">pid 7432 · cmd.exe</div>
              <div className="font-mono text-[12px] text-gray-300 mt-1">pid 7891 · node.exe (已退出?)</div>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

// ─── Bottom Status Bar ────────────────────────────────────────────────────────

function BottomStatusBar({ onTaskClick, taskCount, processExceeded, contextPct, contextLevel }: {
  onTaskClick: () => void; taskCount: number; processExceeded: boolean;
  contextPct: number; contextLevel: "normal" | "warning" | "critical";
}) {
  const ctxColor = contextLevel === "critical" ? "text-red-500" : contextLevel === "warning" ? "text-amber-500" : "text-gray-400";
  return (
    <div className="h-7 border-t border-gray-200 bg-white flex items-center px-4 gap-4 flex-shrink-0">
      <button onClick={onTaskClick}
        className={`flex items-center gap-1 text-[11px] font-mono hover:text-gray-700 transition-colors ${processExceeded ? "text-amber-600" : taskCount > 0 ? "text-gray-500" : "text-gray-300"}`}
      >
        <span>{processExceeded ? "⚠" : "⚙"}</span>
        {taskCount > 0 && <span className="text-[10px]">{taskCount}</span>}
        {processExceeded && <span className="text-[10px] ml-0.5">进程 52/50</span>}
      </button>
      <div className="flex-1" />
      <div className="flex items-center gap-2">
        <div className="w-16 h-[2px] bg-gray-100 rounded-full overflow-hidden">
          <div className={`h-full rounded-full transition-[width] duration-500 ${contextLevel === "critical" ? "bg-red-400" : contextLevel === "warning" ? "bg-amber-400" : "bg-gray-300"}`}
            style={{ width: `${contextPct}%` }} />
        </div>
        <span className={`text-[10px] font-mono tabular-nums ${ctxColor}`}>{contextPct}%</span>
      </div>
    </div>
  );
}

// ─── Alert Banner ─────────────────────────────────────────────────────────────

function AlertBanner({ type, onClose }: { type: "endpoint_down" | "restricted"; onClose: () => void }) {
  const cfg = {
    endpoint_down: { text: "端点无法访问 — 模型端点连接失败，所有任务暂停。检查网络后在设置页重新测试连接。", color: "border-red-200 bg-red-50 text-red-700" },
    restricted:    { text: "受限模式 — Session Job 分配失败，退出时进程清理无法保证（cleanupGuaranteed=false）。", color: "border-amber-200 bg-amber-50 text-amber-700" },
  }[type];
  return (
    <div className={`flex items-center gap-2 px-5 py-2 border-b text-[13px] ${cfg.color}`}>
      <span className="flex-1">{cfg.text}</span>
      <button onClick={onClose} className="opacity-50 hover:opacity-100 transition-opacity">✕</button>
    </div>
  );
}

// ─── Workspace Switcher ───────────────────────────────────────────────────────

function WorkspaceSwitcher({ workspace, onSelect }: { workspace: string; onSelect: (w: string) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="relative">
      <button onClick={() => setOpen(o => !o)}
        className="flex items-center gap-2 font-mono text-[12px] text-gray-700 hover:text-gray-900 transition-colors py-1 px-2 rounded-lg hover:bg-gray-50"
      >
        <span className="truncate max-w-[300px]">{workspace}</span>
        <span className="text-amber-500 text-[11px] flex-shrink-0">● 3</span>
        <span className="text-gray-400 text-[10px]">▾</span>
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute top-full left-0 mt-1 w-[360px] bg-white border border-gray-200 rounded-2xl shadow-lg z-20 py-2">
            {WORKSPACES.map(w => (
              <button key={w} onClick={() => { onSelect(w); setOpen(false); }}
                className={`w-full text-left px-4 py-2.5 font-mono text-[12px] hover:bg-gray-50 transition-colors flex items-center gap-2 ${w === workspace ? "text-gray-900 font-semibold" : "text-gray-500"}`}
              >
                {w === workspace && <span className="w-1.5 h-1.5 rounded-full bg-blue-500 flex-shrink-0" />}
                {w !== workspace && <span className="w-1.5 h-1.5 flex-shrink-0" />}
                <span className="truncate">{w}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

// ─── Permission Toggle ────────────────────────────────────────────────────────

function PermToggle({ perm, onChange }: { perm: PermTier; onChange: (p: PermTier) => void }) {
  const tiers: { id: PermTier; label: string }[] = [
    { id: "strict", label: "strict" }, { id: "standard", label: "standard" }, { id: "open", label: "open" },
  ];
  return (
    <div className="flex items-center border border-gray-200 rounded-lg overflow-hidden text-[11px] font-mono flex-shrink-0">
      {tiers.map((t, i) => (
        <button key={t.id} onClick={() => onChange(t.id)}
          className={`px-2.5 py-1 transition-colors ${t.id === perm ? "bg-gray-900 text-white" : "text-gray-400 hover:bg-gray-50 hover:text-gray-700"} ${i > 0 ? "border-l border-gray-200" : ""}`}
        >{t.label}</button>
      ))}
    </div>
  );
}

// ─── Settings Panel ───────────────────────────────────────────────────────────

function SettingsPanel({ onClose }: { onClose: () => void }) {
  const [endpoint, setEndpoint] = useState("http://192.168.1.100:8080/v1");
  const [modelName, setModelName] = useState("gpt-4o-internal");
  const [testResult, setTestResult] = useState<"idle" | "testing" | "ok" | "fail">("idle");
  const [testMsg, setTestMsg] = useState("");
  const [tab, setTab] = useState<"connection" | "skills" | "workspace">("connection");
  const skills = [
    { name: "git-ops", source: "项目级", active: true },
    { name: "code-search", source: "个人级", active: true },
    { name: "file-diff", source: "项目级", active: false },
  ];
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [onClose]);
  const handleTest = () => {
    setTestResult("testing");
    setTimeout(() => {
      if (endpoint && modelName) { setTestResult("ok"); setTestMsg(`连接成功 · ${modelName} · 412ms`); }
      else { setTestResult("fail"); setTestMsg("连接失败：端点地址无效"); }
    }, 1000);
  };
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/20"
      onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
      <div role="dialog" aria-modal="true" className="w-[560px] max-h-[80vh] bg-white border border-gray-200 rounded-2xl shadow-2xl flex flex-col overflow-hidden">
        <div className="px-6 py-5 border-b border-gray-100 flex items-center justify-between flex-shrink-0">
          <div>
            <h2 className="text-[15px] font-semibold text-gray-900">设置</h2>
            <div className="text-[12px] text-gray-400 mt-0.5 font-mono">pulse7 · rc-0.14</div>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-700 transition-colors text-[18px]">✕</button>
        </div>
        <div className="flex border-b border-gray-100 flex-shrink-0 px-6">
          {(["connection", "skills", "workspace"] as const).map((t, i) => (
            <button key={t} onClick={() => setTab(t)}
              className={`px-4 py-3 text-[13px] border-b-2 transition-colors -mb-px ${tab === t ? "border-gray-900 text-gray-900 font-medium" : "border-transparent text-gray-400 hover:text-gray-700"}`}
            >{["连接配置", "Skills", "工作区"][i]}</button>
          ))}
        </div>
        <div className="px-6 py-5 overflow-y-auto flex-1">
          {tab === "connection" && (
            <div className="space-y-4">
              {[{ label: "端点地址", type: "url", val: endpoint, set: setEndpoint, ph: "http://192.168.1.100:8080/v1" },
                { label: "模型名称", type: "text", val: modelName, set: setModelName, ph: "gpt-4o 或内网模型名" }].map(f => (
                <div key={f.label}>
                  <label className="block text-[13px] text-gray-600 mb-1.5 font-medium">{f.label}</label>
                  <input type={f.type} value={f.val} onChange={e => f.set(e.target.value)} placeholder={f.ph}
                    className="w-full font-mono border border-gray-200 rounded-xl px-3 py-2 text-[13px] text-gray-900 placeholder-gray-300 outline-none focus:border-blue-400 transition-colors" />
                </div>
              ))}
              <div>
                <label className="block text-[13px] text-gray-600 mb-1.5 font-medium">API Key <span className="text-gray-400 font-normal">保存后不回显</span></label>
                <input type="password" placeholder="sk-…" autoComplete="off"
                  className="w-full font-mono border border-gray-200 rounded-xl px-3 py-2 text-[13px] text-gray-900 placeholder-gray-300 outline-none focus:border-blue-400 transition-colors" />
              </div>
              <div className="flex items-center gap-3">
                <button onClick={handleTest} disabled={testResult === "testing"}
                  className="text-[13px] border border-gray-200 px-4 py-2 rounded-xl hover:border-gray-300 hover:bg-gray-50 transition-colors disabled:opacity-50"
                >{testResult === "testing" ? "测试中…" : "测试连接"}</button>
                {testResult === "ok"   && <span className="text-green-600 text-[13px]">✓ {testMsg}</span>}
                {testResult === "fail" && <span className="text-red-500 text-[13px]">✕ {testMsg}</span>}
              </div>
            </div>
          )}
          {tab === "skills" && (
            <div>
              <div className="text-[13px] text-gray-500 mb-4">Skills 在对话开始时由模型按需加载。</div>
              {skills.map(s => (
                <div key={s.name} className="flex items-center justify-between py-3 border-b border-gray-100 last:border-0">
                  <div className="flex items-center gap-2.5">
                    <div className={`w-2 h-2 rounded-full ${s.active ? "bg-green-400" : "bg-gray-200"}`} />
                    <span className="font-mono text-[14px] text-gray-800">{s.name}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    {s.active && <span className="text-[11px] text-green-500">本轮已加载</span>}
                    <span className="text-[11px] text-gray-400">{s.source}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
          {tab === "workspace" && (
            <div className="space-y-4">
              <div>
                <label className="block text-[13px] text-gray-600 mb-1.5 font-medium">当前工作区</label>
                <div className="flex gap-2">
                  <input type="text" defaultValue="E:\projects\auth-service"
                    className="flex-1 font-mono border border-gray-200 rounded-xl px-3 py-2 text-[13px] text-gray-900 outline-none focus:border-blue-400 transition-colors" />
                  <button className="px-3 py-2 text-[13px] border border-gray-200 rounded-xl hover:bg-gray-50 transition-colors">浏览</button>
                </div>
              </div>
            </div>
          )}
        </div>
        <div className="px-6 py-4 border-t border-gray-100 flex justify-end gap-2 flex-shrink-0">
          <button onClick={onClose} className="px-4 py-2 text-[13px] text-gray-600 border border-gray-200 rounded-xl hover:border-gray-300 transition-colors">取消</button>
          <button onClick={onClose} className="px-4 py-2 text-[13px] bg-gray-900 text-white rounded-xl hover:bg-gray-700 transition-colors active:scale-[0.97]">保存</button>
        </div>
      </div>
    </div>
  );
}

// ─── First-Run Wizard ─────────────────────────────────────────────────────────

function FirstRunWizard({ onDone }: { onDone: () => void }) {
  const [step, setStep] = useState(1);
  const [endpoint, setEndpoint] = useState("");
  const [model, setModel] = useState("");
  const [workspace, setWorkspace] = useState("");
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-white">
      <div className="w-[520px]">
        <div className="mb-8 text-center">
          <div className="w-14 h-14 rounded-2xl mx-auto mb-4" style={ORB_STYLE} />
          <div className="text-[24px] font-semibold text-gray-900 mb-1">pulse7</div>
          <div className="text-[14px] text-gray-500">首次运行配置向导 · rc-0.14</div>
        </div>
        <div className="flex items-center gap-2 mb-6">
          {[1, 2, 3].map(n => (
            <div key={n} className={`h-0.5 flex-1 rounded-full transition-colors ${n <= step ? "bg-gray-900" : "bg-gray-200"}`} />
          ))}
          <span className="text-[12px] text-gray-400 font-mono flex-shrink-0">{step}/3</span>
        </div>
        <div className="border border-gray-200 rounded-2xl overflow-hidden bg-white">
          <div className="px-6 py-5">
            {step === 1 && (
              <div className="space-y-4">
                <h2 className="text-[16px] font-semibold text-gray-900">连接配置</h2>
                {[
                  { id: "ep", label: "端点地址", type: "url",  val: endpoint,  set: setEndpoint,  ph: "http://192.168.1.100:8080/v1" },
                  { id: "mn", label: "模型名称", type: "text", val: model,     set: setModel,     ph: "gpt-4o 或内网模型名" },
                ].map(f => (
                  <div key={f.id}>
                    <label htmlFor={f.id} className="block text-[13px] text-gray-600 mb-1.5 font-medium">{f.label}</label>
                    <input id={f.id} type={f.type} value={f.val} onChange={e => f.set(e.target.value)} placeholder={f.ph}
                      className="w-full font-mono border border-gray-200 rounded-xl px-3 py-2.5 text-[13px] text-gray-900 placeholder-gray-300 outline-none focus:border-blue-400 transition-colors" />
                  </div>
                ))}
              </div>
            )}
            {step === 2 && (
              <div className="space-y-4">
                <h2 className="text-[16px] font-semibold text-gray-900">选择工作区</h2>
                <div>
                  <label className="block text-[13px] text-gray-600 mb-1.5 font-medium">工作区路径</label>
                  <div className="flex gap-2">
                    <input type="text" value={workspace} onChange={e => setWorkspace(e.target.value)}
                      placeholder="E:\projects\my-project"
                      className="flex-1 font-mono border border-gray-200 rounded-xl px-3 py-2.5 text-[13px] text-gray-900 placeholder-gray-300 outline-none focus:border-blue-400 transition-colors" />
                    <button className="px-3 py-2 text-[13px] border border-gray-200 rounded-xl hover:bg-gray-50 transition-colors">浏览</button>
                  </div>
                </div>
                <div>
                  <div className="text-[12px] text-gray-400 mb-2">最近使用</div>
                  {["E:\\projects\\auth-service", "E:\\projects\\legacy-api"].map(p => (
                    <button key={p} onClick={() => setWorkspace(p)}
                      className="block font-mono text-[13px] text-blue-500 hover:text-blue-700 py-0.5 transition-colors"
                    >{p}</button>
                  ))}
                </div>
              </div>
            )}
            {step === 3 && (
              <div className="space-y-4">
                <h2 className="text-[16px] font-semibold text-gray-900">确认配置</h2>
                <div className="space-y-2 text-[13px] border border-gray-100 rounded-xl p-4 bg-gray-50">
                  {[["端点", endpoint || "(未设置)"], ["模型", model || "(未设置)"], ["工作区", workspace || "(未设置)"]].map(([k, v]) => (
                    <div key={k} className="flex gap-3">
                      <span className="text-gray-400 w-14 flex-shrink-0">{k}</span>
                      <span className="font-mono text-gray-700 break-all">{v}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
          <div className="px-6 py-4 border-t border-gray-100 flex items-center justify-between bg-gray-50">
            <button onClick={onDone} className="text-[13px] text-gray-400 hover:text-gray-600 transition-colors">
              跳过
            </button>
            <div className="flex gap-2">
              {step > 1 && (
                <button onClick={() => setStep(s => s - 1)}
                  className="px-4 py-2 text-[13px] border border-gray-200 rounded-xl text-gray-600 hover:bg-white transition-colors">上一步</button>
              )}
              {step < 3
                ? <button onClick={() => setStep(s => s + 1)}
                    className="px-4 py-2 text-[13px] bg-gray-900 text-white rounded-xl hover:bg-gray-700 transition-colors active:scale-[0.97]">下一步</button>
                : <button onClick={onDone}
                    className="px-4 py-2 text-[13px] bg-gray-900 text-white rounded-xl hover:bg-gray-700 transition-colors active:scale-[0.97]">开始使用</button>
              }
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── App ──────────────────────────────────────────────────────────────────────

export default function App() {
  const [showWizard, setShowWizard]     = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [activeSession, setActiveSession] = useState("s-001");
  const [showEmpty, setShowEmpty]       = useState(false);
  const [turnStatus]                    = useState<TurnStatus>("done");
  const [busyGuard, setBusyGuard]       = useState(false);
  const [alert, setAlert]               = useState<"restricted" | null>("restricted");
  const [drawerOpen, setDrawerOpen]     = useState(false);
  const [model, setModel]               = useState("gpt-4o-internal");
  const [perm, setPerm]                 = useState<PermTier>("standard");
  const [workspace, setWorkspace]       = useState("E:\\projects\\auth-service");
  const timelineRef = useRef<HTMLDivElement>(null);

  const contextUsed  = 9841;
  const contextBudget = 12000;
  const contextPct    = Math.round(contextUsed / contextBudget * 100);
  const contextLevel: "normal" | "warning" | "critical" =
    contextPct >= 90 ? "critical" : contextPct >= 75 ? "warning" : "normal";
  const runningTasks  = MOCK_BG_TASKS.filter(t => t.status === "running" || t.status === "output_truncated");
  const isBusy = runningTasks.length > 0 || ["running", "streaming", "tool_running", "waiting"].includes(turnStatus);
  const processExceeded = true;
  const connected     = true;

  useEffect(() => {
    if (timelineRef.current && !showEmpty)
      timelineRef.current.scrollTop = timelineRef.current.scrollHeight;
  }, [showEmpty]);

  if (showWizard) return <FirstRunWizard onDone={() => setShowWizard(false)} />;

  return (
    <div className="flex flex-col h-full bg-white overflow-hidden text-gray-900">

      {/* ── Top Bar ── */}
      <header className="flex items-center h-10 border-b border-gray-200 flex-shrink-0 bg-white">
        <div className="flex items-center gap-0 px-3 border-r border-gray-200 h-full flex-shrink-0" style={{ width: leftCollapsed ? 52 : 252 }}>
          <div className="w-5 h-5 rounded-lg mr-2 flex-shrink-0" style={ORB_STYLE} />
          {!leftCollapsed && <>
            <span className="font-semibold text-[14px] text-gray-900">pulse7</span>
            <span className="font-mono text-[11px] text-gray-400 ml-1.5">· rc-0.14</span>
          </>}
        </div>
        <div className="flex-1 flex items-center px-4 min-w-0 h-full border-r border-gray-200">
          <WorkspaceSwitcher workspace={workspace} onSelect={setWorkspace} />
        </div>
        <div className="flex items-center gap-3 px-4 h-full flex-shrink-0">
          <div className="flex items-center gap-1.5 text-[11px] font-mono flex-shrink-0">
            <span className={`w-1.5 h-1.5 rounded-full ${connected ? "bg-green-500" : "bg-red-500"}`} />
            <span className={connected ? "text-gray-500" : "text-red-500"}>
              {connected ? "127.0.0.1:8080 · 已连接" : "已断开"}
            </span>
          </div>
          <div className="w-px h-4 bg-gray-200 flex-shrink-0" />
          <PermToggle perm={perm} onChange={setPerm} />
          <div className="w-px h-4 bg-gray-200 flex-shrink-0" />
          <button onClick={() => setShowWizard(true)}
            className="text-[11px] text-gray-500 hover:text-gray-900 border border-gray-200 hover:border-gray-300 px-2.5 py-1 rounded-lg transition-colors flex-shrink-0"
          >向导</button>
          <button onClick={() => setShowSettings(true)}
            className="text-[11px] text-gray-500 hover:text-gray-900 border border-gray-200 hover:border-gray-300 px-2.5 py-1 rounded-lg transition-colors flex-shrink-0"
          >设置</button>
        </div>
      </header>

      {/* Alert */}
      {alert && <AlertBanner type={alert} onClose={() => setAlert(null)} />}

      {/* Main */}
      <div className="flex flex-1 overflow-hidden">

        {/* Left Sidebar */}
        <LeftSidebar
          collapsed={leftCollapsed}
          onToggle={() => setLeftCollapsed(c => !c)}
          active={activeSession}
          onSelect={id => { setActiveSession(id); setShowEmpty(false); setBusyGuard(false); }}
          onNew={() => { setShowEmpty(true); }}
          isBusy={isBusy}
          onBusySwitch={() => setBusyGuard(true)}
        />

        {/* Content area */}
        <div className="flex-1 flex flex-col overflow-hidden">

          {/* Busy-switch guard banner */}
          {busyGuard && (
            <div className="flex items-center gap-3 px-5 py-2.5 border-b border-amber-200 bg-amber-50 text-[13px] text-amber-800 flex-shrink-0">
              <span className="w-1.5 h-1.5 rounded-full bg-amber-500 flex-shrink-0 pulse-dot" />
              <span className="flex-1">当前会话有任务运行中，中断或完成后再切换</span>
              <button
                onClick={() => setBusyGuard(false)}
                className="text-[12px] text-red-500 hover:text-red-700 border border-red-200 hover:border-red-300 px-2.5 py-1 rounded-lg transition-colors flex-shrink-0"
              >■ 中断当前任务</button>
              <button onClick={() => setBusyGuard(false)} className="text-gray-400 hover:text-gray-700 transition-colors">✕</button>
            </div>
          )}

          {showEmpty ? (
            /* ── Empty State ── */
            <div className="flex-1 flex flex-col overflow-hidden">
              <div className="flex-1 overflow-y-auto">
                <EmptyState workspace={workspace} onSuggest={_s => setShowEmpty(false)} />
              </div>
              <InputCard
                turnStatus={turnStatus}
                onInterrupt={() => {}}
                contextPct={contextPct}
                contextLevel={contextLevel}
                model={model}
                onModelChange={setModel}
              />
            </div>
          ) : (
            /* ── Active Conversation ── */
            <div className="flex-1 flex flex-col overflow-hidden">
              <div ref={timelineRef} className="flex-1 overflow-y-auto">
                <div className="max-w-[800px] mx-auto">

                  {/* Session init (compact) */}
                  <div className="flex items-center gap-3 px-6 py-3 border-b border-gray-100 text-[12px] text-gray-400">
                    <span className="text-green-600 font-medium text-[13px]">会话已建立</span>
                    <span className="font-mono">{workspace}</span>
                    <span>上下文预算 <span className="font-mono text-gray-600">{contextBudget.toLocaleString()}</span> token</span>
                  </div>

                  <UserMsg text="检查 handler.go 里的 JWT 验证逻辑，看看有没有问题，顺便在 middleware.go 里补一个限流中间件" />

                  <AssistantMsg model={model} text="好的，我先读取 handler.go 看看当前的 JWT 验证实现，分析是否有问题，再在 middleware.go 里添加限流中间件。" />

                  <SkillLoadedRecord name="git-ops" />

                  {/* Tool events */}
                  <div className="px-6 py-1 space-y-0.5">
                    {MOCK_TOOLS.slice(0, 2).map(t => <ToolRecord key={t.id} tool={t} />)}
                  </div>

                  <AssistantMsg model={model}
                    text={`handler.go 的 JWT 验证有两个问题：\n\n1. 没有验证 \`exp\` 字段——过期 token 仍然有效\n2. 错误时返回 HTTP 200（应为 401）\n\n我来给 middleware.go 写限流中间件，同时修正这两个问题。`}
                    toolCount={2}
                  />

                  <div className="px-6 py-1">
                    <ToolRecord tool={MOCK_TOOLS[2]} />
                  </div>

                  <AssistantMsg model={model} text="middleware.go 存在硬链接，直接写入会影响未知路径。我先检查链接数量再决定操作方式。" />

                  <div className="px-6 py-1">
                    <ToolRecord tool={MOCK_TOOLS[3]} />
                  </div>

                  <div className="px-6 py-1">
                    <OutsideWriteRecord tool="write_file" requestedPath="C:\Users\admin\AppData\Local\auth-service\rate-limit.log" />
                  </div>

                  <CompactionRecord method="truncate" beforeTokens={11843} afterTokens={7200} discardedBytes={18432} emergency={true} />
                  <ProcessWarning current={52} threshold={50} />
                  <WaitingRecord seconds={38} phase="waiting_first_token" />

                  <AssistantMsg model={model}
                    text="限流中间件已写入 middleware.go，采用滑动窗口算法，每 IP 每分钟 60 次请求。JWT 验证已修正 `exp` 检查和状态码。"
                    toolCount={3}
                    showCode={true}
                  />

                  <TurnResultBlock
                    status="success" rounds={8} elapsedMs={94300}
                    commands={["go vet ./src/...", "go build ./..."]}
                    outsideWrites={[{ path: "C:\\Users\\admin\\AppData\\Local\\auth-service\\rate-limit.log", tool: "write_file" }]}
                    bgProcesses={[{ taskId: "LUID-0x1a4", command: "go test ./... -v -run TestAuth" }]}
                  />

                  {/* Second turn */}
                  <div className="border-t border-gray-100 mt-2">
                    <UserMsg text="再跑一下测试，看看刚才的改动有没有引入新问题" />
                    <div className="px-6 py-1">
                      <ToolRecord tool={{ id: "tc-run", name: "shell", args: { command: "go test ./src/... -v", background: true }, status: "running", elapsedMs: 0, resultFull: "" }} />
                    </div>
                    <WaitingRecord seconds={23} phase="waiting_first_token" />
                  </div>

                  <div className="h-6" />
                </div>
              </div>

              <InputCard
                turnStatus={turnStatus}
                onInterrupt={() => {}}
                contextPct={contextPct}
                contextLevel={contextLevel}
                model={model}
                onModelChange={setModel}
              />
            </div>
          )}
        </div>
      </div>

      {/* Bottom status bar */}
      <BottomStatusBar
        onTaskClick={() => setDrawerOpen(o => !o)}
        taskCount={runningTasks.length}
        processExceeded={processExceeded}
        contextPct={contextPct}
        contextLevel={contextLevel}
      />

      {drawerOpen && (
        <TaskDrawer onClose={() => setDrawerOpen(false)} tasks={MOCK_BG_TASKS}
          processCount={52} processThreshold={50} processExceeded={processExceeded} />
      )}

      {showSettings && <SettingsPanel onClose={() => setShowSettings(false)} />}
    </div>
  );
}

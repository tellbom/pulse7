// pulse7 HTTP/SSE 客户端 — 接入方式与 agent/api_server.go 及 ui/main.js（C3 已验证路径）一致：
// 页面由 pulse7 serve 同源提供，token 经 <meta name="pulse7-token"> 注入（__PULSE7_TOKEN__ 占位符被服务端替换一次）。
// token 不入 URL、不进日志；所有 /api 请求带 Authorization: Bearer。

const tokenMeta = document.querySelector('meta[name=pulse7-token]');

export const token = tokenMeta ? tokenMeta.content : '';

// 占位符未被替换 → 页面不是由 pulse7 serve 提供（本地 dev / 静态部署回测）→ 使用内置演示后端。
export const liveMode = Boolean(token) && token !== '__PULSE7_TOKEN__';

export class ApiError extends Error {
  constructor(message, status, code) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export async function api(method, path, body) {
  let r;
  try {
    r = await fetch(path, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json'
      },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
  } catch (e) {
    throw new ApiError('本地服务不可达（' + e.message + '）', 0, 'network');
  }
  let data = null;
  try {
    data = await r.json();
  } catch (e) {
    /* 空响应体 */
  }
  if (!r.ok) {
    throw new ApiError((data && data.error && data.error.message) || `HTTP ${r.status}`, r.status, data && data.error && data.error.code);
  }
  return data;
}

// SSE：GET /api/events → `event: <type>` + `data: <统一包络 JSON>` + 空行（契约 C1.3）。
// 断开不自动重试、不承诺回放；由调用方呈现断开并提供手动重连。
export function openStream(handlers) {
  const ctrl = new AbortController();
  const run = async () => {
    try {
      const r = await fetch('/api/events', {
        headers: { Authorization: `Bearer ${token}` },
        signal: ctrl.signal
      });
      if (!r.ok) throw new ApiError(`事件流连接失败 HTTP ${r.status}`, r.status);
      handlers.open();
      const reader = r.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      for (;;) {
        const { value, done } = await reader.read();
        if (done) throw new Error('事件流已关闭');
        buffer += decoder.decode(value, { stream: true });
        let end;
        while ((end = buffer.indexOf('\n\n')) >= 0) {
          const frame = buffer.slice(0, end);
          buffer = buffer.slice(end + 2);
          const line = frame.split('\n').find((x) => x.startsWith('data: '));
          if (line) {
            try {
              handlers.event(JSON.parse(line.slice(6)));
            } catch (e) {
              /* 跳过无法解析的帧 */
            }
          }
        }
      }
    } catch (e) {
      if (ctrl.signal.aborted) return;
      handlers.error(e);
    }
  };
  run();
  return ctrl;
}

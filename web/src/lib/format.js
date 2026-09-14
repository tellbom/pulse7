// 展示格式化工具 — 单位与文案规则沿用 api-contract.md（估算值标估算、字节/毫秒原样可读）。

export function formatMs(ms) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function formatBytes(b) {
  if (b < 1024) return `${b}B`;
  if (b < 1048576) return `${(b / 1024).toFixed(1)}KB`;
  return `${(b / 1048576).toFixed(1)}MB`;
}

export function formatRuntime(ms) {
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  if (m === 0) return `${s}s`;
  if (m < 60) return `${m}m${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h${m % 60}m`;
}

// creationTime 是精确 FILETIME 十进制字符串，禁止走 Number（契约 C1.1）。
export function shortTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return String(iso).slice(0, 8);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

export function shortDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getMonth() + 1}-${p(d.getDate())}`;
}

export function dateGroup(iso, now = new Date()) {
  if (!iso) return 'earlier';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return 'earlier';
  const one = 86400000;
  const d0 = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const n0 = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const diff = Math.round((n0 - d0) / one);
  if (diff <= 0) return 'today';
  if (diff === 1) return 'yesterday';
  return 'earlier';
}

export function safeNumber(v, fallback = 0) {
  const n = Number(v);
  return isNaN(n) ? fallback : n;
}

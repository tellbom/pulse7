#!/usr/bin/env bash
# summarize.sh — 真模型 E2E 记录汇总脚本
# 用法: summarize.sh <场景名> <session.jsonl> <audit.jsonl> <总耗时秒>
# 输出: markdown 摘要（轮次 / 每轮延迟 / 上下文增长估算 / 是否收敛）
set -u
S="$1"; SESS="$2"; AUDIT="$3"; ELAPSED="${4:-?}"

# 轮次 = assistant 带 tool_calls 的消息数（每条代表一轮工具循环）
rounds=$(grep -o '"tool_calls":\[' "$SESS" | wc -l)
# 收敛 = 会话最后一条消息是 assistant 且无未配对 tool_call（简化：文件末行为 assistant）
last_role=$(tail -1 "$SESS" | grep -o '"role":"[a-z]*"' | head -1 | cut -d'"' -f4)
converged="UNKNOWN"
[ "$last_role" = "assistant" ] && converged="YES(给出终答)" || converged="NO(中断/未收敛)"

# 工具调用统计（来自 audit；先落临时文件，进程替换路径部分 grep 不可读）
T=$(mktemp)
cp "$AUDIT" "$T" 2>/dev/null || cat "$AUDIT" > "$T"
tool_calls=$(wc -l < "$T")
tool_kinds=$(grep -o '"tool":"[a-z_]*"' "$T" | sort | uniq -c | sed 's/"tool"://;s/"//g' | tr '\n' ' ')

# 每轮延迟：audit 时间戳（RFC3339，含 +08:00）首末差
first_ts=$(grep -o '"ts":"[^"]*"' "$T" | head -1 | cut -d'"' -f4)
last_ts=$(grep -o '"ts":"[^"]*"' "$T" | tail -1 | cut -d'"' -f4)
span=""
if [ -n "$first_ts" ] && [ -n "$last_ts" ]; then
  f=$(date -d "$first_ts" +%s 2>/dev/null); l=$(date -d "$last_ts" +%s 2>/dev/null)
  [ -n "$f" ] && [ -n "$l" ] && span=$((l - f))
fi
rm -f "$T"

# 上下文增长估算：会话总字符数（消息 content 之和的近似 -> token ≈ chars/4）
chars=$(wc -c < "$SESS")
tokest=$((chars / 4))

echo "## 场景 $S 汇总"
echo "| 指标 | 值 |"
echo "|---|---|"
echo "| 工具轮次 | $rounds |"
echo "| 工具调用总数 | $tool_calls |"
echo "| 工具分布 | ${tool_kinds:-无} |"
echo "| 首末工具调用跨度(秒) | ${span:-n/a} |"
echo "| 总耗时(秒) | $ELAPSED |"
echo "| 会话文件大小(字节) | $chars |"
echo "| 上下文 token 估算(chars/4) | $tokest |"
echo "| 是否收敛 | $converged |"

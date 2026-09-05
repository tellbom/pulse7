# 真模型三场景标准回归集

**每次改系统提示词或工具描述后，必须跑这一组**（依据 Pre-RC0.2 任务书 T2）。

## 场景

| # | 任务 | 判定要点 |
|---|---|---|
| S1 | 修复 `calc.py` 的 `people=0` 错误 | 直接动手，不提问，`python calc.py` 输出 `each pays: 30.0` |
| S2 | 在 `utils.py` 加 `trim(s)` 并在 `main.py` 接入 | 直接动手，不提问，输出含 `trim: hello` |
| S3 | 模糊任务"把这个项目整理一下，让它更好" | 只读探索 + **一个具体提问**，工作区零改动 |

## 用法

```cmd
:: 1. 重置三个工作区到初始状态
cmd /c poc\real-e2e\reset.cmd

:: 2. 设置 Key（不落盘）
set WIN7_AGENT_API_KEY=sk-xxx

:: 3. 跑三个场景（参考 run-all.cmd.template 的占位符替换后使用）
:: 每个场景最多 3 次预算

:: 4. 统计
python poc\real-e2e\metrics.py out-s1.jsonl out-s2.jsonl out-s3.jsonl
```

## 判定

- S1/S2 不提问 = 通过（过纠正 = 提示词要往回收）
- S3 提问且零改动 = 通过
- 三场景存档到 `artifacts/<轮次名>-e2e/`

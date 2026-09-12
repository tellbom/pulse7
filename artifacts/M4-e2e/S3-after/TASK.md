# S3 — 项目整理结果

## 原始文件
- `TASK.md` — 任务描述
- `notes.txt` — 随手记（费用记录）
- `old_tmp.py` — 临时测试脚本（已迁移）

## 整理后结构

```
.
├── README.md          # 项目说明
├── TASK.md            # 此文件（原始任务描述）
├── Makefile           # 常用命令
├── notes/             # 笔记/记录
│   ├── README.md
│   └── expenses.md    # 整理后的费用记录
├── src/               # 源代码
│   └── expenses.py    # 费用计算脚本
└── logs/              # 日志
    └── run.log        # 整理日志
```

## 变更说明
- 将零散文件按功能分类到目录中
- 将 `notes.txt` 中的随手记整理为结构化 Markdown 表格
- 将 `old_tmp.py` 重写为有意义的 `src/expenses.py`（含详细输出）
- 添加 `README.md` 项目说明
- 添加 `Makefile` 方便运行

## 运行方式
```bash
make run
```

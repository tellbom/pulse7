# Pre-RC0.2 findings（本轮发现但未修）

| # | 发现 | 影响 | 来源 | 建议 |
|---|---|---|---|---|
| 1 | R2 探索任务撞 30 轮上限：105 次 ls + 22 次压缩 = 探索死循环 | 探索类任务无法收敛；上下文碎片化 | T7-C R2 | **提示词优化**（T8 P0）：加"先看顶层，选 3-5 个关键文件深入，不要逐目录遍历" |
| 2 | 副本上出现 `.claude/` 目录（agent checkpoint 系统产物） | 不是项目改动但 git status 会显示为 untracked | T7-C R2 | 可在 agent 内将 checkpoint 数据目录改到产品目录而非工作区（或 .gitignore 排除） |
| 3 | `git diff --stat` 在 Win7 cmd 下输出用法信息而非差异（`--no-index` 语义冲突） | R1 的验证脚本 git diff 输出不可用 | T7-C R1 | 副本的 git 仓库缺 .git 目录（tar 复制时排除了 .git）导致 git 认为不在仓库内；测试副本应包含 .git 或先 `git init` |
| 4 | PowerShell 2.0（Win7 默认）不支持 `-File` 参数 | 无法直接用 PS 模拟 agent grep 性能 | T7-A | 改用 findstr 或降级语法 |
| 5 | ripgrep 14.x 起 Win10+，Win7 需锁定 13.0.0 | 如集成 rg，版本必须冻结在 13.0.0 | T7-B | 记录在案，集成时注意 |

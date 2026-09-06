# win7-agent RC 0.1 使用说明

win7-agent 是一个在 **Windows 7 SP1 x64** 上本地运行的 AI 编程助手（命令行工具，无界面）。
它通过您配置的 OpenAI 兼容接口调用大模型，在您指定的工作区内读写文件、执行命令，并提供可回退的检查点。

## 一、安装

1. 把整个 `win7-agent` 文件夹复制到目标机器（例如 `C:\win7-agent`）。
2. （可选，建议）以管理员身份运行 `install.cmd`：
   - 会自动检查运行环境并生成配置模板 `config\agent.json`；
   - 如果本机已安装 Sandboxie，会自动配置专用的 `Win7Agent` 沙盒；
   - **任何时候都不需要给系统打补丁**。若沙盒不可用，程序会自动使用内置的降级隔离模式（JobObject），功能不受影响。
3. 安装日志在 `logs\install.log`。

> 不装 Sandboxie 也能用（自动降级模式）；想启用完整沙盒隔离，自行安装 Sandboxie Classic 5.73.2 后重跑 `install.cmd` 即可。

## 二、配置

编辑 `config\agent.json`，只需要关心这几项：

```json
{
  "base_url": "https://你的接口地址/v1",
  "api_key": "你的密钥",
  "model":    "模型名",
  "workspace": "工作区路径（agent 只能读写这个目录内的文件）"
}
```

命令行参数优先于配置文件（`win7-agent.exe --help` 查看全部参数）。

## 三、开始使用

打开命令提示符（cmd）：

```
cd C:\win7-agent
win7-agent.exe                  :: 进入交互模式（提示符 > ，输入 /exit 退出）
win7-agent.exe exec "任务描述"   :: 单次执行一个任务
win7-agent.exe doctor           :: 环境自检（排查问题第一步）
```

每次任务结束会打印本次执行过的 shell 命令清单（这些操作的影响无法通过回退撤销）。
工作区的每次改动都有检查点，随时可以要求回退（“回滚到上一个检查点”）。

## 四、出问题看哪里

| 问题 | 看哪里 |
|---|---|
| 环境不对 / 沙盒模式疑问 | 运行 `win7-agent.exe doctor`，或看 `logs\install.log` |
| 接口连不上 / 模型报错 | 检查 `config\agent.json` 的 base_url / api_key / model；网络需允许访问接口地址 |
| 想知道 agent 干过什么 | `data\sessions\audit.jsonl`（全部工具调用记录）、`data\sessions\sess-*.jsonl`（完整对话） |
| 想恢复文件 | 让 agent 回滚检查点，或用 `runtime\git\cmd\git.exe log refs/win7-agent/checkpoints/` 查看 |

## 五、卸载

运行 `uninstall.cmd`（加 `/full` 参数会连 Sandboxie 一起卸载）。
**您的代码、文档等工作区文件不会被删除。**

## 六、安全说明

- agent 只能读写配置的工作区目录内的文件；
- 执行 shell 命令前默认需要确认（`--yolo` 可跳过，请谨慎）；
- git 的提交/推送等改写历史的操作被禁止，需要时请您自己在终端执行。

## 七、项目约定文件 AGENT.md（可选但推荐）

在工作区根目录放一个 `AGENT.md`，写清楚项目约定，agent 每次启动都会把它作为项目规则注入（上限 8KB，超出自动截断）。模型就不用猜你的项目规矩了。

示例（放在工作区根目录，文件名必须是 `AGENT.md`）：

```markdown
# 项目约定
- 本项目使用 Python 3.6，禁止使用 f-string（用 .format()）
- 运行测试：python run_tests.py
- 代码风格：4 空格缩进，禁止 tab
- 禁止修改 legacy/ 目录下的任何文件
```


## 八、自动化调用（exec 模式退出码）

| 退出码 | 含义 |
|---|---|
| 0 | 任务正常完成 |
| 1 | 出错（网络/模型/工具失败） |
| 2 | 模型以提问结束，等待用户回答后 `--resume` 续跑 |
| 130 | 用户按 Ctrl-C 中断 |

在脚本中调用 `win7-agent.exe exec "任务"` 后检查 `%ERRORLEVEL%` 即可区分。

## 九、会话管理

```
win7-agent.exe --list                    :: 列出最近 20 个会话（时间/工作区/首条消息/条数）
win7-agent.exe --resume <id> exec "继续"  :: 按会话 id 恢复（id 见 --list 输出的文件名）
win7-agent.exe --resume latest exec "继续" :: 接最近一次会话
```

## 十、Ctrl-C 中断

- 第一次 Ctrl-C：杀掉正在运行的子进程、补写会话、打印摘要、退出码 130
- 第二次 Ctrl-C：立即强制退出（不等清理）

## 十一、运行日志

agent 自身输出同时写入 `data\logs\agent.log`（与控制台同步）。控制台输出
在某些场景（计划任务/远程重定向）下可能丢失，agent.log 始终可靠。

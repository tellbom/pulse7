# 阶段三证据

GUI空工具结果修复：empty-tool-win7.py为端侧159项全量脚本；empty-tool-live.py/controller.py为官方deepseek-v4-pro经Win7 serve的空ls→write→read验证。原始empty-tool-win7.txt与empty-tool-live-win7.txt仅保留本地（遵守正式仓库不收原始日志的规则）。构建包含用户未提交正式GUI资源，非干净提交构建；结果、哈希与部署说明见phase3-report最后一节。用户GUI文件未改，runtime仅本地补齐。

C2见c2-build.json、c2-results.json、c2-comparison.json；生产源码提交f8f2c2a，构建时为该提交前的修改状态，精确改动文件SHA见c2-build。

C3：c3-api-first-win7.txt/c3-api-final-win7.txt为Win7五项API专项；c3-live-first-win7.txt保留首次历史ID查询失败，c3-live-second-win7.txt为修正后真实HTTP固定模型联调。c3-vm-preflight.txt为启动窗口。c3-live.py为当前联调脚本，首轮只差fixture目录及异常堆栈记录，失败未被覆盖。产品构建SHA嵌在各live日志中；当前构建来源唯一开发树的C3未提交状态，不是最终干净全量证据。

node-runtime下工具用于随后C4构建，不属于C3运行验证；Node16.18下载源nodejs.org/dist/v16.18.0/win-x64/node.exe，未修改历史树tools。

C4：c4-browser-first 至 seventh-win7.txt 是独立 Win7 夹具的浏览器验证，全部保留；详细失败和修正链见 phase3-report.md C4 节。c4-browser.py 为最终夹具，控制端上传后在 Win7 Python3.7 运行 Chrome109 原生 sandbox。每份日志记录当次产物哈希与起止时间；源码均来自唯一开发树 7dd6066 上的 C4 修改状态。c4-win7.txt 为较早五项 API 专项，c4-connectivity.txt 为命令行引号导致的探测失败，不是 VM 失联或产品失败。

最终源码59af113（本开发树干净提交）：build-hashes.json列构建命令与哈希；final-amd64-win7.txt缺H_PRODUCT，configured日志缺Git PATH并跳过16项，git-path日志为预检查启动错误；complete-env日志为157项、0跳过、exit0。final-win7.py为完整环境版本，前序差别详见报告。cli-listener-win7.py/txt是同一产物CLI无监听端侧证据。

model-controller.py与model-guest-run.py负责官方DeepSeek实测；phase3-s1/s2/s3各自的log/jsonl/meta/audit/spec均来自Win7，同一59af113产品哈希。三组均401，非功能验收通过。model-python-path.txt记录新VM缺全局python，夹具仅补子进程PATH。R1/R2按用户本次裁决延期，没有生成替代证据。

录入纠正后的phase3-s1/s2/s3-key-corrected文件组：同一59af113产品，官方DeepSeek真实模型，独立Win7会话。401归因为执行方漏录一位。S2-key-corrected有embedded Python环境阻碍，完整保留；phase3-s2-python-corrected在独立Python副本中按原命令重跑。prepare-python-regression.py/txt记录仅副本改._pth；verify-model-regression.py/txt独立复核S1/S2运行输出与产物哈希、S3零改动。每组meta存起止时间、命令、源二进制哈希与文件前后哈希；详情见报告最后一节。

## 2026-09-13 GUI 后端增量

- gui-backend-win7.py：Win7 全量执行器（新日志覆盖写入，不读取历史测试结果）。
- gui-backend-win7.txt：本地保留、git ignored，amd64 163 PASS / 0 SKIP / exit 0，包含 VM 启动时间、运行窗口与受测二进制哈希。
- gui-backend-focused-win7.txt：本地保留、git ignored，新增链路用例后 TestGUI 五项通过 / exit 0；未重新宣称全量 164 项。
- build-hashes.json 的 gui_backend_20260913：本轮二进制与源码来源；基于 8be7874 + 后端未提交差异 + 既有 GUI 资源，不是干净提交构建。
- ../gui-backend-frontend-handoff.md：字段、前端修改项与验收边界。没有浏览器或真实模型新增证据。
## 2026-09-13 会话隔离与配置开放验证

session-isolation-win7.py / config-controls-win7.py 是 Win7 执行器；同名前缀 .txt 为本地原始日志（git ignored）。前者全量166项、后者全量170项，均0跳过、exit0。build-hashes.json 的 config_and_sessions_20260913 记录运行窗口、VM启动时间与上传哈希。源码来自 be8e130 上的工作树增量，包含并行配置/目录浏览草稿及原有GUI资源；不声称干净提交构建。配置风险/契约详见 ../config-controls-and-session-isolation.md。
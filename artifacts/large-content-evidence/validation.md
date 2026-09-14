# 验证记录

2026-09-13，主代理接手测试子代理额度中断后的工作。

- amd64：产品 build、test -c、vet exit 0。
- 386：产品 build、test -c、vet exit 0。未运行 386 测试。
- 后端 `git diff --check -- agent/*.go`：exit 0（Git 提示 LF/CRLF 转换，不是内容测试）。全仓已有前端尾随空白不由本轮改动。
- Win7 SSH：使用已有 new-vm-controller.py，连接 192.168.140.128:22，15 秒 banner 超时。实际错误 `Error reading SSH protocol banner`，最终 `paramiko.ssh_exception.SSHException: No existing session`。
- 本次没有上传、没有执行测试、没有取得本次 VM 启动时间、没有重启或绕到宿主机运行。运行项数、失败项数、exit code 均无本次运行数据。
- 产物哈希和当前顶层 Go 源文件快照哈希存于 build-hashes.json。来源不是干净提交，不能沿用既往测试日志证明本次通过。

恢复后使用 large-final-win7.py 在端侧执行 amd64 全量一次，保留首次结果。该脚本与 exe 为本地测试制品；本文件不是其执行结果。

## 192.168.140.130 验收结果

上述旧地址未验证记录已由此次新地址实测补齐。Windows 7 Ultimate 启动时间 20260913201629.932215+480；本轮完整输出直接由新进程抓取，无旧端侧日志读取。

全量：175 PASS / 0 FAIL / 0 SKIP，exit 0，运行时间戳 1789307155.823995 至 1789307230.2305865。产品与测试 SHA256 经端侧 certutil 和脚本两次核对均与本地相同。五个 TestLarge 测试通过。

独立 HTTP：第一次运行因测试脚本 meta 属性解析正则过时而 FAIL；日志 win7-live-first-130.log 保留。只修脚本后 status=PASS，时间戳 1789307308.6453247 至 1789307311.9095929，端口释放 true。实际写文件 1,100,000 字节，附件参数 1,100,038 字节分五页取回，比对成功；包含鉴权、权限请求、恢复、中断。模拟模型仅用于可重复驱动，不使用 API key。

本次没有源码变更，因此未重复全量。386、Sandboxie、真实模型和浏览器未验收。

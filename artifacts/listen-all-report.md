# 0.0.0.0 监听变更

用户明确批准扩大监听范围。newAPIServer 使用 net.Listen("tcp4", "0.0.0.0:0")；启动日志与 /api/listener 返回保持一致。随机端口、现有 Bearer token、静态页面自动注入 token 行为不变。README 与 api-contract.md 已同步。

API 测试的绑定断言改为 0.0.0.0；SSE 测试连接目标仍为 127.0.0.1，避免把通配绑定地址当作目标地址。未删除未鉴权请求 401、密钥不回传等断言。

验证：Go 1.20.14、CGO_ENABLED=0、GOOS=windows，amd64 产品与测试编译通过，386 产品编译及 vet 通过，修改范围 diff 检查通过。本轮连接 192.168.140.130:22 在 sock.connect 阶段 TimeoutError: timed out，未上传或执行 Win7 回归，未做跨机器 HTTP 实测，未用宿主机运行替代。此前压缩改造的 182 项通过不作为本次监听改动的实测结果。

本地候选：artifacts/large-content-evidence/listen-all-amd64.exe。工作树仍含此前未提交改动，未提交、打 tag 或 push。未修改用户前端、系统防火墙或历史树。

访问范围说明：页面会自动发放本进程 token，能打开页面者就能操作 Agent；API 的 401 检查不是远程登录认证。远程实际可达性还取决于防火墙和路由。

# 当前完成分支升格 main：正式仓库检查

代码基线：`d1822e8`，开发树 `E:/codex-worktrees/win7-agent/pre-phase3`。旧目录 `E:/win7-agent` 只读，不合并/rebase旧main，不读取旧main作为产品代码来源。

## 旧目录盘点

忽略CRLF/LF差异比较66份非产品代码候选：44份同路径内容相同，1份DELIVERY为当前更新版，21份只在旧路径存在。后者按文件名再查当前归档，7份F1–F4/G/H/阶段三/Direction/Process-HTTP/review已经存在等价版本，保留当前文件。

- 补入唯一仍有效的缺失设计文档：`artifacts/pulse7-UI-Design-Brief.md` v2，原样复制至`artifacts/decisions/`。它是后续正式UI方向，不冒充当前实现。
- 不迁移9月7/8旧代码评审、旧测试输出/探针/overlay、9月10评审请求、Pre-Phase3/P1旧计划：当前报告及后续裁决已覆盖，旧调试现场不进入正式源码树。
- 旧tools只含go/dl工具链与下载内容，不迁移。旧dist/installer/PoC冻结产物不作为现行实现来源。没有发现需要新增的顶层项目LICENSE；未擅自选择开源协议，第三方原许可证保留。

## 已完成整理

- 产品目录Git树完全不变：agent=`7c7fc8a998c8e2166e2504f48d62a0d455ab83c0`，ui=`30706d0b6ddea4445553af26c04a68742e24cfc0`；待提交与d1822e8一致，未引入旧源码。
- 新增README、现行DELIVERY和第三方许可入口；旧DELIVERY及早期根目录任务/评估文档归入docs/history。现行说明明确配置路径、serve入口、构建和验收延期项。
- 3436份旧包/运行时/PoC/冻结/原始测试制品从索引移除，磁盘原件保留。旧dist共2828文件、约1.30GB，不再进入正式源码树。
- Markdown报告/决策、必要构建哈希和阶段三回归脚本保留；日志/session/audit/cache/临时文件忽略。历史附件不公开随库，artifacts/README明确证据链接边界。
- 两份原未跟踪的nongit checkpoint报告纳入档案，未改其内容。
- 初次清理后索引227文件约1.65MB；未命中真实sk格式密钥、私钥标记、password/passwd字面量；无exe/dll/7z/zip/log/jsonl/pyc。测试用dummy值/示例不作为凭据。Git diff --check通过。此扫描是文件树检查，不是祖先历史清理。

## 推送阻断：祖先提交与远端旧引用中的凭据

当前基线祖先实际包含真实API Key：`poc/scripts/ff-e2e.cmd:3`等12个脚本。代表blob=`4b713873325085ff3788c1e1ddf12978408a1c82`；在`d1822e8`中仍可读取，其最后修改提交为`5af374c8e259fea8b0c73b83501c3dfd47600d7a`。本文不记录密钥内容。删除最新树中的脚本并不能删除父提交里的密钥。

远端检查：origin=`https://github.com/tellbom/pulse7.git`，默认HEAD为main，main=`c517b4a8b75612524c51b166ef454da601ef3baa`。按远端返回OID核对本地对应对象，以下旧引用的PoC脚本已确认含凭据：

- 分支：encoding-pagination、file-encoding、final-fix、main、master、output-layering、slow-network。
- tag：rc-0.2、rc-0.3、rc-0.3.1、rc-0.3.2、rc-0.3.3、rc-0.4、rc-0.5、rc-0.6、rc-0.7。
- m2-freeze、m3-complete、rc-0.1在本次PoC脚本检查未命中，不据此声称全历史无凭据。

因此“保留当前HEAD祖先链直接推main”和“正式仓库不含凭据”不能同时满足。准备采用的最小发布方案：以整理后的**同一文件树**创建无旧父提交的发布快照，作为新main；本地保留原开发线与旧main引用。这样产品blob不变，但新main提交ID与父提交关系会改变，需要用户明确同意此点。含凭据的远端旧分支/tag若仍保留，仓库其他引用仍能访问密钥，是否移除须同时裁决；不擅自删除既有tag。

本次尚未切换main、尚未push、尚未改远端引用。正式推送必须以再次读取的远端main OID作为显式force-with-lease预期值；若远端变化则停止，不覆盖未知提交。旧凭据已经存在远端历史，须由凭据持有人作废/轮换；Git引用整理不能保证托管平台缓存或他人克隆中的旧对象消失。

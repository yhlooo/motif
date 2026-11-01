# MVP 设计对齐修正

## 依据与优先级

已逐项核对总体 MVP 与采集、整理、Web 三份细化文档，包括 `origin/main` 的 `23379cb`（动态 CR/评论/范围）和 `a16e449`（参考资料清理）。后更新的“已确认”用户决定优先于旧总体方案及旧实现说明；尚未确认的草案不整体作为契约。没有通过削弱确认决定来适配旧代码。

## 修正与验收

| 已确认要求 | 原实现偏差 / 本次修正 | 验证 |
|---|---|---|
| CR 跟随源/目标分支 | 冻结输入改为打开/主动刷新重建；commit + batch/patch 身份复用审批，新项 pending，离开范围保留历史 | `TestRefreshStableIdentitiesAndRebatch`、Web 过期写入测试、Chromium |
| 一致视图 | 源采集/分批版本和发布版本事务检查；详情一致读取，diff/评论/审批携带版本，过期 409 | `TestReviewPublishRejectsChangedSourceRevision`、HTTP 回归 |
| GitHub 三点内容与可达提交集 | 去掉线性限制，拓扑枚举非 merge 提交，投影连续状态链，merge 独有变化未知补齐 | `TestDAGRangeProjectsBothBranchesAndMergeOnlyChanges`、`TestMergeOnlyRangeStillReviewsFinalChanges`（零非 merge 提交但有独有差异仍可创建 CR） |
| 评论按 batch/代码位置映射 | 原锚点不变，跨组合选择逐步映射路径和存活行；删除或离开范围隐藏 | `TestCommentsMapInsertedDeletedAndRenamedLines`、Chromium 评论/回复 |
| 历史改写可验证迁移 | 新增 old/new、origin event/batch；amend/rebase/squash/split 生成新提交派生记录；冲突/不确定部分 unknown；原始历史不变，新提交不继承审批 | review 包的 amend/rebase/squash/split/冲突测试；真实安装脚本 + Git amend/rebase |
| 部分提交 close/carry | 工具内 amend 继续真实调用；改写后的逆变换可验证才保留配对，否则降级并诊断 | `TestAmendInsideToolPreservesPartialCommitPair`、`TestRebasePreservesVerifiedInverseVirtualPairs` |
| 相对 Git 基准存储 | 原完整快照改为稀疏覆盖、删除标记；未变字节从 Git 读取，不写 Motif blob；旧格式可读 | `TestSparseSnapshotsKeepOnlyOverridesAndRequireBase`、采集/重开库测试 |
| 纯 Go + GORM、历史兼容 | 保持 glebarez/sqlite，全 DB 操作使用 GORM；增量 v1→v6，保留历史 | v1 fixture、约束/回滚/竞争游标、CGO=0 全量测试与双平台构建 |
| Git hook 兼容 | 四种通知原样转发 argv/stdin，已有 hook 收到相同输入和退出码；Motif 失败放行 | streaming hook 测试、真实 Git 通知集成 |
| 安装诊断及共享卸载 | 实际 CLI 查询有效启用/信任；共享 hooksPath 所有权、linked worktree、原 hook/符号链接与备份冲突保护 | 安装测试、外部共享路径测试、doctor mock 与实际 CLI 0.163.0 查询 |
| 项目隔离/完整卸载 | 新增映射表纳入项目删除；独占 blob 删除，共享内容保留；单 Agent 卸载保留历史 | capture/store/integration 原有及调整后的删除回归 |
| 参考资料只保留文档/链接 | 删除此前 SQLite/GORM/旧驱动源码副本，保留版本固定索引与官方 README；不新增 Codex 源码/schema | reference 清单及研究文档链接核对 |

旧版 v1 CR 没有保存来源 batch UID。升级时可以证明相同事件/提交/内容的非 batch 项接回原记录；旧 batch 不靠相同标题或成员猜身份，以免把已删除重建批次当成旧批次批准。无法证明的项显示升级提示、当前新项 pending，历史审批和评论保留。`TestLegacyReviewRefreshOnlyAdoptsProvableIdentity` 覆盖安全接回与不误继承两种情况。

派生 batch 是独立可管理实体：复制可证实的来源说明，具备真实成员，可删除/重新整理；不会在读取时重建已被用户删除的批次。已批准旧项的历史保留不等于新提交自动批准。

## 最终验证记录（2026-11-01）

- `CGO_ENABLED=0 go test ./...`、`go vet ./...`、`git diff --check`、前端 `node --check` 通过。
- `CGO_ENABLED=0` Linux amd64 与 macOS arm64 构建通过。
- 真实已安装 hook：临时仓库正常提交/切换/引用事务、amend 与无冲突 rebase；CLI 查询到新提交上的来源事件与继承批次说明，原提交历史仍保留。
- Chromium：真实二进制、临时 Git 与 hook；创建 CR、合并选择、评论/回复、批准后源分支新增提交（旧项保持批准，新项 pending）、删除源 batch 后刷新（新身份 pending，旧评论隐藏），均通过；桌面/窄屏检查，脚本错误 0。
- 实际 `codex-cli 0.163.0` 的只读 `app-server hooks/list` 查询在隔离 CODEX_HOME 中核实。项目未信任时 hook 缺失；项目受信任而命令未信任时 enabled=true、trust=untrusted，doctor 均报告未生效。未修改用户实际信任配置。

## 保留边界

未运行真实 Codex 模型会话，不能把输入协议测试和有效配置查询当作模型执行到 hook 的全链路验证。明确保留串行采集、Git 基准对象依赖、不实现常规 GC、不恢复未观测中间修改等已确认一期边界。历史改写的细粒度归属只在可验证时继承，其余明确未知。

旧实现说明和存储说明已同步修订；总体 MVP 增加后续确认决定优先的解释，三份确认来源文档原文保持不变。

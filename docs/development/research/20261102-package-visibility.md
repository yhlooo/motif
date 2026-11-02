# pkg / internal 包可见性评审

日期：2026-11-02。范围：当前 `pkg` 和 `internal` 全部 13 个 Go 包，以及 `cmd/motif`、测试和发布配置中的调用点。本文为评审建议，未修改实现，也不是已采纳设计。

## 结论

业务实现放在顶层 `internal` 总体合理；两个 `pkg` 包没有显示出需要向外部 Go 程序提供接口的用途，建议移入 `internal`。更明显的内部边界问题是 `review.Service` 暴露整个工作区，而工作区又暴露 Git 和 Store，使 Web 绕过应用服务访问持久化。其次是部分仅包内使用的符号仍被导出。

应区分三个层次：包能否被项目外部导入、符号能否被其它包访问、哪些依赖应由架构允许。顶层 `internal` 约束第一层，不会自动限制同项目中 transport 对 adapters 的依赖；目录层次也不会自动给子包以父包私有成员访问权。`pkg` 是目录命名，不能因为叫“共享基础能力”就推导出需要对外开放。

Go 官方建议优先将辅助包放入 `internal`，依据见[原文摘录](../reference/go/20261102-module-layout.md)。下面的具体迁移与接口判断来自本项目职责和调用点，不是 Go 强制要求。

## 逐包判断

| 包 | 当前判断 | 建议 |
| --- | --- | --- |
| `pkg/i18n` | 对外开放理由不足 | 移至 `internal/i18n`；它嵌入的是 Motif 消息资源，并非独立通用翻译库。当前只有 CLI 生产代码导入。`LocaleFS` 仅包内使用，应私有化。 |
| `pkg/version` | 对外开放理由不足 | 移至 `internal/version`；描述的是当前 Motif 二进制的版本和构建信息。同步 `.goreleaser.yaml` 中版本注入路径。跨包读取接口和构建注入点有用途。 |
| `internal/domain` | 包级可见性合理，部分接口可精简 | 事实、计划、投影输入输出需要跨 app/adapters 使用，应保留导出。`DeltaDigest` 仅在本包使用，可收回；State 的别名与转发需统一定位。 |
| `internal/domain/state`（包名 `change`） | 独立包及节点封装合理 | `root`、`node` 私有，State 对外提供不可变操作，这一边界很好。持久化 API 被 SQLite 使用，不能简单藏到 `domain/internal`。建议统一包名与路径，减少认知成本。 |
| `internal/content` | 合理 | Git、本地内容引用和租约被多个适配器使用，保留项目内共享；没有理由仅因基础设施性质而迁入 `pkg`。 |
| `internal/adapters/gitrepo` | 包位置合理，实例状态暴露偏多 | 缓存、锁、计数器已私有。`Root/GitDir/CommonDir/ObjectFormat/Objects` 可被调用者修改，考虑私有字段配只读查询；当前 `Git` 确有应用层调用，不能直接改小写。 |
| `internal/adapters/sqlite`（包名 `store`） | 持久化实现隐藏较好，导出面偏宽 | `db`、record 类型私有合理。`PreparedStates`、`PrepareCapture`、`ContentHash` 仅包内使用，可收回；`ProjectionWarnings` 未找到调用，可考虑删除。`Dir` 应避免外部可改，防止 DB 与对象目录不一致。 |
| `internal/app/workspace` | 装配位置合理，聚合对象边界偏宽 | `Project` 匿名嵌入 `*gitrepo.Repository`，并公开 `Store`，等于暴露整个适配器组合。装配代码可以了解具体实现，但不应因此让所有上层调用者获得全部能力。 |
| `internal/app/capture` | 包位置合理，入口可收敛 | `Codex`、Git 通知等作为 CLI 入口合理；`Rewrite`、`References` 当前只由包内 Git 通知分派调用，可私有化。`CodexInput` 当前由跨包测试构造，可另用测试输入结构后再收回。 |
| `internal/app/review` | 包位置合理，服务边界需加强 | 保留 Preview/Create/Refresh、评论映射等业务入口；`CommentAnchor` 仅由本包调用，可私有化。`Repo`、`Worktree` 公共字段使调用方能替换服务依赖和身份；建议构造时传入、运行时私有。 |
| `internal/app/integration` | 基本合理 | 安装、校验、卸载、诊断供 CLI 使用。`Manifest/File/Diagnostic/HookState` 是结果结构及其组成类型，保留导出能方便调用者构造、处理结果。 |
| `internal/transport/cli`（包名 `commands`） | 包位置合理，符号导出明显偏多 | main 需要 `NewCommand`、`ExitCode`。选项类型、构造函数、context 辅助函数、`Msg*` 等多数仅包内使用，可私有化；空 `Options` 可删除。`UsageError` 目前也仅包内构造。 |
| `internal/transport/web` | 自身封装合理，对应用层的访问过宽 | `server`、资源和路由方法私有，`Serve`/`Handler` 导出适合作为启动与 HTTP 测试接口。但 handlers 直接访问 Store 和 Git，应逐步通过 review 应用接口完成审阅操作。 |

## 优先处理的内部边界

### 1. Review 服务不是完整的审阅调用边界

具体证据：`internal/app/review/review.go:21` 的 `Service` 公开 Repo/Worktree，`internal/app/workspace/workspace.go:12` 的 Project 公开底层适配器。`internal/transport/web/server.go` 经由 `s.service.Repo.Store` 完成列表、历史投影读取、选区恢复、审批、线程读取、回复等操作；也直接调用 Repository 的 Diff、Branches 和 Config。

这意味着 transport 了解审阅持久化的调用组合，未来增加统一校验或调用顺序要求时，不能只修改 Service。直接读存储不等于当前存在错误，但应用边界没有覆盖它所代表的完整用例。

建议先将审阅读取、diff、审批、评论和回复等操作收敛到应用服务，再把依赖及身份字段收为私有；不要仅改字段大小写后增加 `Store()` getter，那不会减少可见能力。可以继续使用具体适配器，无需为每个表建立接口。CLI 的启动装配可以直接打开 Store；需要区分装配依赖与请求处理依赖。

### 2. State 有两组访问入口，且存储格式穿透到 SQLite

`domain/types.go:23` 起别名及转发 `change.File/State` 和七个函数；SQLite 既通过 domain 使用 State，又直接导入 state 包恢复状态、解析 `NodeRecord`。这是同一概念的双入口，而非两个独立模型。

可选择：明确 state 是共享值包，调用者直接使用它，减少 domain 转发；或者由 domain 提供完整门面。前者更自然，但应随设计统一决定。`PreparedStates` 的准备代码目前反序列化 `NodeRecord` 来登记内容引用，因此把 NodeRecord 改私有前，需要为持久化过程提供明确的内容引用访问方式。当前直接删除导出会破坏实际用途。

`Clone` 在不可变 State 上仅返回原值，保留它并无复制安全收益；可以在统一 State 接口时清理，不必为此单独拆包。底层 AVL 节点已经隐藏，不需要继续增加目录来强化同一边界。

### 3. 构造后的适配器身份不宜任意更改

`gitrepo.Open` 检查数据目录与工作树的关系，并解析 Git 路径和对象格式；Store.Open 打开数据库并设定对象目录。此后公开可写字段允许程序内部调用者改变这些设定，同时保留原缓存或连接。当前未发现调用方这样修改，属于不变量保护不足，而非已证实运行故障。建议提供只读信息访问，构造时固定依赖。

## 不应机械收回的导出

- `gitrepo.ReadStats`、`Store.TransactionStats` 被 review 跨包测试/基准使用，当前有明确观测用途。
- `web.Handler` 被外部测试包使用，也是合理的 HTTP 组装接口。
- `review.PreviewRange/CommitRange` 被跨包测试使用，并能表达固定 commit 范围；只因生产调用来自包内就收回，可能损失合理能力，需要结合预期接口判断。`ResolveTracking/BranchTracking` 则目前仅包内调用，可另行评估。
- `MappedThread/LineRange`、`HookState`、`CallTransition/ChangeCommit` 等是公开结果/计划中的组成类型，保留命名能力合理，即使调用者不直接拼写类型名。
- `TableName`、`MarshalJSON/UnmarshalJSON`、`Error/Unwrap` 等满足框架或标准接口，不能按静态调用计数判断多余。
- `LoadReviewStates` 在适配器外部测试中使用，应先明确是否保留为显式状态加载接口，再决定收回。

## 建议顺序与验证边界

先确认应用是否准备提供外部 Go 库接口；按当前用途，建议两个 pkg 包全部内部化。随后收回无跨包用途的实现符号，最后以完整应用用例收敛 Web 访问底层适配器的通道。State 双入口及构造后身份封装可以随接口整理一起处理，不建议为了层次对称增加大量嵌套 internal 或仓储接口。

本次核对源码导出、生产和测试调用、发布配置及已有架构设计；没有修改 Go 代码，也未运行测试。仅靠仓库检查无法证明不存在仓库外部消费者；“移入 internal”是基于当前项目用途的建议，不是外部兼容性已得到证明。

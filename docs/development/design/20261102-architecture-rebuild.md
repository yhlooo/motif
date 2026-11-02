# 从状态、事实与投影重新组织 Motif

状态：用户已授权实施，核心模型已落地。本文以 `b2f7b56` 为重构前基线；实际结构、验证及边界见[实施记录](20261102-architecture-rebuild-implementation.md)。用户明确项目没有用户，无需兼容旧数据库、API 和内部接口。

依据：[重构后评审](../research/20261102-post-refactor-review.md)、[已确认采集规则](20261101-capture.draft1.md)、[已确认整理规则](20261101-organization.draft1.md)、[已确认 Web 规则](20261101-web.draft1.md)。本文中的模型和算法是本项目的架构决定；性能数字只以实施记录中的实测为依据。

## 1. 判断：更好的实现应该少表达几次同一件事

现实现已经能表达主要产品语义，但一个文件状态先由 Git 读取、转成全仓 hash map，再经 Store 回调恢复、复制为审阅项两端状态，最后重新比较为 diff。业务规则和内容准备又混在 Store 中，通过 context 隐式传递准备结果。这使实现成本取决于全仓规模，理解成本取决于调用顺序。

建议围绕三个明确对象重新实现：

1. **State：某个时刻有哪些路径、每个路径是什么内容。** 不可变、共享结构，通过内容引用读取字节。
2. **Change：一次观测或经过验证的派生变换。** 记录发生了什么；提交归属、分批和改写来源单独关联。
3. **Projection：针对明确 commit 范围发布的一条展示状态链。** 不可变；审批和评论单独保存，不把来源记录或审阅状态揉进投影对象。

核心链路为：观察 → 纯计算变换 → 原子记录；明确范围 + 来源版本 → 构建投影 → 原子切换发布指针。所有读取固定对象身份，所有写入显式声明预期版本。

这不是将现有 Store 拆成更多小 Store。内容引用、状态表示、事务输入和发布模型都需要改变。

### 1.1 从产品必需的不变量推导实现

Motif 的核心职责是：保存可证明的修改过程，再把选定提交范围组织为可审阅的解释。由此可以直接推导五个约束：

| 产品事实 | 必需的不变量 | 架构决定 |
|---|---|---|
| 历史记录要解释过去发生的变化 | 已记录内容不能随当前分支或进程缓存改变 | 内容和状态具有可持久解析的不可变引用 |
| 每次工具修改通常只涉及仓库的一部分 | 每个历史步骤不应拥有一份独立全仓副本 | 路径结构共享，步骤只引入变化节点 |
| 分批和提交归属可能晚于实际修改确定 | 修改事实与后续解释具有不同生命周期 | Change、CommitChange、Batch 分开建模 |
| 一个审阅页面内的多个请求必须看到同一内容 | 阅读对象的身份在整个请求组合中固定 | 不可变 Projection，以发布指针选择当前版本 |
| 审批和评论发生在代码投影之后 | 内容计算不能覆盖并发产生的审阅行为 | 决定与评论独立存储，事务校验当前投影 |

这些约束来自产品本身，不是套用某种架构风格。因而本方案明确选择结构共享、显式内容定位及不可变发布模型；不再把完整 map 复制、Git 回调注册及原地覆盖投影作为长期实现方向。

### 1.2 为什么值得整体替换

当前实现依靠额外机制补偿表示方式的不足：内容 hash 丢失 Git 定位信息后，要注册读取闭包；每项拥有完整状态后，要靠多层缓存抑制重复读取；投影项原地更新后，要管理 active 状态和历史位置；Store 负责外部准备后，要通过 context 告知事务哪些 IO 已完成。

新模型直接保留定位信息、共享状态结构、固定投影身份、显式传入发布计划。这些补偿机制因此可以删除。其价值首先是缩小正确性需要考虑的状态空间，其次才是性能。

路径树的实现需要额外代码，但它集中解决所有快照和投影的共享问题，拥有独立、可对照验证的接口；比在采集、改写、审阅和 Store 各自处理全状态复制更合适。它必须保持为路径查找、更新和差异比较三个窄能力，不能发展成通用版本数据库。

明确结论：在没有用户数据兼容成本的当前阶段，应按本方案重构。基准用于确定缓存容量和发现实现瓶颈，测试用于检验不变量；它们不替代架构选择，也不要求先保留两套生产实现才能作出决定。尚未实测只意味着不能承诺加速倍数，不意味着对依赖方向、生命周期划分和状态模型没有明确判断。

## 2. 保留的产品语义与可以删除的历史负担

保留：连续快照补记 unknown；串行 Agent 归属；轮次结束前分批；同轮次、同工作区、同提交边界连续 AI patch 成批；部分提交 close/carry；改写可验证才继承来源；三点内容与可达非 merge 提交集；连续组合 diff；稳定身份审阅；评论按位置映射；分支更新失败保留已发布范围；项目级安装卸载和 hook 兼容。

删除：v1–v7 数据库兼容、旧完整快照格式、旧项身份猜测、字段别名、旧 API/JSON 协议、旧内部包接口。新数据库直接建立新 schema，不编写旧库迁移器。开发数据通过显式重置命令处理，绝不因启动发现旧库而自动删除目录；目录版本不匹配时直接说明如何重置。此处仅是方案，本文不执行任何数据删除。

依然使用 Go、SQLite、GORM 和本地内容库；它们不是本次问题的根源。不引入服务端、消息队列、通用事件溯源框架或每张表一个仓储接口。真实开发历史继续保留，不因“无需兼容旧实现”而取消产品自身的历史语义。

## 3. 模块及依赖方向

```text
CLI / HTTP / hook adapter
          │
          ▼
application：Capture / Organize / Rewrite / Refresh / Review
          │                       │
          ▼                       ▼
domain：状态、转换、归属、投影规则   ports：按用例定义输入输出接口
                                  ▲
                                  │
                    adapters：Git / SQLite / object store
```

领域不依赖 GORM、Git 子进程、HTTP 或文件路径布局。应用服务负责准备 IO、调用领域计算、提交结果及有限重试。适配器依赖领域类型和接口，彼此不注册回调；入口负责装配。

实际目录：`internal/domain`（状态与纯规划）、`internal/content`（内容定位及对象库）、`internal/app`（采集、改写、审阅及集成）、`internal/adapters/{gitrepo,sqlite}`、`internal/transport/{cli,web}`。没有建立未被实际替换需求使用的通用 ports 层。只有确有外部消费需求的能力才放 pkg。目录不是目标，禁止循环依赖和显式输入才是目标。

| 模块 | 负责 | 输入/输出边界 |
|---|---|---|
| GitReader | 解析引用、提交图、树条目及 blob 字节 | repo ID + OID；不认识数据库事务 |
| ContentResolver | 按引用读字节、精确比较内容、准备文本变换 | ContentRef；不读取业务记录 |
| StateEngine | 路径查找、状态差异、共享更新 | StateRef / Delta；不全仓读取字节 |
| CapturePlanner | 观测分类、游标转换、close/carry | CaptureInput → CapturePlan |
| RewritePlanner | 旧到新提交的来源验证与未知降级 | RewriteInput → RewritePlan |
| ProjectionBuilder | 来源选择、重放、项身份、闭合及诊断 | ProjectionInput → ProjectionDraft |
| Repository | 一致读取、跨行约束、CAS 发布 | 按用例接收显式计划；不解析内容 |
| ReviewService | 决定、评论和映射用例 | projection ID + stable item identity |

领域规划器需要文本合并结果时，先返回明确的内容准备需求，由应用服务交给解析器，再以准备好的输入继续计算；不在纯规则函数中偷偷执行 Git。状态算法本身可依赖只读的内存对象视图，视图缺少输入时必须显式返回需求/错误。

## 4. 内容：引用本身足以重新找到字节

```go
type ContentRef struct {
    Kind ContentKind // GitBlob 或 LocalBlob，构造器保证互斥字段
    Repository RepositoryID // GitBlob 使用
    ObjectFormat ObjectFormat
    OID ObjectID
    Digest SHA256 // LocalBlob 使用
}
type Entry struct {
    Mode FileMode
    Content ContentRef // 普通文件与符号链接的原始字节
    Gitlink ObjectID   // 子模块，不解析成普通内容
}
```

Git 树条目保留 OID，不为建立 State 而读取所有文件并计算 SHA-256。采集到的变化写入本地 blob，以 SHA-256 定位。符号链接保存链接目标字节；mode、gitlink、原始路径字节属于显式状态。采集范围变化使用独立 coverage 标记，不能与“路径不存在”混淆。

`ContentResolver.Read(ctx, ref)` 根据引用直接访问 Git 或本地对象库。无需 `hash → closure` 来源登记表，重启和缓存淘汰不影响定位。Git 对象缺失仍明确报错；本方案不增加保留引用，也不承诺 Git GC 后仍可恢复。

不同引用可能表示相同字节，尤其是 GitBlob 与 LocalBlob。必须区分：

- 引用相同可快速证明内容相同；引用不同不能证明字节不同。
- 状态树的结构摘要用于对象身份与缓存，不能直接当作字节等价性证明。
- 精确比较仅对候选变化路径读取内容。可缓存字节摘要；关键归属验证按所需精度比较实际内容，不靠标题、相似路径或来源解释猜测。

工作区扫描先比较 mode、范围与候选文件内容；若字节与已有 Git 引用一致，复用旧引用。删除后恢复、改写生成相同内容等路径同样做规范化，避免表示差异产生伪 patch。文本合并生成的新字节先写对象库，再产生 LocalBlob 引用。

## 5. 状态：不可变的持久化路径索引

采用以原始路径字节排序的持久化 AVL 索引。State 只持有根节点；每个节点包含一条路径的 Entry、左右子树引用和派生高度/数量。Set/Delete 只复制并重新平衡搜索路径，未变子树共享；旧 State 不会被修改，也不存在无限增长的 parent+delta 恢复链。

选择 AVL 而非初稿中的目录覆盖树，是因为当前用例主要需要精确路径查询、更新和两状态差异。AVL 用一套局部平衡规则实现这些能力，不必额外维护 Git 子树、overlay、删除覆盖和多种节点归并语义。目录结构不是来源归属的边界，路径索引即可承载已确认语义。

Git 树首次读取只列举路径、mode 和 blob OID，不读取全仓文件字节；条目排序后一次建立平衡索引。工作区观测以此前状态或 Git 基准为起点，复用未变引用，只更新变化路径；仍需扫描采集范围内文件以观测真实内容。索引不是文件系统变化监视器。

节点按确定性编码计算 SHA-256，存入本地对象库；Snapshot 保存根引用及基准 commit。初始基准索引会持久化路径元数据，后续快照共享这些节点。没有复制未变 Git blob 字节。对象归属表按项目记录节点及本地内容的持有关系，准备新根时遇到本项目已持有的子树即可停止遍历。

主要接口：Lookup、Set、Delete、ChangedPaths、Root、Persist、Restore。相同结构子树可直接跳过；旋转导致结构不同的部分通过有序比较判定，语义相同不要求物理根 ID 相同。内容身份与存储引用分开比较。

首次冷恢复仍需加载该根所含索引节点；之后通过有容量限制的节点缓存共享不可变节点。没有声称实现按路径惰性加载整个 Git 树。组合 diff 只恢复选择两端，评论映射才恢复选择内中间状态，不为单项读取恢复全部审阅项。

这种表示将状态保留成本从“每步一份全仓 map”改为“一份初始索引，加变化路径的新节点”。完整重建基准索引、工作区观测和冷恢复仍存在与路径总数有关的成本，需分别测量。

## 6. 事实、提交归属和分批分别建模

```text
Observation：采样时刻、工作区、执行上下文、SnapshotRef
Change：before/after root、观测区间、kind、call、pair
CommitChange：commit OID、顺序、change ID、归属依据
RewriteDerivation：旧来源 → 新 change、验证结果与诊断
Batch：独立身份、说明、所属边界
BatchMember：batch → 有序 change
```

Change 追加后不改 before/after 或来源；提交时新增关联，不再修改事件行上的 final_commit。unknown、tool_edit、virtual_close、virtual_carry 用明确枚举表达。提交归属可以晚于观测建立，原始事实与归属关系的生命周期因此分开。

改写后新增派生 Change 和关联，来源边可多对多，能表达 squash/split；不把派生记录混进工作区原始 sequence。每个 commit 的来源清单有显式 revision，并明确选用原始还是某个已发布派生清单；多个工作区竞争声明同一 commit 时报告歧义，不静默拼接。

Batch 是可管理实体，删除只移除当前分组，不删除 Change。派生 batch 保存 origin batch 关联和独立 UID；用户删除后不得在 GET 或 Refresh 时悄悄重建。新来源到达时可在明确的整理事务中补建派生组，受已删除标记/操作版本约束。自动 unknown 分组与 AI 分组分开规则：自动组成员变化必须改变批次内容 revision，未知项身份包含其内容/成员指纹，不能把扩展后的未知内容视为已批准。

## 7. 采集：显式计划、显式版本、一次提交

```go
type CaptureInput struct {
    Cursor Cursor
    ActiveCall *Call
    Observation Observation
    Head CommitInfo
    PreparedStates PreparedStates
}
type CapturePlan struct {
    ExpectedCursorVersion uint64
    Receipt Receipt
    NewCursor Cursor
    Changes []Change
    Associations []CommitChange
    Calls []CallTransition
    Snapshots []Snapshot
}
```

应用流程：

1. 在短只读事务中取得 cursor、active call、必要关联及版本。
2. 读取 Git/工作区，准备状态和内容；规划器计算缺失间隔 unknown、调用开始/结束与提交闭合。
3. 写入不可变对象。产出明确的 CapturePlan，计划不携带事务句柄或读取闭包。
4. SQLite 单事务校验 receipt、cursor、必要业务版本及引用合法性，追加记录并更新 cursor。
5. 竞争则重新观测和规划，有限重试。相同 receipt/相同输入直接返回既有结果，不提高来源 revision；receipt 相同而内容冲突明确失败。

事务内允许查询跨行规则和操作记录，不允许 Git、读大文件、内容哈希及投影计算。对象写入和删除协调见第 12 节，不能只靠事务前一次 os.Stat 声称引用永久安全。

Pre 到 Post 的变化只有执行身份能证明时归为 AI；旧 open call 不代表正在运行。缺失 Post、迟到 Post、Interrupt 与工具内 Git 提交继续按原确认规则处理。串行采集前提不扩大为并发 Agent 自动归因承诺。

部分提交设：上次状态 P、提交树 C、当前工作区 W。先补记 P→W 的实际观测，再记录 W→C 的 close，将已闭合片段关联提交；未提交链从 C→W 的 carry 开始。具体观测顺序及在工具调用中的归属以真实执行上下文决定，不能将 close/carry 标成新 AI 编辑。配对仅在两项均处于范围且逆变换可验证时省略；组合链最终仍须精确闭合。

## 8. 改写：一个独立的归属计算用例

RewriteService 取得 old/new 映射、原来源清单、批次快照及新旧 commit 树，交给 RewritePlanner。按新提交顺序精确重放可证明片段；不能唯一归属的路径生成 unknown，保存具体原因。旧事实不动，新 commit 不继承旧 commit 的审批。

结果以 RewritePlan 发布：预期 cursor、受影响来源 revision、派生 changes、来源边、独立 batches、commit 清单和恢复观察。单事务校验后发布，避免一半提交已经关联、一半尚未恢复游标。相同通知的重试返回同一已发布结果。算法版本纳入派生结果身份；算法升级通过显式重建生成新版本，不在读取路径悄悄改变来源。

## 9. CR：不可变投影加一个发布指针

```text
Review：标题、可选 BranchTracking、trackingVersion、publishedProjectionID
Projection：明确 CommitRange、merge-base、输入签名、算法版本、摘要、诊断
ProjectionItem：projectionID、position、stableIdentity、beforeRoot、afterRoot、来源清单
Decision：reviewID + stableIdentity → 当前决定
Thread：reviewID、原始 projection/item/路径/侧/行范围/内容引用
```

Projection 一旦发布不再修改；新刷新产生新 Projection，再原子切换 Review 的指针。不再将旧项改 active=false、搬负 position、覆盖快照引用。历史投影通过 ID 固定，读取不依赖“当前项表恰好没变”。现阶段保留历史投影，不必新增历史浏览界面。

稳定身份依旧以 commit + batch/change UID 为核心；新提交、新 batch UID 均为新身份。完整显示内容另有 payloadDigest，覆盖状态变换、来源及说明；用于诊断同身份内容是否变化。不能未经产品讨论便把“内容变则撤销所有旧批准”引入本次重构。未知补齐身份必须带精确净变化指纹；自动 unknown batch 的成员变化按第 6 节处理。

构建输入来自一个一致来源快照：提交集合、每提交来源清单、批次 revision 与内容、来源诊断。ProjectionBuilder 不读可变分支，只接受完整 commit 对与该快照。

提交范围继续采用既定 merge-base→source 内容、target..source 非 merge 提交的逆拓扑枚举。无共同祖先/多个最佳 merge-base 明确失败。提交重放为连续状态链，merge 独有变化及遗漏用最终 unknown 项补齐。闭合失败的候选不得发布；补齐成功不意味着历史归属完整。

### 9.1 刷新与版本

1. 读取 Review 指针、trackingVersion 和仓库来源 revision R。
2. 可选解析分支得到候选 OID；直接 commit CR 跳过跟踪解析。
3. 签名由仓库 ID、两端 OID、比较模式、R、算法版本组成。未变直接复用。
4. 读取一致来源快照，确认 revision；在事务外构建并持久化候选对象。
5. 再次检查跟踪分支；变化则有限重试。Git 与 SQLite 不存在跨系统原子性，发布含义是成功捕获的明确 OID 对。
6. SQLite 事务校验旧发布指针、trackingVersion、R 及项目未删除，插入投影/项/摘要并切指针。

来源 revision 先采用**每仓库单调 revision**，只有影响投影的实际变化才递增。一次无变化观察、重复 receipt、其他项目采集、审批/评论均不递增。接受同仓库无关变化可能触发重建；不从第一版设计精细依赖图。签名变化而投影语义摘要相同，可更新已检查输入记录并复用当前投影，不改写旧投影的输入元数据。

刷新 attempt 独立编号，较旧 attempt 不覆盖较新的提示。分支解析、构建和数据库故障分类，失败保留发布指针。详情可继续读取，实际缺失内容只影响依赖它的请求。

### 9.2 审批与读取

读请求显式指定 projectionID；它是内容版本。详情和列表只读摘要，不恢复状态。diff 只读选择端点；GET 无构建副作用。旧投影可按 ID 读取，但写请求必须在事务内确认它仍是当前发布投影。

批量审批检查全部选中 stableIdentity 属于当前投影后一次写入；空选择、未知项或过期版本整体拒绝。Decision 单独有 revision；并发修改同一决定使用预期 revision，防止旧页面把后来决定覆盖。投影切换与审批共享 SQLite 事务序列，构建器不读取决定，因此构建期间的新决定不会被旧候选覆盖。

Review 状态由当前投影有效项和决定计算/事务维护摘要：任一拒绝为 changes_requested，全批准为 approved，其余 in_review；零项明确为无待审阅项，不能显示“0 项全部批准”。

## 10. 评论：保存原锚点，映射结果可丢弃

评论锚点保存创建时 projectionID、所属 stableIdentity、原路径字节、old/new 侧、行范围、StateRef/ContentRef。回复引用 thread ID，携带当前 projectionID；事务校验所属稳定项仍可见。旧提交上的评论不因相同文本迁移到新提交。

映射先确定所属项仍在目标投影，再沿状态链跟踪该路径与存活行，精确内容重命名仅在唯一时追踪。位置删除或归属项离开范围则隐藏。映射失败不修改原锚点，不把它吸附到附近相似行；内容缺失与“位置已删除”分开返回。

MapThreads 的输入是原锚点、固定目标投影和连续选择，不扫描/恢复全仓每项。缓存键包括锚点、目标投影/选择、映射算法版本；缓存可清空，原评论和定位所需状态必须可从持久引用恢复。

## 11. 持久化布局与约束

| 数据组 | 表/对象 | 主要约束 |
|---|---|---|
| 项目与执行上下文 | repositories、worktrees、turns、calls、cursors、receipts | 作用域、游标 CAS、receipt 唯一 |
| 观测事实 | observations、snapshots、changes | 不可变记录；工作区原始序列唯一 |
| 提交归属 | commit_manifests、commit_changes、rewrite_derivations | 每提交清单版本明确；有序成员唯一 |
| 整理 | batches、batch_members、batch_operations | 成员唯一；连续/边界约束；删除操作可识别 |
| 审阅 | reviews、projections、projection_items | 发布指针；投影内顺序/身份唯一 |
| 协作记录 | decisions、threads、comments、refresh_attempts | review 作用域；决定 revision；原锚点不变 |
| 内容 | 本地 blobs、state nodes；数据库 object_refs | 不可变对象；项目持有关系 |

SQLite 负责关系、索引、约束、事务；对象库负责字节和状态节点。GORM 类型留在适配器内，不能作为领域实体。领域规划负责规则推导；事务必须校验规划依赖的版本，不能把“事务外验证过”当作跨行约束不会变化的证明。

Repository 接口围绕 `LoadCaptureInput / CommitCapture`、`LoadRewriteInput / CommitRewrite`、`LoadProjectionInput / PublishProjection`、`WriteDecisions / AddThread / Reply` 定义。禁止把 `func(tx *gorm.DB)` 暴露给应用层；内部可以复用事务助手。

持久对象使用带格式版本的确定性编码，字段类型、原始路径字节和排序规则明确。结构身份摘要与内容字节等价性继续区分。引用缺失/损坏必须报错，不返回空状态。

## 12. 生命周期、删除与缓存

纯数据库事务无法覆盖外部对象删除。采用统一的本机数据目录生命周期锁：普通用例持有共享租约，全量卸载持有排他租约；跨进程实现，hook 与 Web 使用同一机制。租约覆盖准备、对象写入、数据库提交及读取所需时间。Git 自身 GC 不受此锁保护，缺失错误仍需处理。

项目卸载先关闭其新用例入口，排他租约内事务标记删除、移除项目关系和 object_refs，再删除确认为无其他项目持有的本地对象。失败清理用明确删除任务记录重试；项目可见性不依赖文件删除是否完成。不为减少事务时间而在其他进程仍使用引用时直接 unlink。单 Agent 卸载只解除集成，不走数据删除。

普通失败发布产生的不可达对象允许保留；本次不增加后台 GC。未来 GC 也必须服从生命周期锁和持有关系，不能扫描 hash 后随意删除。

缓存分别限制条目数与估算字节数：Git 子树条目、内容字节、内容等价摘要、diff、行映射。缓存只保存可重算值，不保存唯一的内容定位来源；读取错误不作为永久负缓存。容量是可调实现参数，性能数字以基准验证，不在设计中预先承诺。

## 13. 接口和前端围绕固定投影组织

建议 API：创建/刷新返回 publishedProjectionID 和独立 refresh 状态；详情返回该 ID、范围、轻量项及审阅摘要；diff/threads 指定 projectionID + 连续项选择；写请求指定 projectionID，决定另带 decisionRevision。

错误分为输入/规则错误、过期版本、跟踪失败、缺失对象、内容损坏、存储故障。仅跟踪/构建失败可带可读的旧发布指针；数据库故障不包装成正常空结果。

前端打开时显式刷新，然后固定投影；并行 diff/threads 使用同一 ID。请求序号防止迟到结果覆盖新选择，刷新切换时清除旧行选择和评论输入定位。审批只更新决定摘要；不重新构建代码投影。旧投影写入返回冲突并引导重新加载。

安装脚本和 Codex 输入协议属于边界适配器，保持现有已确认行为；本次不改变外部 hook 语义，不因重构免兼容就破坏已有 Git hook 的 argv/stdin/退出码转发。

## 14. 不选的方案及代价

- **全部复制进 Motif 内容库：** 能减轻 Git 对象依赖，但违背已确认共享基准边界并扩大内容占用；不采用。
- **只存 patch 文本：** 难以可靠处理 mode、二进制、组合状态和精确位置映射；State/Delta 才是事实表达，diff 文本是输出。
- **无界 snapshot 父链：** 写入简单，旧状态恢复成本随历史增长；改用路径树结构共享。
- **每次刷新覆盖当前项表：** 容易读到跨版本组合，也需要 active/position 搬移；不可变投影让一致性直接由 ID 表达。
- **为所有查询构造通用事件重放：** 事实追加不等于整个应用必须从日志恢复；保留普通关系表和显式游标。

代价也明确：路径树和对象协议需要专门实现；不可变投影增加元数据及节点保留；显式计划使类型更多；生命周期锁让卸载等待正在执行的用例。接受这些成本，是为了让正确性可由对象身份、版本和边界解释，而非依赖隐含调用顺序。

## 15. 实施顺序与完成标准

允许直接替换 schema/API，不做新旧双写与迁移。仍按纵向能力验证，避免一次切换后无法定位语义问题：

1. 实现 ContentRef、路径树、对象库与精确等价性；建立独立领域类型与适配器装配。
2. 新 schema + CapturePlan，贯通 Pre/Post/Stop、未知补记、close/carry 及分批。
3. RewritePlan，覆盖 amend/rebase/squash/split 和来源不确定降级。
4. 不可变 Projection 与发布指针，接入纯读取、决定、评论映射和前端。
5. 生命周期锁、项目删除、安装/诊断适配；移除旧 Store、兼容迁移、回调注册和旧协议。

最终验收按不变量组织：

- 状态往返保持路径原字节、mode、符号链接、二进制、gitlink 和 coverage；混合 Git/本地引用不会产生伪变化。
- 随机状态更新、组合选择与独立全量参考模型比较；共享节点不可被调用者修改，历史深度不产生无界 overlay 解析链。
- Capture/Rewrite 重复通知幂等、receipt 冲突失败、cursor 竞争拒绝，迟到事件不重算历史；原始事实不被改写。
- 部分提交和改写投影的末态精确一致；未知诊断不能被末态补齐抹掉。
- 固定 projectionID 的并行读取永远一致；发布失败不动旧指针；过期/部分无效批量写入全部回滚；构建期间审批不会丢失。
- 删除/recreate batch、新 commit、范围离开再返回、评论删除行及唯一重命名符合身份规则。
- 每个仓库来源 revision 仅随真实来源变化递增；跨项目和决定写入不重建投影。
- 故障注入覆盖对象先写后 DB 失败、进程中断、内容丢失、卸载竞争；不出现可见记录指向未完成本地对象。
- 检查依赖图无 Store↔Git 回调；事务内注入器禁止外部内容 IO；context 只承载取消、截止时间和观测信息。
- 真实 hook/CLI、浏览器主流程、CGO=0 构建和全量测试重新验收，不能继承旧实现验证结论。

性能基准至少分文件数、改动路径数、事件数、batch 数、提交数，并区分冷/热缓存。记录峰值内存、Git 进程/字节、DB 事务完整耗时与端到端耗时。重点验证固定改动量时增加全仓文件数不再导致每项完整状态复制，以及只读单选择不恢复所有项；不设未经测量的倍数承诺。

## 16. 推荐决定

建议采纳这套整体模型，而不是继续在当前 Store 周围添加准备缓存。最关键的三项是：可独立定位的 ContentRef、共享结构的 StateRef、不可变 Projection。它们确定后，显式计划、短事务、按版本读取及缓存边界都能从模型自然推导。

用户已授权实施。实施记录区分本次已完成的模型、验证与保留边界；后续变更继续围绕这些不变量演进。

# 基于 Git 基线与稀疏覆盖的快照优化方案

状态：用户已采纳，核心实现已落地。日期：2026-11-06。基线：d430e4b。

本文保留采纳时的方案推导；实际接口、取舍与验证见 [实施记录](20261106-snapshot-performance-implementation.md)。实现以实施记录为准。

依据：当前实现、[性能调查](../research/20261106-snapshot-performance.md)、[早期采集规则](20261101-capture.draft1.md)、[现有架构](20261102-architecture-rebuild.md)。Git 行为参考原文见 [status](../reference/git/20261106/git-status.adoc)、[ls-files](../reference/git/20261106/git-ls-files.adoc)、[update-index](../reference/git/20261106/git-update-index.adoc)、[diff](../reference/git/20261106/git-diff.adoc)。

## 1. 推荐决定与目标

快照持久化改为“固定 Git tree + 相对该 tree 的完整稀疏覆盖”。commit 保留业务边界与来源，tree 标识实际文件基线。不构建或保存全仓 Motif 节点；不以 Git 临时提交作为每次快照载体；不把快照做成无限增长的前向增量链。

采集默认沿用当前实际工作区字节语义，不把 clean filter、换行或 LFS 转换后的提交字节冒充磁盘字节。允许枚举路径和扫描元数据，重点消除无条件全仓 ReadFile/哈希、全仓状态节点恢复与同步落盘。

本次只改状态表示与采集机制，不改 Agent 归属、unknown、分批、提交 close/carry、改写验证和审阅规则。原来的完整 State 仍可作为审阅计算的工作表示，但不再是采集和存储必须经过的格式。

目标分开衡量：

- 首次干净普通仓库：记录 tree 与空覆盖，不写 N 个状态节点，也不读取 N 个文件内容。
- 后续少量修改：读取必要候选内容，保存少量新增 blob，更新大小与未提交差异量相关的清单。
- Git 和文件元数据检查仍可能 O(N)，不承诺整个快照严格 O(本次修改数)。
- 有大量未跟踪内容、转换路径或未提交差异时，成本按实际需采集数据增长，不伪称常数耗时。

## 2. 数据表示

概念结构（字段类型在实施时按现有编码统一）：

```
Snapshot {
  id, repository_id, worktree_id, base_commit, trigger, observed_at
  state_descriptor_id
}
StateDescriptorV2 {
  repository_id, object_format, base_tree_oid // unborn 使用显式 empty baseline
  overlay_digest, overlay_entries
}
OverlayEntry = Upsert(path_bytes, mode, content_ref/content_id/gitlink_oid)
             | Delete(path_bytes)
             | OutOfScope(path_bytes, relevant_metadata)
```

- path 是原始字节，清单按字节排序，以 BLOB 或既有无损编码存储，不能依赖 UTF-8 JSON key。
- Upsert 是该时刻完整文件内容的不可变引用，不是文本补丁。二进制、空文件、符号链接目标、可执行位与 gitlink 都明确表达。
- Delete 不等于空文件，OutOfScope 不等于 Delete；继承只表示“该路径使用基线条目”。
- 每份清单包含相对固定 tree 的全部覆盖，而非仅相对前一快照的变化。恢复无需追溯快照链。
- 基线里不存在的路径恢复为不存在后，应从覆盖清单移除，不保留无意义 tombstone。与基线内容和模式完全相同的 Upsert 也移除。
- 相同 tree 和规范化覆盖可以直接判相同；不同 tree / 不同表示的 digest 不同，只能表示身份不同，不能据此断言语义有差异。
- 清单作为一个版本化不可变记录在 SQLite 事务内发布；索引字段关系化保存。避免重新产生“一条覆盖一个需要 fsync 的节点对象”。多个快照可复用同一描述符。

例如 HEAD 为 C0，第一次修改 a 并新建 b：S1=(C0.tree,{a:blobA,b:blobB})。随后只改 b：S2=(C0.tree,{a:blobA,b:blobB2})，a 内容直接复用。a 恢复基线后 S3 覆盖只剩 b。每次读取一份完整小清单即可。

## 3. 变化发现：Git 候选 + 元数据校验 + 特殊路径补集

### 3.1 候选不是直接等于快照

普通仓库通过稳定、NUL 分隔的 Git 输出取得 staged、unstaged、untracked 候选，关闭 rename 检测；快照表达新增/删除即可，rename 展示放在审阅层。候选可以多，不能漏。HEAD、index、工作区各自不同；读取内容始终读工作区，不拿 index blob 替代磁盘内容。

可从 `git status --porcelain=v2 -z --untracked-files=all --no-renames --ignore-submodules=none` 实现初版候选发现；具体命令组合通过行为矩阵验证后确定。只需要路径或 tree 差异时选 plumbing 命令，并明确禁止 textconv、external diff 与 rename 扫描。untracked 必须展开到文件，不能只记录目录。沿用不修改用户 index 的边界（GIT_OPTIONAL_LOCKS=0），不执行 git add / stash / commit 来取快照。

候选至少取以下并集：

1. Git 报告的 staged、unstaged 与未跟踪路径；
2. 上次 overlay 的全部路径，检查恢复基线、删除和 scope 变化；
3. 基准与当前 index 路径集合差异，尤其 staged delete 后磁盘仍保留原文件；
4. 元数据发生变化、缓存不存在或失效的路径；
5. Git 可能跳过的 assume-unchanged / skip-worktree 等路径，以及存在工作区字节转换的路径。

所有路径先分类再读取。祖先目录验证按目录批量复用，保持不跟随祖先 symlink 越出仓库的安全边界。忽略判定按 NUL 路径批处理；已跟踪路径不能因 .gitignore 而消失。

### 3.2 普通干净文件如何复用基线

不能声称任意仓库只要 status 为空就能引用提交字节。普通路径必须满足：Git 基线和 index 关系已确认、路径可见且模式匹配、没有字节转换或抑制检查属性，且变化检测的 stat 条件可信。满足时复用 Git blob，不读取内容。

对于 clean/smudge、text/eol、working-tree-encoding 等可能改变字节的路径：首次必须建立实际工作区内容身份；不能仅凭 status 干净复用 blob。后续使用已验证的缓存复用，缓存失效则重新读这些路径。属性变化要使受影响路径失效（无法可靠界定时保守扩大），不能只读 .gitattributes 自身。LFS 通过 filter 同样处理，不强行 hydrate 内容。

assume-unchanged / skip-worktree 路径不依赖 Git status 的省略结果；单独做元数据/存在性检查，必要时读取。sparse checkout 不在本次顺手重新定义：先保持当前基准+index+实际磁盘的采集边界，缺失路径行为用现有逻辑作对照；若要把 sparse 缺失定义为“继承”而非删除，必须单独确定产品语义。

Git 忽略 executable bit 或平台 symlink 行为也不能直接覆盖 Motif 的 mode/type 语义，需元数据补验。冲突 index 保持当前明确报错，不静默生成不完整快照。

### 3.3 跨 hook 缓存

文件缓存是可丢弃加速信息，不是历史事实：按 worktree 隔离，保存 path、type、device/inode（可用时）、size、mtime_ns、ctime_ns、mode、已确认的内容引用、检查时刻/时间戳安全信息，以及影响其有效性的属性/配置信息。

优先为未提交、转换与特殊路径缓存；普通干净路径复用 Git 已有的检测能力，避免为 N 个普通文件在每次 hook 重写 N 条数据库记录。若需要完整元数据表，按批更新，不逐路径事务或 fsync。

同 stat 不等于数学证明相同字节。遵循明确的非对抗文件系统与时间戳模型；与采集时间过近的 racy 条目、时间粒度不足、缓存版本不匹配、身份变化必须重读，不能仅用 size+mtime。异常文件系统需要保守回退，并在诊断里说明实际读取量。不存在“无文件事件源、不读内容、还能对任意伪造元数据保证无遗漏”的方案。

初版先保证仅重读未提交及特殊路径的正确性；稳定后再利用缓存降低“相对 HEAD 一直 dirty、相邻快照没变化”的重复读取。内容哈希仍用于实际读取的数据和不可变引用，并非完全取消哈希。

## 4. 与现有领域计算接入

不能只给当前 AVL State 外包一层含 Git 回调的 lazy state：现有领域规划器是纯计算，这会把 git IO 隐藏到 Equal / Lookup / Apply 中。

建议明确分开持久化身份与计算材料：

- Snapshot/Review 持有 StateRef（v2 描述符），不要求装载完整 State。
- 应用层 StateResolver 负责读取 tree 条目/覆盖/内容，准备领域计算需要的有限输入。
- CapturePlanner 消费已准备的状态引用及精确语义变化集，沿用来源归属和游标规则；既不启动 Git，也不将 digest 不同直接当成有变化。
- 审阅、改写、评论等需要现有完整 State 的路径，初版在应用层显式 materialize 后复用当前纯算法。持久化输出通过同一描述符编码器，不引入第二套互不兼容的状态存储。
- 原始 AVL 可保留为计算用数据结构，随后按性能需要优化；不要求一次重写全部审阅算法。

同基线两状态的候选仅为两个覆盖的路径并集；覆盖缺省端按需查 baseline。不同基线的候选为 Git 两个 tree 的变化路径与两侧覆盖路径的并集。按路径解析两个端点、比较实际内容身份和模式，再得到语义变化；没有必要读取两棵 tree 的全部 blob。Git blob 与本地 blob 的语义身份按仓库对象格式统一或进行准确字节比较，不能只比较存储引用。

原有 CapturePlanner 的 Equal 调用不能机械替换为 digest 对比。需引入显式 PreparedTransition（before_ref、after_ref、精确变化/相等结论），由应用层在规划前准备。不同表示下的 close/carry、重放闭合与稳定审阅身份均加入回归验证。

完整 materialize 只用于确实要枚举的冷路径；常规 hook 不允许为了满足旧接口重新构建全仓 State。

## 5. 提交边界和中间状态

- staged/unstaged 只用于发现路径，本方案记录最终工作区，暂不增加“完整 index 快照”这一产品功能。
- 部分提交 C0→C1 后，工作区剩余修改形成以 C1.tree 为基线的新覆盖；旧快照永久保留 C0，不覆盖历史。
- 仍按当前规则产生 virtual_close / virtual_carry、关联真实提交，不因覆盖表示简化而丢失事件。
- commit、checkout、reset、amend/rebase 要重新确认基线。捕获期间 HEAD 或 index 发生变化则有限重试；超过预算返回明确可诊断失败，不发布混合基线快照。
- Git 对象可读取是当前系统已有依赖。本方案显式扩大到 tree 引用，不能忽略生命周期：初版保留既有仓库对象存在前提；如果要支持 reflog 到期后仍可审阅，应另实现 Motif 专用 Git refs 固定可达性或独立对象保留，不把复制全仓 blob 放回 hook 热路径。引用创建/删除需要与数据库恢复协议一起设计，不能写完 DB 后任意删 ref。

## 6. 发布、锁和持久化

单次流程：

1. 解析事件、打开轻量 Store，尽早检查已完成 receipt 和无需采集的事件。
2. 读 cursor/version 和基线身份，枚举候选与 scope。
3. 稳定读取必要文件，计算新内容引用，生成完整覆盖；对读取前后元数据变化进行有限重试。
4. 准备新 blob 的持久化，复用已有内容；归属和存在检查按批查询。只有真正新内容才 fsync，目录同步可合并，不能移除持久性屏障。
5. 在短 SQLite 事务中发布描述符、blob ownership、快照/事件/receipt 与 cursor CAS，提交事务后快照才对外可见。
6. 最后更新可丢弃文件缓存；缓存绑定观测版本，失败或竞态只能损失性能，不能污染已提交快照。

正常打开已初始化 Store 不拿全数据目录排他锁；迁移、删除和清理仍按需互斥。持久化前到引用发布期间持共享生命周期保护，并先验证当前多进程嵌套锁与删除行为，避免引入新的锁序死锁。阻塞锁改为可取消获取。不同 worktree 不共享采集游标；同 worktree 保持 CAS 和有限重试，不承诺并发工具的精确归属。

失败可以留下待回收 blob，不能留下引用未持久化内容的已发布快照。清单在数据库内避免额外外部发布窗口；publish 和删除共享同一对象归属协议。不要以异步 hook、跳过快照或增大 timeout 代替完成采集。

文件系统级多文件原子快照不在当前保证范围。读取前后校验和 HEAD/index 复核只能减少竞态，不能把一次目录遍历变成同一时刻的文件系统快照。

## 7. 清除与采集无关的放大

- autoUnknown 改为只读取受本次变更影响的 segment 和相邻分组，不在每次提交中装载全部工作区历史；保持既有自动分组的连续性与分组修改语义。
- Stop 反馈按 turn 查询必要事件，避免全历史查询。
- 早期 receipt 检查只作快速返回；最终事务仍检查 receipt、digest 与 cursor，防止并发重复提交。
- 启动、Git 枚举、元数据检查、内容读取、持久化准备、锁等待、事务提交分别计时；统计 N、候选数、读取字节、覆盖条数、新 blob 数、fsync 数和历史查询行数。

## 8. 实施顺序

### 第一阶段：建立验证与新表示

增加分段日志和基准。落地 StateDescriptorV2、明确 resolver 和材料准备边界；将 baseline 持久化改为 tree 引用，覆盖清单作为单条不可变对象事务保存。普通 hook 不恢复或持久化全仓 AVL。初期可继续使用现有全量读取作为候选算法，先单独验证格式替换与业务语义一致性，不一次把存储与变化发现的错误混在一起。

交付条件：3 万文件干净首次不再产生 3 万节点对象/6 万次同步；原有采集、审阅、改写、评论和删除回归通过。此时只声称消除节点落盘放大，不声称已消除全量内容读取。

### 第二阶段：候选采集与保守补集

将全量读取替换为 Git 候选+元数据/属性补验；普通干净路径不读内容，转换/特殊路径保守处理。先允许重复读取 dirty 文件，再增加跨 hook 文件缓存。批量 check-ignore、归属查询以及增量 autoUnknown 同步完成。

交付条件：普通 3 万/10 万路径仓库只改一个文件，日志证实不读取全仓字节；特殊路径与基线算法逐状态一致；首次转换仓库读取量可以明显高于普通仓库，必须如实报告。

### 第三阶段：减少特殊大输入开销

依据实测决定是否引入对象 pack / 批量落盘、Git 基线条目缓存、进一步惰性审阅解析。第一版不必引入 daemon、文件监听器或创建隐藏 Git commit。若大量新增内容本身超预算，应流式读取/哈希并评估 pack，不能把大输入问题隐藏在缓存假设下。

数据库格式变更不能静默删旧数据。初版建议显式新写 v2、保留旧格式 reader 或提供单独迁移；是否可完全重置仅在实施时根据当前项目数据要求决定，不套用过去“尚无用户”的前提。

## 9. 验证矩阵与验收指标

以旧全量采集器作为测试 oracle（仅测试使用，执行两者时工作区固定），比较实际路径、内容字节、mode、gitlink 和 scope，不比较两种格式的 Root。至少覆盖：

- 干净首次、无变化重复、单文件、大文件、大量新增未跟踪、dirty 但本次未修改、恢复 HEAD；
- staged、unstaged、同文件两者兼有；staged delete 但磁盘仍存在；git rm --cached；mode/type 变化；
- 二进制、空文件、symlink、祖先 symlink、非 UTF-8/换行/空白路径；
- .gitignore 与全局 ignore 变化、tracked 被 ignore、scope exit/return、批量删除；
- text/eol、clean/smudge、LFS 指针/实际字节、属性变化、assume-unchanged、skip-worktree、sparse、冲突；
- unborn HEAD、worktree、多次部分提交、checkout/reset、amend/rebase、submodule 未初始化；
- 缓存丢失/损坏、同 size 修改、racy 时间戳、原子替换、读取中变化、HEAD/index 变化；
- 重复投递、游标冲突、并发删除、blob 写入后/DB 提交前进程终止、重启后解析与删除；
- 十万历史事件下采集查询规模、其他 worktree 正在持久化时的启动延迟。

性能报告分文件数 N、总字节 B、overlay 大小 K、本次实际读取 C 四个维度，至少提供 1万/3万/10万路径；首次、无变化、单文件变化各自测 p50/p95，冷/热缓存分开。验收首先是算法工作量：干净 baseline 新节点对象=0、普通干净路径读取字节=0、无变化新内容对象=0、历史查询不随全部历史增长。默认 30 秒内完成是端到端目标，需要指定磁盘/平台与输入上限后实测，不能提前承诺任意大仓库的固定秒数或倍数。

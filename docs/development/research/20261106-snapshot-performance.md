# 大仓库快照性能调查

调查日期：2026-11-06。代码基线：origin/main d430e4b。本节保留实施前调查结果；后续实现与验证见 `../design/20261106-snapshot-performance-implementation.md`。

## 结论

当前持久化使用结构共享 AVL，但采集并没有做到按变化量读取。首次采集有逐节点 SQL 查询、独立对象文件和同步落盘放大；后续每个 hook 新建进程，会完整恢复上次状态、重建 HEAD 状态，并全量读取和哈希工作区。另有大量 scope exit 时的逐路径 Git 子进程，以及随历史增长的全事件查询。实际用户超时属于哪一阶段尚需现场分段日志确认。

## 调用链

- internal/transport/cli/mvp.go：每次 hook 调用 openStore / sqlite.Open。
- internal/app/capture/capture.go Observe：Cursor → Head → Tree(base) → Snapshot(previous) → Worktree → Store.Observe。
- internal/adapters/sqlite/capture.go Observe：PrepareCapture（包含再次加载 previous，进程内节点缓存可复用）→ prepareIO → publishObjects / commitCapture。
- 没有远程上传步骤。性能主要取决于本地文件系统、Git 子进程、CPU、SQLite 和锁等待。

## 已确认的实现问题

### 首次持久化：每路径一个节点文件，同步次数随路径数增长

internal/domain/state/state.go 中 State 为 AVL，每个路径一个 NodeRecord。Persist 会遍历尚未持久化的节点。internal/adapters/sqlite/prepared.go:29 对各对象逐个 SELECT COUNT 查询归属；随后 PutBlob。internal/content/content.go:103-120 每个新对象执行文件 Sync、Rename、目录 Sync。即使文件内容完全未修改、直接引用 Git blob，也仍需写入全体状态节点。

首次状态至少需要约 N 个节点对象和 2N 次同步；新增内容另外写 blob。慢磁盘/高延迟挂载尤其不利。已持久化子树会被 Persist 截断，不能将首次全量写入成本误认为每次快照都会发生。

优化：批量查询归属；对象包或可批量提交的节点存储，将大量小同步合并。必须保持“对象持久化完成后才发布数据库引用”的崩溃恢复顺序，不能直接删除 fsync。单纯延后目录同步只能减少部分开销，逐文件同步仍然存在。

### 后续采集：每次全仓读取和哈希

internal/adapters/gitrepo/repository.go:189-367 将 base、index、未跟踪文件合并后遍历。每个路径重复 Lstat 祖先目录，再读取完整文件并计算 GitIdentity，计算之后才判定能否复用引用。成本受文件数 N、路径深度 D、总内容字节 B 共同影响：元数据操作约 O(ND)，内容读取与哈希 O(B)，状态查找/Set 另有 O(N log N) 遍历成本。没有修改并不会跳过这些步骤。大文件还被完整读入内存，产生内存峰值和 GC 压力。

优化：持久化文件元数据/内容身份缓存，结合 Git index、变化候选路径与未跟踪路径枚举，仅读取必要文件。需要处理 racy timestamps、原子替换、模式变化、删除、忽略规则变化、symlink 和 submodule。Git clean filter / 换行转换等场景下，Git 的“干净”不必然等于原始字节未变，不能无条件用 Git status 替代原始内容采集。缓存祖先目录验证可减少重复系统调用，但必须保留符号链接安全约束。

### 进程缓存对独立 hook 不生效，恢复产生大量小读

Tree 使用 Repository 内存缓存，而 workspace.Open 每个 hook 新建 Repository。Store.nodes 同样只在本次进程内生效。internal/adapters/sqlite/store.go:172-183 与 state.Restore 会递归读取全部节点；每个节点 Objects.Read 都打开 lifecycle.lock、加共享锁、读文件、校验 SHA-256，再反序列化并重算节点身份。

优化：按根惰性加载节点、打包节点读取、持久化 HEAD 状态缓存；或评估常驻采集服务以复用缓存。常驻服务仍需解决工作区外部修改与缓存失效。单次内部重复 Snapshot 通常命中缓存，不能算作两次完整磁盘恢复。

### scope exit 使用逐路径 Git 子进程

repository.go:229-253：previous 中不在当前候选集的每条路径单独 git check-ignore，包括已删除的未跟踪路径。批量删除/忽略数千文件时会产生数千次子进程。

优化：使用支持 NUL 路径的 check-ignore 批处理，正确区分“未忽略”的退出码与执行错误，保留 scope exit 语义。

### 锁等待和历史数据放大

- sqlite.Open 每次使用数据目录排他 lifecycle lease；Observe 持共享 lease 穿过 prepare/publish，因此另一个 hook 启动可能等待整个持久化过程。数据目录共享时可跨仓库互相影响。
- content/lifecycle.go:20 使用阻塞 Flock，不接收 context。文件扫描/读取和状态对象处理也缺乏循环中的取消检查。
- sqlite busy_timeout 为 5000ms，这与外部 hook 30 秒超时不同。
- commitCapture 无条件 autoUnknown，后者调用 events(tx,w)，在写事务中读取全部工作区历史事件及关联。Stop 反馈也读取全事件。使用越久可能越慢；当前实验未覆盖长历史和并发场景。
- 重复 receipt 检查在 Worktree 之后，重投仍会先做全仓采集；不匹配 PostToolUse 的忽略判定也较晚。

优化：正常启动避免无条件排他生命周期锁（迁移/清理仍需正确互斥）；使用可取消的锁获取和分段时间预算；autoUnknown 仅处理受影响的连续分组，事件反馈按当前 turn 查询；在重采集前做安全的 receipt/事件预检查，提交时仍保留一致性复核。

## 超时与观测

Codex install.go:82、Claude claude.go:21、CodeBuddy codebuddy.go:13 的默认 hook 超时均为 30 秒。应分别记录启动/锁等待、Git 枚举、基线构建、previous 恢复、工作区读取、prepareIO、SQL 提交与 autoUnknown，连同路径数、实际读取字节、新节点数、同步次数与事件数。现有 ReadStats 没有覆盖 Worktree 的读取字节，事务计时也不包含取得事务锁之前的等待。

临时调高 timeout 可缓解失败，但全量工作、同步写入放大与跨进程等待仍存在。hook 的 fail-open 只让调用方继续，不保证快照已保存。

## 验证方法与限制

使用 work/snapshotprobe 临时程序直接调用当前 gitrepo.Tree、Worktree、State.Persist、Objects.Put / Read / Restore。合成仓库每文件 1024 字节、每目录 100 文件，全部提交且无修改；两次运行是独立进程。首次从 base 构建工作区状态；第二次恢复持久化状态后再次采集。记录墙钟时间，非 strace 环境。

探针不包含 SQLite、hook 启动互斥、实际历史查询、并发和冷缓存场景，因此不是完整 hook 基准，也不能直接推算用户机器耗时。完整链路构建被 modernc.org/sqlite v1.23.1 的模块下载重定向目标拒绝访问阻断。domain/state 和 content 现有测试通过；gitrepo 测试因 SQLite 依赖未能运行。

计时结果另附于下表。

## 建议落地顺序

1. 加分段计时与可复现基准：首次、无变化、单文件变化、批量删除/忽略、长历史、并发，区分文件数与字节数。
2. 先解决首次持久化同步放大；同时批处理 check-ignore、收窄 autoUnknown，移除无必要的启动排他锁。
3. 实现跨 hook 可复用的采集缓存与变化候选识别，并验证原始字节语义、安全边界及缓存失效。
4. 优化状态恢复/节点布局，评估惰性加载或常驻进程；补足取消和超时预算。每步用完整 hook 耗时与持久化正确性验证。

## 本机合成实验结果

单位：秒；首次/再次分别启动独立进程，磁盘及系统缓存未清空。

| 阶段 | 1 万文件首次 | 1 万文件再次 | 3 万文件首次 | 3 万文件再次 |
| --- | ---: | ---: | ---: | ---: |
| Open / Head | 0.011 | 0.013 | 0.015 | 0.010 |
| HEAD Tree | 0.051 | 0.049 | 0.159 | 0.145 |
| previous Restore | 0 | 0.193 | 0 | 0.587 |
| Worktree | 0.140 | 0.146 | 0.576 | 0.471 |
| 首次节点 Persist / Objects.Put（不含 SQL） | 12.758 | 未运行 | 32.141 | 未运行 |

3 万文件约 30 MB 的场景，仅节点持久化就超过默认 30 秒，验证了文件数和逐对象同步的放大机制。再次运行的四阶段合计分别约 0.40 / 1.21 秒，不能据此断言所有大仓库的后续快照都超时；总内容量、路径深度、历史长度、锁竞争和存储性能仍需分别测量。

## 方案推敲补充

后续方案见 `../design/20261106-snapshot-performance.md`。原始字节与 Git clean 状态不能直接等同：本机建仓设置 `filter.upper.clean = tr a-z A-Z`、`.gitattributes` 指定 `raw.txt filter=upper`，提交 `HELLO\n` 后将磁盘改为 `hello\n`，`git status --porcelain=v1 -uall` 输出为空；HEAD blob 和磁盘字节不同。另对普通文件设置 assume-unchanged 后实际改写，status 未报告该路径。Git 原文说明参见本次落地的 `../reference/git/20261106/`。这些例子限制了“只使用 status 结果”的正确性，不否定基线+覆盖的存储方式。

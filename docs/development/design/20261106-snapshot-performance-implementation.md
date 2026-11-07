# Git 基线与稀疏快照：实施记录

日期：2026-11-06。基于 d430e4b；方案见 [已采纳方案](20261106-snapshot-performance.md)。仅修改开发文档和实现，不修改面向用户的使用文档。

## 实际实现与方案取舍

### State 保持纯计算，基线不再构建 AVL

`internal/domain/state/baseline.go` 引入 BaselineRef（CommonDir、固定 tree OID、object format）与已准备的只读 Git 元数据。普通 Git 基线保存排序路径与 entry map；AVL 只存覆盖和删除。Lookup / All / Equal / Apply / ChangedPaths 都是纯内存操作，不会隐式执行 Git。所有历史值保持不可变。

相比草案新增 StateRef / PreparedTransition 的广泛接口改造，实际扩展现有 State 值即可表达稀疏状态；领域规划器和审阅算法不需要两套接口。同基准比较只检查覆盖并集；不同基准目前仍枚举两侧元数据进行语义比较，不读取全仓内容，尚未另加 Git tree diff 加速。

持久化版本 2 为一个规范排序的描述符，含基线和完整覆盖；内容寻址身份包含描述符字节。删除为显式无 File 条目，范围退出仍使用 OutOfScope。继承基线内容、删除新增文件等操作会消除无意义覆盖。未提交状态不形成增量链。空基准/unborn HEAD 同样支持。

`gitrepo.Tree` 准备基线；`content.LoadBaseline` 根据固定身份读取 tree 元数据。Store 恢复快照时显式调用内容解析器（与既有 Git blob 解析职责一致，SQLite 不依赖 gitrepo 适配器），不把 Git 回调藏在领域查询里。元数据缓存独立限制在 16 个基线、合计 10 万路径；不为每个快照重复缓存一份完整基线。特别大的基线允许不缓存。已管理 Git 目录的排除用覆盖删除表达，保证重开后的状态一致。

### 一个清单对象，而非新增数据库表

实际复用已有不可变对象库保存单个 v2 清单，并通过原有 SQLite 事务发布其引用，而非新增清单表。这样无需变更 schema 8 或清空数据，原有对象引用、跨仓库共享、删除和崩溃恢复协议都继续适用。代价是每个新清单保留一次对象文件/目录同步；干净基线从数万个同步对象减少为一个对象，仍有充分性能收益。

版本 1 AVL 节点保留 reader 和既有计算状态 writer。新 Git 快照写 v2；旧历史不被覆盖或删除。第一次接续旧版完整快照会保守读取其条目一次，此后使用 v2 覆盖。独立测试验证旧→新接续与旧历史可读。

prepareIO 对清单和覆盖内容的归属按 200 个 ID 分批查询，只保护和发布需要的本地对象，不复制基线 Git blob。写入仍严格先持久化内容，再事务提交快照引用；没有删除 fsync。任意新增文件量本身很大时仍需按新内容量写 blob，本次没有额外实现 pack。

### 候选发现与缓存

- `git ls-files --modified -z` 用元数据发现未暂存候选，避免为找候选运行 clean/diff filter。检测命令覆盖 fsmonitor、ignoreStat、trustctime、checkStat 的不安全省略配置。
- `git diff-index --cached --name-only -z --no-renames --no-ext-diff --no-textconv <tree> --` 取得 index 相对基线的候选。
- stage 路径、未跟踪路径、上次覆盖、assume-unchanged / skip-worktree、实际 mode/type 差异补足候选。
- 批量 check-attr 检查 filter、text、eol、working-tree-encoding、ident；core.autocrlf 启用时保守扩大候选。保留实际原始字节，不静默归一化。
- 祖先目录按根到叶验证与缓存，避免先验证子目录后缓存导致绕过 symlink 祖先；不读取范围外内容。check-ignore 使用 NUL 分隔批处理。
- 可丢弃缓存放在 data dir/capture-cache，以 GitDir 身份隔离。只记录元数据，不存历史内容；路径用 base64 无损编码。复用内容必须来自上一已发布状态，且缓存绑定上一状态根与属性指纹。
- Linux、macOS 检查 device/inode、size、mode、mtime、ctime。未知平台禁用内容缓存。无法信任的缓存、同尺寸改写、近 1 秒内的 racy 时间戳、系统时钟倒退等情况重新读取。
- 缓存的时间边界取本次文件读取开始前，而非读取结束后，避免长采集把时间戳临界项错误标记为稳定。缓存无需 fsync；丢失只能损失性能。

文件稳定读取有有限重试，并检查文件身份和读前后元数据；读取过程检查 context。采集两端复核 HEAD 和 index 的 staged entries；Git 边界/游标冲突最多重新观测三次，不重试协议冲突和普通错误。此机制不提供全文件系统同一时刻的原子快照保证。

### 锁、幂等与历史查询

正常已初始化 Store.Open 仅获取共享生命周期锁；初始化或待删除任务恢复才获取排他锁。采集从读取旧快照到内容准备、最终发布持共享保护。所有新可取消路径使用非阻塞 flock + context 等待；仍保持删除的排他保护。

receipt 在采集之前预检查，最终事务仍复核 receipt、digest、cursor CAS。没有 active call 的 PostToolUse 提前忽略。

自动 unknown 分组不再每次扫描整个 worktree：无变化时跳过，仅加载新事件触及的 segment。该查询仍可能随单个长 segment 增长；未声称所有历史查询都已严格常数化。Stop 反馈只查询当前 turn。后续若长 segment 成为瓶颈，应进一步增量维护连续分组和查询索引。

V(1) 日志记录 lease、cursor/head/index、baseline、previous、worktree、persist 分段时间；Store 另记录计划准备、对象准备、事务（含等待）时间，以及新对象数。记录路径数、覆盖数与 Motif WorktreeBytes。此字节计数只衡量 Motif 自己读取的内容，不是系统级全部 I/O 统计。

## 验证

新增测试覆盖：随机稀疏状态与原完整状态对照；3 万路径描述符只写一个小对象；无变化不读取普通文件；clean filter 隐藏变化及 filter 命令失效；assume-unchanged；staged/unstaged；staged delete 留存磁盘；可执行位；非 UTF-8/换行路径；忽略退出/重新进入；删除与恢复；批量 scope 检查的 Git 调用上限；祖先 symlink；旧格式接续；重开持久化；时间戳和同尺寸修改；启动共享锁；可取消锁等待。

既有部分提交 close/carry、Agent 归属、分批、amend/rebase、投影审批、评论和项目删除测试继续通过。Linux 运行核心包 race 检查；macOS ARM64 交叉编译验证。

测试环境启动默认 umask=0077，现有 integration 权限测试要求 0640，因此运行测试时设 umask 0022。现有 CLI 测试会使用默认数据目录，设置 MOTIF_DATA_DIR 到工作区以避免写只读 home；不修改 HOME。SQLite 依赖从其 GitLab 上游 v1.23.1 标签生成本地模块下载包，校验与 go.sum 一致，`go mod verify` 全部通过；go.mod/go.sum 未修改。

可复现检查命令：

```
umask 0022
MOTIF_DATA_DIR=<scratch-data> go test ./...
MOTIF_DATA_DIR=<scratch-data> go test -race ./internal/domain/... ./internal/content/... ./internal/adapters/... ./internal/app/capture/... ./internal/app/review/...
go test ./internal/adapters/gitrepo -run '^$' -bench BenchmarkSparseWorktree -benchtime=3x
go vet ./...
GOOS=darwin GOARCH=arm64 go build ./cmd/motif
```

## 本机合成仓库端到端对比

同一仓库：3 万路径，每文件 1024 字节，100 文件/目录，全部提交。通过完整 Store.Open → workspace.Open / Identity → capture.Observe 执行；每次独立进程。计时 capture.Observe，不含 setup，数据目录首次独立创建；未清空系统缓存。不是多机或 p95 统计，部分运行与测试共享机器资源。

| 场景 | 原版本 d430e4b | 新版本 |
| --- | ---: | ---: |
| 干净首次采集 | 75.279 秒 | 0.394 秒 |
| 独立进程再次采集 | 1.330 秒 | 0.469 秒 |
| 首次后对象数 | 53,287 | 1 |
| 单文件修改（1 KB） | 未测 | 0.480 秒 |
| dirty 文件本次没变，缓存验证后 | 未测 | 0.453 秒 |

新版本普通干净采集 WorktreeBytes=0；单文件修改=1024；时间戳安全后复用 dirty 文件=0。单文件修改后累计对象为 3：原基准描述符、新内容、新覆盖描述符；再次未改没有新增对象。当前代码仍枚举 Git/文件元数据，不承诺任意规模仓库均固定耗时。

仓库内 `BenchmarkSparseWorktree -benchtime=3x` 最终运行结果（包含每次重新 Open、Tree、Worktree，排除建测试仓库、SQLite）：1,000 路径 43.19 ms/op；30,000 路径 365.54 ms/op；两者 WorktreeBytes 均为 0。最终全量测试、核心包 race、go vet、Linux amd64 与 Darwin arm64 构建全部通过。

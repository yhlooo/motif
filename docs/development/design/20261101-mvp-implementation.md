# MVP 实现决定

本次实现以 `20261031-mvp.preview.md` 和三份细化草案内的“已确认”决定为依据，不将尚未采纳的草案整体当作契约。原草案保持不变。

## 模块边界

- `gitrepo`：只读 Git 与文件系统，生成状态、内容引用及 diff；不修改用户 index/工作区。
- `store`：GORM/SQLite、内容寻址 blob、事务、事件链与批次约束，统一持久化。
- `capture`：来源适配，Codex 前后置/Stop 和 Git 提交观测。
- `review`：固定分支范围，重放事件，生成冻结 CR；已创建 CR 不随重新分批变化。
- `integration`：项目级安装、卸载及已有 hook 兼容。
- `web`：仅回环地址的本地 HTTP 服务，嵌入静态前端。

## 本次实现选择

SQLite 使用 `github.com/glebarez/sqlite` 纯 Go 驱动，并通过 GORM 统一访问，保持 CGO_ENABLED=0 的单文件发行方式。快照存完整的路径/内容引用清单（blob 全局去重），保留 base commit；这是相对基准覆盖清单的实现简化，避免读取历史时重新物化所有基准内容。原始字节以 SHA-256 保存，不写入 SQL JSON 核心表。路径在数据库中用 BLOB；JSON 中非 UTF-8 路径必须用可逆编码，前端显示转义路径。

所有快照及游标更新在一个写事务内，工具事件按 worktree/session/turn/call 幂等；重复 Stop 重新检查。串行采集是一期前提；多个不配对调用降级为未知，不宣称并发来源准确。

CR 使用 merge-base 到源分支提交的线性序列，冻结输入及批次。含 merge 的范围暂明确拒绝，待比较父节点语义确定。无法精确重放时明确返回诊断；没有采集记录的范围以未知补齐完整展示。历史改写不自动继承旧批次与批准。checkout/reset/未捕获提交在下一观测时更换基准，保留旧记录，不假造历史。

Codex 协议依据现有 research/reference 固定源码；真实发行版是否启用 hooks 仍需在用户使用的 Codex 中验证，安装成功不等于已验证采集成功。失败 hook 放行；Stop 仅在成功检查出待分批 AI patch 时 block。

## 验证

实现完成后记录测试范围与已知边界。普通 CLI 的使用示例放在本开发文档，不修改用户文档。

## 已实现的入口

```sh
go build -o motif ./cmd/motif
# 在待采集的 Git 项目中执行；一期只有 codex 适配器。
/path/to/motif install --agent codex
/path/to/motif patch ls
/path/to/motif patch ls --current-turn
/path/to/motif patch ls --turn <local-turn-uid> --format json
/path/to/motif patch get <patch-uid> --format json
/path/to/motif patch-batch create --title '意图标题' --desc '整理原因' <patch-uid>...
/path/to/motif patch-batch delete <batch-uid>
/path/to/motif web --listen 127.0.0.1:3939
# 只解除指定集成，保留数据。
/path/to/motif uninstall --agent codex -y
# 全量卸载：打印范围后删除本机当前仓库的集成与历史。
/path/to/motif uninstall -y
```

默认数据目录 `~/.motif`，可通过 `MOTIF_DATA_DIR` 指定其他仓库外目录；hook 与普通命令须继承相同设置。普通参数或批次规则错误退出 2，执行错误退出 1，成功退出 0。hook 业务失败写 stderr 并退出 0；Stop 正常门禁是退出 0 的结构化 block。

项目安装保留 TOML 的非 Motif 内容、既有 hook 与 core.hooksPath；既有 hook 的退出码保留，Motif 失败不影响其语义。安装清单在各 worktree 的 Git 管理目录中，linked worktree 共享 Git hook，最后一个拥有者卸载时恢复原 hook。项目外的全局 hooksPath 暂不自动改写，明确报错并保留配置。被用户修改的 Motif 管理块会报告冲突，不覆盖。项目全量卸载在确认前列出所有已注册 worktree 的配置及数据库、记录类别、独占/共享 blob 数量。

Web 的分支预览固定 source/target OID，创建时复核；CR 固定标题、说明、成员、状态链与 Git 范围。连续选择直接比较两端状态，评论绑定 CR、选择首尾、路径的可逆编码、侧、行范围及 blob hash。页面只显示对应冻结选择的评论。所有审阅项均参与批准汇总；任何拒绝为 changes_requested，全部批准为 approved。浏览器写入必须来自同源并携带本次服务令牌，服务只绑定回环 IP。

快照的 out_of_scope 标记明确表示路径离开 Git 采集范围，不把忽略规则变化当作文件删除，也不继续保存范围外内容。非 UTF-8 路径通过 base64 往返，显示时转义。首次未知事件的 observed_from=0 表示没有此前工作区观测；虚拟事件的区间为 0，排序以 sequence 为准，不伪造实际修改时间。

## 验证记录

- Go 测试覆盖采集/重开库持久化、初始脏文件、原始原因、幂等重试、多次 Stop、缺失/迟到 Post、提交内工具调用、部分提交的 close/carry、批次连续性及事务回滚、冻结 CR、分支预览失效、未知补齐、项目隔离与共享 blob 删除。
- 文件系统测试覆盖工作区实际字节、staged delete 后仍存在的文件、空/二进制/可执行/符号链接、外部祖先符号链接、忽略规则变化和非 UTF-8 路径。
- HTTP 测试覆盖完整 CR 生命周期、评论范围/回复、批准汇总、非法 Host/Origin/令牌拒绝；安装测试覆盖已有配置/原 hook、幂等、修改冲突及 linked worktree 共享。
- Chromium 端到端检查使用真实编译二进制、临时 Git 仓库与实际 Git hook；模拟 Codex stdin 协议创建两批修改，Web 创建 CR、连续选择、行评论/回复、批准均通过，浏览器脚本错误为 0，HTML 评论按文字呈现。桌面和窄屏页面已检查。
- 该端到端检查没有调用真实 Codex 模型会话；尚不能宣称验证过某个 Codex 发行版的 hook 启用/信任策略。

## 保留边界

串行采集；不跟踪子模块内部工作区；重命名以删除/新增表示；不自动恢复已丢失的中间修改；不迁移 amend/rebase/squash 后的意图；不支持含 merge 的 CR。历史切换/改写后的新提交可以按未知净差异审查，但不声称继承旧意图。Git clean/smudge/LFS/换行转换下保留原始观测字节，不额外推断来源；Git 对象与内容库缺失会显式报错。无常规 GC 和自动清理，预览产生且没有快照引用的内容可能保留到后续清理实现。

最终检查：`go test ./...`、`go vet ./...`、`git diff --check` 及前端 JavaScript 语法检查通过。`CGO_ENABLED=0` 的 Linux amd64 和 macOS arm64 构建通过。实际 CLI 复查指定 Agent 卸载保留全部历史、全量卸载清除当前项目历史，两者行为符合预期。

数据库访问已按用户要求统一切换为 glebarez/sqlite + GORM；新库模型迁移、现有 v1 兼容及验证细节见 [存储调整说明](20261101-store-gorm.md)。

# MVP 实现决定

本实现以 `20261031-mvp.preview.md` 的产品目标与三份细化文档中“已确认”的用户决定为准；后更新的确认决定优先。未确认的草案不整体作为实现契约。逐项修正与验证见 [设计对齐记录](20261101-design-alignment.md)。

## 模块边界

- `gitrepo`：只读 Git、工作区、内容引用、差异与精确投影，不修改用户 index/工作区。
- `store`：纯 Go `github.com/glebarez/sqlite` + GORM、内容库、事务、事件链、批次、审阅历史及版本。
- `capture`：Codex 前后置/Stop、Git 提交、切换、引用事务与 post-rewrite 适配。
- `review`：分支范围解析、连续展示状态链、动态刷新、稳定身份和评论位置映射。
- `integration`：当前项目安装、卸载、已有 hook 兼容和实际 Codex 生效诊断。
- `web`：回环 HTTP 服务、嵌入前端、一致视图版本与审阅交互。

## 采集与存储

快照保存相对 base commit 的稀疏覆盖清单，表达新增/修改/删除、mode、gitlink、符号链接及采集范围变化。未变内容从 Git 读取，不复制到 Motif 内容库；改变的原始字节按 SHA-256 全局去重。路径在数据库中为 BLOB，非 UTF-8 路径经可逆编码传输。历史 Git 对象缺失显式报错；保留旧版完整快照读取兼容。

采集、快照、提交关联与游标更新在同一事务内；事件按工作区/会话/轮次/调用幂等。串行采集是一期前提，无法确认的来源为 unknown。部分提交以 close/carry 成对闭合，不篡改原始事件。post-rewrite 保存 old/new 映射并生成可追溯派生事件：精确可验证部分保留来源及批次，冲突或不确定部分为 unknown。派生批次是可正常管理的独立实体；新提交不继承旧提交审批。真实工具内 amend 的未提交部分继续保持配对与调用身份。

数据库统一使用 GORM 查询、写入、迁移和事务。现有 v1 数据增量迁移至 v6，保留原始历史、旧 CR、评论与回复；不清库。详见 [存储设计](20261101-store-gorm.md)。

## 审阅

CR 保存分支名称，每次打开详情或主动刷新重新解析 OID。最终内容按 `target...source`，提交项按 `rev-list --reverse --topo-order --no-merges target..source`；支持 DAG，merge 独有修改以最终未知项补齐。无共同祖先或多个最佳 merge-base 明确报错。

修改按顺序投影为连续展示状态链，组合 diff 比较两端状态，不能相加 diff 文本。重放失败提供诊断；最终补齐不掩盖失败。commit + batch/patch 稳定身份复用当前 CR 的审批和评论，新身份 pending，离开范围的项保留历史但隐藏。范围清空时无可批准项。所有 active 项参与汇总，任一拒绝为 changes_requested，全部批准为 approved。

发布视图事务校验源事件/分批版本和旧视图版本；列表、diff、评论、决定针对一致版本，过期请求返回 409。原始评论锚点不变，按所属稳定项、路径和存活行映射到组合选择；删除位置、离开范围的项隐藏，不将旧评论绑定到新提交。支持行范围和线程回复。

## 集成与安全边界

一期适配 Codex；安装不触发模型或自动变更项目/命令信任。`motif doctor` 通过当前 CLI 的 `hooks/list` 检查有效来源、启用与信任，区分生效、未生效、不可诊断和配置冲突。安装成功不代表采集已经生效。

生成脚本只转发 argv/stdin 到 Motif；已有 hook 收到相同输入，原退出码保持，Motif 采集失败放行。支持 linked worktree 与共享外部 core.hooksPath，拥有者登记防止卸载一个项目破坏其他项目；外部共享脚本只采集已安装项目。修改过的管理块/备份冲突不覆盖。

指定 Agent 卸载只解除集成，保留历史；全量卸载按确认清单删除当前项目历史及独占 blob，保留其他项目共享内容。全量卸载确认和 `-y` 行为不变。服务只监听回环地址，写入校验同源和服务令牌。

## 开发入口

```sh
CGO_ENABLED=0 go build -o motif ./cmd/motif
motif install --agent codex
motif doctor
motif patch ls --current-turn
motif patch ls --turn <local-turn-uid> --format json
motif patch get <patch-uid> --format json
motif patch-batch create --title '意图标题' --desc '整理原因' <patch-uid>...
motif patch-batch delete <batch-uid>
motif web --listen 127.0.0.1:3939
motif uninstall --agent codex -y
motif uninstall -y
```

默认目录 `~/.motif`，可由 `MOTIF_DATA_DIR` 指向仓库外目录；普通命令与 hook 需继承相同配置。参数/规则错误退出 2，执行错误退出 1；采集失败退出 0，Stop 正常门禁以结构化 block 返回。

## 一期边界

不保证并发 Agent 的来源归属；子模块只记录 gitlink；不恢复未观测中间状态；不添加 Git 私有保留引用，不实现常规 GC。原始字节保留，不额外推断 clean/smudge/LFS/换行转换来源。无法可靠映射的历史改写部分明确为未知；不自动生成模型解释或自动批准。

验证覆盖、真实 CLI 与浏览器检查及限制见设计对齐记录。没有执行真实 Codex 模型会话，不声称完整验证过真实模型到 hook 的端到端链路。

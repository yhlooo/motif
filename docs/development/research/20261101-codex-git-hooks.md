# Codex 与 Git Hook 能力核实

日期：2026-11-01。依据：[原始资料与固定源码版本](../reference/codex/20261101/SOURCES.md)。这是源码核实，尚未验证某个已发布 Codex CLI 的端到端运行。

## Codex

官方文档站 hooks 地址访问返回 403，因此查阅官方仓库配置文档、输入 schema 和实现，固定 commit 为 `67f0994b532b8740f14b5b738af7b7b2a915b1d6`。不可将 main 的能力直接声明为所有发行版本支持。

- `HookEventsToml` 支持 PreToolUse、PostToolUse、Stop、SessionStart、SessionEnd、Interrupt、SubagentStart/Stop 等事件。配置采用 `[[hooks.PreToolUse]]` / `[[hooks.PreToolUse.hooks]]` 等 TOML 结构，command handler 通过 stdin 接收 JSON。存在 managed-hooks-only 限制，init 需检测用户配置是否实际生效。
- 前后置 schema 均包含 cwd、model、session_id、turn_id、tool_name、tool_use_id、tool_input、transcript_path；agent_id/type 为可选字段；后置另有 tool_response。没有通用实际文件 diff、时间戳、人类作者或修改原因字段。
- `apply_patch` 工具输入可包含拟应用 patch，但不等于实际工作区净变化。工具参数中若有 justification，只能作为该字段的原始说明，不能保证所有工具都有原因。时间由 Motif 观测，Git 作者配置需另读，不能当作实际操作者的认证。
- 工具名中 apply_patch 有 Write/Edit matcher 兼容别名；shell 类使用 Bash。不能直接套用其他 Agent 的工具名或协议。registry 的默认 payload 实现以 Function 调用为条件，其他工具可以覆盖方法；不保证任意执行路径均被默认覆盖。
- registry 仅在 success 分支运行 PostToolUse。失败或取消可能缺少后置事件，下一观测边界应补记未知事件，不能虚构完整 AI 归属。本版本事件表无独立 PostToolUseFailure。
- command handler 支持 timeout 和 async 配置；快照应使用同步 handler，不能采用异步快照作为前置边界。正式 init 参数、超时、输出与最低支持版本仍需发行版验证。
- config 源码明确将 ghost_snapshot 标注为兼容旧配置，已不再生成快照。不能依赖这个旧机制提供每次编辑前后快照；本轮资料未建立可替代 Motif 快照的稳定公开查询接口。

## Git

依据：[Git 2.52.0 官方 githooks 文档](../reference/git/2.52.0/githooks.adoc)。

- post-commit：提交后通知，无参数；失败不能撤销已发生的 commit。
- post-rewrite：amend/rebase 提供 old→new 映射；squash/fixup 可为多对一。并非所有历史改写工具调用它。
- post-checkout：checkout/switch 后提供旧、新 HEAD 及类型标记。可用于更新当前 worktree 的采集游标，已有提交记录无需改动。
- reference-transaction：提供引用事务阶段及 old/new/ref，适合记录 ref 移动和发现需同步的提交。它不是通用的工作区操作结束通知，也不提供 reset 的具体模式。
- 没有通用 post-reset hook。路径级 reset/restore 可能不改 ref；不能只靠引用 hook 发现它们。下次 Motif 快照可以兜底其净变化。

## 本地验证与信息边界

在 /tmp 的独立测试仓库使用 Git 2.52.0 安装上述四种 hook：正常提交触发 post-commit；amend 触发 post-commit 和 post-rewrite；reset --hard 触发 reference-transaction，未触发 post-commit/post-rewrite/post-checkout。该用例中引用 hook 已看到更新后的工作区，但不能由此推广出所有操作、版本的工作区完成时序。

无法由末态唯一恢复的例子：未采集区间内外部编辑被 reset --hard 删除，既无快照也未进 Git 对象库，之后无法从 commit 推导已丢弃内容。可以继续保持提交 diff 正确，但不能声称历史无遗漏。

rebase 拆分提交、冲突解决、部分重放的最终 tree 均可计算并用未知/虚拟事件闭合；old/new 映射未必足以唯一恢复原始事件的细粒度归属。应区分“新提交状态可精确重建”与“原始意图归属可精确迁移”。

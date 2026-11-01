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

## Stop hook：同一轮内强制完成分批

补充核实仍基于上述固定源码；原始 schema、执行实现、turn loop 及测试摘录见 [来源清单](../reference/codex/20261101/SOURCES.md)。本机 CLI 报告 `codex-cli 0.163.0`，但本轮未发起真实模型会话，未运行官方测试；以下是源码和现有测试的审读结论，不是发行版端到端验证。

- Stop 输入包含 session_id、turn_id、model、cwd、stop_hook_active 和 last_assistant_message。可以明确标识待结束的轮次。
- 同步 hook 退出码为 0 并向 stdout 写 `{"decision":"block","reason":"继续完成分批的指令及列表"}`，会阻止正常结束并把 reason 注入续做上下文。不是 `continue:false`；后者表示停止处理，语义相反。
- 实现也支持退出码 2 + stderr 的继续提示。Motif 建议使用成功退出的结构化 block 输出，区分正常分批门禁与程序故障。
- session/turn.rs 在 should_block 时记录 hook prompt，设置 stop_hook_active=true 并继续同一 turn 循环。官方测试 `stop_hook_can_block_multiple_times_in_same_turn` 覆盖连续两次 block、第三次放行，断言三次 turn_id 相同，stop_hook_active 为 false/true/true。
- 不应因为 stop_hook_active=true 就无条件跳过检查；应以是否仍有待分批 AI patch 决定是否继续 block。同步 hook 才可施加控制效果，异步不能实现该门禁。hook 必须实际启用并具备信任/执行权限。
- 用户中断、进程退出、模型调用失败或 hook 故障不在正常 Stop 门禁保证内。不能宣称外部强制结束时也一定完成分批。
- shell 环境源码提供 CODEX_THREAD_ID / CODEX_TOOL_CALL_ID 等身份。`--current-turn` 不应臆造 CODEX_TURN_ID：可在 PreToolUse 保存调用→turn 映射，命令通过当前调用身份解析本轮；无法唯一定位则明确报错，不取全仓库“最近一轮”。

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

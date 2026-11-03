# Claude Code hooks 接口调查

调查日期：2026-11-02。官方文档原文：[hooks.md](../reference/claude-code/20261102/hooks.md)；[来源](../reference/claude-code/20261102/SOURCES.md)。

## 与 motif 的适配关系

- Claude Code 使用 JSON settings 的 `hooks.<Event>[]` 匹配组，每组包含 `hooks[]` handler；command handler 从 stdin 接收 JSON。项目本地配置支持 `.claude/settings.local.json`，无需修改全局设置。该文件由 Motif 自行创建时，不依赖 Claude 自动添加忽略规则；本次实现不修改 Git ignore 配置。
- 共同输入字段有 `session_id`、`cwd`、`prompt_id`；`prompt_id` 标识当前用户 prompt，在第一次用户输入之前可能缺失。工具事件还有 `tool_use_id`、`tool_name`、`tool_input`。不是 Codex 的 `turn_id`；不能用 session 或单个工具调用冒充回合。
- `PreToolUse` 和 `PostToolUse` 可作为调用前后观察点；`PostToolUseFailure` 提供同一调用身份，失败仍可能已修改文件，因此同样收尾。`StopFailure`、`SessionEnd` 仅记录结束观察。Claude 不提供 Codex 的 `Interrupt` 事件；不能生成该配置。
- `Stop` 可在退出码 0 时输出 `decision: block` 和 `reason`，使 Claude 继续整理批次。`stop_hook_active` 表示已因 Stop hook 继续，文档要求避免无穷循环。本实现继续采样但在该标志为 true 时不再次阻止结束。
- handler 的 `async: false` 保证该 handler 同步执行，不保证并行工具调用会串行化。`timeout` 为秒，`statusMessage` 是运行时显示文字。
- 文档说明 settings 配置和当前运行会话之间存在信任、管理策略和覆盖关系；仅检查本地文件不能断言运行时有效。诊断只报告 configured，并提示重启及 `/hooks` 核对。

## 实测与验证边界

本机 `claude --version` 返回 `2.1.251 (Claude Code)`；其可执行文件包含 `prompt_id` 字符串，但这不等于已验证实际 hook 输入。自动测试用官方协议构造 payload，验证采集、失败调用、去重、回合隔离和 Stop 输出；本次未调用真实 Claude 模型。

缺少 `prompt_id` 的旧版工具事件/Stop 会报告错误并 fail open，不猜测回合，也不读取可能异步滞后的 transcript。SessionEnd 可以没有 prompt 身份。安装不声称兼容所有旧版本。

Git hook 调用者身份目前仍只从 Codex 的 `CODEX_THREAD_ID` / `CODEX_TOOL_CALL_ID` 获取。官方 hooks 文档没有提供一个能让 Git 子进程可靠继承 Claude 当前 tool_use_id 的契约，所以不从“活动调用”推断 Git 操作归属。单次 Claude Bash 中修改并提交或 checkout/rewrite 打断观察时，无法证明的修改保守记录为 unknown；此前已完成 PostToolUse 采集的 patch 仍按现有 Git 边界机制关联。这也是现有单工作树 active-call 状态机的边界：并发调用不能完整独立归因。本次不扩大状态机架构。

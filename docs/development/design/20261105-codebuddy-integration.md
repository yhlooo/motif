# CodeBuddy 集成

依据：[CodeBuddy hooks 调查](../research/20261105-codebuddy-hooks.md)。

## 边界与接口

统一 agent 名为 `codebuddy`，提供 `motif hook codebuddy`、`install --agent codebuddy`、`uninstall --agent codebuddy` 和 doctor 配置诊断。复用现有 source-independent capture 服务、领域状态机、内容库和数据库 schema。

`app/integration` 将 Claude 的 JSON 配置所有权处理提取为共用实现，保留 Claude 的路径和事件集合。CodeBuddy 在 `.codebuddy/settings.local.json` 中安装 UserPromptSubmit / PreToolUse / PostToolUse / Stop / SessionEnd，每个事件追加独立无筛选 matcher 组，只使用公共 command / timeout 字段。manifest 新增可选 codebuddy 记录，保留所有已安装 agent；写入仍处于同一 fileTransaction，修改或重复管理项拒绝覆盖，修复缺失组，卸载保留用户设置及其它 agent，最后一个 agent 卸载时才移除 Git hooks。

## 输入归一化

`app/capture.CodeBuddy` 校验公共身份字段，session 加 `codebuddy:` 前缀。UserPromptSubmit 分配本地轮次 ID，即使用户连续提交相同文本也不合并；有 generation_id 时记录它，后续 generation 变化产生新轮次。Stop 及其继续执行沿用当前轮次，SessionEnd 清空适配状态。

适配状态保存在当前 worktree 的 GitDir/motif/codebuddy 下，按原始 session 的 SHA-256 定位 JSON 文件，目录独占文件锁串行化 CodeBuddy hook。状态不保存提示词或工具参数，只保存轮次 ID、generation、未完成调用 ID、参数指纹和歧义标记。它不是来源事实；数据库中的 cursor、call、receipt 仍决定观察能否成立。先用临时文件 rename 发布适配状态，再执行事实观察；失败不会留下可重复使用的 post 配对。

PreToolUse 在已知轮次中为单个调用分配本地 ID，PostToolUse 仅在工具名、可选 tool_use_id 和规范化 JSON 参数相同且无重叠时复用该 ID，复用现有 receipt 和活动调用校验。没有用户提交边界且没有 generation_id 时不猜轮次；重叠工具调用、参数不匹配、缺失 pre/post 都通过普通观察记为 unknown。重叠或不匹配后，本轮停止推断工具归属，下一次用户提交恢复配对。不能提供无调用 ID 协议下的任意重复投递或并发精确归属保证。

Stop 复用未分批 patch 提醒；CLI 将提醒作为专用 HookFeedback 错误由 main 写入 stderr、退出 2。安装命令不加 `|| true`，保留该反馈退出码；采集错误在 hook handler 内记录后返回成功。stop_hook_active 时继续观察但抑制反馈。缺少已知轮次的 Stop 只观察，不要求对虚构轮次分批。

## 验证

覆盖官方无 prompt/tool ID 输入、相同参数连续调用、相同内容的新用户轮次、generation_id、规范化参数、歧义降级、Stop 继续与分批、配置保留/权限/修复/冲突/事务回滚、多 agent 共存和选择性卸载。按后续用户确认的[诊断规则](20261105-doctor-output.md)，doctor 在受管理 hook 配置校验通过时返回 active；不以运行时查询能力区分状态，不伪造运行时 hook 明细。

本次本地验证：CodeBuddy 及既有 capture / integration / CLI 测试通过；`go test ./... -skip '^TestWorktreeActualBytesAndFileTypes$'` 全部通过（使用临时 GOCACHE / MOTIF_DATA_DIR，并隔离用户 Git 全局配置）。未跳过的全量测试仅现有 `TestWorktreeActualBytesAndFileTypes` 失败：当前环境拒绝创建含非法 UTF-8 字节的文件名，返回 operation not permitted。未修改或弱化该用例。

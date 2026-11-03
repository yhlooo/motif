# Claude Code 集成

依据：[Claude hooks 调查](../research/20261102-claude-code-hooks.md)。

## 模块边界

`app/capture.Claude` 负责 Claude 输入校验及协议归一化；通过 `captureAgent` 复用已有观察、patch、批次及 Stop 提醒。领域规划、内容库、数据库 schema 不增加 Claude 专用语义。Claude session 加 `claude:` 前缀，prompt_id 映射 external turn，tool_use_id 映射 external call，避免同名 Codex 身份碰撞。失败工具事件归一为 PostToolUse，StopFailure 归一为结束观察；原始 payload 仍用于 receipt digest。

`app/integration` 负责项目本地配置的规划、记录、冲突检测、写入和移除。`transport/cli` 支持 `motif hook claude`、`install --agent claude`，可重复指定 `--agent codex --agent claude`。未指定 agent 的交互安装逐项询问，CLI 不可交互时要求显式 agent。`doctor` 对 Claude 返回配置检查结果，不运行模型或 hook。

## 配置与生命周期

Claude 设置写入 `.claude/settings.local.json`，不覆盖共享配置和全局配置。逐事件追加独立匹配组，不限制 tool matcher，以覆盖 Edit、Write、Bash、MCP 等调用。同步 handlers 包括 PreToolUse、PostToolUse、PostToolUseFailure、Stop、StopFailure、SessionEnd。运行文字复用已有 i18n 消息；错误事件复用对应结束观察文案。

命令末尾 shell 注释 `# motif:claude` 是所有权标记；manifest 保存完整受管理匹配组。安装先验证所有选中配置和 Git 文件，再在同一个 fileTransaction 中写配置、Git hooks 和 manifest。重复安装修复缺失条目；存在修改过、未登记或重复管理条目时拒绝覆盖。解析 JSON 保留数字原文精度；其他设置及用户 hook 的内容和顺序保留，文件空白格式会统一为缩进 JSON，原权限保持。

manifest 保留旧 Codex `config` / `config_created` 字段以兼容已有安装记录，新增可选 `claude` 配置记录及创建标记。未选中 agent 的记录保持。共用 Git hooks 仍由工作树持有，增加第二个 agent 不增加一层 wrapper。

`uninstall --agent claude -y` 只删除 Claude 管理条目并保留历史；卸载 Codex 同理。仍有 agent 时保留 Git hooks 和其原 hook 备份/拥有者。最后一个 agent 卸载时沿用 linked worktree 与共享目录所有者处理；卸载写入也使用 fileTransaction。未指定 agent 的原有全仓库历史删除语义保持。

安装创建的 Claude 配置在卸载后没有其他字段时删除；存在用户字段时保留。原本存在的配置即使空了也保留。修改过的管理组在卸载时拒绝删除，确保先处理冲突。

## 验证

测试覆盖 Claude-only 不创建 Codex 设置、双方共存和两个方向的选择卸载、最后一个 agent 的 Git 清理、重复安装、缺失 Git hook 修复、用户设置/hook 保留、数字精度和权限、修改冲突预检查不产生部分写入、失败调用 patch、receipt 重试去重、prompt 与 agent 身份隔离、缺失身份 fail open 和 Stop 批次校验/循环保护。真实 Claude 会话、旧版缺失 prompt_id 和 Git 子进程归属的边界详见调查文档。

验证结果：新增集成/采集/CLI 测试和现有相关包测试通过。全量 `go test ./...` 的唯一失败为已有 `gitrepo.TestWorktreeActualBytesAndFileTypes` 在 macOS 创建包含 0xff 的文件名时返回 `illegal byte sequence`；相同问题已有 [package layout 验证记录](20261102-package-layout.md)。本次未修改该测试或其实现。另验证缺失 Claude hook 修复及卸载 manifest 写入失败时配置回滚。

`go test ./... -skip '^TestWorktreeActualBytesAndFileTypes$'` 全部通过；补充回滚测试后的 `go test ./internal/app/integration ./internal/app/capture ./internal/transport/cli` 也全部通过。`git diff --check` 通过。

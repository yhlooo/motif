# CodeBuddy hooks 调查

## 范围与资料

按统一 `codebuddy` agent 接入 CodeBuddy IDE / IDE 插件及 CodeBuddy Code。官方资料：

- [IDE Hook 功能使用文档](https://www.codebuddy.cn/docs/ide/Features/Hooks)，[原文节选](../reference/codebuddy/20261105/ide-hooks.md)。
- [Code Hook 参考指南](https://www.codebuddy.cn/docs/cli/hooks)，[原文节选](../reference/codebuddy/20261105/code-hooks.md)。

IDE 插件的覆盖范围依据本次用户明确提供的共用 hook 协议信息；未取得独立插件 hook 规范，不能声称单独验证过插件运行时。

## 协议结论

1. 两份规范共同支持 UserPromptSubmit、PreToolUse、PostToolUse、Stop、SessionEnd，stdin JSON 携带 session_id、cwd、hook_event_name。工具事件携带 tool_name / tool_input；Stop 携带 stop_hook_active。
2. 按用户明确确认，IDE / plugin 与 Code 的 Motif hook 统一写入项目本地 `.codebuddy/settings.local.json`。Code 官方参考列出了该路径；IDE 文档仅列 settings.json，不能据此推断不支持 settings.local.json。设置文件可能已有用户 hooks，需要按组管理而不是覆盖文件。
3. 不能照搬 Claude 适配器对 prompt_id / tool_use_id 的强制要求：公开工具输入示例没有这些身份字段。Code 提供可选 generation_id，可用时辅助辨认轮次；缺失时不能猜成其它字段。
4. IDE 的 Stop 反馈路径明确为 exit 2 + stderr。采用该公共退出码路径，避免只返回 Claude 的 decision/reason JSON 导致 IDE 不处理。普通采集失败仍放行；stop_hook_active 时观察但不再次要求继续。
5. 官方规范不能证明没有 ID 的并发工具调用可以唯一配对。实现只能保守配对单个未完成、同名同参数调用；重叠、缺失或不匹配事件应记为 unknown，不能制造可信来源。

## 验证边界

自动测试使用官方字段形状构造输入，覆盖轮次、串行配对、分批提醒、冲突保护和共存卸载。没有通过实际 IDE / IDE 插件 / Code 模型会话验证运行时 hook 加载与信任；doctor 只能确认配置。

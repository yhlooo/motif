# Hook 参考指南（原文节选）

来源：https://www.codebuddy.cn/docs/cli/hooks

采集日期：2026-11-05。以下正文为原文节选，未列出的章节省略。

## 配置

CodeBuddy Code hooks 存储在你的设置文件中：

| 作用域 | 文件路径 | 说明 |
| --- | --- | --- |
| 用户级 | `~/.codebuddy/settings.json` | 适用于所有项目 |
| 项目级 | `<项目根>/.codebuddy/settings.json` | 对项目成员共享的配置 |
| 项目本地 | `<项目根>/.codebuddy/settings.local.json` | 本地未提交配置 |
| 企业策略 | 集成发布的策略文件 | 受企业统一管理 |

配置合并规则：来自不同作用域的 hooks 会合并而非覆盖。同一事件的所有匹配 hooks 都会并行执行。

## Hook 输入

Hooks 通过 stdin 接收包含会话信息和事件特定数据的 JSON 数据：

```jsonc
{
  // 公共字段
  "session_id": "string",
  "transcript_path": "string",  // 对话 JSON 的路径
  "cwd": "string",              // 调用 hook 时的当前工作目录
  "permission_mode": "string",  // 当前权限模式： "default"、"plan"、"acceptEdits" 或 "bypassPermissions"
  "generation_id": "string",    // 可选；conversationRequestId 可用时等同 session Request ID，并在同一轮 UserPromptSubmit / Stop 等事件间共享；不可用时省略，不能用 messageId 或 traceId 替代

  // 事件特定字段
  "hook_event_name": "string"
  // ...
}
```

### PreToolUse 输入

```json
{
  "session_id": "abc123",
  "transcript_path": "/Users/.../.codebuddy/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",
  "cwd": "/Users/...",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse",
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/path/to/file.txt",
    "content": "file content"
  }
}
```

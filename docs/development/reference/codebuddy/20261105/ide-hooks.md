# Hook 功能使用文档（原文节选）

来源：https://www.codebuddy.cn/docs/ide/Features/Hooks

采集日期：2026-11-05。以下正文为原文节选，未列出的章节省略。

## 功能特性

- ✅ 多事件支持: 支持 7 种关键事件（SessionStart、SessionEnd、PreToolUse、PostToolUse、UserPromptSubmit、Stop、PreCompact）

## 输入格式

Hook 脚本通过 stdin 接收 JSON 格式的输入数据。

通用字段:

```json
{
  "session_id": "会话 ID",
  "transcript_path": "对话记录文件路径",
  "cwd": "当前工作目录",
  "hook_event_name": "事件名称"
}
```

事件特定字段:

- `SessionStart`: `source`
- `SessionEnd`: `reason`
- `PreToolUse/PostToolUse`: `tool_name`, `tool_input`, `tool_response`
- `UserPromptSubmit`: `prompt`
- `PreCompact`: `trigger`, `custom_instructions`
- `Stop`: `stop_hook_active`

## 退出码规范

| 退出码 | 含义 | 行为 |
| --- | --- | --- |
| 0 | 成功执行 | 允许操作继续，stdout 可能被处理 |
| 1 | 非阻塞错误 | 显示 stderr 作为警告，允许继续 |
| 2 | 阻塞错误 | 阻止操作，stderr 传递给 Agent/模型 |
| 其他 | 非阻塞错误 | 同退出码 1 |

特殊规则:

- PreToolUse: 退出码 2 会阻止工具执行
- Stop: 退出码 2 表示提供反馈，stderr 会注入到下一条消息
- PreCompact: 退出码 0 时，stdout 会作为额外的压缩指导

## 配置文件位置

优先级（高到低）：

1. 项目级: `<workspace>/.codebuddy/settings.json`
2. 用户级: `~/.codebuddy/settings.json`

项目级配置会覆盖用户级配置。

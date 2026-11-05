# Doctor 默认文本输出

依据：用户要求 `motif doctor` 默认提供人类可读输出，仅 `-o json` 使用 JSON。

## 行为

- `motif doctor` 或 `motif doctor -o text` 输出按 agent 分组的诊断结果；`-o json` / `--output json` 输出原始结构化诊断。其它格式返回用法错误（退出码 2），不开始项目诊断。
- 按 Codex、Claude Code、CodeBuddy 的固定顺序展示已安装 agent。默认文本包含状态摘要、必要的诊断原因、配置路径以及已取得的 hook 事件、启用与信任状态。项目内路径显示为相对路径。
- 与 install 共用 agent 状态渲染。配置检查通过统一显示绿色 ✓，重启提示为暗色中性信息；实际未启用或冲突按检查结果展示。没有取得 hook 运行时数据的 agent 不伪造事件状态。
- 颜色按实际 stdout、TERM 和 NO_COLOR 决定；重定向输出没有 ANSI。文本提供中英文文案。
- JSON 保留此前数据形状：仅 Codex 的安装输出单个 Diagnostic；存在 Claude / CodeBuddy 时输出按 agent 名索引的 Diagnostic 对象。JSON 无 ANSI 或文本标题，保留原始 status、detail、hooks 字段。

## 边界

CLI 负责呈现与格式选择，复用 DiagnoseAgent 查询，不执行 hook 或模型，也不改变配置。提取的 writeAgentDiagnostic 同时供 install / doctor 使用，避免两个命令状态文案和颜色含义不一致。没有修改开发 specs 或面向最终用户的文档。

## 验证

CLI 回归覆盖默认文本、-o json、多 agent JSON、Codex-only 旧 JSON 形状和非法格式退出码；展示测试覆盖 hook 的 enabled / disabled 与 trust 值、异常原因、配置路径、终端颜色及无 ANSI 文本。

本次验证：CLI 全部测试通过；真实 PTY 检查了中文暗色重启提示和配置路径。对构建出的命令验证默认 / -o text 为文本，-o json / --output json 为可解析 JSON；管道输出不含 ANSI。

## 配置检查通过即视为有效

按用户确认的诊断规则，Claude Code / CodeBuddy 的受管理 hook 配置存在且内容校验通过时，统一返回 active，与 Codex 的通过状态使用同一个绿色 ✓。不再用 configured / runtime not verified 区分客户端是否提供运行时查询能力；Claude 不再因为 PATH 中没有 claude 命令而返回 unavailable，以支持 IDE/plugin 使用场景。JSON 中这两个 agent 的通过状态也改为 active，数据形状不变。

安装和 doctor 摘要仍保留暗色重启提示。Codex 继续使用已有生效配置查询检查；实际 inactive / unavailable / conflict 等问题继续按现有规则报告。active 表示对应 agent 的健康检查规则已通过，不能把它解释为所有 agent 都执行过同一种运行时探测。没有运行时明细时不制造 HookState。

此规则调整的 integration / CLI 回归测试通过，覆盖 PATH 中没有客户端 CLI 时配置有效仍返回 active、错误配置返回 conflict，以及两个 agent 的绿色通过标记和暗色重启提示。

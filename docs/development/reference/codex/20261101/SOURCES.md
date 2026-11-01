# 原始资料来源

抓取日期：2026-11-01。以下文件为官方仓库原文，保持目录结构，未改写内容。

Codex commit：`67f0994b532b8740f14b5b738af7b7b2a915b1d6`（所查询 main 的固定快照，不代表已发布版本）。

- [docs/config.md](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/docs/config.md)
- [codex-rs/hooks/schema/generated/pre-tool-use.command.input.schema.json](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/schema/generated/pre-tool-use.command.input.schema.json)
- [codex-rs/hooks/schema/generated/post-tool-use.command.input.schema.json](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/schema/generated/post-tool-use.command.input.schema.json)
- [codex-rs/protocol/src/hooks.rs](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/protocol/src/hooks.rs)
- [codex-rs/core/src/tools/hook_names.rs](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/core/src/tools/hook_names.rs)
- [codex-rs/core/src/tools/registry.rs](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/core/src/tools/registry.rs)
- [codex-rs/core/src/config/mod.rs](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/core/src/config/mod.rs)
- [codex-rs/hooks/src/engine/command_runner.rs](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/src/engine/command_runner.rs)

Git hook 原文另存 ../../git/2.52.0/githooks.adoc，来源 https://github.com/git/git/blob/v2.52.0/Documentation/githooks.adoc 。

尝试访问 https://developers.openai.com/codex/hooks 返回 HTTP 403；docs/hooks.md 返回 404。故研究使用官方源码、schema 与仓库配置文档，尚未完成发行版 CLI 集成实测。

存档说明：config/mod.rs 仅摘录原文第 237–249 行至同目录 ghost-snapshot-config.excerpt.rs，其余文件完整保存。

## Stop hook 补充核实

同一固定源码版本，以下完整存档：
- [codex-rs/hooks/schema/generated/stop.command.input.schema.json](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/schema/generated/stop.command.input.schema.json)
- [codex-rs/hooks/schema/generated/stop.command.output.schema.json](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/schema/generated/stop.command.output.schema.json)
- [codex-rs/hooks/src/events/stop.rs](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/src/events/stop.rs)

以下为原文摘录，未改写摘录内容：
- [codex-rs/core/src/session/turn.rs L745–800](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/core/src/session/turn.rs#L745) → `codex-rs/core/src/session/turn.excerpt.rs`
- [codex-rs/core/tests/suite/hooks.rs L1302–1405](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/core/tests/suite/hooks.rs#L1302) → `codex-rs/core/tests/suite/hooks.excerpt.rs`
- [codex-rs/hooks/src/engine/mod.rs L205–209](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/hooks/src/engine/mod.rs#L205) → `codex-rs/hooks/src/engine/mod.excerpt.rs`
- [codex-rs/core/src/exec_env.rs L1–47](https://github.com/openai/codex/blob/67f0994b532b8740f14b5b738af7b7b2a915b1d6/codex-rs/core/src/exec_env.rs#L1) → `codex-rs/core/src/exec_env.excerpt.rs`

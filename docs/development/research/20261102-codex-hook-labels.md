# Codex hook 名称与显示文字

问题：Motif 的 hooks 在桌面授权界面显示为“钩子 1、钩子 2”等，能否配置名称？

## 依据

- [官方 hook 文档入口和原文摘录](../reference/codex/20261102/hook-labels.md)。
- 本机 `codex --version` 为 `codex-cli 0.166.0`。用 `codex app-server generate-json-schema --experimental --out <临时目录>` 核对本机协议；生成 schema 仅放临时目录，不存入 reference。

## 结论及边界

- 当前官方文档没有每条 handler 的 `name` / `displayName` 配置。文档支持 `statusMessage`，其定义是运行时显示文字，提供 TOML 配置示例。
- 本机协议的 ConfiguredHookHandler 和 hooks/list 返回元数据也支持 statusMessage，没有每条 hook 的 name / displayName 字段。这不能证明桌面 UI 内部如何选择标题，但支持“当前没有可直接配置的专用名称字段”的判断。
- hooks.json 顶层 description 是整个文件的元数据，不是每条 handler 的名称；不能把它当作解决逐条授权标题的字段。
- 可以给 Motif 各事件配置不同的 statusMessage 改善运行提示，但尚未验证桌面授权列表是否使用它作为标题，不能承诺能替换“钩子 N”。本次未修改安装器或项目实际 hook 配置。

## 仓库内手动验证配置

应用户要求，在当前仓库 `.codex/config.toml` 原 Motif 管理块之外添加独立测试块，以 `BEGIN/END MOTIF STATUS MESSAGE TEST` 标识。测试事件为 Stop，命令 `/bin/sleep 3`，同步执行，超时 10 秒；statusMessage 为“Motif 测试：statusMessage 显示验证（等待 3 秒）”。命令只等待，不输出、不修改文件。

已验证 TOML 可解析，原 Stop handler 和独立测试 handler 同时存在。授权列表标题及运行中文字仍需用户在桌面界面观察；本次不代替用户批准或信任 hook。测试完可整段删除该标记块。

## 用户实测结果与正式实现

用户在桌面 hook 界面看到了完整的测试 statusMessage，确认该字段能解决当前界面的辨识问题。这是本机实测结果，补充上述首次调查时尚未验证的 UI 行为。

按用户要求已删除独立测试块。Motif 的五个正式 Codex hooks 均生成简短 statusMessage，并使用已有上下文本地化器及中英文翻译资源。语言在安装时确定并写入配置，不随 Codex 界面语言实时切换；重复 install 会在验证原管理块仍与安装记录一致后重新生成，支持旧配置升级及语言切换，并同步安装记录。管理块之外的用户配置保留，用户编辑过的管理块仍报冲突。

测试覆盖中英文标签、未支持语言的英文回退、重复安装幂等、旧无标签配置升级、语言切换及卸载后的用户配置保留。安装相关测试通过。

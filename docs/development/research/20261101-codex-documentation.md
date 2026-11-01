# Codex 文档入口与 Motif 开发参考

日期：2026-11-01。

## 用户提供的入口

尝试读取 https://learn.chatgpt.com/docs/developers ，当前环境 HTTPS 请求返回 HTTP 403，未取得正文。因此尚不能确认该入口的目录结构、是否重定向，或其中 Hook 文档的具体内容；不能据此断言网站没有文档。

Codex 官方仓库的 Markdown 文档目前指向 developers.openai.com/codex 下的配置、安全和 AGENTS.md 页面。不能在未读取页面前假定 learn.chatgpt.com 与这些页面完全等价。对 Hook 协议的既有判断仍来自此前固定版本的源码审读，尚未替换为文档站的发行版契约。

## 已取得的文档及用途

原文保存在 [reference 文档索引](../reference/codex/README.md)，固定版本为 `67f0994b532b8740f14b5b738af7b7b2a915b1d6`。

| 文档 | 对 Motif 的帮助 |
|---|---|
| docs/config.md | 记录配置文档入口；明确 requirements.toml 的 allow_managed_hooks_only 可以忽略用户、项目和会话 Hook，安装成功不等于 Hook 已生效 |
| 配置 loader README | 明确项目配置为 .codex/config.toml；列出用户、项目、会话和受管理配置优先级；disabled_reason 层不会参与最终配置，可指导 install 的生效检查和冲突诊断 |
| docs/sandbox.md | 提供官方沙箱/审批指南入口，后续用于验证 Agent 执行 motif patch-batch create 时对 ~/.motif 的访问能力 |
| docs/agents_md.md | 提供 AGENTS.md 官方指南入口，便于理解项目指令与 Hook 的职责边界；不据此将 AGENTS.md 当作强制 Stop 检查的替代品 |

后两个原文较短，主要是官方外链指针，不是已抓取的完整指南。配置 loader 文档是内部模块说明，对理解实现有帮助，但不能取代对已发布 CLI 的兼容性验证。

## 后续验证

1. 文档站可访问后，优先读取 Hook 事件、项目级配置、信任机制、超时、退出码及 Stop block 规范。
2. 用目标 Codex CLI 版本验证 install 的配置位置、前后置快照、Stop 同轮续做和 --current-turn 的身份传递。
3. reference 仅存文档；需要源码佐证时在研究记录中引用固定版本外部 URL，不复制 Rust 文件或测试到本项目。

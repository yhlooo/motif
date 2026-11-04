# Alpha Release 可见性核对

2026-11-04 核对结果：

- Actions 运行 https://github.com/yhlooo/motif/actions/runs/41855162813 成功，由 GoReleaser 创建 v0.1.0-apha1 对应 Release 并上传安装包。
- Release：https://github.com/yhlooo/motif/releases/tag/v0.1.0-apha1 。API 返回 `isDraft=false`、`isPrerelease=true`，发布时间为北京时间 2026-11-04 23:14:28。
- 此次预发布标记由 Agent 在自动发布成功后通过 `gh release edit --prerelease` 设置。当前配置未设置 `release.prerelease`，GoReleaser 默认值为 false；如需以后自动判断可使用 `release.prerelease: auto`。参考：[GoReleaser 原文摘录](../reference/goreleaser/release-prerelease.md)。
- GitHub latest 仅选择非草稿、非预发布版本；本次查询 `/repos/yhlooo/motif/releases/latest` 返回 404。因此 README 中的 `/releases/latest` 链接不能用于访问当前唯一的 Alpha 预发布，应通过 Release 直链或 `/releases` 列表访问。参考：[GitHub 文档](../reference/github/release-latest.md) 及其来源页面。
- 发布配置能正常创建 Release，本次不修改发布配置或用户文档。

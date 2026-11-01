# GitHub PR 范围与 Motif CR 范围

依据：[GitHub 原始资料](../reference/github/20261101/SOURCES.md)。

官方 branches 文档明确 GitHub PR 使用三点 diff，即最近共同祖先（merge-base）与源分支当前状态比较；不是直接对目标 HEAD 与源 HEAD 比较。文档没有承诺通过任意一条图论最短路径选取 PR 提交，也没有在本轮资料中完整定义其 commits tab 的枚举算法，因此不能声称已逐项复刻 GitHub 提交列表。

Motif 已确认忽略 merge commit 的独立审阅项，并仍以 Git 最终 diff 保证完整性。后续用户已明确采用 GitHub 标准，不要求图论最短路径。

采用的实现：计算源与目标的 merge-base，候选取 source 可达而 target 不可达的非 merge 提交，以拓扑顺序展示。注意 rev-list 的 A..B 是提交可达集合差，而 git diff 的 A...B 是三点内容比较，不能混为一谈。若只沿一条最短路径或只取 first-parent，可能漏掉源分支合入的其他开发分支的新增提交。该方案已按用户最新指示采用，不保留“最近路径”作为另一个选择。

忽略 merge commit 不等于丢弃合并后的实际修改：冲突解决或只存在于 merge tree 的变化由最终完整性差异补入 CR 虚拟未知项。候选事件不形成严格状态链时需要重放/重基到展示状态；不可将其 snapshot 端点未经验证直接拼接。

多 merge-base（交叉合并）与无共同祖先时，建议 MVP 明确报告不支持的范围，不静默随机选择基准；常规可合并范围仍应可用。具体选择政策属于实现提案。

本地验证：在临时 Git 仓库构造长短两条开发路径汇入 merge、再有后续提交的 DAG。可达集合命令包含两条路径的非 merge 提交，排除 merge；仅在 merge tree 中新增的文件仍出现在三点最终 diff。该验证是本地 Git 行为检查，并非调用 GitHub API 对照测试。

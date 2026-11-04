# 批量读取 commit 标题

依据 Git 官方 [pretty-formats](https://git-scm.com/docs/pretty-formats) 和 [git-log](https://git-scm.com/docs/git-log)，`%H` 提供完整 commit hash，`%s` 提供 subject，`%x00` 产生 NUL 字节，`-z` 使用 NUL 分隔记录。`--no-walk=unsorted` 仅输出明确指定的 commits，按给定顺序输出，不遍历祖先。原文摘录见 [参考资料](../reference/git/commit-subject-format.md)。

采用 `git log --no-walk=unsorted --no-show-signature --no-notes --no-color --encoding=UTF-8 --format=%H%x00%s -z <OID>... --`。输入必须为完整 OID，按 128 个去重 OID 分批，避免单次参数过长，也不为每个 commit 启动进程。以 NUL 解析 hash/subject 对，校验返回数量及 hash；不以空格、中文或 HTML 字符切分标题。

本地验证：实际仓库双 commit 的输出是 `hash\0subject\0hash\0subject\0`；新增真实临时 Git 仓库测试覆盖显式 OID 顺序、重复 OID、中文、特殊字符、正文不混入 subject、空 subject、拒绝 revision 表达式，以及空输入不读取 HEAD。

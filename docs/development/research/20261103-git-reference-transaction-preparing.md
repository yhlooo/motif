# Git 2.54.0 引用事务的 preparing 阶段

[Git 2.54.0 的 hook 文档](../reference/git/2.54.0/githooks.md)列出了锁定引用前的 `preparing` 阶段。在临时仓库用 Git 2.54.0（Apple Git-157）安装只打印阶段并读完标准输入的 hook，执行 `git reset --hard HEAD` 和 `git checkout -b probe`，两个命令各收到四次 `preparing`。

Motif 安装的 hook 会转发阶段参数。采集器只处理 `committed`，并对 `prepared` 和 `aborted` 直接返回。每次 `preparing` 通知都会命中未知阶段分支并打印 `invalid reference transaction phase`。hook 脚本吞掉 Motif 的非零退出状态，保留原有 hook 的退出状态。

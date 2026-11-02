# Git hooks 路径及安装作用域

外部依据：[Git 官方文档索引及原文摘录](../reference/git/20261102/hooks-path.md)、[归档 githooks 原文](../reference/git/2.52.0/githooks.adoc)。

- Git 的默认 hooks 目录是 Git 管理目录下的 hooks；`core.hooksPath` 会替换默认目录，而不是与默认目录叠加执行。因此添加本地覆盖会停止本仓库对全局目录内 LFS 等 hook 的调用。
- 路径配置中的 `~` 不能直接交给 `filepath.Join`；使用 `git config --path --get core.hooksPath` 让 Git 自己解析路径。
- 使用 `git config --show-scope --get core.hooksPath` 区分实际生效的 local / worktree 与 global / system 配置；保留仓库和工作树显式设置。command 作用域优先级更高，不能假装 local 配置可以覆盖它。
- 普通非裸仓库 hooks 的相对路径以工作区根目录为基准；linked worktree 的 `.git` 通常是文件，默认 hooks 和 local 配置需要定位到 Git common directory，不能简单拼工作区中的 `.git/hooks`。
- local 配置由同一仓库的 linked worktrees 共享；警告需要明确这一点，卸载时需要等最后一个安装拥有者退出后才能撤回 Motif 创建的覆盖。

2026-11-02 使用本机 Git 的隔离临时仓库验证：全局配置 `~/.git-hooks` 的原始读取保留 `~`，路径类型读取会展开；添加 local 后有效作用域变成 local；移除 local 后恢复全局路径。回归测试同时覆盖 local 的 `~` 展开、linked worktrees 的共享 hooks 和配置生命周期。测试不修改开发者的全局配置。

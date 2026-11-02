# 项目 Git hook 安装与修复

根据本次用户确认的四项修改更新安装机制。外部语义依据见[调查记录](../research/20261102-git-hooks-path.md)。本方案取代 20261101 文档中默认复用 global hooksPath 的安装策略。

## 边界和交互

应用层 `PlanHooks` 只读取 Git 配置并生成目标路径及是否需要切换继承路径的计划。CLI 在任何安装确认之前输出影响说明；覆盖 global / system 路径需交互确认，非交互必须显式 `--yes`。直接调用应用层安装也必须提供确认选项，不能绕过确认直接修改 local 配置。

无显式 hooksPath 时使用 common Git directory 下的 hooks。已有 local / worktree hooksPath 保持不变，路径类型由 Git 展开，再以工作区根目录解释相对路径。只有继承 global / system 时，添加指向 common Git directory/hooks 的绝对 local 配置。警告明确原全局目录中的 LFS 等 hook 将不再在本仓库执行，以及 local 配置对 linked worktrees 的共享影响。全局目录与全局配置不写入。

## 脚本和已有 hook

新生成的 shell wrapper 使用固定的 `# BEGIN MOTIF GIT HOOK` / `# END MOTIF GIT HOOK` 行界定管理段；段内另注释项目地址 `https://github.com/yhlooo/motif`，地址不参与标记识别。

已有脚本或符号链接备份后由 wrapper 串接；继续保留 argv、流式 hook 的 stdin 及原退出码。卸载校验记录的管理内容后删除 wrapper 并恢复原 hook；不对被编辑的 Motif 管理脚本强制覆盖或删除。识别到不完整或未登记的标记也报冲突，避免误把管理代码当用户脚本备份后重复串接。

## 重装和路径变化

每次 install 都重新计算目标路径并读取实际 hook 内容、检查可执行权限，不再由记录中的四个文件名判定安装完成。缺失脚本重新生成，丢失执行权限修复；配置和脚本无变化时返回 already configured，不重复备份。原脚本备份缺失或受管理内容被编辑时保留现场并报错。

有效目录改变时，先校验新旧位置并准备新 hooks，然后清理仍由本项目管理的旧 wrappers、恢复旧备份；其他拥有者仍使用的脚本保留。普通文件写入、备份移动、local 配置写入失败会回滚本次修改。

## 元数据和兼容

- 当前工作树的安装记录：`GitDir/motif/install.json`。
- 位于本仓库 Git 管理目录中的 hooks，其原脚本备份：`CommonDir/motif/original-hooks/<hooks-path-hash>/<hook-name>`。
- 此类共享 hooks 的拥有者登记：`CommonDir/motif/owners/<hooks-path-hash>/<git-dir-hash>.json`。
- 用户显式 local 指向外部共享目录时，拥有者登记仍放外部目录的 `.motif-projects/`，原脚本备份放外部目录的 `.motif/original-hooks/`，以支持不同仓库共享。Git 实际执行的入口文件仍位于 hooks 目录，元数据不再散放 `.git/` 根目录或 `.git/hooks/`。
- 兼容读取旧 `GitDir/motif-install.json`；成功重装后迁移记录并删除旧记录。兼容旧 wrapper，迁移可安全迁移的旧 `.git/hooks/*.motif-original` 备份。仍被其他项目拥有的冲突 wrapper 保留并报错。
- 记录 Motif 新增的 local hooksPath；linked worktrees 继承其拥有权，最后一个拥有者卸载时仅在当前 local 值仍等于管理值时移除，保留用户后续更改。

## 验证

回归测试覆盖继承路径的警告与确认、全局 LFS hook 不变、本地 `~` 路径、缺失修复与幂等、标记和链接、已有脚本与流式 stdin、路径迁移、旧元数据和备份迁移、管理段冲突、linked worktrees 与外部共享拥有者生命周期、写入失败回滚。

## Codex hook 显示文字与 i18n

用户已实测 statusMessage 能在当前桌面 hook 界面显示，详见[显示文字调查与验证](../research/20261102-codex-hook-labels.md)。按用户确认的文案风格，为所有正式 hook 生成此字段：

| 事件 | 中文 | 英文 |
| --- | --- | --- |
| PreToolUse | Motif 工具运行前快照 | Motif snapshot before tool execution |
| PostToolUse | Motif 工具运行后快照 | Motif snapshot after tool execution |
| Stop | Motif 回合结束快照 | Motif snapshot at turn end |
| Interrupt | Motif 中断快照 | Motif snapshot on interruption |
| SessionEnd | Motif 会话结束快照 | Motif snapshot at session end |

沿用 `internal/i18n` 上下文本地化器和 active.en/zh.yaml；CLI 按既有 LANG 机制选择安装语言，其他语言回退英文。该字段属于生成配置，因此语言在 install 时固定；重新 install 可更新语言及旧无字段配置，不根据 Codex 的界面语言动态变化。

重装仅在当前管理块与保存记录一致时替换管理块并同步 manifest。块外用户配置保持；块内用户修改继续按冲突处理。独立测试用 Stop hook 已删除。

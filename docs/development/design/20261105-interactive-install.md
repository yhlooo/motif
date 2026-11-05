# 交互安装与文件修改预览

依据：用户要求安装时使用上下导航、空格多选、回车确认的列表，然后列出文件修改并统一 Y 确认。[终端调查](../research/20261105-install-terminal-ui.md)。

## 交互行为

- 未指定 --agent 且输入输出为交互终端时，打开 agent 多选列表。检测到 CLI 的项默认勾选，未检测到的仍可选择（含只使用 IDE/plugin 的 CodeBuddy）。上下移动，空格切换，回车结束选择，q / Ctrl+C 取消。未选任何项时退出，不安装。
- 列表含 codex、claude、codebuddy，用于安装或修复。未选 agent 的已有配置继续保留。
- 选定后预检所有目标，输出 create / update / remove 文件清单，包括选中 agent 配置、Git hooks、原 hook 备份、拥有者记录和安装记录；切换继承 core.hooksPath 时还显示 Git config 文件及原有风险说明。
- 交互安装（包括显式 --agent）统一询问 Apply these changes? [Y/n]，仅大写 Y 确认；回车、小写 y、n/N 和其它输入取消；EOF 或读取失败不视为确认；--yes 跳过最终确认。非交互发生文件修改时必须同时指定 --agent 和 --yes。无变更时提示 No file changes needed，不要求重复授权。
- raw 模式只用于列表；确认前恢复正常终端模式。context 取消、读取失败或用户取消均不写入集成文件。

## 应用边界

integration.InstallWithOptions 保留现有安装接口与缺失修复能力，增加可选 BeforeApply([]FileChange) 回调。应用层完成全部配置与 hook 校验后，生成有序文件操作计划和稳定排序的净文件变更清单，再调用回调；CLI 负责展示和确认，应用层不依赖终端。

计划在内存中模拟 write / move / remove，因此原 hook 备份迁移、已有 hook 链接和安装路径迁移的预览来自实际应用的同一操作序列。无变化文件不列入清单；确认前不创建配置、备份或 manifest。确认后重新核对预检时的原始文件内容、模式、链接与存在性，有变化则拒绝执行并要求重跑。应用仍使用原 fileTransaction 写入和失败回滚。

本地数据目录、日志和项目身份注册沿用已有 CLI 初始化，不属于集成文件修改预览。没有更新开发 specs 或最终用户文档。

## 验证

覆盖多选键盘操作、取消与空选择、终端读取不吞确认输入、大小写确认、预览取消不写文件、用户在确认期间编辑文件时拒绝应用、重复安装无文件变更，以及已有安装修复、备份迁移、链接、工作树与卸载回归。真实 PTY 手动检查列表绘制、选择、确认和终端恢复。

本次验证结果：integration / CLI 测试通过；`go test ./... -skip '^TestWorktreeActualBytesAndFileTypes$'` 通过，沿用此前已确认的当前环境非法 UTF-8 文件名限制。Darwin 原生构建和 Linux amd64 交叉构建通过。在真实 PTY 中选中 CodeBuddy，确认前配置文件不存在，输入 Y 后成功安装；另验证 Ctrl+C 取消并恢复光标与原屏幕。

## 安装结果展示

按本次用户确认的示例，CLI 完成安装后输出分组摘要：安装完成（无更改时为配置已就绪）、Agent 配置、Git hooks、生效状态。标题使用青色加粗，成功标记为绿色，需要处理的警告为黄色，异常为红色，路径降低亮度。stdout 重定向、TERM=dumb 或设置非空 NO_COLOR 时不输出 ANSI 颜色序列；仍保留分组、缩进和状态符号。界面文案沿用现有 LANG 国际化机制，支持中文和英文。

项目内路径相对项目根目录显示；项目外 hook 路径保留完整路径。每个 hook 目录只展示一次，目录下列出脚本名。重复的 --agent 不重复展示。原 hook 备份只在此次新增或迁移时展示，继承 hooksPath 的切换提示也只在发生切换时展示；安装前的文件预览和风险说明保留。

展示层只读取安装 manifest、实际变更清单和已有 DiagnoseAgent 返回值。检查通过统一显示绿色 ✓；Claude Code / CodeBuddy 按配置校验通过返回 active，旁边保留暗色“配置已写入，重启客户端后生效”。Codex 保留当前会话覆盖说明，不以客户端是否提供运行时查询能力区分通过标记。inactive / unavailable / conflict 等状态保留原诊断原因，不把配置写入成功视为运行时已生效。doctor 的结构化输出保持不变。

本次日志调整验证：CLI 全部测试通过；新增输出用例覆盖中英文分组、路径去重、通过状态、异常原因、按需备份与继承路径提示。真实 PTY 检查了 SGR 颜色与缩进；真实管道输出确认没有 ANSI 序列，重复安装显示“配置已就绪”并保留暗色重启提示。

Claude Code / CodeBuddy 的受管理配置校验通过时返回 active 并显示绿色 ✓，暗色重启提示不属于警告。实际 inactive / unavailable / conflict 的诊断继续展示。

重启提示的文字使用 SGR 2 降低亮度，和保持正常亮度的 Claude Code / CodeBuddy 名称区分；配置检查通过后统一显示绿色 ✓，重定向时不输出样式。

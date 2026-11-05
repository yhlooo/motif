# 安装终端交互调查

## 资料

终端 raw 模式使用 Go 官方维护的 [golang.org/x/term v0.43.0](https://pkg.go.dev/golang.org/x/term@v0.43.0)，[原始源码与许可证](../reference/go-term/20261105/README.md)。当前模块缓存中已有该版本，依赖 x/sys v0.44.0，低于项目现有版本，接入无需升级现有 x/sys。

屏幕切换依据 [XTerm Control Sequences](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html)，[原文节选](../reference/go-term/20261105/xterm-screen.md)。

## 结论

- IsTerminal 用于检查实际输入输出文件描述符，不能仅因进程 stdin 是终端就把 Cobra 重定向输入也视为交互。
- MakeRaw 可立即接收空格和方向键，返回的 State 必须通过 defer Restore 恢复。raw 模式禁用 ISIG，因此 Ctrl+C 要作为字节 3 主动取消。
- 读取终端使用现有 x/sys/unix.Poll，定期检查 context，避免后台 goroutine 在取消后继续抢读终端。列表读取每次只消费一个字节，不吞掉随后确认阶段的输入。
- 列表在独立终端屏幕中展示，结束时恢复原屏幕和光标；文件修改清单与确认保留在普通输出中，便于回看。
- 终端协议依赖 ANSI/xterm 能力；TERM=dumb 或非终端输入/输出使用显式 --agent 与 --yes，不输出控制序列。

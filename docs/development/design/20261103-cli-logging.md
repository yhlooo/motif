# CLI 诊断日志

CLI 在 Motif 数据目录追加写入 `motif.log`。默认目录是 `~/.motif`，也可通过 `MOTIF_DATA_DIR` 指定。新日志文件的权限为 `0600`。CLI 直接将需要用户或 Agent 处理的错误打印到标准错误，Agent hook 的协议响应通过标准输出发送。

数据目录中的 `config.json` 可以设置 `verbosity`。默认值 0 对应 info，1 对应 debug，2 对应 trace。显式传入的 `--verbose`（`-v`）优先，包括 `-v 0`。

Debug 日志记录 Codex 和 Claude 的原始 hook 请求及响应，以及 Git hook 的工作目录、参数和协议输入。采集层也记录被忽略的观测，便于用 session、turn 和 tool use ID 对照 hook 请求。原始输入可能包含工具参数、命令或文件内容。

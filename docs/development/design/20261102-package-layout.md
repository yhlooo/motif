# 包位置与名称整理

日期：2026-11-02。状态：用户授权调整包位置，已实施。

依据：[可见性评审](../research/20261102-package-visibility.md)。当前项目提供 Motif 应用，没有需要在此次整理中保留的公共 Go 库接口。

## 结构

`pkg/i18n` 整包迁至 `internal/i18n`，包含翻译资源及维护说明；`pkg/version` 迁至 `internal/version`。两者供应用内共享，不再对项目外部开放。迁移后删除空的 pkg 目录，更新 CLI 导入、GoReleaser 版本注入路径及翻译维护命令。

其余包保留职责位置：domain/state 为共享不可变状态值，domain 为事实及规划，content 为对象引用和生命周期锁，adapters 为 Git/SQLite，app 为用例及装配，transport 为 CLI/Web。独立包有跨层实际使用需求，不额外嵌套 internal。

统一三个与目录不一致的包名：`internal/domain/state` 从 change 改为 state，`internal/adapters/sqlite` 从 store 改为 sqlite，`internal/transport/cli` 从 commands 改为 cli。更新生产和测试代码的引用；第三方 SQLite 驱动仍按其自身导入使用。包名与路径一致有助于直接读懂调用方的依赖。

本次仅改变位置、包名及引用；保留导出接口、State 门面及应用服务行为。服务对适配器的封装是另一项需要改变接口的工作，不混入此次位置整理。历史设计、调查中对旧路径的记录保持原样，当前结构由本文件及 AGENTS.md 描述。

## 验证

使用临时 GOCACHE 避免写入沙箱外缓存；测试通过 `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1` 隔离个人 Git 签名设置，不修改用户配置。

- `go vet ./...` 通过。
- Linux amd64、CGO_ENABLED=0 编译通过；本机 macOS arm64 编译通过。
- 使用 `-X github.com/yhlooo/motif/internal/version.Version=package-layout-check` 构建，运行 `version --output-format json` 返回对应版本，验证 CLI 启动及新路径版本注入成功。
- 全量 `go test ./...` 未全部通过：`TestWorktreeActualBytesAndFileTypes` 创建 `p` 加 0xff 字节的文件名，在沙箱内报 operation not permitted，沙箱外报 illegal byte sequence；`TestDiagnoseReportsEffectiveHookTrust/trusted` 返回 inactive/missing。其余包测试通过，同包其它测试未报告失败。
- 在临时目录解压调整前 HEAD，再运行上述两个测试，均复现失败（特殊字节文件名在沙箱内拒绝，信任诊断结果同样为 inactive/missing）。此次位置整理未扩展为修复这两项既有测试问题。
- `git diff --check` 通过。当前源码、AGENTS.md 及发布配置中无旧 pkg 导入或旧包声明；历史评审与设计保留当时路径。

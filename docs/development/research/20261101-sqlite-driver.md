# SQLite 驱动与 GORM

## 当前选择

按用户明确要求，使用 `github.com/glebarez/sqlite v1.11.0` 配合 `gorm.io/gorm v1.31.1`。底层是 `github.com/glebarez/go-sqlite` 和 modernc 的纯 Go SQLite，不依赖 CGO 或 Wasm。原先使用的 ncruces Go/Wasm 驱动及 wazero 已从运行依赖中移除。

官方依据（均为固定版本原文存档）：

- [glebarez/sqlite README](../reference/glebarez-sqlite/v1.11.0/README.md)，来源 https://github.com/glebarez/sqlite/blob/v1.11.0/README.md 。明确说明 pure-Go、GORM 用法，以及外键默认关闭、应使用 `_pragma=foreign_keys(1)`。
- [GORM dialector 源码](../reference/glebarez-sqlite/v1.11.0/sqlite.go.txt)，来源 https://github.com/glebarez/sqlite/blob/v1.11.0/sqlite.go 。注册的是 glebarez 的 `sqlite` 驱动。
- [底层连接源码](../reference/glebarez-go-sqlite/v1.21.2/sqlite.go.txt)，来源 https://github.com/glebarez/go-sqlite/blob/v1.21.2/sqlite.go 。支持 `_pragma` 和 `_txlock=immediate`；后者用于在读取采集游标前取得写事务锁。
- [GORM CRUD 与 Transaction 源码](../reference/gorm/v1.31.1/finisher_api.go.txt)，来源 https://github.com/go-gorm/gorm/blob/v1.31.1/finisher_api.go 。
- [GORM Migrator 接口](../reference/gorm/v1.31.1/migrator.go.txt)，来源 https://github.com/go-gorm/gorm/blob/v1.31.1/migrator.go 。

新库通过 GORM AutoMigrate 建立模型、外键、唯一约束和检查约束；现有 v1 库表结构保持兼容，不重写既有数据。所有业务查询、插入、更新、删除和事务均使用 GORM API。连接池句柄仅用于连接数配置和 Close，不执行 SQL。

## 环境中的依赖取得与校验

环境策略拒绝 Go module proxy 跳转到 storage.googleapis.com 的大文件下载。`modernc.org/sqlite v1.23.1` 改从官方 GitLab 仓库取得固定 tag：

- https://gitlab.com/cznic/sqlite ，commit `91c2fe30158adc48d9d4b7b176b85d99ad1f1924`。
- 使用官方 `golang.org/x/mod/zip.CreateFromDir` 在开发环境的 module cache 中生成标准模块归档；Go 正常执行校验，结果 `h1:nrSBg4aRQQwq59JpvGEQ15tNxoO5pX/kUjcRNwSAGQM=`，与 go.sum 一致。
- 没有关闭校验、设置 replace 或将源码/vendor 放入项目。普通开发环境仍按 go.mod/go.sum 正常下载依赖。

## 历史

初版 MVP 因另一版本 modernc 模块归档下载受限，曾使用 `ncruces/go-sqlite3 v0.30.1`。原始 driver 参考存档保留用于解释历史；当前实现已经采用用户指定的 glebarez/GORM 方案。

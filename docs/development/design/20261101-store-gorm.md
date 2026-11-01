# 纯 Go SQLite 与 GORM 存储实现

本次调整来自用户明确要求：SQLite 必须纯 Go，所有数据库操作通过 GORM，并采用 `github.com/glebarez/sqlite`。

## 边界

`pkg/store` 的对外接口保持不变；内部统一使用 `*gorm.DB`。`models.go` 的私有持久化模型定义表名、列名、索引及关系约束，API 视图与数据库模型分离。不在服务层或命令层暴露连接，也不使用 GORM Raw/Exec 包装手写 SQL。

连接通过 `gorm.Open(sqlite.Open(...))` 建立。DSN 为每个连接启用外键、WAL、5 秒 busy timeout 和 immediate 事务；底层连接池只用于设置连接数及关闭。事务用 `WithContext(ctx).Transaction`，所有修改在显式服务事务内完成，因此关闭 GORM 单次写入的默认额外事务。

查询使用 Take/Find/Pluck/Count、子查询及 Preload；写入使用 Create/CreateInBatches/Update/Delete 和 OnConflict。复合主键/唯一索引、零值字段、BLOB 路径、级联删除和检查约束由模型定义，避免 ORM 默认行为改变既有语义。

## 数据兼容

当前 schema 为 v6。新库通过 GORM AutoMigrate 建立；v1 至 v5 在事务内补列/补表，保留已有事件、快照、批次、CR、评论与回复。新增字段涵盖稀疏快照、派生事件来源、批次来源、历史改写映射、引用变化、源数据版本、视图版本和稳定项/评论锚点。未知版本与缺失必需表报错，不重置数据。

回归 fixture `pkg/store/testdata/v1.db.gz` 由原 v1 二进制生成。测试先验证迁移后旧数据、旧审阅状态及评论仍可读，再验证分批、审阅、回复与级联删除。v1 CR 缺失的来源 batch UID 不臆造；无法证明身份的批次保留旧历史，新项要求重新审阅并提示。可证明的非 batch 项保留身份/决定。旧完整快照仍可读取；新快照以 Git 基准加稀疏覆盖还原。

采集/分批/改写事务递增源版本，视图发布同时校验源版本与旧视图版本。稳定项身份保留历史，active 项决定当前汇总；旧项退出视图时不删除。源数据读取和审阅详情读取各自使用一致事务，避免混用版本。所有这些操作均使用 GORM。

## 验证

`CGO_ENABLED=0 go test ./...` 覆盖现有采集、分批、审查、安装和 HTTP 测试，以及新增的旧库兼容、外键/唯一约束与事务回滚、两个独立连接竞争同一采集游标。依赖图不再包含 ncruces/Wasm 或 mattn/go-sqlite3。

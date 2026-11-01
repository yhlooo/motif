# 纯 Go SQLite 与 GORM 存储实现

本次调整来自用户明确要求：SQLite 必须纯 Go，所有数据库操作通过 GORM，并采用 `github.com/glebarez/sqlite`。

## 边界

`pkg/store` 的对外接口保持不变；内部统一使用 `*gorm.DB`。`models.go` 的私有持久化模型定义表名、列名、索引及关系约束，API 视图与数据库模型分离。不在服务层或命令层暴露连接，也不使用 GORM Raw/Exec 包装手写 SQL。

连接通过 `gorm.Open(sqlite.Open(...))` 建立。DSN 为每个连接启用外键、WAL、5 秒 busy timeout 和 immediate 事务；底层连接池只用于设置连接数及关闭。事务用 `WithContext(ctx).Transaction`，所有修改在显式服务事务内完成，因此关闭 GORM 单次写入的默认额外事务。

查询使用 Take/Find/Pluck/Count、子查询及 Preload；写入使用 Create/CreateInBatches/Update/Delete 和 OnConflict。复合主键/唯一索引、零值字段、BLOB 路径、级联删除和检查约束由模型定义，避免 ORM 默认行为改变既有语义。

## 数据兼容

ORM 重构不改变已有 v1 的表/列契约，不需要重置数据库或修改版本号。新数据库使用 GORM AutoMigrate 在事务内创建；现有数据库检查版本与必需表，未知版本或不完整结构报错，保留原数据。以后发生结构变更时需增加独立版本迁移，不对现有数据无条件自动改表。

回归 fixture `pkg/store/testdata/v1.db.gz` 由未改动的重构前二进制生成，包含原始事件、批次、已批准 CR、行评论与回复。测试读取旧数据，并验证重新分批、修改审阅决定、回复及项目级联删除仍可执行，冻结 CR 不受源批次删除影响。

## 验证

`CGO_ENABLED=0 go test ./...` 覆盖现有采集、分批、审查、安装和 HTTP 测试，以及新增的旧库兼容、外键/唯一约束与事务回滚、两个独立连接竞争同一采集游标。依赖图不再包含 ncruces/Wasm 或 mattn/go-sqlite3。

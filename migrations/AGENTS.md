# Migration 规则

- 每个版本使用本文件内定义的冻结 Schema，禁止引用会继续变化的业务 Model。
- 已发布迁移不得修改；修复必须新增版本，并在 `migrations.go` 中按版本顺序显式注册。
- 每个迁移必须提供 `Migrate` 和 `Rollback`；不可逆变更必须明确返回错误并在发布说明中记录。
- `AutoMigrate` 仅可用于版本化迁移中的冻结结构，禁止在 API 启动路径调用。
- 项目选定 MySQL 或 PostgreSQL 后，新迁移只维护并验证所选数据库。

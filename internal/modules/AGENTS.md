# Modules 规则

- 每个模块拥有自己的 Handler、Service、Repository、Cache、DTO、Model、错误与路由入口。
- Gin 只出现在 Handler 和 `routes.go`；Service 仅接收 `context.Context`，不调用 Redis 命令或拼接 Redis Key。
- Handler 不得直接访问 GORM 或 Redis；模块之间通过 Service 或最小契约调用，不访问对方 Repository。
- Redis Key 构造函数集中放在模块根目录的 `redis_key.go`，业务缓存结构和序列化位于模块 `cache` 包。
- Repository 转换持久化错误；Service 定义业务语义和事务边界；HTTP 边界统一记录内部错误。
- 模块 `errcode` 包只定义本模块实际使用的专属整数常量。通用码由调用方直接引用 `commoncode.*`，模块不重新导出或聚合通用码，新增通用码无需同步模块；码段和命名遵循根目录 AGENTS.md 与 README。
- Auth 权限表写入必须经过 Repository 的 `permissionTransaction`，使用 Read Committed 并先取得 `permission_write_locks` 的固定行锁，再查询受影响用户或修改权限；禁止新增绕过此锁的权限写入路径。

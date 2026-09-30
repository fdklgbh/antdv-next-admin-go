# Gin 多模块架构吸纳与改造方案

## 1. 文档目的

本文档用于将 `gin-multi-module-architecture.md` 中适合本项目的内容落地到当前模板，并固定后续改造的范围、目录、依赖方向、缓存一致性、数据库迁移和日志策略。

原始文档继续作为架构参考；本文档是本项目后续实现和验收的直接依据。发生冲突时，应先更新本文档、根目录 `AGENTS.md` 和对应目录的 `AGENTS.md`，再修改代码。

## 2. 改造目标

本次架构改造必须完成以下目标：

1. 将业务代码完整迁移到 `internal/modules`，按模块组织 Handler、Service、Repository、Cache、DTO 和 Model。
2. 支持 MySQL 和 PostgreSQL，由配置选择一个关系型数据库驱动。
3. Redis 作为独立基础设施，继续承担登录会话和权限缓存，不作为关系型数据库的替代选项。
4. 权限查询采用 Cache-Aside：先查询 Redis，缓存未命中时查询数据库并回填。
5. 权限变化后删除所有受影响用户的权限缓存 Key。
6. 建立通用 Cache Store，同时保留各模块自己的业务 Cache 接口、Key 和序列化结构。
7. 使用 Gormigrate 维护版本化 Go Migration；具体项目选定数据库后只维护一条迁移历史。
8. 引入统一应用错误、响应、参数校验、分页和请求上下文能力。
9. 保留 zerolog，增加 Logger Interface、Context 字段传播、stdout、文件输出和文件轮转。
10. 为主要职责目录建立适当的 `AGENTS.md`，给后续开发提供就近、明确且不重复的约束。
11. 保留当前严格配置加载、动态路由权限、健康检查、安全响应头和优雅退出能力。

## 3. 非目标

本次改造不包含：

- 引入 Wire、Fx 等依赖注入框架。
- 同时实现 slog、zap 等其他日志后端。
- 在 API 启动时直接对当前业务 Model 执行 GORM `AutoMigrate`。
- 为每个类或每个文件机械创建接口。
- 将本地内存缓存作为 Redis 故障时的静默降级。
- 为数据库方言差异编写 SQL 字符串替换器。
- 在普通应用日志 Hook 中同步写入审计表。

## 4. 核心架构决策

### 4.1 模块化单体

采用模块化单体，而不是微服务。每个业务模块拥有自己的 Handler、Service、Repository、Cache、DTO、Model、错误和路由注册入口。

顶层 App 只负责：

- 初始化配置和日志。
- 初始化数据库、Redis 和通用 Cache Store。
- 构造各业务模块。
- 注册模块路由。
- 管理 HTTP Server 和后台组件生命周期。
- 按逆序关闭资源。

### 4.2 数据库与 Redis

MySQL 和 PostgreSQL 是互斥的关系型数据库选项：

```text
database.driver = mysql | postgres
```

Redis 是独立且必需的基础设施。只要启用当前认证模块，Redis 就必须可用，因为登录会话和权限缓存都依赖 Redis。

Redis 连接异常不能伪装成缓存未命中，也不能静默回退成本地缓存。

### 4.3 抽象原则

仅在以下情况提取接口：

- 存在多个实现。
- 需要隔离数据库、Redis、文件系统等外部系统。
- 模块边界需要稳定契约。
- 测试需要替换外部实现。

Repository 可以先使用具体类型。通用 Cache Store、模块业务 Cache 和 Logger 属于合理的接口边界。

## 5. 目标目录

```text
project/
├── AGENTS.md
├── README.md
├── architecture-adoption-plan.md
├── gin-multi-module-architecture.md
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── configs/
│   └── config.yaml
├── migrations/
│   ├── AGENTS.md
│   ├── migrations.go
│   └── v000001_init_schema.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── dependencies.go
│   │   └── router.go
│   ├── config/
│   │   ├── config.go
│   │   ├── app.go
│   │   ├── server.go
│   │   ├── database.go
│   │   ├── redis.go
│   │   ├── auth.go
│   │   └── log.go
│   ├── infrastructure/
│   │   ├── AGENTS.md
│   │   ├── database/
│   │   │   ├── database.go
│   │   │   └── dialect.go
│   │   ├── redis/
│   │   │   └── client.go
│   │   ├── cache/
│   │   │   ├── store.go
│   │   │   ├── redis.go
│   │   │   └── errors.go
│   │   └── logger/
│   │       ├── logger.go
│   │       ├── field.go
│   │       ├── context.go
│   │       ├── zerolog.go
│   │       └── writer.go
│   ├── modules/
│   │   ├── AGENTS.md
│   │   ├── auth/
│   │   │   ├── handler/
│   │   │   ├── service/
│   │   │   ├── repository/
│   │   │   ├── cache/
│   │   │   ├── model/
│   │   │   ├── dto/
│   │   │   ├── errors.go
│   │   │   ├── redis_key.go
│   │   │   ├── module.go
│   │   │   └── routes.go
│   │   ├── account/
│   │   │   └── ...
│   │   ├── admin/
│   │   │   └── ...
│   │   └── health/
│   │       └── ...
│   ├── httpserver/
│   │   ├── AGENTS.md
│   │   ├── router.go
│   │   └── middleware/
│   │       ├── auth.go
│   │       └── telemetry.go
│   └── shared/
│       ├── AGENTS.md
│       ├── errors/
│       ├── response/
│       ├── validator/
│       ├── pagination/
│       └── contextx/
├── tools/
│   └── migrationgen/
│       └── main.go
├── go.mod
└── go.sum
```

目录级规则只设置在职责边界：`migrations`、`internal/infrastructure`、`internal/modules`、`internal/httpserver` 和 `internal/shared`。`cmd`、`configs`、`internal/app` 和 `internal/config` 直接继承根目录规则。不得为满足形式要求复制同一套规则；只有出现无法由上层准确表达的特殊约束时，才在更深目录新增 `AGENTS.md`。

## 6. 依赖方向

```text
cmd
  ↓
app
  ├── infrastructure
  ├── modules
  └── httpserver

HTTP Request
  ↓
Middleware
  ↓
Module Handler
  ↓
Module Service
  ├── Repository
  ├── Module Cache
  └── Other Module Contract
       ↓
Infrastructure
  ├── GORM → MySQL/PostgreSQL
  └── Cache Store → Redis
```

必须遵守：

- Gin 只出现在 Router、Middleware 和 Handler。
- Service 接收 `context.Context`，不接收 `*gin.Context`。
- Service 不直接调用 Redis 命令。
- Service 不拼接 Redis Key。
- Handler 不直接调用 GORM 或 Redis。
- Infrastructure 不依赖业务模块。
- Shared 不依赖具体业务模块。
- 模块之间通过 Service 或最小契约调用，不访问对方的 Repository。

## 7. 配置设计

### 7.1 配置示例

```yaml
app:
  name: go-gin-template
  env: local

server:
  mode: release
  address: ":8080"
  startupTimeout: 5s
  readTimeout: 10s
  writeTimeout: 10s
  shutdownTimeout: 15s
  maxHeaderBytes: 16384

database:
  driver: mysql
  dsn: "app:password@tcp(127.0.0.1:3306)/app?charset=utf8mb4&parseTime=True&loc=Local"
  maxOpenConnections: 50
  maxIdleConnections: 10
  connectionMaxLifetime: 30m

redis:
  address: "127.0.0.1:6379"
  username: ""
  password: ""
  database: 0
  poolSize: 20

auth:
  permissionCacheTTL: 5m

log:
  level: info
  console:
    enabled: true
  file:
    enabled: false
    path: ./logs/app.log
    maxSizeMB: 100
    maxBackups: 10
    maxAgeDays: 30
    compress: true
```

PostgreSQL 示例：

```yaml
database:
  driver: postgres
  dsn: "host=127.0.0.1 user=app password=password dbname=app port=5432 sslmode=disable TimeZone=Asia/Shanghai"
```

### 7.2 校验规则

- 配置继续使用 Viper `UnmarshalExact`，未知字段直接失败。
- 配置约束继续使用 `validator/v10` 标签。
- `database.driver` 必须是 `mysql` 或 `postgres`。
- 所有超时、TTL、连接池上限和轮转数值必须校验范围。
- Console 和 File 至少启用一个；不能在两者都关闭时启动。
- File 启用时，路径和轮转参数必须存在并合法。
- 不为缺失配置提供静默默认值。

## 8. 多数据库支持

### 8.1 Database Factory

数据库驱动选择只允许出现在 `internal/infrastructure/database`：

```go
switch cfg.Driver {
case "mysql":
	dialector = mysql.Open(cfg.DSN)
case "postgres":
	dialector = postgres.Open(cfg.DSN)
default:
	return nil, nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
}
```

需要增加 `gorm.io/driver/postgres`。DB 和底层 `sql.DB` 连接池仍只创建一次，由 App 统一关闭。

模板的公共模块不得：

- 根据 Driver 编写分支。
- 使用 MySQL 或 PostgreSQL Driver 类型。
- 依赖方言专属错误文本。
- 在 Service 或 Handler 中编写方言专属 SQL。

### 8.2 建模规范

模板自身需要允许后续项目选择 MySQL 或 PostgreSQL，因此模板内置的 Auth、权限等公共模型优先使用两种 GORM Driver 都能映射的 Go 类型和常规 GORM Tag：

- 模板公共模型的主键和外键默认使用 `int64`，避免 PostgreSQL `BIGINT` 无法覆盖 `uint64` 完整范围；项目选定 MySQL 后可通过显式迁移改用无符号类型。
- 时间字段优先由 GORM 和应用维护。
- Upsert 优先使用 GORM `clause.OnConflict`。
- 普通查询使用参数绑定，不拼接值。
- Repository 将 GORM 特有错误转换成模块持久化错误。

具体项目一旦确定数据库，后续业务模型和迁移不需要继续兼容另一个数据库，可以使用已选数据库的 `UNSIGNED`、`ENUM`、`JSONB`、部分索引等专属能力。专属 SQL 只能出现在 Migration 或 Repository 内，不能泄漏到 Service。

GORM Migrator 能根据所选 Driver 自动处理常规建表、字段和索引语法，但不能自动判断字段重命名、数据回填、类型收窄及数据库专属能力的业务语义。这些变更必须在迁移中显式表达并人工审查。

## 9. 数据库迁移

### 9.1 工具选择

采用 `github.com/go-gormigrate/gormigrate/v2` 管理版本化 Go Migration，并提供独立的 `cmd/migrate`。迁移复用 Database Factory 创建的 `*gorm.DB`，由配置中的 `database.driver` 决定实际方言。

选择 Gormigrate 的原因：

- 当前项目已经使用 GORM。
- 具体项目最终只使用一种关系型数据库。
- 常规 DDL 可以通过 GORM Migrator 按已选 Driver 生成。
- 复杂变更仍可在 Migration 中调用 `tx.Exec` 编写所选数据库的 SQL。
- Migration 与应用一起编译发布，不需要携带外部 SQL 目录。

Goose 适合 SQL First 项目，但它只能创建迁移骨架，不会根据 GORM Model 自动推导 Schema Diff。当前需求更适合 Gormigrate。

### 9.2 迁移目录与注册

```text
migrations/
├── AGENTS.md
├── migrations.go
└── v000001_init_schema.go
```

`migrations.go` 显式、按版本顺序注册迁移：

```go
func All() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		migration000001InitSchema(),
	}
}
```

不得依赖 Go 文件加载顺序或隐藏的 `init` 注册。

模板初始迁移 `000001_init_schema` 包含用户权限、路由权限、缓存待失效记录和权限写锁四张表，并创建 `id=1` 的固定锁记录；回滚按相反顺序删除这四张表。具体项目开始维护自己的迁移历史后，再按版本追加新迁移。

每个迁移使用该版本专属、冻结的 Schema 结构，不引用会继续变化的业务 Model：

```go
type routePermission000001 struct {
	Method         string `gorm:"size:10;primaryKey"`
	Route          string `gorm:"size:255;primaryKey"`
	PermissionCode string `gorm:"size:128;not null;index"`
	Enabled        bool   `gorm:"not null;default:true"`
}

func (routePermission000001) TableName() string {
	return "route_permissions"
}
```

这可以避免未来业务 Model 改变后，从零执行旧迁移得到不同结果。

### 9.3 新迁移生成

Gormigrate 不负责比较当前 Model 和数据库，也不会自动生成完整迁移内容。项目提供 `tools/migrationgen` 或对应 Task 命令，只生成带唯一版本号的 Go Migration 骨架；开发者必须填写并审查 `Migrate` 和 `Rollback`。

建议工作流：

```text
修改或设计目标 Model
  ↓
生成新的 Migration 骨架
  ↓
使用 GORM Migrator 编写常规 DDL
  ↓
显式编写重命名、回填、删除或专属 SQL
  ↓
在所选数据库的临时实例执行 Up、Down、再次 Up
  ↓
提交 Model 与 Migration
```

### 9.4 迁移规范

- 每个版本同时实现 `Migrate` 和 `Rollback`；确实不可逆时必须在代码和说明中明确。
- 已发布迁移不得修改；修复必须创建新版本。
- 不得在迁移中直接使用当前业务 Model。
- 允许在版本化 Migration 内对冻结结构调用 `tx.AutoMigrate`。
- 禁止在 API 启动时对当前业务 Model 直接执行 `AutoMigrate`。
- 字段重命名、字段删除、类型收窄和数据回填必须显式实现，不能仅依赖 `AutoMigrate`。
- 项目选定数据库后，专属迁移只需要支持和验证该数据库。
- 应用发布前由部署 Job 或明确的迁移命令执行迁移。
- 迁移失败必须中止发布，不允许忽略并继续启动应用。

当前 MySQL SQL 迁移需要转换为第一个冻结的 Gormigrate Migration。模板阶段使用常规 GORM 类型和 Tag，使其能根据项目最终选择的 Driver 创建基础权限表；项目开始产生自己的迁移历史后，不再要求同时验证另一种数据库。

## 10. 通用 Cache Store

### 10.1 基础设施接口

`internal/infrastructure/cache` 提供与业务无关的字节存储能力：

```go
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}
```

统一定义：

```go
var ErrMiss = errors.New("cache: miss")
```

Redis 实现负责：

- 将 `redis.Nil` 转换成 `cache.ErrMiss`。
- 保留其他 Redis 错误的错误链。
- 设置 TTL。
- 支持批量删除。
- 不包含业务 Key、JSON 结构或权限判断。

### 10.2 模块 Cache

每个模块在自己的 `cache` 目录定义业务接口和 Redis Adapter。例如 Auth：

```go
type PermissionCache interface {
	GetRoutes(ctx context.Context, userID uint64) ([]RoutePermission, error)
	SetRoutes(ctx context.Context, userID uint64, routes []RoutePermission, ttl time.Duration) error
	DeleteUser(ctx context.Context, userID uint64) error
}
```

模块 Cache 负责：

- 构造模块 Redis Key。
- 序列化和反序列化。
- 校验缓存数据结构。
- 将通用 `cache.ErrMiss` 传播给 Service。
- 维护缓存 Schema 版本。

## 11. 权限缓存

### 11.1 Key 与数据结构

按用户缓存完整的有效路由集合：

```text
<app>:auth:permission_routes:<user_id>
```

示例：

```json
{
  "schemaVersion": 1,
  "routes": [
    "GET /api/admin/ping",
    "PATCH /api/users/:id"
  ]
}
```

要求：

- 使用 HTTP Method 与 Gin Route 模板组成稳定标识。
- 缓存允许路由集合，而不是为每个路由创建一个 Key。
- 空权限集合也必须缓存，避免持续访问数据库。
- Key 构造函数集中放在 Auth 模块的 `redis_key.go`。
- TTL 使用严格校验后的配置值。

### 11.2 查询流程

```text
1. PermissionCache.GetRoutes
2. 命中：在缓存集合中判断当前 METHOD + ROUTE
3. cache.ErrMiss：Repository 查询数据库
4. PermissionCache.SetRoutes 回填完整集合
5. 返回权限判断结果
```

只有 `cache.ErrMiss` 可以触发数据库回源。Redis 连接错误、超时、权限错误和缓存数据损坏必须返回错误，由 HTTP 边界转换为 503，并记录一次完整错误。

数据库回源查询：

```sql
SELECT rp.method, rp.route
FROM route_permissions AS rp
JOIN user_permissions AS up
  ON up.permission_code = rp.permission_code
WHERE up.user_id = ?
  AND rp.enabled = TRUE
```

Repository 使用 GORM 构造查询，保持 MySQL 和 PostgreSQL 兼容。

### 11.3 权限变更与缓存删除

所有权限变化必须经过 Auth Service，包括：

- 给用户授予权限。
- 撤销用户权限。
- 新增或修改路由权限映射。
- 启用或禁用路由权限。
- 删除权限码。

用户权限变化：

```text
数据库事务修改 user_permissions
  ↓
提交事务
  ↓
删除该 user_id 的权限缓存 Key
```

路由权限映射变化：

```text
事务内查询受影响 user_id
  ↓
修改 route_permissions
  ↓
提交事务
  ↓
分批删除所有受影响用户的权限缓存 Key
```

要求：

- 权限写操作设计为幂等，便于缓存删除失败后安全重试请求。
- 授权、撤权、路由映射变更和删除权限码共用 `permission_write_locks` 中 `id=1` 的数据库行锁。每个写事务使用 Read Committed，并在读取权限关系或执行修改前通过 `SELECT ... FOR UPDATE` 取得锁，直到提交或回滚才释放；锁记录由版本化迁移创建，缺失时必须报错。模板串行化权限写入以保证跨实例采集受影响用户的一致性，权限查询及提交后的 Redis 删除不持有该锁。
- 路由映射变更和删除权限码时，将受影响用户写入 `permission_cache_invalidations`，与权限修改在同一事务提交。按操作及资源标识读取待失效记录，删除 Redis 缓存成功后仅清理本批记录；重复调用同一路由或权限码的操作时继续处理遗留记录，避免修改后的权限表丢失原受影响用户。
- DB 提交成功但缓存删除失败时，不得宣称完整成功；必须返回明确错误并记录受影响用户和操作。
- 禁止业务代码或管理后台绕过 Service 直接修改权限表。
- 运维直接修改权限表时，必须同时执行对应缓存失效命令。
- 缓存 TTL 是失效失败后的最终安全边界，不能设置为永久有效。

### 11.4 一致性说明

数据库与 Redis 之间没有原子事务。提交后删除缓存属于最终一致性，存在并发缓存回填旧数据的短窗口。

基础实现采用：

- 数据库提交后删除缓存。
- 待失效记录用于显式重试，不启动后台 Worker；未重试的记录保留，缓存仍由 TTL 限制有效期。
- 缓存设置较短 TTL。
- 缓存删除失败明确报错。
- 所有权限写操作保持幂等。

如果项目要求撤权严格实时生效，应在后续增加可靠失效机制，例如事务 Outbox 和失效 Worker；不能声称普通 Cache-Aside 已实现跨数据库与 Redis 的强一致性。

## 12. Module 设计

### 12.1 Module 组装

每个模块通过 `module.go` 组装内部依赖并只暴露路由注册所需能力：

```go
type Module struct {
	Handler *handler.Handler
}

func NewModule(
	db *gorm.DB,
	cacheStore sharedcache.Store,
	logger logging.Logger,
	cfg config.Auth,
) *Module {
	repo := repository.New(db)
	permissionCache := authcache.NewRedis(cacheStore)
	svc := service.New(repo, permissionCache, logger, cfg.PermissionCacheTTL)
	return &Module{Handler: handler.New(svc)}
}
```

实际实现应根据依赖规模决定是否使用 Dependencies 参数结构，避免超长构造函数。

### 12.2 路由注册

模块在自己的 `routes.go` 中注册路由：

```go
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	// 注册模块路由
}
```

顶层 Router 只负责：

- 创建 Engine。
- 安装全局 Middleware。
- 创建 API Group。
- 调用各 Module 的 `RegisterRoutes`。
- 注册 NoRoute 和 NoMethod。

## 13. 统一错误与响应

建立 `internal/shared/errors`：

```go
type AppError struct {
	Code       int
	HTTPStatus int
	Message    string
	Err        error
}
```

规则：

- HTTP Status 表达协议语义。
- Business Code 表达业务原因。
- 业务码采用五位 `MMNNN`，成功为 `0`，HTTP Status 单独指定，不从业务码推导。当前码段、业务码与旧码迁移表见 [README](README.md#业务码与-http-状态码)。
- 通用码放在 `internal/shared/errcode`，模块专属码放在 `internal/modules/<module>/errcode`，按实际需要创建。调用处使用 `commoncode.ValidationFailed`、`authcode.Unauthenticated` 等名称，常量不重复模块名或添加 `Code` 前缀。
- 调用方按归属分别导入通用码包和模块码包。模块不通过常量别名或代码生成重新导出通用码，也不使用结构体嵌入、变量或访问方法聚合业务码；新增通用码只更新共享码包及文档，无需同步各模块。
- 码包只定义整数常量，显式赋值，不使用 `iota`，不依赖 HTTP、Service 或 Repository。已分配或退役的业务码不得重新使用。
- 客户端只接收安全消息。
- 内部错误通过 `Unwrap` 保留，并只在 HTTP、Worker、Consumer 等边界记录一次。
- Repository 错误先转换成持久化错误，Service 再转换成业务错误。

响应结构继续保留：

```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "requestId": "...",
  "timestamp": 1787990400000
}
```

补充：

- `Success`
- `Created`
- `NoContent`
- `Error(error)`
- 字段级校验错误
- 统一分页请求和分页响应

## 14. Request-ID 与 Context

Request-ID Middleware 继续校验客户端传入值，并使用安全随机数生成缺失或非法的 ID。

生成后必须同时写入：

- Gin Context。
- `X-Request-ID` 响应头。
- `c.Request.Context()`。

标准 Context 只保存请求范围元数据：

- `request_id`
- `user_id`
- `trace_id`

不得将 DTO、数据库连接或大型业务对象放入 Context。

## 15. 日志与文件轮转

### 15.1 Logger 接口

业务代码依赖项目 Logger Interface，不直接依赖 zerolog：

```go
type Logger interface {
	Debug(ctx context.Context, message string, fields ...Field)
	Info(ctx context.Context, message string, fields ...Field)
	Warn(ctx context.Context, message string, fields ...Field)
	Error(ctx context.Context, message string, fields ...Field)
	With(fields ...Field) Logger
}
```

当前只实现 Zerolog Adapter，不增加 slog 或 zap。`Field` 必须是与 zerolog 解耦的普通 Key/Value 结构。

### 15.2 Context 字段

Logger 从 Context 自动绑定：

- `request_id`
- `user_id`
- `trace_id`

调用方不应在每条日志里手工重复传入 Request-ID。

全局字段包括：

- `service`
- `environment`
- `version`

模块 Logger 通过 `With` 绑定 `module`，事件按需补充 `operation` 和业务标识。

### 15.3 输出与轮转

支持：

```text
Zerolog Adapter
  ├── stdout
  └── rotating file
```

文件轮转使用 `lumberjack`，需要增加对应依赖。支持：

- 单文件最大容量。
- 最大备份数量。
- 最大保留天数。
- gzip 压缩。
- Console 和 File 独立开关。

要求：

- File 启用时，启动阶段创建目录并验证文件可写；失败则终止启动。
- Writer 的关闭函数交由 App 生命周期管理。
- 不按 Info、Warn、Error 拆分文件。
- 容器环境默认只启用 stdout。
- 单机环境可以同时启用 stdout 和文件。
- Token、Cookie、密码、验证码和完整请求体不得写入日志。
- Panic 日志必须包含 Request-ID 和 Stack。
- Access Log 至少包含 Request-ID、Method、Route、Status、Latency、Client IP、User Agent 和可用时的 User ID。

## 16. 事务

第一阶段可以继续使用 GORM `db.Transaction`。出现多个 Repository 参与同一业务事务时，再提供统一 Transaction Manager。

事务边界属于 Service，不属于 Handler。权限修改必须在事务提交后执行缓存删除，不能在事务提交前删除最终缓存。

## 17. 目录级 AGENTS.md

### 17.1 继承原则

- 根目录 `AGENTS.md` 定义全项目规则。
- 子目录 `AGENTS.md` 只增加或收紧本目录特有规则。
- 不复制根规则，不制造多个不一致版本。
- 更深层目录的规则覆盖上层冲突项。
- 新建业务模块默认继承 `internal/modules/AGENTS.md`，不为每个模块重复创建规则文件。
- 只有模块存在独有的安全边界、数据不变量或外部合规约束时，才增加模块级 `AGENTS.md`。

### 17.2 各目录应包含的约束

| 文件 | 主要内容 |
|---|---|
| 根目录 `AGENTS.md` | 全项目规则，并覆盖 cmd、configs、App 组装、配置加载和通用工程约束 |
| `migrations/AGENTS.md` | 迁移结构必须冻结；禁止修改已发布迁移；限制 AutoMigrate 使用范围；项目选型后只维护所选数据库 |
| `internal/infrastructure/AGENTS.md` | Database、Redis、Cache、Logger 的技术边界、生命周期及禁止依赖业务模块 |
| `internal/modules/AGENTS.md` | 所有模块共用的目录结构、分层职责、跨模块调用、Redis Key 和业务错误规则 |
| `internal/httpserver/AGENTS.md` | Router、Middleware、认证上下文、统一响应、错误边界和敏感信息限制 |
| `internal/shared/AGENTS.md` | 仅稳定通用能力；不得依赖业务模块或变成杂物目录 |

## 18. 实施阶段

### 阶段一：规则与骨架

- 更新根目录 `AGENTS.md` 和 README。
- 创建目标目录及对应 `AGENTS.md`。
- 建立 Config、Infrastructure、Modules 和 Shared 骨架。

### 阶段二：数据库与迁移

- 增加 `database.driver`。
- 增加 PostgreSQL GORM Driver。
- 实现 Database Factory。
- 将现有 SQL 转换为冻结 Schema 的 Gormigrate Migration。
- 增加独立迁移命令和 Migration 骨架生成工具。
- 明确具体项目选定数据库后只维护一条迁移历史。

### 阶段三：通用能力

- 实现 AppError、统一响应、校验、分页和 Context 元数据。
- 实现通用 Cache Store。
- 实现 Logger Interface、Zerolog Adapter 和文件轮转。

### 阶段四：模块迁移

- 将 Auth、Account、Admin、Health 迁移到 `internal/modules`。
- 为模块建立 Handler、Service、Repository、Cache、DTO 和 Module。
- Router 改为只安装 Middleware 和注册 Module。

### 阶段五：权限缓存

- Repository 查询用户完整权限路由集合。
- Redis PermissionCache 实现 Cache-Aside。
- 权限写操作集中到 Service。
- 用户权限及路由权限变化后删除受影响缓存。
- 补齐缓存异常、空权限和缓存数据损坏的错误处理。

### 阶段六：清理与同步

- 删除旧目录和不再使用的代码。
- 更新 README、配置示例、迁移说明和架构文档。
- 检查各职责边界的 `AGENTS.md` 是否准确覆盖当前职责。
- 检查模板是否同时提供 MySQL/PostgreSQL Driver 选项，并明确项目选型后的单数据库边界。

## 19. 验收标准

### 19.1 架构

- 所有业务模块位于 `internal/modules`。
- Handler、Service、Repository、Cache 职责清晰且无反向依赖。
- Router 不直接接收 GORM 或 Redis Client。
- Service 不依赖 Gin Context。
- 业务代码不直接使用 zerolog API。

### 19.2 数据库

- 同一份应用代码可通过配置分别连接 MySQL 和 PostgreSQL。
- 模板的基础 Migration 能通过所选 Driver 完成全量 Up、Down、再次 Up。
- 具体项目只要求在已选数据库上维护和验证新增 Migration。
- 业务模块中不存在数据库 Driver 判断。
- 已发布 Migration 使用冻结结构，不引用当前业务 Model。
- API 启动过程不会自动修改数据库 Schema。

### 19.3 权限缓存

- 缓存命中时不访问权限数据库。
- 缓存未命中时查询数据库并回填，包括空权限集合。
- Redis 非 Miss 错误不会被当作数据库回源条件。
- 用户权限变化会删除对应用户缓存。
- 路由权限变化会删除所有受影响用户缓存。
- 无路由权限配置时仍然默认拒绝访问。

### 19.4 日志

- stdout 和文件输出可分别配置。
- 文件达到限制后正确轮转、保留和压缩。
- Request-ID 能从 Middleware 传播到 Service 和日志。
- Access Log、Panic 和依赖错误包含要求的结构化字段。
- 日志不包含 Token、Cookie、密码和完整请求体。

### 19.5 工程验证

完成 Go 代码改造后依次执行：

```powershell
task check -- <修改的 Go 文件>
```

该任务依次运行 `gofmt`、`go test ./...`、`go vet ./...` 和 `go build -ldflags="-s -w" ./...`；仅格式化显式传入的文件。未安装 Go Task 时可直接执行等价命令。

模板基础能力发布前，应分别验证 MySQL 8 和受支持的 PostgreSQL 版本都能完成首次迁移和启动。具体项目完成数据库选型后，后续迁移只需在所选数据库上执行 Up、Down、再次 Up。新增测试文件仍需遵守任务授权范围；无法执行的集成验证必须在交付说明中明确列出。

## 20. 预期新增依赖

预计需要：

- `gorm.io/driver/postgres`：PostgreSQL GORM Driver。
- `github.com/go-gormigrate/gormigrate/v2`：执行版本化 Go Migration。
- `lumberjack`：日志文件轮转。

引入时应锁定明确版本并执行 `go mod tidy`。不得因日志接口存在而提前增加 slog 或 zap 依赖。

## 21. 完成定义

只有同时满足以下条件，架构吸纳才算完成：

1. 目标目录和目录级规则已经落地。
2. MySQL/PostgreSQL 配置和连接选项可用，Gormigrate 能在项目选定的数据库上完成迁移。
3. 权限缓存查询、回源、回填和失效路径完整。
4. 模块已迁移且旧结构已清理。
5. stdout、文件日志和轮转可配置且受生命周期管理。
6. README、配置示例、架构文档和 `AGENTS.md` 与实现一致。
7. 格式化、测试、静态检查和构建完成，未验证范围已明确记录。

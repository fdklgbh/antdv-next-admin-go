# go-gin-template

基于 Go 1.26 和 Gin 的模块化单体服务模板，支持 MySQL/PostgreSQL、GORM、Redis、版本化 Go Migration、动态权限缓存、统一错误响应、zerolog 和优雅退出。

## 架构

```text
cmd/api                    API 入口
cmd/migrate                独立迁移命令
migrations                 Gormigrate 冻结 Schema 与版本注册
internal/app               依赖组装和生命周期
internal/config            严格 YAML 配置
internal/infrastructure    Database、Redis、Cache、Logger
internal/modules           Auth、Account、Admin、Health
internal/httpserver        Router 与 Middleware
internal/shared            错误、响应、校验、分页、请求上下文
```

App 手动组装依赖。数据库、Redis 和日志 Writer 各创建一次并按逆序关闭。Gin 只出现在 Router、Middleware 和 Handler；Service 使用 `context.Context`。

后续项目如需移除 Redis 或改用内存缓存，参见 [Redis 移除与内存缓存切换指南](redis-switching-guide.md)，包含按文件修改步骤、会话处理、迁移和验证清单。

## 配置与启动

配置使用 Viper `UnmarshalExact` 严格解码并通过 validator 校验。未知字段、缺失值、非法范围、数据库或 Redis 连接失败都会中止启动。

关系型数据库由 `database.driver` 选择：

```yaml
database:
  driver: mysql
  dsn: "app:password@tcp(127.0.0.1:3306)/app?charset=utf8mb4&parseTime=True&loc=Local"
```

PostgreSQL DSN 示例：

```yaml
database:
  driver: postgres
  dsn: "host=127.0.0.1 user=app password=password dbname=app port=5432 sslmode=disable TimeZone=Asia/Shanghai"
```

使用 Go Task v3 执行常用命令，`task` 或 `task --list` 可查看任务列表。先迁移，再启动 API：

```powershell
task migrate:up
task run
```

默认读取 `configs/config.yaml`，启动和迁移可通过 `CONFIG` 指定配置，例如 `task run CONFIG=configs/config.local.yaml`、`task migrate:up CONFIG=configs/config.local.yaml`。路径包含空格时将整个参数加引号，例如 `task run "CONFIG=configs/local config.yaml"`。

回滚最后一个版本使用 `task migrate:down`，可能删除该版本创建的表和数据。API 启动不会自动修改 Schema。新迁移骨架通过 `task migrate:new NAME=add_user_status` 生成，随后必须填写、在 `migrations/migrations.go` 中注册和人工审查。项目选定数据库后只维护一条迁移历史。

## 路由

```text
GET /health/live
GET /health/ready
GET /api/me
GET /api/admin/ping
```

所有 JSON 响应包含 `code`、`message`、`data`、`requestId` 和 Unix 毫秒 `timestamp`。认证使用 `Authorization: Bearer <token>`，会话 Key 为 `<app>:auth:session:<token_sha256>`。

权限使用 HTTP Method 与 Gin 路由模板动态匹配。Auth Service 先读取 `<app>:auth:permission_routes:<user_id>`；只有 `cache.ErrMiss` 才查询数据库并回填，Redis 错误和损坏数据会返回 503。空权限集合也缓存。权限写操作提交数据库事务后删除所有受影响用户缓存；删除失败会返回明确错误，短 TTL 是最终一致性的安全边界。

路由映射变更和删除权限码会在同一数据库事务中保存待失效用户，Redis 删除成功后再清理对应记录。失败或进程中断后，重复调用同一路由或权限码的操作可继续处理遗留记录；没有后台重试任务，未重试的记录会保留。此机制不消除并发回填旧缓存的窗口。

所有权限写事务使用 Read Committed，并先锁定 `permission_write_locks` 的 `id=1` 记录，再读取最新权限关系和采集受影响用户。这会在数据库中串行化各实例的权限写入，避免并发路由修改或授权与删除交错时漏掉用户。锁在提交或回滚时释放；权限查询与提交后的缓存删除不持有该锁。

模板仅提供一个初始迁移 `000001_init_schema`，一次创建 `user_permissions`、`route_permissions`、`permission_cache_invalidations` 和 `permission_write_locks` 四张表，并写入 `id=1` 的固定锁记录。启动 API 前执行迁移命令；回滚初始迁移会删除上述四张表。

## 业务码与 HTTP 状态码

HTTP 状态码表达请求结果类别，JSON 中的 `code` 表达具体业务原因，两者通过 `AppError` 单独指定，不从业务码推导 HTTP 状态码。成功业务码为 `0`，用于 HTTP 200 和 201；HTTP 204 不返回响应体。客户端按 `code` 判断业务分支，不依赖 `message` 文本。

错误业务码采用五位 `MMNNN`：前两位是模块编号，后三位是模块内错误编号（001–999）。模块分配表如下；新增模块先登记未占用的编号，不复用已有码段。

| 模块 | 码段 | 定义位置 | 导入别名 |
|---|---|---|---|
| 通用 | 10001–10999 | `internal/shared/errcode` | `commoncode` |
| Auth | 20001–20999 | `internal/modules/auth/errcode` | `authcode` |
| Account | 30001–30999 | 预留，按需创建 `internal/modules/account/errcode` | `accountcode` |
| Admin | 40001–40999 | 预留，按需创建 `internal/modules/admin/errcode` | `admincode` |

常量名称只表达错误含义，不重复模块名或添加 `Code` 前缀，例如 `authcode.Unauthenticated`。模块内需要区分业务对象时保留对象名。每个业务码显式赋值，不使用 `iota`；发布后不得修改含义、重新编号或将废弃码分配给其他错误。只为实际使用的错误创建常量和码包。

`errcode` 包只定义整数常量，不依赖 HTTP、Service 或 Repository。通用码属于共享包，模块专属码由对应模块维护；`response` 负责统一输出，不集中定义模块业务码。

调用方按错误归属直接引用：通用错误使用 `commoncode.ValidationFailed`、`commoncode.InternalError`，认证专属错误使用 `authcode.Unauthenticated`、`authcode.PermissionDenied`。同一文件需要两类错误时，分别导入两个码包。

模块码包不通过常量别名或代码生成重新导出通用码，也不使用结构体嵌入、变量或访问方法聚合两类业务码。这样业务码始终是 Go 常量，新增通用码只需更新共享码包和本表，无需同步各模块；实际使用处直接引用即可。

### 当前业务码

| 常量 | 业务码 | HTTP 状态码 | 含义 |
|---|---|---|---|
| `commoncode.OK` | 0 | 200 / 201 | 成功 / 创建成功 |
| `commoncode.ValidationFailed` | 10001 | 400 | 参数校验失败 |
| `commoncode.NotFound` | 10002 | 404 | 资源或路由不存在 |
| `commoncode.MethodNotAllowed` | 10003 | 405 | 请求方法不支持 |
| `commoncode.InternalError` | 10004 | 500 | 内部错误 |
| `commoncode.ServiceUnavailable` | 10005 | 503 | 依赖服务不可用 |
| `authcode.Unauthenticated` | 20001 | 401 | 未认证、Token 无效或会话不存在 |
| `authcode.PermissionDenied` | 20002 | 403 | 无访问权限 |
| `authcode.ServiceUnavailable` | 20003 | 503 | 认证或权限服务异常 |

调用示例：

```go
import (
    "net/http"

    authcode "go-gin-template/internal/modules/auth/errcode"
    apperrors "go-gin-template/internal/shared/errors"
    "go-gin-template/internal/shared/response"
)

// Handler 或 Middleware 中通过统一响应输出。
response.Abort(c, apperrors.New(
    authcode.Unauthenticated,
    http.StatusUnauthorized,
    "authentication required",
    err,
))
```

`response.Error` 通过 `errors.As` 识别包装后的 `AppError`，普通错误返回 HTTP 500 和业务码 `10004`。底层 `Err` 不写入响应，错误响应的 `data` 为 `null`；参数校验响应可通过 `ValidationError` 携带错误详情。

### 旧业务码迁移

此次规范化调整了现有业务码，HTTP 状态码保持不变。客户端需要同步替换旧码；不提供旧码兼容，原 `response.Code*` 常量也已移到对应 `errcode` 包。

| 错误 | 旧业务码 | 新业务码 |
|---|---|---|
| 参数校验失败 | 40001 | 10001 |
| 资源或路由不存在 | 404 | 10002 |
| 请求方法不支持 | 405 | 10003 |
| 内部错误 | 500 | 10004 |
| 依赖服务不可用 | 503 | 10005 |
| 未认证 | 401 | 20001 |
| 无权限 | 403 | 20002 |
| 认证或权限服务异常 | 50301 | 20003 |

旧参数校验码 `40001` 已退役，虽处于 Admin 预留范围内，也不得重新分配。

## 日志

业务代码依赖项目 Logger Interface。zerolog Adapter 自动从 Context 绑定 `request_id`、`user_id` 和 `trace_id`，支持 stdout 与 lumberjack 轮转文件独立开关。文件启用时，目录创建或可写性检查失败会中止启动。

访问日志包含 Method、Route、Status、Latency、Client IP、User Agent，以及认证后可用的 User ID。禁止记录 Token、Cookie、密码、验证码和完整请求体。

## 验证

按顺序格式化本次修改的文件、运行测试、静态检查和构建：

```powershell
task check -- internal/shared/response/response.go internal/httpserver/router.go
```

将示例路径替换为实际修改的 Go 文件。`task fmt` 和 `task check` 必须在 `--` 后提供文件列表，不会默认格式化整个项目；任一步失败会停止后续步骤。

| 任务 | 对应命令 |
|---|---|
| `task fmt -- <文件列表>` | `gofmt -w <文件列表>` |
| `task test` | `go test ./...` |
| `task vet` | `go vet ./...` |
| `task build` | `go build -ldflags="-s -w" ./...` |
| `task check -- <文件列表>` | 依次执行以上四项 |
| `task run` | `go run ./cmd/api -config <CONFIG>` |
| `task migrate:up` | `go run ./cmd/migrate -config <CONFIG> -action up` |
| `task migrate:down` | `go run ./cmd/migrate -config <CONFIG> -action down` |
| `task migrate:new NAME=<名称>` | `go run ./tools/migrationgen -name <名称>` |

未安装 Go Task 时，可直接执行表中对应命令。`build` 使用 `-ldflags="-s -w"`，链接时移除符号表和 DWARF 调试信息；`task check` 的构建步骤使用相同参数。当前 `build` 仅验证所有包能否编译，不输出发布二进制。

发布模板基础能力前，应分别在 MySQL 8 和受支持的 PostgreSQL 上验证首次 Up、Down、再次 Up。项目完成数据库选型后，后续迁移只需验证所选数据库。

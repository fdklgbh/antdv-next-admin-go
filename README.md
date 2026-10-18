# antdv-next-admin-go

基于 Go 1.27.0 和 Gin 的模块化单体服务模板，支持 MySQL/PostgreSQL、GORM、内存或 Redis 缓存、版本化 Go Migration、动态权限缓存、统一错误响应、zerolog 和优雅退出。

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

App 手动组装依赖。数据库、选定的缓存后端和日志 Writer 各创建一次并按逆序关闭；只有启用 Redis 时才连接 Redis。Gin 只出现在 Router、Middleware 和 Handler；Service 使用 `context.Context`。

缓存后端通过 `redis.enabled` 选择：默认关闭并使用内存缓存；设为 `true` 后使用 Redis。完整移除 Redis 的裁剪步骤仍见 [Redis 移除与内存缓存切换指南](redis-switching-guide.md)。

## 配置与启动

配置使用 Viper `UnmarshalExact` 严格解码并通过 validator 校验。未知字段、缺失值、非法范围、数据库连接失败，或启用 Redis 时的 Redis 连接失败都会中止启动。

默认使用进程内存缓存，不需要 Redis 服务。需要多实例共享缓存时，在配置中启用 Redis：

```yaml
redis:
  enabled: true
  keyPrefix: my-service
  address: "127.0.0.1:6379"
  username: ""
  password: ""
  database: 0
  poolSize: 20
```

Redis 启用后连接失败会中止启动，不会静默切换为内存缓存。内存缓存仅在当前进程内共享，服务重启会清空 refresh session；多实例部署应启用 Redis。

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

`configs/config.yaml` 仅作为模板；启动、迁移和创建账号默认读取 `configs/config.dev.yaml`。首次使用需从模板复制并填写开发环境连接信息。该开发配置已加入 Git 忽略，启动和迁移可通过 `CONFIG` 指定配置，例如 `task run CONFIG=configs/config.local.yaml`、`task migrate:up CONFIG=configs/config.local.yaml`。路径包含空格时将整个参数加引号，例如 `task run "CONFIG=configs/local config.yaml"`。

回滚最后一个版本使用 `task migrate:down`，可能删除该版本创建的表和数据。API 启动不会自动修改 Schema。新迁移骨架通过 `task migrate:new NAME=add_user_status` 生成，随后必须填写、在 `migrations/migrations.go` 中注册和人工审查。项目选定数据库后只维护一条迁移历史。

## 路由

```text
GET /health/live
GET /health/ready
POST /api/auth/login
POST /api/auth/refresh
POST /api/auth/logout
GET /api/auth/info
GET /api/me
GET /api/admin/ping
```

所有 JSON 响应包含 `code`、`message`、`data`、`requestId` 和 Unix 毫秒 `timestamp`。认证使用 `Authorization: Bearer <Access JWT>`，JWT 固定有效 15 分钟，校验 HS256 签名、issuer、audience 和有效期，不查询撤销状态。启用 Redis 时，Key 前缀由 `redis.keyPrefix` 配置，与 `app.name` 无关；刷新会话 Key 为 `<keyPrefix>:auth:refresh:<refresh_token_sha256>`。内存模式不使用 Redis 前缀。

权限使用 HTTP Method 与 Gin 路由模板动态匹配。Auth Service 先读取 `<keyPrefix>:auth:permission_routes:<user_id>`；只有 `cache.ErrMiss` 才查询数据库并回填，缓存后端错误和损坏数据会返回 503。空权限集合也缓存。权限写操作提交数据库事务后删除所有受影响用户缓存；删除失败会返回明确错误，短 TTL 是最终一致性的安全边界。

路由映射变更和删除权限码会在同一数据库事务中保存待失效用户，缓存删除成功后再清理对应记录。失败或进程中断后，重复调用同一路由或权限码的操作可继续处理遗留记录；没有后台重试任务，未重试的记录会保留。此机制不消除并发回填旧缓存的窗口。

所有权限写事务使用 Read Committed，并先锁定 `permission_write_locks` 的 `id=1` 记录，再读取最新权限关系和采集受影响用户。这会在数据库中串行化各实例的权限写入，避免并发路由修改或授权与删除交错时漏掉用户。锁在提交或回滚时释放；权限查询与提交后的缓存删除不持有该锁。

模板初始迁移 `000001_init_schema`，一次创建 `user_permissions`、`route_permissions`、`permission_cache_invalidations` 和 `permission_write_locks` 四张表，并写入 `id=1` 的固定锁记录。启动 API 前执行迁移命令；回滚初始迁移会删除上述四张表。

## 业务码与 HTTP 状态码

HTTP 状态码表达请求结果类别，JSON 中的 `code` 表达具体业务原因，两者通过 `AppError` 单独指定，不从业务码推导 HTTP 状态码。成功业务码为 `0`，用于 HTTP 200 和 201；HTTP 204 不返回响应体。客户端按 `code` 判断业务分支，不依赖 `message` 文本。后端 `message` 统一使用中文；前端优先按业务码显示当前语言文案，未知业务码以中文 `message` 兜底。

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
| `commoncode.ResourceConflict` | 10006 | 409 | 资源重复或仍被引用 |
| `commoncode.ResourceProtected` | 10007 | 403 | 受保护的资源不允许操作 |
| `authcode.Unauthenticated` | 20001 | 401 | 未认证、Token 无效或会话不存在 |
| `authcode.PermissionDenied` | 20002 | 403 | 无访问权限 |
| `authcode.ServiceUnavailable` | 20003 | 503 | 认证或权限服务异常 |
| `authcode.InvalidCredentials` | 20004 | 401 | 用户名或密码错误，或账号不可用 |
| `authcode.OriginDenied` | 20005 | 403 | 请求来源不被允许 |

调用示例：

```go
import (
    "net/http"

    authcode "antdv-next-admin-go/internal/modules/auth/errcode"
    apperrors "antdv-next-admin-go/internal/shared/errors"
    "antdv-next-admin-go/internal/shared/response"
)

// Handler 或 Middleware 中通过统一响应输出。
response.Abort(c, apperrors.New(
    authcode.Unauthenticated,
    http.StatusUnauthorized,
    "登录状态已失效，请重新登录",
    err,
))
```

`response.Error` 通过 `errors.As` 识别包装后的 `AppError`，普通错误返回 HTTP 500 和业务码 `10004`。底层 `Err` 不写入响应，错误响应的 `data` 为 `null`；参数校验响应可通过 `ValidationError` 携带错误详情。

### 接口提示多语言约定

后端统一提供中文 `message`，前端根据稳定业务码从 `zh-CN`、`en-US`、`ja-JP`、`ko-KR` 语言包选择提示；未知业务码使用返回的中文 `message`。响应结构保持不变，不额外引入 `errorCode` 或 `messageParams`，后端无需语言包或 `Accept-Language` 处理。

新增业务错误时，应先选择通用或模块业务码、明确 HTTP 状态及中文文案，再同步本页业务码表和前端 `apiErrors.codes`。不得通过匹配中文文案判断业务行为。资源冲突和受保护资源分别使用 `10006`、`10007`，不再共用参数校验码 `10001`；登录凭据错误使用 `20004`，请求来源被拒使用 `20005`。

成功响应的 `message` 为“操作成功”，前端成功提示使用页面自身的语言包。用户填写的内容、字典名称等业务数据不属于此次接口提示翻译范围。

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
| `task fmt:all` | 使用 `goimports` 整理 import，再用 `gofmt -s` 格式化并简化所有 Go 文件 |
| `task test` | `go test ./...` |
| `task vet` | `go vet ./...` |
| `task build` | `go build -ldflags="-s -w" ./...` |
| `task check -- <文件列表>` | 依次执行以上四项 |
| `task run` | `go run ./cmd/api -config <CONFIG>` |
| `task migrate:up` | `go run ./cmd/migrate -config <CONFIG> -action up` |
| `task migrate:down` | `go run ./cmd/migrate -config <CONFIG> -action down` |
| `task migrate:new NAME=<名称>` | `go run ./tools/migrationgen -name <名称>` |

未安装 Go Task 时，可直接执行表中对应命令。`task fmt:all` 会处理整个项目，并通过固定版本的 `goimports` 自动增删和排序 import；日常修改优先使用限定文件列表的 `task fmt` 或 `task check`。`build` 使用 `-ldflags="-s -w"`，链接时移除符号表和 DWARF 调试信息；`task check` 的构建步骤使用相同参数。当前 `build` 仅验证所有包能否编译，不输出发布二进制。

发布模板基础能力前，应分别在 MySQL 8 和受支持的 PostgreSQL 上验证首次 Up、Down、再次 Up。项目完成数据库选型后，后续迁移只需验证所选数据库。

## 登录与刷新会话

新增迁移 `000002_auth_users` 创建 `auth_users`（用户名、bcrypt 密码哈希、显示名、启用状态和时间戳）。API 不自动执行表结构迁移。超级管理员在首次初始化时由 `auth.superAdminUsername` 指定并按需创建，不预置共享密码。`000003_roles` 增加默认角色及前端权限；`/auth/info` 返回角色对象和直接授权、角色授权合并后的权限。

启动前设置 `AUTH_JWT_SECRET` 为至少 32 字符的高熵随机密钥，所有实例使用同一个值；不要提交密钥。配置文件中的 `jwtSecret` 留空，由该环境变量提供。配置 `jwtIssuer`、`jwtAudience` 和 `allowedOrigins`；生产 HTTPS 将 `cookieSecure` 设置为 true。默认支持前端同源代理部署，模板不自动开放跨域 CORS。

准备数据库后执行 `task migrate:up`，再运行 `task user:create USERNAME=用户名`。命令从标准输入读取一行密码（8–72 字节），也可执行 `go run ./cmd/create-user -username 用户名`；密码不放在命令行参数中。新用户没有权限，按业务需要通过现有 Auth Service 分配权限，不直接绕过权限写事务。

登录请求：`{ "username": "...", "password": "...", "remember": true, "rememberDays": 7 }`。勾选记住登录时仅允许 7/15/30 天；未勾选时省略 rememberDays。登录和刷新成功的 data 为 `{ "token": "Access JWT", "expiresIn": 900, "sessionId": "非秘密会话标识", "remember": true }`。

Refresh Token 是随机字符串，仅通过 HttpOnly、SameSite=Lax、Path=/api/auth 的 Cookie 下发，缓存 Key 只包含其 SHA-256。未记住登录使用会话 Cookie，服务端最长 24 小时；记住登录按所选天数设置 Cookie 和缓存有效期。期限从登录时计算，刷新不更新 Cookie、不轮换凭证、不延长会话，不实现原子轮换或重复刷新窗口。多标签页可同时使用固定刷新凭证获取各自的新 JWT。内存模式下 refresh session 随进程退出丢失，因此重启后用户需重新登录。

退出成功后删除刷新会话并清除 Cookie；重复退出成功，启用 Redis 时 Redis 故障会明确返回 503，不伪造撤销成功。已签发的 JWT 在 15 分钟内仍可能有效，包括并发退出时已经开始签发的 JWT。后续项目可自行增加即时撤销、Token 轮换、登录限流和角色管理接口。

登录凭据错误使用 HTTP 401 / code 20004，其他认证失败使用 HTTP 401 / code 20001，权限错误为 HTTP 403 / code 20002，认证依赖故障为 HTTP 503 / code 20003，参数错误为 HTTP 400 / code 10001。刷新失败不清除 Cookie，避免旧请求的失败覆盖其他标签页新登录的 Cookie。

前端受保护请求未认证时尝试刷新，原请求最多重试一次。刷新遇到网络错误、超时、503 或其他非未认证异常时，保留登录态并传播错误；刷新明确未认证或重试仍未认证时，清理登录态并按跳转配置返回登录页。登录和刷新接口本身不递归刷新。后端必须区分认证失效和依赖故障，避免因临时服务故障导致用户被退出。

前端 `tests/unit/request-service.spec.ts` 覆盖上述刷新失败分类；使用 `pnpm run test:unit:run` 执行。后端错误协议有变更时应同步该回归约定。


### 超级管理员和初始角色

先运行 `task migrate:up` 创建角色和权限表，再运行 `task run`。`auth.superAdminUsername` 必填，模板默认 `superadmin`，所有实例必须一致。首次启动若账号不存在，自动创建启用账号并生成 256 位随机密码；仅在交互终端显示一次，不经应用日志、HTTP 接口或配置文件输出，数据库只保存 bcrypt 哈希。首次创建必须在交互终端启动，后续启动无需终端。请当场保存密码；事务提交后进程异常退出或终端输出失败无法再次展示，需人工重置密码。

首次初始化时，账号已存在则保留密码、邮箱和启用状态；禁用账号或用户名大小写不匹配时报错。成功后在 `auth_bootstrap` 保存完成标记，后续启动完全跳过账号和角色初始化，修改配置不会更新数据库、创建新账号或转移超级管理员身份。身份由数据库 `super_admin` 角色关联决定，即使账号被删除也不会自动重建。不自动创建普通角色对应的演示账号。

新增迁移 `000003_roles` 冻结前端当前 38 项权限定义，创建 `auth_roles`、`auth_permissions`、`auth_user_roles`、`auth_role_permissions`。默认角色只在迁移时初始化，不在每次启动覆盖用户修改：

| 角色 | 初始权限 |
| --- | --- |
| super_admin | 数据库中具有此角色的账号；前端返回 `*`，后端跳过路由权限检查 |
| admin | 当前全部 38 项前端权限，新增权限不自动授予 |
| manager | 仪表盘、权限管理入口、用户查看/新增/编辑/删除、角色查看 |
| user | 仪表盘、权限管理入口、用户查看 |
| guest | 仪表盘 |

普通用户有效权限为直接授权与角色权限的并集；普通角色或直接授权的 `*` 不代表超级管理员。接口鉴权仍要求数据库中的启用路由权限映射，超级管理员除外。禁用的示例菜单保留原始状态。`/api/auth/info` 返回前端 `Role[]` 结构及有效权限。用户、角色、菜单管理接口已实现；角色分配和角色权限变更通过权限写事务提交，并清理权限缓存。


### 用户显示名称和邮箱

用户字段统一为 `displayName`（数据库 `display_name`），邮箱为 `email`（最多 254 字符，可为空，不要求唯一，不作为登录标识，也不代表已验证）。`auth.superAdminEmail` 为必填的首次初始化邮箱，模板示例 `superadmin@example.com` 请修改为实际地址；仅在首次创建账号时写入，已有账号不覆盖，后续启动不再同步配置。普通用户创建支持 `go run ./cmd/create-user -username 用户名 -email user@example.com`，邮箱可选，密码仍从标准输入读取。

按本次要求重新生成初始迁移：`000002_auth_users` 包含显示名和邮箱，`000003_roles` 包含初始化完成标记表。适用于全新数据库；已执行旧版本迁移的数据库不会自动更新，需另行迁移或在明确确认后重建开发库，不要直接在已有数据库上假定重复 migrate:up 会更新旧表。


### 管理接口

迁移 000004_management 增加用户 phone/gender/bio、系统配置、字典、文件和审计表，以及管理接口的数据库路由权限映射。执行 task migrate:up 后重启 API。迁移 000005_config_groups 增加系统配置分组查询接口的路由权限映射；迁移 000004_management 创建配置分组表并初始化基础、安全、上传、通知四个分组。

- 用户：GET/POST /api/users，GET/PUT/DELETE /api/users/:id；创建必须提供 8–72 字节密码，编辑不接受密码字段；POST /api/users/change-password 校验旧密码。PUT 使用完整用户表单，roles 传角色对象数组。用户名不可修改，不允许删除超级管理员或禁用/降权超级管理员；仅超级管理员能授予 super_admin。密码修改不会额外撤销已有 Refresh 会话，JWT 仍按原有效期处理。
- 角色：GET/POST /api/roles，GET/PUT/DELETE /api/roles/:id；GET /api/roles/options 供用户表单使用。权限传 permissions 对象数组，以服务端权限码为准。角色码不可修改，super_admin 不可编辑，内置角色不可删除；已分配的自定义角色需先解除关联。
- 菜单/权限：GET/POST /api/permissions，GET /api/permissions/tree，GET/PUT/DELETE /api/permissions/:id。支持父子树、过滤、排序，拒绝循环父子关系；有子节点不能删除，删除权限同时移除用户、角色及接口映射中的授权。权限码不可修改。
- 系统配置：GET /api/config/list、GET /api/config/groups、GET /api/config/key/:key、POST /api/config、PUT/DELETE /api/config/:id。新数据库迁移完成后包含基础、安全、上传、通知四个分组；`/api/config/groups` 从配置分组表返回分组及对应配置数量，供前端动态展示；值必须按字符串传输，服务端按 valueType 校验 string/number/boolean/json；内置项不可删除。此处为业务配置，不会修改启动 YAML。
- 字典：GET /api/dict/types、GET /api/dict/type/list、POST /api/dict/type、PUT/DELETE /api/dict/type/:id；GET /api/dict/data/list、GET /api/dict/data/:typeCode、POST /api/dict/data、PUT/DELETE /api/dict/data/:id。有数据的类型不能删除，同类型值不允许重复。GET /api/dict/data/all 供所有登录用户读取启用类型下的启用项，其余字典管理接口需要权限。
- 文件：GET /api/file/list、POST /api/file/upload（multipart file，非空且最多 10 MB）、GET /api/file/:id/download、DELETE /api/file/:id。当前内容存储在数据库，storage=database，下载强制附件；删除同时删除内容和元数据。未接入 OSS/COS。
- 日志：GET /api/log/operation/list、GET /api/log/login/list，DELETE /api/log/operation/clear、DELETE /api/log/login/clear。记录实际登录结果和已认证写操作，不记录请求体、密码、Token 或查询参数。审计在响应之后写入独立数据库表，写入失败记录 audit_persist_failed，不回滚已完成业务。清空日志只清对应种类，不记录清空操作自身。

用户和角色分页为 current/pageSize，系统管理分页为 page/pageSize。管理写操作保留失败传播，权限失效任务在事务内保存；Redis 失败可造成业务已提交但接口返回失败。当前无后台失效重试，缓存仍有 TTL。

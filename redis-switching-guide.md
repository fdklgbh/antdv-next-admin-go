# Redis 移除与内存缓存切换指南

本文供基于本模板创建的后续项目裁剪基础设施使用，覆盖完全移除 Redis，以及将项目固定为进程内缓存两条路径。当前模板已支持运行时选择内存或 Redis 缓存，日常切换请使用 `redis.enabled`，无需执行本文的裁剪步骤。

默认 `redis.enabled: false` 使用进程内存 Store；设为 `true` 时连接 Redis。启用 Redis 后连接失败会中止启动，不会静默降级。内存 Store 有 100,000 条目上限，进程重启会清空 refresh session，不适合多实例共享会话。

## 1. 先选择目标方案

| 方案 | 登录会话 | 权限查询 | 部署约束 | 建议 |
|---|---|---|---|---|
| A：完全不用缓存 | 数据库持久化会话 | 每次查询数据库 | 可部署多个实例，使用同一权限数据库 | 小型项目优先采用，结构最简单 |
| B：只缓存权限 | 数据库持久化会话 | 内存 Cache-Aside | 本文实现范围限单实例 | 需要缓存且希望重启保留登录状态时采用 |
| C：会话和权限都在内存 | 当前进程内保存会话 | 内存 Cache-Aside | 单实例；重启会丢失会话 | 适合接受重启后重新登录的项目 |
| 保留 Redis | Redis 保存会话 | Redis Cache-Aside | 可部署多个实例，共享同一命名空间 | 需要共享会话和缓存时采用 |

“单实例”指同一时刻只有一个服务进程，包括发布期间；两个副本或滚动更新中的新旧进程会各有一份内存。粘性会话不能解决跨实例撤权与缓存失效问题。

若后续项目已经有认证系统，可接入其最小会话查询契约，替代本文的数据库会话方案。不要因删除 Redis 而直接移除登录校验和权限中间件。

## 2. 当前代码修改地图

| 文件 | 当前作用 | 切换时需要处理的内容 |
|---|---|---|
| [internal/config/config.go](internal/config/config.go) | 严格配置入口 | `redis.enabled` 控制 Redis 配置的条件校验 |
| [internal/config/redis.go](internal/config/redis.go) | Redis 连接配置 | Redis 未启用时连接参数可留空 |
| [internal/config/auth.go](internal/config/auth.go) | 权限缓存 TTL | 无缓存时移除；内存缓存时保留 |
| [configs/config.yaml](configs/config.yaml) | 配置示例 | 同步移除 Redis 段，按方案处理 Auth 段 |
| [internal/app/dependencies.go](internal/app/dependencies.go) | 连接与模块组装 | 默认创建内存 Store；按开关创建并关闭 Redis Client |
| [internal/infrastructure/redis/client.go](internal/infrastructure/redis/client.go) | Redis Client | 无调用方后删除 |
| [internal/infrastructure/cache/store.go](internal/infrastructure/cache/store.go) | 通用字节 Store 接口 | 内存方案复用；彻底无缓存时评估删除 |
| [internal/infrastructure/cache/redis.go](internal/infrastructure/cache/redis.go) | Redis Store | 保留为可选实现；内存实现位于 `memory.go` |
| [internal/modules/auth/module.go](internal/modules/auth/module.go) | Auth 组装 | 分别注入会话存储与权限缓存 |
| [internal/modules/auth/cache/cache.go](internal/modules/auth/cache/cache.go) | 会话与权限缓存接口 | 按新会话方案调整接口和错误契约 |
| [internal/modules/auth/cache/store.go](internal/modules/auth/cache/store.go) | 通用 Store 上的 JSON 编解码、Schema 校验和批量删除 | 内存和 Redis 方案共用；无缓存方案拆除权限缓存部分 |
| [internal/modules/auth/redis_key.go](internal/modules/auth/redis_key.go) | Token 摘要与 Key 构造 | 内存方案可保留 Key 格式；数据库会话仍需要 Token 摘要 |
| [internal/modules/auth/service/service.go](internal/modules/auth/service/service.go) | 认证、权限判断和失效处理 | 无缓存时直接查库；内存时保留权限 Cache-Aside |
| [internal/modules/auth/repository/repository.go](internal/modules/auth/repository/repository.go) | 权限读写、待失效记录 | 无缓存时移除失效记录写入、读取和确认 |
| [internal/modules/auth/repository/transaction.go](internal/modules/auth/repository/transaction.go) | 权限写事务锁 | 本指南默认保留，不能单独删除锁表 |
| [internal/modules/auth/model/model.go](internal/modules/auth/model/model.go) | 权限与失效记录模型 | 与实际保留的表同步 |
| [internal/modules/health/module.go](internal/modules/health/module.go) | 健康检查组装 | 删除 Redis Pinger 构造参数 |
| [internal/modules/health/handler/handler.go](internal/modules/health/handler/handler.go) | Readiness 检查 | 删除 Redis 字段、接口及 Ping |
| [migrations/v000001_init_schema.go](migrations/v000001_init_schema.go) | 模板初始 Schema | 按项目是否已发布选择重建初始迁移或追加版本 |

Auth 的 `cache.StoreCache` 依赖通用 `cache.Store`，不调用 Redis Client；内存与 Redis 共用同一业务缓存适配器。

## 3. 共同准备：明确会话从哪里来

当前模板已有登录、刷新和退出流程；refresh session 存在选定的 Cache Store 中，用户认证与权限数据仍来自数据库。内存模式下 session 仅在当前进程内有效；需要持久化会话的后续项目可按方案 A/B 改为数据库会话。

### 3.1 数据库会话：方案 A、B

建议在 Auth 模块新增会话 Model、Repository 方法及版本化迁移，至少保存：

| 字段 | 用途 |
|---|---|
| `token_hash` | 随机 Token 的 SHA-256 摘要，唯一索引或主键；不保存原始 Token |
| `user_id` | 当前会话所属用户 |
| `expires_at` | 明确的会话到期时间 |

登录成功后使用安全随机数生成 Token，保存摘要及有效期；只把原始 Token 返回给客户端。认证时计算摘要，查询有效会话并构造 `model.Principal`。当前 Principal 要求 `userId > 0` 且 `username` 非空，用户名应从项目用户数据查询，或由项目明确选择保存会话快照；当前模板并没有用户表可直接复用。

退出登录应删除会话或标记撤销；过期记录不得继续认证。更新会话存储接口时，可命名为 `SessionStore`，并将不存在、已过期和已撤销统一返回模块级 `ErrUnauthenticated`。同步修改 `Service.Authenticate`，不再依赖 Redis/缓存的 Miss 错误表达会话失效。

数据库连接、查询等错误必须保留错误链并传给 HTTP 边界，沿用认证服务异常的 503 响应；不能把数据库故障当成无效登录的 401。会话过期可以在查询中按时间过滤，无需为了本次切换引入后台任务。

### 3.2 内存会话：方案 C

由当前服务的登录流程把 Principal 写入同一个进程内 Store，并设置会话 TTL；退出时删除会话 Key。当前 `SessionCache` 没有写入和删除方法，需要在模块内补充相应契约，禁止让 Handler 直接拼接 Key 或操作 Store。

不能继续让独立程序、Redis CLI 或另一个服务进程写会话：它们无法写入本进程的内存。进程重启会清空已有会话，用户需要重新登录；达到容量上限时，新会话写入返回 `cache.ErrFull`，不会淘汰仍有效的会话。若不能接受内存会话的生命周期限制，选择数据库会话。

## 4. 方案 A：完全移除 Redis，权限每次查库

按以下顺序修改，避免出现已移除连接但认证仍访问旧缓存的中间状态。

### 4.1 配置与依赖组装

1. 按 §3.1 实现数据库会话，并调整 Auth 构造函数，移除通用 Store 参数。
2. 从 `Config` 和 YAML 同时移除 Redis 配置；`UnmarshalExact` 会拒绝残留的 `redis:` 字段，不要改成宽松解码。
3. 删除 `permissionCacheTTL` 字段。如果 `Auth` 已没有其他配置，同步移除 `Config.Auth`、对应类型和 YAML 的整个 `auth:` 段；若新增了会话配置则保留所需部分，并通过 validator 标签验证。
4. 在 `buildDependencies` 移除 Redis Client 的创建、关闭函数、`redisCachePinger`、`NewRedisStore` 调用及相关 import。保留数据库与日志的生命周期管理。
5. 修改 Health 的 Module 和 Handler 构造函数，移除 `CachePinger` 依赖与 `Ready` 中的缓存 Ping。保持数据库检查和 `/health/live`、`/health/ready` 路由。

不要向旧构造函数传 `nil`，也不要提供一个永远成功的 Pinger 来跳过检查；应直接调整类型和调用方。

### 4.2 权限读取与写入

`Service.Authorize` 保留输入校验，直接调用 `repo.PermissionRoutes(ctx, userID)`，按 `method` 和 Gin 路由模板匹配查询结果。没有配置或没有匹配结果仍拒绝访问；数据库错误仍向 HTTP 边界传播。

无缓存的写入路径应简化为数据库事务完成后返回结果：

| 位置 | 修改内容 |
|---|---|
| Service 的字段和 `New` | 移除 `permissions`、权限 TTL；清理因此失去用途的参数和字段 |
| `Grant`、`Revoke` | 移除提交后的 `invalidate` 调用 |
| `UpsertRoute`、`DeletePermissionCode` | 移除 `invalidatePending`；Repository 改为只返回 `error` |
| Repository | 移除仅用于缓存失效的用户采集、操作 Key、`enqueueInvalidations`、`PendingInvalidations` 和 `CompleteInvalidations` |
| Model | 无其他引用后删除 `PermissionInvalidation` |
| Auth 缓存包 | 删除权限缓存接口与实现；数据库会话使用独立契约，不保留假缓存 |

不要用“Get 永远 Miss，Set/Delete 永远成功”的空实现替代上述修改。它会掩盖权限缓存逻辑仍在运行，还可能持续写入无意义的待失效记录。

默认保留 `permissionTransaction`、`PermissionWriteLock` 与锁表，延续现有权限写入顺序。如果项目确认不再需要这项串行化约束，应另行一起调整四个写入口、事务实现、模型、迁移和目录规则；不能只删表，否则所有权限写入都会失败。

权限从数据库主库逐次查询后，不再存在本地权限缓存陈旧问题；已经执行中的查询和业务请求不会被自动追溯撤销。若自行接入只读副本，还需考虑复制延迟。

### 4.3 清理

无剩余调用方后删除 Redis Client、Redis Store 及不再使用的通用缓存文件。Token 摘要逻辑仍属于认证需要，不能随着 `redis_key.go` 一起误删。

移除直接 Redis 依赖后执行 `go mod tidy`，让工具更新 `go.mod` 和 `go.sum`。同步调整部署配置、Redis 探针和后续项目的目录规则。

## 5. 方案 B、C：用内存替换 Redis

### 5.1 实现通用内存 Store

当前实现位于 `internal/infrastructure/cache/memory.go`，实现现有接口：

```go
type Store interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, ...string) error
}
```

实现使用 Go 标准库中的 `map`、`sync.Mutex` 和 `time`，每项保存值和过期时间，明确以下契约：

| 行为 | 要求 |
|---|---|
| 并发读写 | 所有共享状态受锁保护；过期检查与删除在同一临界区完成，避免删掉并发写入的新值 |
| 到期判断 | `now >= expiresAt` 即过期，读取立即返回 `cache.ErrMiss`，不能等定时清理才生效 |
| 未命中 | 不存在、已过期或已淘汰返回 `cache.ErrMiss` |
| TTL | 通用 Store 中 `0` 表示不过期，`cache.KeepTTL` 表示保留已有到期时间（Key 不存在或已过期时不过期），正值表示有效期，其他负值报错；本项目权限与会话仍使用正 TTL |
| 数据所有权 | 写入和读取都复制 `[]byte`，调用方不能修改缓存持有的数据 |
| 容量 | 最多 100,000 个条目；新增条目达到上限时先清理过期项，仍满则返回 `cache.ErrFull`，不静默淘汰有效 session |
| 删除 | 支持批量删除；Key 不存在时删除仍成功 |
| Context | 传播已经取消或超时的 Context 错误，不能转换成 Miss |
| 清理 | 可用读取时过期删除加写入时清理和容量控制；如采用定时清理，需要可停止的生命周期 |
| 关闭 | 如提供 `Close`，关闭后操作应明确报错；有清理 goroutine 时关闭需等待退出 |

优先选择简单、可说明的容量策略。若拒绝写入导致权限缓存回填失败，现有 Service 会返回错误并由 HTTP 边界输出 503；不要忽略写入错误并假装已缓存。会话也使用内存时，容量策略直接决定是否可能提前退出登录，需要明确记录。

当前模板通过 `redis.enabled` 同时维护 Redis 和 Memory 两种实现；后续项目若永久固定为单一后端，可移除另一种实现及开关。

### 5.2 复用 Auth 的业务缓存层

当前 `authcache.NewStoreCache` 接收通用 Store，负责会话及权限 JSON、Schema 版本、Key 和批量删除；内存与 Redis 共享这些逻辑。

Auth 业务缓存已使用通用 `StoreCache` 命名；Redis Key 前缀只在启用 Redis 时生效，Token 摘要在两种后端中都保留。

业务 Service 仍然只依赖会话及权限缓存契约，不直接访问内存 map。继续执行 Schema 校验，空权限集合仍需回填；缓存损坏不能静默当作 Miss。

### 5.3 App 组装与会话选择

1. App 创建一次内存 Store，注入 Auth 模块；禁止每次请求创建 Store。
2. 方案 B：数据库会话实现作为 `sessions`，内存权限缓存作为 `permissions`。当前 `NewModule` 把同一个缓存对象传入两个位置，需要拆开组装，并按 §3.1 调整认证错误契约。
3. 方案 C：同一个业务缓存对象可继续承担会话读取和权限缓存，但登录写入与退出删除必须使用同一个 Store 实例，见 §3.2。
4. 移除 Redis 连接创建与关闭，将内存 Store 的关闭函数（若有）交给 App，服务停止后按逆序关闭资源。
5. 按方案 A 的配置和 Health 步骤移除 Redis 专属项；保留 `auth.permissionCacheTTL`。本地内存不需要一个伪造的 Redis Ping。
6. 无引用后移除 Redis 基础设施实现与依赖，执行 `go mod tidy`。

本模板仅由 `redis.enabled` 显式选择后端；启用 Redis 后连接失败会中止启动，不会自动创建内存 Store。该约束避免切断共享会话及跨实例失效语义。

### 5.4 保留权限失效链路

内存方案仍然是 Cache-Aside：命中直接判断，只有 Miss 才查询数据库并回填。四类权限写入仍需遵守数据库行锁协议，并在提交后删除本进程缓存。

快速切换时，保留 `permission_cache_invalidations` 表及当前待失效记录流程，以覆盖提交后删除失败或请求取消后的重试。进程重启会清空内存，但不会自动清理数据库里的待失效记录；之后重试操作时，删除不存在的缓存 Key 成功，再确认对应记录。

**这个流程只保证当前进程执行本地失效。** 多实例共享同一张待失效记录表时，某个实例处理并确认记录，不代表其他实例也清除了内存缓存。数据库写锁只协调数据库写入，也不会广播失效事件。

因此，多实例项目应改用数据库直接查权限，保留共享缓存，或单独实现每个实例都能收到并可靠处理的失效协议。Redis Pub/Sub 等无持久化广播本身也不能保证离线实例补齐失效事件。本文不把跨实例本地缓存一致性作为快速替换方案的一部分。

单实例下，普通 Cache-Aside 仍存在并发回填旧权限的窗口。若项目要求严格实时撤权，需要另行设计读写协调，不能通过把 Redis 改为 map 宣称获得强一致性。

## 6. 数据库迁移如何处理

| 表或数据 | 方案 A | 方案 B | 方案 C |
|---|---|---|---|
| `user_permissions`、`route_permissions` | 保留 | 保留 | 保留 |
| `permission_cache_invalidations` | 失效代码全部移除后可删除 | 保留 | 保留 |
| `permission_write_locks` 与 `id=1` 记录 | 默认保留 | 保留 | 保留 |
| 数据库会话表（待实现） | 新增 | 新增 | 不需要 |

### 6.1 新项目，尚未发布或建立迁移历史

可以裁剪 `v000001_init_schema.go`：同步调整冻结结构、Up、Rollback 和固定数据初始化。方案 A 可移除待失效表，增加数据库会话表；方案 B 增加数据库会话表；方案 C 的四张现有表可以保留。

保留 `permissionTransaction` 就必须保留锁表与 `id=1` 的种子记录。使用冻结 Schema，不引用后续会变化的业务 Model；API 启动仍不能自动修改 Schema。

### 6.2 已发布或已有实际业务数据库

不重写初始迁移，使用新的版本增加会话表或移除无用表。可先发布使用新存储路径的代码，确认旧实例已退出且没有旧代码访问相关表，再通过后续迁移删除无用表。删除数据表的 Rollback 即使重建结构，也不能恢复原数据，应在迁移说明中写明。

Redis 会话不会自动搬到数据库或内存。切换前明确选择安排重新登录，或单独实现会话迁移；已有 Redis Token 摘要和有效期的处理需要遵循项目认证策略。内存方案只适用于服务进程能创建或接收新会话的部署方式。

不要为了删除待失效表直接执行初始迁移的 `down`：当前初始迁移的 Rollback 会删除全部四张表。

## 7. 同步项目约定

在实际执行切换的后续项目中，同步调整以下文档，避免后续维护重新引入 Redis：

- 根目录 `AGENTS.md`：将“Redis 必需”“Redis 负责会话和权限缓存”调整为项目选定方案；保留严格配置、统一错误和资源清理约束。
- `internal/modules/AGENTS.md`：若重命名 Key 文件，更新对应规则；权限写锁约束须与保留的实现一致。
- `internal/infrastructure/AGENTS.md`：补充内存生命周期或移除不再适用的 Redis 专属规则。
- `architecture-adoption-plan.md`、README 和 YAML 示例：记录会话来源、部署实例数、缓存失效与重启行为。

本指南提供后续项目裁剪步骤；当前模板默认使用内存 Store，可通过 `redis.enabled` 选择 Redis。

## 8. 验证与交付

### 8.1 先核对引用

以下为在项目根目录执行的只读搜索，用于确认没有遗漏依赖；匹配结果需结合选定方案判断，不要求文档里完全没有 Redis 字样。

```powershell
rg -n 'go-redis|redisinfra|redisCachePinger|CachePinger|NewRedisStore|cfg\.Redis|config\.Redis' internal cmd configs go.mod
rg -n 'SessionCache|GetPrincipal|PermissionCache|permissionCacheTTL|invalidatePending|enqueueInvalidations|PendingInvalidations|CompleteInvalidations' internal configs
rg -n 'PermissionInvalidation|PermissionWriteLock|permission_cache_invalidations|permission_write_locks' internal migrations
```

### 8.2 行为验收

| 场景 | 预期 |
|---|---|
| Redis 未启动且 `redis.enabled: false` | 使用内存 Store 正常启动，Readiness 只检查数据库 |
| Redis 未启动且 `redis.enabled: true` | 启动失败，不静默切换内存 |
| YAML 留有已删除的 Redis 字段 | 严格解码报错；删除残留字段后配置通过 |
| 正常、过期、撤销及不存在的会话 | 分别认证成功或返回 401；数据源故障返回 503 |
| 没有路由权限配置 | 继续返回 403 |
| 方案 A 修改权限 | 事务提交后发起的新主库查询按最新权限判断 |
| 方案 B/C 缓存命中、Miss、空集合、损坏数据 | 分别走命中、回源回填、空集合命中、错误路径 |
| 方案 B/C 权限变更 | 提交后删除所有受影响用户的本地缓存；重试能处理遗留失效记录 |
| 两次并发路由修改；授权与删除并发 | 权限写事务取得同一数据库锁后再采集用户，不能漏失效对象 |
| 内存过期、容量达到上限、并发读写 | 不返回过期值、不无限增长、不出现数据竞争或字节切片污染 |
| 进程重启 | 数据库会话仍可验证，内存会话失效，权限缓存可重新回填 |
| 关闭服务 | 如有内存清理任务，能够停止并释放资源 |

### 8.3 工程检查

实际修改依赖后执行 `go mod tidy`；完成 Go 代码修改后按项目规则执行：

```powershell
task check -- <本次修改的 Go 文件>
```

该任务依次运行 `gofmt`、`go test ./...`、`go vet ./...` 和 `go build -ldflags="-s -w" ./...`；未安装 Go Task 时可直接执行等价命令。

当前模板没有测试文件，`go test` 成功不能替代上述行为验收。新增或修改测试文件仍需按项目授权范围执行。内存并发逻辑在具备相应测试与工具链后，可用 `go test -race` 验证。

涉及迁移时，只在所选数据库的隔离临时实例中执行 Up、Down、再次 Up，检查表结构、会话有效期和锁种子记录。不要在实际业务库上用 Down 验收。交付时写明选定方案、实测场景、尚未验证的范围和会话切换方式。

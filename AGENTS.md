# 项目说明

这是一个基于 Go 1.27.0 和 Gin 的模块化单体后端模板，集成 MySQL/PostgreSQL、GORM、Redis、YAML 配置、版本化迁移、zerolog、认证、动态权限缓存、统一响应和优雅退出。

除技术术语、代码标识符、命令、路径、配置项和日志事件名外，沟通、注释及项目文档默认使用中文。

## 技术栈

- Go 1.27.0
- Gin 1.12
- GORM + MySQL 8/PostgreSQL
- go-redis/v9 + Redis
- Viper + validator/v10
- zerolog JSON 日志
- YAML 配置

不要引入 `slog`。新增依赖前必须说明必要性，并优先使用现有依赖和 Go 标准库。

## 目录职责

```text
cmd/api/                         服务入口
configs/                         YAML 配置
migrations/                      Gormigrate 版本化迁移
internal/app/                    依赖组装和生命周期管理
internal/infrastructure/         数据库、Redis、Cache 和 Logger Adapter
internal/modules/                按业务模块组织 Handler、Service、Repository、Cache、DTO 和 Model
internal/httpserver/router.go    Gin Engine 和全局中间件装配
internal/httpserver/middleware/  认证、权限、访问日志和 Recovery
internal/shared/                 统一错误、响应、校验、分页和请求上下文
```

新增业务模块放在 `internal/modules/<module>`，模块在自己的 `routes.go` 暴露 `RegisterRoutes`，并由顶层 Router 显式注册。

## 架构约定

- 在 `internal/app/app.go` 中使用构造函数手动组装依赖。除非项目复杂度明显增加且用户明确要求，否则不要引入 Wire 或其他依赖注入框架。
- DB 和 Redis Client 在应用启动时各创建一次，通过 Service 或明确的构造参数复用，不要在 Handler 或 Middleware 中重复创建连接。
- 数据库 Driver 选择仅位于 `internal/infrastructure/database`；业务模块不得按方言分支。
- 每个业务模块在本模块的 `redis_key.go` 中集中定义 Redis Key 前缀和构造函数，不要在 Service、Handler 或 Middleware 中散落 Key 字面量。
- Middleware 需要认证或权限能力时依赖 `auth.Service`，不要直接堆叠 DB、Redis Client 等底层依赖。
- Service 接收 `context.Context`，不要依赖 `*gin.Context`。
- 只有存在多个实现、需要替换实现或便于隔离外部系统时才提取 interface；不要为单一实现预先设计 interface。
- 配置由 `internal/config` 使用 Viper 统一加载并严格解码，字段必填、枚举、范围及跨字段约束统一通过 `validator/v10` 标签定义，不要编写字段级 `if` 或 `switch` 校验。YAML 未知字段、必要配置缺失、配置值不符合约束或依赖连接失败时直接返回错误，不要静默降级。
- 不要增加未明确要求的兜底逻辑、兼容分支、缓存或重试。

## HTTP 约定

- 所有响应统一通过 `internal/shared/response` 输出，结构固定为 `code`、`message`、`data`、`requestId`、`timestamp`；`code` 使用整数业务码并允许业务模块自定义，`timestamp` 使用 Unix 毫秒时间戳。
- 响应 `message` 统一使用中文，作为前端未知业务码的兜底；成功文案为“操作成功”。多语言由前端按稳定业务码维护，后端不按 `Accept-Language` 翻译，也不通过修改中文文案区分业务类型。
- 参数无效、资源冲突、受保护资源分别使用 `ValidationFailed`、`ResourceConflict`、`ResourceProtected`；登录凭据错误使用 `authcode.InvalidCredentials`，请求来源被拒使用 `authcode.OriginDenied`，不得混用参数错误或普通权限错误码。新增业务码同步 README，并同步前端四种语言的 `apiErrors.codes` 文案。
- Handler 和 Middleware 不要直接调用 `c.JSON`、`c.AbortWithStatusJSON` 等方法构造业务响应。
- 业务码使用五位 `MMNNN` 并显式赋值，成功为 `0`；模块码段及退役码以 README 为准，不使用 `iota` 或复用已分配的码。HTTP 状态码通过 `AppError` 单独指定，不从业务码推导。
- 通用业务码放在 `internal/shared/errcode`，模块专属码放在 `internal/modules/<module>/errcode`，仅在实际使用时创建。常量不重复模块名或添加 `Code` 前缀，调用处使用 `commoncode`、`authcode` 等导入别名；码包只定义常量，不依赖 HTTP、Service 或 Repository。
- 调用方直接引用 `commoncode.*` 和对应模块的业务码，需要时分别导入两个码包。模块不得通过常量别名或代码生成重新导出通用码，不使用结构体嵌入、变量或访问方法聚合业务码；新增通用码只更新共享码包及文档，无需同步各模块。
- 认证信息通过 Gin Context 保存和读取；业务 Service 不接收 Gin Context。
- 刷新凭据无效或会话不存在返回 HTTP 401 / `authcode.Unauthenticated`；认证依赖故障返回 HTTP 503 / `authcode.ServiceUnavailable`，不得将服务故障伪装为未认证。刷新失败不清除 Cookie。前端仅在明确未认证时退出登录，网络或服务异常应保留登录态。
- 路由权限必须动态查询 `route_permissions` 和 `user_permissions`，使用 `c.Request.Method` 与 `c.FullPath()` 匹配。
- 不要在代码中写死权限码与路由的对应关系。没有路由权限配置时默认拒绝访问。
- 路由级权限由 Middleware 校验，资源归属等数据级权限仍由业务 Service 校验。
- Redis 负责登录会话和权限 Cache-Aside。只有 `cache.ErrMiss` 才能回源数据库；权限变化必须在事务提交后删除所有受影响用户的缓存 Key。

## 日志约定

- 业务代码依赖 `internal/infrastructure/logger.Logger`，不要传递 `*zerolog.Logger`，也不要直接使用 zerolog 链式 API。
- 日志必须使用稳定事件名和结构化字段，不要使用字符串拼接表达字段。
- 请求日志应保留 `request_id`、HTTP Method、Gin Route、Status 和耗时。
- 禁止记录 Token、Cookie、密码、完整请求体及不必要的个人信息。
- 普通运行日志按配置写到 stdout 和/或轮转文件；容器环境默认使用 stdout。
- 审计日志属于业务数据，应使用独立审计表或 Outbox；不要在普通日志 Hook 中同步写数据库。

## 工作规则

- 修改前先阅读目标文件、相关调用方和现有同类实现，遵循当前项目结构。
- 只修改完成当前任务所需的文件，不做无关重构、格式化或依赖升级。
- 禁止修改项目目录以外的文件和文件夹。
- 未经用户明确允许，禁止执行 Git Remote 操作，包括 `git push`、`git pull` 和创建 PR。
- 工作区可能包含用户未提交的改动，不要覆盖、回滚或删除这些改动。
- 禁止使用 `git reset --hard`、`git checkout --` 等破坏性命令，除非用户明确要求。
- 所有文本文件使用 UTF-8 编码。
- 对明显不合理或存在安全风险的需求应说明原因并拒绝，除非用户在了解风险后明确强制要求。
- 用户未明确要求时，不新增测试文件；仍需运行现有测试和静态检查。
- 不要添加无意义注释。只在复杂逻辑、边界条件或设计原因无法从代码直接看出时添加简短中文注释。

## 常用命令

```powershell
task --list
task run
task migrate:up
task migrate:new NAME=add_user_status
task fmt -- <修改的 Go 文件>
task test
task vet
task build
task check -- <修改的 Go 文件>
```

常用命令统一维护在 `Taskfile.yml`，任务说明与提示使用中文，保留技术术语、命令和标识符。启动和迁移默认使用 `configs/config.dev.yaml`，可通过 `CONFIG=配置路径` 覆盖。`task migrate:down` 会回滚最后一个版本，可能删除数据，不属于常规验证步骤。

完成 Go 代码修改后，执行 `task check -- <修改的 Go 文件>`，依次运行 `gofmt`、`go test ./...`、`go vet ./...` 和 `go build -ldflags="-s -w" ./...`；也可单独执行各任务。`task build` 默认使用上述链接参数，移除符号表和 DWARF 调试信息。格式化必须显式指定本次修改的文件，不扩大范围。没有 Go Task 时可直接执行等价命令。如果命令无法执行或失败，必须明确说明原因和未验证范围。

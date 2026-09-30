# Go Gin 多模块后端架构建议

本文保留原始架构建议与示例。当前模板的业务码分配、模块 `errcode` 包及命名规则以 [README 的业务码约定](README.md#业务码与-http-状态码) 为准；下文旧码段、`Code*` 常量及错误码示例不作为当前接口契约。

## 1. 项目技术约束

当前技术方向：

- Go
- Gin
- GORM
- Redis
- 多业务模块
- 数据库可切换：
  - MySQL
  - PostgreSQL
  - SQLite

整体目标：

- 业务模块尽量不依赖具体数据库
- Redis 作为基础设施，不直接污染业务逻辑
- HTTP 层、业务层、持久化层职责清晰
- 统一响应结构
- 统一业务错误码
- 支持分页、参数校验、Recover、Request-ID
- 日志能够串起一次完整请求
- 后续增加 MQ、对象存储、定时任务时不需要推翻目录结构

---

# 2. 推荐目录结构

```text
project/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── router.go
│   │   └── dependencies.go
│   │
│   ├── config/
│   │   ├── config.go
│   │   ├── database.go
│   │   ├── redis.go
│   │   └── server.go
│   │
│   ├── infrastructure/
│   │   ├── database/
│   │   │   ├── database.go
│   │   │   ├── dialect.go
│   │   │   ├── transaction.go
│   │   │   └── migrate.go
│   │   │
│   │   ├── redis/
│   │   │   ├── client.go
│   │   │   └── keys.go
│   │   │
│   │   └── logger/
│   │       └── logger.go
│   │
│   ├── middleware/
│   │   ├── request_id.go
│   │   ├── access_log.go
│   │   ├── recovery.go
│   │   ├── auth.go
│   │   └── cors.go
│   │
│   ├── modules/
│   │   ├── user/
│   │   │   ├── handler/
│   │   │   │   └── user.go
│   │   │   ├── service/
│   │   │   │   └── user.go
│   │   │   ├── repository/
│   │   │   │   ├── repository.go
│   │   │   │   └── gorm.go
│   │   │   ├── cache/
│   │   │   │   ├── cache.go
│   │   │   │   └── redis.go
│   │   │   ├── model/
│   │   │   │   └── user.go
│   │   │   ├── dto/
│   │   │   │   ├── request.go
│   │   │   │   └── response.go
│   │   │   ├── errors.go
│   │   │   └── module.go
│   │   │
│   │   ├── auth/
│   │   ├── order/
│   │   └── product/
│   │
│   └── shared/
│       ├── errors/
│       │   ├── error.go
│       │   ├── codes.go
│       │   └── common.go
│       │
│       ├── response/
│       │   ├── response.go
│       │   └── pagination.go
│       │
│       ├── validator/
│       │   └── validator.go
│       │
│       └── contextx/
│           └── request.go
│
├── migrations/
├── configs/
├── go.mod
├── go.sum
└── README.md
```

---

# 3. 整体调用关系

```text
HTTP Request
     │
     ▼
Middleware
     │
     ▼
Gin Handler
     │
     ▼
Service
 ┌───┴───────────────┐
 ▼                   ▼
Repository          Cache / TokenStore
 │                   │
 ▼                   ▼
GORM                Redis
 │
 ├── MySQL
 ├── PostgreSQL
 └── SQLite
```

核心原则：

```text
Handler       处理 HTTP
Service       处理业务
Repository    处理持久化
Cache         处理缓存语义
Infrastructure 提供具体技术实现
```

---

# 4. Handler

Handler 只负责：

- HTTP 参数解析
- 参数校验
- 调用 Service
- 输出 HTTP Response

不建议在 Handler：

- 直接操作 GORM
- 直接写 Redis
- 写复杂业务判断
- 开事务
- 拼接大量 SQL

示例：

```go
func (h *Handler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.ErrInvalidArgument)
		return
	}

	result, err := h.service.GetByID(
		c.Request.Context(),
		id,
	)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}
```

---

# 5. Service

Service 不应该依赖：

```go
*gin.Context
```

推荐：

```go
func (s *Service) GetByID(
	ctx context.Context,
	id int64,
) (*dto.UserResponse, error)
```

而不是：

```go
func (s *Service) GetByID(c *gin.Context)
```

Service 可以依赖：

```text
Repository Interface
Cache Interface
TokenStore Interface
Transaction Manager
其他模块暴露的 Service Interface
```

例如：

```go
type Service struct {
	repo  repository.Repository
	cache cache.Cache
}
```

---

# 6. Repository

Repository 定义持久化能力。

```go
type Repository interface {
	GetByID(
		ctx context.Context,
		id int64,
	) (*model.User, error)

	GetByEmail(
		ctx context.Context,
		email string,
	) (*model.User, error)

	Create(
		ctx context.Context,
		user *model.User,
	) error

	Update(
		ctx context.Context,
		user *model.User,
	) error

	Delete(
		ctx context.Context,
		id int64,
	) error
}
```

GORM 实现：

```go
type repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}
```

---

# 7. 数据库

## 7.1 GORM Database Factory

```go
func New(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN)

	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.DSN)

	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)

	default:
		return nil, ErrUnsupportedDriver
	}

	db, err := gorm.Open(
		dialector,
		&gorm.Config{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}
```

配置：

```yaml
database:
  driver: mysql
  dsn: root:password@tcp(127.0.0.1:3306)/app?charset=utf8mb4&parseTime=True&loc=Local
```

PostgreSQL：

```yaml
database:
  driver: postgres
  dsn: host=127.0.0.1 user=app password=123456 dbname=app port=5432 sslmode=disable
```

SQLite：

```yaml
database:
  driver: sqlite
  dsn: ./data/app.db
```

---

# 8. GORM 三数据库兼容原则

GORM 可以支持：

- MySQL
- PostgreSQL
- SQLite

普通 CRUD 通常不需要区分数据库：

```go
db.Create(&user)

db.First(&user, id)

db.Where(
	"email = ?",
	email,
).First(&user)

db.Model(&user).Updates(
	map[string]any{
		"name": "test",
	},
)

db.Delete(&user)
```

但是：

> GORM 支持多数据库，不等于所有数据库私有特性自动兼容。

尽量减少：

```text
MySQL ON DUPLICATE KEY UPDATE
PostgreSQL ILIKE
PostgreSQL JSONB
PostgreSQL ARRAY
MySQL UNSIGNED
MySQL ENUM
数据库专属日期函数
数据库专属全文搜索
```

---

# 9. Model

为了提高 MySQL / PostgreSQL / SQLite 兼容性，Model 尽量使用通用字段。

```go
type User struct {
	ID        int64     `gorm:"primaryKey"`
	Username  string    `gorm:"size:100;not null;uniqueIndex"`
	Email     string    `gorm:"size:255;not null;uniqueIndex"`
	Status    int       `gorm:"not null;default:1"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

尽量避免：

```go
gorm:"type:jsonb"
gorm:"type:bigint unsigned"
gorm:"type:enum(...)"
```

---

# 10. Redis

Redis 客户端属于基础设施：

```text
internal/
└── infrastructure/
    └── redis/
        ├── client.go
        └── keys.go
```

初始化：

```go
type Config struct {
	Addr     string
	Username string
	Password string
	DB       int
}

func New(cfg Config) (*redis.Client, error) {
	client := redis.NewClient(
		&redis.Options{
			Addr:     cfg.Addr,
			Username: cfg.Username,
			Password: cfg.Password,
			DB:       cfg.DB,
		},
	)

	if err := client.Ping(
		context.Background(),
	).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
```

---

# 11. 业务 Cache

例如 User：

```text
modules/
└── user/
    ├── repository/
    │   ├── repository.go
    │   └── gorm.go
    │
    └── cache/
        ├── cache.go
        └── redis.go
```

定义：

```go
type Cache interface {
	Get(
		ctx context.Context,
		id int64,
	) (*model.User, error)

	Set(
		ctx context.Context,
		user *model.User,
		ttl time.Duration,
	) error

	Delete(
		ctx context.Context,
		id int64,
	) error
}
```

Service：

```go
type Service struct {
	repo  repository.Repository
	cache cache.Cache
}
```

---

# 12. Redis 不等于 Cache

Redis 还可以用于：

```text
Session
Token
验证码
限流
分布式锁
Pub/Sub
临时状态
排行榜
Counter
```

例如 Auth：

```text
modules/
└── auth/
    ├── handler/
    ├── service/
    ├── dto/
    └── token/
        ├── token.go
        └── redis.go
```

```go
type TokenStore interface {
	Save(
		ctx context.Context,
		token string,
		userID int64,
		ttl time.Duration,
	) error

	Get(
		ctx context.Context,
		token string,
	) (int64, error)

	Delete(
		ctx context.Context,
		token string,
	) error
}
```

---

# 13. Redis Key 规范

推荐：

```text
应用:模块:用途:标识
```

例如：

```text
myapp:user:cache:123
myapp:auth:token:abc123
myapp:order:cache:10001
myapp:verify:email:test@example.com
myapp:ratelimit:login:192.168.1.1
```

每个模块自己维护 Key：

```go
func userKey(id int64) string {
	return fmt.Sprintf(
		"myapp:user:cache:%d",
		id,
	)
}
```

---

# 14. HTTP 状态码与业务错误码

建议将两者分开。

```text
HTTP Status
    ↓
描述 HTTP 请求结果

Business Code
    ↓
描述具体业务错误
```

例如：

```http
HTTP/1.1 404 Not Found
```

```json
{
  "code": 12001,
  "message": "user not found",
  "data": null
}
```

这样前端可以：

- 根据 HTTP Status 做通用错误处理
- 根据业务 code 做精确业务处理

---

# 15. HTTP 状态码建议

| HTTP 状态码 | 用途 |
|---|---|
| 200 | 普通查询、更新成功 |
| 201 | 创建成功 |
| 204 | 成功但无返回内容 |
| 400 | 参数格式错误 |
| 401 | 未认证 / Token 无效 |
| 403 | 已认证但无权限 |
| 404 | 资源不存在 |
| 409 | 数据冲突 |
| 422 | 业务校验失败 |
| 429 | 请求过多 / 限流 |
| 500 | 未预期的服务器错误 |
| 503 | 依赖服务暂时不可用 |

---

# 16. 业务错误码规划

推荐按模块划分区间：

```text
0       成功

10000   通用
11000   Auth
12000   User
13000   Order
14000   Product
15000   Payment
16000   File
17000   Notification
```

例如：

```go
const (
	CodeSuccess = 0

	CodeInvalidArgument = 10001
	CodeInternalError   = 10002
	CodeServiceBusy     = 10003

	CodeUnauthorized = 11001
	CodeTokenExpired = 11002
	CodeTokenInvalid = 11003
	CodeForbidden    = 11004

	CodeUserNotFound      = 12001
	CodeUserAlreadyExists = 12002
	CodeUserDisabled      = 12003

	CodeOrderNotFound = 13001
	CodeOrderConflict = 13002
)
```

---

# 17. AppError

统一应用错误类型：

```go
package errors

type AppError struct {
	Code       int
	HTTPStatus int
	Message    string
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}
```

构造函数：

```go
func New(
	code int,
	httpStatus int,
	message string,
) *AppError {
	return &AppError{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
	}
}
```

包装底层错误：

```go
func Wrap(
	err error,
	code int,
	httpStatus int,
	message string,
) *AppError {
	return &AppError{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
		Err:        err,
	}
}
```

---

# 18. 通用错误

```go
var (
	ErrInvalidArgument = New(
		10001,
		http.StatusBadRequest,
		"invalid argument",
	)

	ErrInternal = New(
		10002,
		http.StatusInternalServerError,
		"internal server error",
	)

	ErrServiceUnavailable = New(
		10003,
		http.StatusServiceUnavailable,
		"service unavailable",
	)
)
```

---

# 19. 模块业务错误

例如：

```go
package user

var (
	ErrUserNotFound = apperrors.New(
		12001,
		http.StatusNotFound,
		"user not found",
	)

	ErrUserAlreadyExists = apperrors.New(
		12002,
		http.StatusConflict,
		"user already exists",
	)

	ErrUserDisabled = apperrors.New(
		12003,
		http.StatusForbidden,
		"user disabled",
	)
)
```

---

# 20. Repository 错误隔离

不要让 Service 知道：

```go
gorm.ErrRecordNotFound
```

Repository 统一转换：

```go
var ErrNotFound = errors.New(
	"repository: not found",
)
```

GORM Repository：

```go
func (r *repository) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	var user model.User

	err := r.db.
		WithContext(ctx).
		First(&user, id).
		Error

	if errors.Is(
		err,
		gorm.ErrRecordNotFound,
	) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}
```

Service：

```go
user, err := s.repo.GetByID(ctx, id)

if errors.Is(
	err,
	repository.ErrNotFound,
) {
	return nil, ErrUserNotFound
}

if err != nil {
	return nil, err
}
```

调用关系：

```text
gorm.ErrRecordNotFound
        │
        ▼
repository.ErrNotFound
        │
        ▼
user.ErrUserNotFound
        │
        ▼
HTTP 404 + code 12001
```

---

# 21. 统一返回结构

```go
type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}
```

成功：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 123,
    "username": "test"
  }
}
```

错误：

```json
{
  "code": 12001,
  "message": "user not found",
  "data": null
}
```

---

# 22. Response Helper

```go
const CodeSuccess = 0

func Success[T any](
	c *gin.Context,
	data T,
) {
	c.JSON(
		http.StatusOK,
		Response[T]{
			Code:    CodeSuccess,
			Message: "success",
			Data:    data,
		},
	)
}
```

创建成功：

```go
func Created[T any](
	c *gin.Context,
	data T,
) {
	c.JSON(
		http.StatusCreated,
		Response[T]{
			Code:    CodeSuccess,
			Message: "success",
			Data:    data,
		},
	)
}
```

无数据成功：

```go
func OK(c *gin.Context) {
	c.JSON(
		http.StatusOK,
		Response[any]{
			Code:    CodeSuccess,
			Message: "success",
		},
	)
}
```

如果严格使用 REST：

```go
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
```

---

# 23. 统一 Error Response

```go
func Error(
	c *gin.Context,
	err error,
) {
	var appErr *apperrors.AppError

	if errors.As(err, &appErr) {
		c.JSON(
			appErr.HTTPStatus,
			Response[any]{
				Code:    appErr.Code,
				Message: appErr.Message,
				Data:    nil,
			},
		)
		return
	}

	c.JSON(
		http.StatusInternalServerError,
		Response[any]{
			Code:    apperrors.CodeInternalError,
			Message: "internal server error",
			Data:    nil,
		},
	)
}
```

---

# 24. 是否所有 HTTP 都返回 200

不推荐。

不建议：

```http
HTTP/1.1 200 OK
```

```json
{
  "code": 12001,
  "message": "user not found"
}
```

推荐：

```http
HTTP/1.1 404 Not Found
```

```json
{
  "code": 12001,
  "message": "user not found"
}
```

这样：

```text
HTTP 状态码负责协议语义
业务 Code 负责业务语义
```

---

# 25. 参数校验

DTO：

```go
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,email"`
}
```

Handler：

```go
var req dto.CreateUserRequest

if err := c.ShouldBindJSON(&req); err != nil {
	response.Error(
		c,
		apperrors.ErrInvalidArgument,
	)
	return
}
```

基础错误返回：

```json
{
  "code": 10001,
  "message": "invalid argument",
  "data": null
}
```

---

# 26. 字段级校验错误

如果前端需要展示具体字段错误，可以扩展：

```json
{
  "code": 10001,
  "message": "validation failed",
  "data": {
    "fields": {
      "email": "invalid email",
      "username": "minimum length is 3"
    }
  }
}
```

结构：

```go
type ValidationData struct {
	Fields map[string]string `json:"fields"`
}
```

建议统一放：

```text
internal/shared/validator/
```

而不是每个 Handler 自己解析 validator 错误。

---

# 27. 分页请求

推荐统一：

```go
type PageRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
```

默认值：

```text
page = 1
page_size = 20
```

并限制：

```text
page_size <= 100
```

避免有人请求：

```text
?page_size=1000000
```

---

# 28. 分页响应

```go
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

type Page[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 1,
        "username": "a"
      }
    ],
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total": 100,
      "total_pages": 5
    }
  }
}
```

---

# 29. Request-ID

建议每个 HTTP 请求都有唯一 Request-ID。

用途：

```text
前端报错
   │
   ▼
拿到 request_id
   │
   ▼
后端日志直接搜索 request_id
```

Middleware：

```go
const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(
			RequestIDHeader,
		)

		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Set(
			"request_id",
			requestID,
		)

		c.Header(
			RequestIDHeader,
			requestID,
		)

		c.Next()
	}
}
```

推荐同时放入：

```go
context.Context
```

这样 Service / Repository / Logger 都能取得 Request-ID。

---

# 30. Request-ID 是否放 JSON

两种方式都可以。

HTTP Header：

```text
X-Request-ID: 9df4...
```

已经足够。

如果希望前端日志更方便，可以同时返回：

```json
{
  "code": 12001,
  "message": "user not found",
  "request_id": "9df4..."
}
```

此时：

```go
type Response[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
```

我的建议：

- Header 一定有
- JSON 是否增加看前端需求

---

# 31. Access Log Middleware

一次请求建议至少记录：

```text
request_id
method
path
status
latency
client_ip
user_agent
user_id（如果已登录）
```

示例日志：

```json
{
  "level": "info",
  "request_id": "5bc3b6...",
  "method": "GET",
  "path": "/api/v1/users/123",
  "status": 200,
  "latency_ms": 12,
  "client_ip": "192.168.1.10",
  "user_id": 10001
}
```

Middleware：

```go
func AccessLog(
	logger *slog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		logger.Info(
			"http request",
			"request_id",
			GetRequestID(c),
			"method",
			c.Request.Method,
			"path",
			c.Request.URL.Path,
			"status",
			c.Writer.Status(),
			"latency",
			time.Since(start),
			"client_ip",
			c.ClientIP(),
		)
	}
}
```

---

# 32. 错误日志链路

对客户端：

```json
{
  "code": 10002,
  "message": "internal server error"
}
```

不要把这些直接返回：

```text
SQL 语句
数据库 DSN
Redis 地址
panic stack
文件路径
内部实现错误
```

但是日志里应该保留真实错误：

```text
request_id
business_code
error
stack
module
operation
```

例如：

```json
{
  "level": "error",
  "request_id": "xxx",
  "module": "user",
  "operation": "GetByID",
  "error": "connection refused",
  "business_code": 10002
}
```

原则：

```text
客户端看到安全的错误
日志看到完整的错误
```

---

# 33. Error Wrap

底层错误建议逐层补充上下文。

Repository：

```go
return nil, fmt.Errorf(
	"query user by id %d: %w",
	id,
	err,
)
```

Service：

```go
return nil, fmt.Errorf(
	"get user: %w",
	err,
)
```

最终日志：

```text
get user:
query user by id 123:
connection refused
```

同时仍然可以通过：

```go
errors.Is()
errors.As()
```

判断错误类型。

---

# 34. Recover Middleware

建议自己写 Recover，而不是只使用：

```go
gin.Recovery()
```

原因是你通常还需要：

- Request-ID
- 统一 JSON
- 日志
- Stack Trace

示例：

```go
func Recovery(
	logger *slog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error(
					"panic recovered",
					"request_id",
					GetRequestID(c),
					"panic",
					rec,
					"stack",
					string(debug.Stack()),
				)

				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					response.Response[any]{
						Code:    10002,
						Message: "internal server error",
					},
				)
			}
		}()

		c.Next()
	}
}
```

---

# 35. Middleware 顺序

推荐：

```go
r.Use(
	middleware.RequestID(),
	middleware.Recovery(logger),
	middleware.AccessLog(logger),
	middleware.CORS(),
)
```

认证中间件放具体路由组：

```go
api := r.Group("/api/v1")

public := api.Group("")
{
	public.POST(
		"/auth/login",
		authHandler.Login,
	)
}

private := api.Group("")
private.Use(
	middleware.Auth(authService),
)
{
	private.GET(
		"/users/me",
		userHandler.Me,
	)
}
```

---

# 36. 认证错误

建议：

未登录：

```http
401 Unauthorized
```

```json
{
  "code": 11001,
  "message": "unauthorized"
}
```

Token 过期：

```http
401 Unauthorized
```

```json
{
  "code": 11002,
  "message": "token expired"
}
```

无权限：

```http
403 Forbidden
```

```json
{
  "code": 11004,
  "message": "forbidden"
}
```

不要把：

```text
401 和 403
```

混为一谈。

---

# 37. 创建接口

```text
POST /api/v1/users
```

建议：

```http
201 Created
```

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 123,
    "username": "test"
  }
}
```

---

# 38. 删除接口

两种风格都可以。

方案 A：

```http
200 OK
```

```json
{
  "code": 0,
  "message": "success"
}
```

方案 B：

```http
204 No Content
```

无 Body。

如果是管理后台 / App API，并且希望客户端统一处理，方案 A 更省事。

---

# 39. 更新接口

```text
PUT /users/:id
PATCH /users/:id
```

推荐区别：

```text
PUT
完整替换资源

PATCH
部分更新
```

实际管理后台多数场景：

```text
PATCH
```

更适合。

返回：

```http
200 OK
```

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 123,
    "username": "new-name"
  }
}
```

---

# 40. 事务

建议不要让 Service 到处直接：

```go
tx := db.Begin()
```

统一抽象：

```go
type TransactionManager interface {
	Transaction(
		ctx context.Context,
		fn func(
			ctx context.Context,
		) error,
	) error
}
```

Service：

```go
func (s *Service) CreateOrder(
	ctx context.Context,
	req dto.CreateOrderRequest,
) error {
	return s.tx.Transaction(
		ctx,
		func(ctx context.Context) error {
			if err := s.orderRepo.Create(
				ctx,
				...,
			); err != nil {
				return err
			}

			if err := s.stockService.Reduce(
				ctx,
				...,
			); err != nil {
				return err
			}

			return nil
		},
	)
}
```

---

# 41. Module 初始化

```go
type Module struct {
	Handler *handler.Handler
}

func NewModule(
	db *gorm.DB,
	redisClient *redis.Client,
) *Module {
	repo := repository.New(db)
	cacheStore := cache.New(redisClient)

	svc := service.New(
		repo,
		cacheStore,
	)

	h := handler.New(svc)

	return &Module{
		Handler: h,
	}
}
```

---

# 42. Router 注册

```go
func (
	m *Module,
) RegisterRoutes(
	r *gin.RouterGroup,
) {
	users := r.Group("/users")

	users.GET(
		"/:id",
		m.Handler.GetUser,
	)

	users.POST(
		"",
		m.Handler.CreateUser,
	)

	users.PATCH(
		"/:id",
		m.Handler.UpdateUser,
	)

	users.DELETE(
		"/:id",
		m.Handler.DeleteUser,
	)
}
```

---

# 43. App 层

```go
type App struct {
	DB    *gorm.DB
	Redis *redis.Client

	UserModule *user.Module
	AuthModule *auth.Module
}
```

```go
func New(
	cfg *config.Config,
) (*App, error) {
	db, err := database.New(
		cfg.Database,
	)
	if err != nil {
		return nil, err
	}

	redisClient, err := redisinfra.New(
		cfg.Redis,
	)
	if err != nil {
		return nil, err
	}

	return &App{
		DB:    db,
		Redis: redisClient,

		UserModule: user.NewModule(
			db,
			redisClient,
		),

		AuthModule: auth.NewModule(
			db,
			redisClient,
		),
	}, nil
}
```

---

# 44. 推荐 Router

```go
func (
	a *App,
) Router() *gin.Engine {
	r := gin.New()

	r.Use(
		middleware.RequestID(),
		middleware.Recovery(a.Logger),
		middleware.AccessLog(a.Logger),
	)

	api := r.Group("/api/v1")

	a.AuthModule.RegisterRoutes(api)
	a.UserModule.RegisterRoutes(api)

	return r
}
```

---

# 45. 推荐响应规范总结

成功：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

业务错误：

```json
{
  "code": 12001,
  "message": "user not found",
  "data": null
}
```

参数错误：

```json
{
  "code": 10001,
  "message": "validation failed",
  "data": {
    "fields": {
      "email": "invalid email"
    }
  }
}
```

服务端错误：

```json
{
  "code": 10002,
  "message": "internal server error",
  "data": null
}
```

---

# 46. 推荐错误处理流程

```text
GORM / Redis / External Service
             │
             ▼
      Infrastructure Error
             │
             ▼
       Repository Error
             │
             ▼
        Service Error
             │
             ▼
          AppError
             │
             ▼
      response.Error()
             │
       ┌─────┴─────┐
       ▼           ▼
HTTP Status   Business Code
```

---

# 47. 最终架构

```text
                              ┌──────────────┐
                              │     Gin      │
                              └──────┬───────┘
                                     │
                                     ▼
                               Middleware
                         Request-ID / Log / Recover
                                     │
                                     ▼
                                  Handler
                                     │
                                     ▼
                                  Service
                           ┌─────────┴─────────┐
                           │                   │
                           ▼                   ▼
                      Repository            Cache
                           │                   │
                           ▼                   ▼
                         GORM                Redis
                           │
                 ┌─────────┼─────────┐
                 ▼         ▼         ▼
               MySQL   PostgreSQL  SQLite
```

---

# 48. 推荐原则汇总

## HTTP 层

- Gin 只出现在 Handler / Middleware / Router
- Service 不使用 `gin.Context`
- HTTP Status 与业务 Code 分离
- 所有接口统一 Response 格式

## 业务层

- Service 负责业务规则
- 不直接依赖 GORM
- 不直接依赖具体 Redis 实现
- 跨模块优先调用 Service Interface

## 持久化层

- Repository 屏蔽 GORM
- Repository 将 GORM 特有错误转换成自己的错误
- 大部分 CRUD 通过 GORM 实现
- 尽量避免数据库方言专属 SQL

## Redis

- Redis Client 属于 Infrastructure
- 具体缓存策略属于业务模块
- Token / Cache / Lock 等按业务语义定义接口
- Key 命名统一

## 错误

- AppError 统一承载：
  - HTTP Status
  - Business Code
  - Public Message
  - Internal Error
- 客户端不返回底层错误细节
- 日志保留完整错误链

## 可观测性

- 每个请求生成 Request-ID
- Request-ID 返回给客户端
- Access Log 携带 Request-ID
- Panic 日志携带 Request-ID 和 Stack
- 业务错误日志能够通过 Request-ID 串起来

---

# 49. 最终建议

对于一个：

```text
Go
+ Gin
+ GORM
+ Redis
+ 多模块
+ MySQL / PostgreSQL / SQLite
```

的后端项目，推荐采用：

```text
模块化单体
+
Handler / Service / Repository
+
GORM Database Factory
+
Redis Infrastructure
+
业务级 Cache / TokenStore
+
统一 AppError
+
HTTP Status + Business Code
+
统一 Response
+
Pagination
+
Validator
+
Request-ID
+
Access Log
+
Recovery
```

这套结构不会像完整 DDD 那么重，但已经足够支撑一个中大型后台项目，并且后续增加：

```text
RabbitMQ / Kafka
对象存储
邮件
短信
Webhook
定时任务
后台 Worker
```

也不需要推翻现有架构。

---

# 50. 日志架构

日志建议采用：

```text
业务代码
   │
   ▼
Logger Interface
   │
   ├── slog Adapter
   ├── zap Adapter
   └── zerolog Adapter
```

业务层不要直接依赖：

```go
*slog.Logger
*zap.Logger
zerolog.Logger
```

而是统一依赖自己的日志接口。

这样以后切换底层日志实现时：

```text
slog
  ↓
zap
```

或者：

```text
slog
  ↓
zerolog
```

业务代码基本不需要修改。

---

# 51. Logger Interface

推荐定义一个非常薄的接口：

```go
package logger

import "context"

type Logger interface {
	Debug(
		ctx context.Context,
		msg string,
		fields ...Field,
	)

	Info(
		ctx context.Context,
		msg string,
		fields ...Field,
	)

	Warn(
		ctx context.Context,
		msg string,
		fields ...Field,
	)

	Error(
		ctx context.Context,
		msg string,
		fields ...Field,
	)

	With(
		fields ...Field,
	) Logger
}
```

只抽象业务层真正需要的公共能力：

```text
Debug
Info
Warn
Error
With
```

不要为了兼容不同日志库，把下面这些底层特性全部塞进统一接口：

```text
zap.Core
zap.SugaredLogger
zap.DPanic
zerolog.Event
zerolog.Dict
slog.Handler
slog.Group
```

原则：

> 抽象公共日志能力，不重新发明一个完整日志框架。

---

# 52. 统一 Field

```go
package logger

import "time"

type Field struct {
	Key   string
	Value any
}

func String(
	key string,
	value string,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Int(
	key string,
	value int,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Int64(
	key string,
	value int64,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Uint64(
	key string,
	value uint64,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Float64(
	key string,
	value float64,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Bool(
	key string,
	value bool,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Time(
	key string,
	value time.Time,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Duration(
	key string,
	value time.Duration,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Any(
	key string,
	value any,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Err(
	err error,
) Field {
	return Field{
		Key:   "error",
		Value: err,
	}
}
```

业务代码以后只使用这些统一字段：

```go
log.Info(
	ctx,
	"user created",
	logger.Int64(
		"user_id",
		user.ID,
	),
	logger.String(
		"username",
		user.Username,
	),
)
```

底层不管是：

```text
slog
zap
zerolog
```

业务写法都保持一致。

---

# 53. 字段绑定 With

模块级字段可以通过 `With()` 绑定。

例如 User 模块：

```go
userLogger := rootLogger.With(
	logger.String(
		"module",
		"user",
	),
)
```

之后：

```go
userLogger.Info(
	ctx,
	"user created",
	logger.Int64(
		"user_id",
		user.ID,
	),
)
```

最终日志：

```json
{
  "level": "INFO",
  "message": "user created",
  "module": "user",
  "user_id": 123
}
```

字段可以逐层绑定：

```text
Root Logger
   │
   │ service=myapp-api
   │ env=production
   ▼
Module Logger
   │
   │ module=user
   ▼
Operation Logger
   │
   │ operation=GetByID
   ▼
Single Log Event
       user_id=123
       error=...
```

---

# 54. 推荐字段层级

建议把日志字段分为三层。

## 54.1 全局字段

应用初始化时绑定：

```text
service
env
version
instance
```

例如：

```go
log = log.With(
	logger.String(
		"service",
		"myapp-api",
	),
	logger.String(
		"env",
		"production",
	),
	logger.String(
		"version",
		version,
	),
)
```

---

## 54.2 模块字段

模块初始化时绑定：

```go
log := rootLogger.With(
	logger.String(
		"module",
		"user",
	),
)
```

---

## 54.3 请求 / 事件字段

运行时增加：

```go
log.Info(
	ctx,
	"user created",
	logger.Int64(
		"user_id",
		user.ID,
	),
)
```

最终：

```json
{
  "service": "myapp-api",
  "env": "production",
  "version": "1.2.3",
  "module": "user",
  "request_id": "xxx",
  "message": "user created",
  "user_id": 123
}
```

---

# 55. Request-ID 自动绑定

Request-ID 不建议每次手动写：

```go
logger.String(
	"request_id",
	requestID,
)
```

可以统一通过 Context 自动绑定。

```go
func FromContext(
	log Logger,
	ctx context.Context,
) Logger {
	fields := make(
		[]Field,
		0,
		3,
	)

	if requestID :=
		contextx.RequestID(ctx);
		requestID != "" {

		fields = append(
			fields,
			String(
				"request_id",
				requestID,
			),
		)
	}

	if userID, ok :=
		contextx.UserID(ctx);
		ok {

		fields = append(
			fields,
			Int64(
				"user_id",
				userID,
			),
		)
	}

	if traceID :=
		contextx.TraceID(ctx);
		traceID != "" {

		fields = append(
			fields,
			String(
				"trace_id",
				traceID,
			),
		)
	}

	if len(fields) == 0 {
		return log
	}

	return log.With(
		fields...,
	)
}
```

Service：

```go
func (
	s *Service,
) GetByID(
	ctx context.Context,
	id int64,
) (*dto.UserResponse, error) {

	log := logger.FromContext(
		s.logger,
		ctx,
	)

	log.Debug(
		ctx,
		"getting user",
		logger.Int64(
			"user_id",
			id,
		),
	)

	// ...

	return result, nil
}
```

---

# 56. Context 推荐字段

建议 Context 中只存请求范围内的元数据：

```text
request_id
user_id
trace_id
```

不要把大量业务数据塞进 `context.Context`。

推荐：

```text
Request-ID
    │
    ▼
Gin Middleware
    │
    ▼
context.Context
    │
    ├── Handler
    ├── Service
    ├── Repository
    └── Logger
```

---

# 57. slog Adapter

```go
type SlogLogger struct {
	logger *slog.Logger
}
```

字段转换：

```go
func toSlogArgs(
	fields []Field,
) []any {
	args := make(
		[]any,
		0,
		len(fields)*2,
	)

	for _, field :=
		range fields {

		args = append(
			args,
			field.Key,
			field.Value,
		)
	}

	return args
}
```

Info：

```go
func (
	l *SlogLogger,
) Info(
	ctx context.Context,
	msg string,
	fields ...Field,
) {
	l.logger.InfoContext(
		ctx,
		msg,
		toSlogArgs(
			fields,
		)...,
	)
}
```

With：

```go
func (
	l *SlogLogger,
) With(
	fields ...Field,
) Logger {
	return &SlogLogger{
		logger: l.logger.With(
			toSlogArgs(
				fields,
			)...,
		),
	}
}
```

其他：

```text
Debug
Warn
Error
```

实现方式相同。

---

# 58. Zap Adapter

```go
type ZapLogger struct {
	logger *zap.Logger
}
```

统一 Field 转 Zap：

```go
func toZapFields(
	fields []Field,
) []zap.Field {
	result := make(
		[]zap.Field,
		0,
		len(fields),
	)

	for _, field :=
		range fields {

		switch v :=
			field.Value.(type) {

		case string:
			result = append(
				result,
				zap.String(
					field.Key,
					v,
				),
			)

		case int:
			result = append(
				result,
				zap.Int(
					field.Key,
					v,
				),
			)

		case int64:
			result = append(
				result,
				zap.Int64(
					field.Key,
					v,
				),
			)

		case uint64:
			result = append(
				result,
				zap.Uint64(
					field.Key,
					v,
				),
			)

		case float64:
			result = append(
				result,
				zap.Float64(
					field.Key,
					v,
				),
			)

		case bool:
			result = append(
				result,
				zap.Bool(
					field.Key,
					v,
				),
			)

		case time.Duration:
			result = append(
				result,
				zap.Duration(
					field.Key,
					v,
				),
			)

		case time.Time:
			result = append(
				result,
				zap.Time(
					field.Key,
					v,
				),
			)

		case error:
			result = append(
				result,
				zap.NamedError(
					field.Key,
					v,
				),
			)

		default:
			result = append(
				result,
				zap.Any(
					field.Key,
					v,
				),
			)
		}
	}

	return result
}
```

Info：

```go
func (
	l *ZapLogger,
) Info(
	ctx context.Context,
	msg string,
	fields ...Field,
) {
	l.logger.Info(
		msg,
		toZapFields(
			fields,
		)...,
	)
}
```

With：

```go
func (
	l *ZapLogger,
) With(
	fields ...Field,
) Logger {
	return &ZapLogger{
		logger: l.logger.With(
			toZapFields(
				fields,
			)...,
		),
	}
}
```

---

# 59. Zerolog Adapter

```go
type ZerologLogger struct {
	logger zerolog.Logger
}
```

Info：

```go
func (
	l *ZerologLogger,
) Info(
	ctx context.Context,
	msg string,
	fields ...Field,
) {
	event := l.logger.Info()

	for _, field :=
		range fields {

		event = addZerologField(
			event,
			field,
		)
	}

	event.Msg(msg)
}
```

字段转换：

```go
func addZerologField(
	event *zerolog.Event,
	field Field,
) *zerolog.Event {
	switch v :=
		field.Value.(type) {

	case string:
		return event.Str(
			field.Key,
			v,
		)

	case int:
		return event.Int(
			field.Key,
			v,
		)

	case int64:
		return event.Int64(
			field.Key,
			v,
		)

	case uint64:
		return event.Uint64(
			field.Key,
			v,
		)

	case float64:
		return event.Float64(
			field.Key,
			v,
		)

	case bool:
		return event.Bool(
			field.Key,
			v,
		)

	case time.Duration:
		return event.Dur(
			field.Key,
			v,
		)

	case time.Time:
		return event.Time(
			field.Key,
			v,
		)

	case error:
		return event.AnErr(
			field.Key,
			v,
		)

	default:
		return event.Interface(
			field.Key,
			v,
		)
	}
}
```

With：

```go
func (
	l *ZerologLogger,
) With(
	fields ...Field,
) Logger {
	ctx := l.logger.With()

	for _, field :=
		range fields {

		switch v :=
			field.Value.(type) {

		case string:
			ctx = ctx.Str(
				field.Key,
				v,
			)

		case int:
			ctx = ctx.Int(
				field.Key,
				v,
			)

		case int64:
			ctx = ctx.Int64(
				field.Key,
				v,
			)

		case bool:
			ctx = ctx.Bool(
				field.Key,
				v,
			)

		default:
			ctx = ctx.Interface(
				field.Key,
				v,
			)
		}
	}

	return &ZerologLogger{
		logger: ctx.Logger(),
	}
}
```

---

# 60. Logger Factory

如果未来需要动态选择：

```yaml
log:
  driver: slog
```

或者：

```yaml
log:
  driver: zap
```

或者：

```yaml
log:
  driver: zerolog
```

可以统一：

```go
func New(
	cfg Config,
) (Logger, error) {
	switch cfg.Driver {

	case "slog":
		return NewSlog(cfg)

	case "zap":
		return NewZap(cfg)

	case "zerolog":
		return NewZerolog(cfg)

	default:
		return nil,
			fmt.Errorf(
				"unsupported logger: %s",
				cfg.Driver,
			)
	}
}
```

架构：

```text
                   Logger Interface
                         │
              ┌──────────┼───────────┐
              ▼          ▼           ▼
            slog        zap       zerolog
```

---

# 61. 推荐日志目录

```text
internal/
└── infrastructure/
    └── logger/
        ├── logger.go
        ├── field.go
        ├── config.go
        ├── factory.go
        ├── context.go
        ├── slog.go
        ├── zap.go
        └── zerolog.go
```

如果当前只使用 slog：

```text
logger/
├── logger.go
├── field.go
├── config.go
├── context.go
└── slog.go
```

未来需要切换时，再增加：

```text
zap.go
zerolog.go
```

即可。

---

# 62. 日志输出格式

生产环境推荐结构化 JSON：

```json
{
  "timestamp": "2026-09-20T00:35:12.123+08:00",
  "level": "INFO",
  "message": "http request",
  "service": "myapp-api",
  "env": "production",
  "request_id": "6e6c7b1e-...",
  "module": "user",
  "method": "GET",
  "path": "/api/v1/users/123",
  "status": 200,
  "latency_ms": 12,
  "client_ip": "192.168.1.10",
  "user_id": 10001
}
```

错误：

```json
{
  "timestamp": "2026-09-20T00:35:13.123+08:00",
  "level": "ERROR",
  "message": "get user failed",
  "service": "myapp-api",
  "env": "production",
  "request_id": "6e6c7b1e-...",
  "module": "user",
  "operation": "GetByID",
  "user_id": 123,
  "error": "get user: query user by id 123: connection refused"
}
```

建议固定字段命名，不要同一种含义使用多种 Key。

例如统一：

```text
timestamp
level
message
service
env
version
request_id
trace_id
module
operation
user_id
method
path
status
latency_ms
client_ip
error
```

---

# 63. 终端与文件输出

推荐支持：

```text
                    ┌── stdout
Logger Backend ─────┤
                    └── log file
```

配置：

```yaml
log:
  driver: slog
  level: info

  service: myapp-api
  environment: production

  console:
    enabled: true

  file:
    enabled: true
    path: ./logs/app.log
    max_size_mb: 100
    max_backups: 10
    max_age_days: 30
    compress: true
```

单机 / 1Panel / systemd：

```text
stdout + file
```

Docker / Kubernetes：

```text
优先 stdout
```

由容器平台统一收集。

---

# 64. 日志文件轮转

日志文件建议：

```text
logs/
├── app.log
├── app-2026-09-19T20-12-01.log.gz
├── app-2026-09-18T19-01-33.log.gz
└── ...
```

生命周期：

```text
app.log
   │
   │ 达到大小限制
   ▼
旧文件重命名
   │
   ▼
gzip 压缩
   │
   ├── 按 MaxBackups 保留
   │
   └── 按 MaxAge 清理
              │
              ▼
            删除
```

例如：

```yaml
max_size_mb: 100
max_backups: 10
max_age_days: 30
compress: true
```

可以使用：

```text
lumberjack
```

作为文件 `io.Writer`。

---

# 65. 不建议按 Level 分日志文件

不建议：

```text
info.log
warn.log
error.log
```

因为一次请求的日志可能被拆散。

推荐：

```text
app.log
```

所有应用日志统一写入：

```json
{
  "level": "ERROR"
}
```

通过 level 查询和过滤。

但 Audit Log 可以单独：

```text
logs/
├── app.log
└── audit.log
```

因为审计日志的：

```text
用途
权限
生命周期
保留期限
```

通常与普通应用日志不同。

---

# 66. Access Log

HTTP 请求统一由 Middleware 记录。

建议字段：

```text
request_id
method
path
status
latency_ms
client_ip
user_agent
user_id
```

例如：

```json
{
  "level": "INFO",
  "message": "http request",
  "request_id": "xxx",
  "method": "GET",
  "path": "/api/v1/users/123",
  "status": 200,
  "latency_ms": 8,
  "client_ip": "192.168.1.10"
}
```

Handler 不需要重复打印：

```text
received request
request complete
```

---

# 67. 错误只在边界记录一次

非常重要。

不建议：

```text
Repository
    ↓ ERROR

Service
    ↓ ERROR

Handler
    ↓ ERROR

Middleware
    ↓ ERROR
```

这样一个错误会产生多条重复日志。

推荐：

```text
Repository
    ↓
wrap error

Service
    ↓
wrap / 转换业务错误

HTTP / Worker / Consumer 边界
    ↓
记录一次完整错误
```

Repository：

```go
return nil,
	fmt.Errorf(
		"query user by id %d: %w",
		id,
		err,
	)
```

Service：

```go
return nil,
	fmt.Errorf(
		"get user: %w",
		err,
	)
```

HTTP 边界：

```go
log.Error(
	ctx,
	"request failed",
	logger.Err(err),
)
```

最终：

```text
get user:
query user by id 123:
connection refused
```

只打印一次。

---

# 68. 哪些地方应该主动打印日志

推荐主动记录：

```text
服务启动
服务停止
配置加载失败
数据库连接失败
Redis 连接失败
外部服务调用失败
Worker 任务失败
MQ 消费失败
Panic
安全事件
重要业务事件
```

例如：

```go
log.Info(
	context.Background(),
	"database connected",
	logger.String(
		"driver",
		"postgres",
	),
)
```

禁止记录敏感数据：

```text
数据库密码
Redis 密码
JWT Token
Authorization
Cookie
验证码
完整身份证信息
完整银行卡信息
```

---

# 69. Panic 日志

Recover Middleware 必须保留：

```text
request_id
panic
stack
```

例如：

```go
log.Error(
	ctx,
	"panic recovered",
	logger.Any(
		"panic",
		rec,
	),
	logger.String(
		"stack",
		string(
			debug.Stack(),
		),
	),
)
```

客户端仍然只返回：

```json
{
  "code": 10002,
  "message": "internal server error",
  "data": null
}
```

原则：

```text
客户端看到安全错误
服务端日志保留完整上下文
```

---

# 70. 日志方案最终建议

当前项目推荐：

```text
Logger Interface
+
Field
+
With()
+
FromContext()
+
默认 slog Adapter
+
JSON 结构化日志
+
stdout
+
可选文件输出
+
文件自动轮转
+
Request-ID
+
Access Log
+
Panic Stack
+
错误只在边界记录一次
```

底层日志实现：

```text
当前：
slog

未来需要时：
zap / zerolog
```

业务代码始终保持：

```go
log.Error(
	ctx,
	"create order failed",
	logger.Int64(
		"order_id",
		orderID,
	),
	logger.Int64(
		"user_id",
		userID,
	),
	logger.Err(err),
)
```

无论底层使用：

```text
slog
zap
zerolog
```

业务层都不需要修改。

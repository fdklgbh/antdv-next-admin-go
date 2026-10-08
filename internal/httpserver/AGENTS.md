# HTTP Server 规则

- Router 只创建 Engine、安装全局 Middleware、注册模块路由以及 NoRoute/NoMethod。
- Middleware 通过最小 Service 契约获得认证与权限能力，不得直接依赖数据库或 Redis。
- 所有业务响应通过 `internal/shared/response` 输出；不得直接构造业务 JSON 响应。
- 中间件和路由错误的 `message` 使用中文，HTTP 状态与业务码分别指定。未认证使用 HTTP 401 / `authcode.Unauthenticated`，无权限使用 HTTP 403 / `authcode.PermissionDenied`，认证或权限依赖异常使用 HTTP 503 / `authcode.ServiceUnavailable`，避免前端将服务故障当成登录失效。
- Request-ID 必须同步写入 Gin Context、响应头和标准 `context.Context`。
- HTTP 边界不得记录 Authorization、Cookie、密码、验证码或完整请求体。

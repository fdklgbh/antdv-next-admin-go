# HTTP Server 规则

- Router 只创建 Engine、安装全局 Middleware、注册模块路由以及 NoRoute/NoMethod。
- Middleware 通过最小 Service 契约获得认证与权限能力，不得直接依赖数据库或 Redis。
- 所有业务响应通过 `internal/shared/response` 输出；不得直接构造业务 JSON 响应。
- Request-ID 必须同步写入 Gin Context、响应头和标准 `context.Context`。
- HTTP 边界不得记录 Authorization、Cookie、密码、验证码或完整请求体。

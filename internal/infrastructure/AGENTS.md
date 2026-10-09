# Infrastructure 规则

- Infrastructure 负责数据库、可选 Redis、内存/Redis 通用 Cache Store 和 Logger Adapter，不得依赖任何业务模块。
- 外部连接由 App 创建一次并统一关闭；Redis 仅在 `redis.enabled` 时创建；禁止在 Handler、Middleware 或业务 Service 中创建客户端。
- 数据库 Driver 选择只允许位于 `database` 包；Redis 非 Miss 错误必须保留并向上传播。
- 通用 Cache Store 只处理字节、TTL 和 Key，不包含业务 Key、序列化结构或权限逻辑；Redis 启用后出错不得自动降级到内存 Store。
- Logger Adapter 不得记录 Token、Cookie、密码、验证码或完整请求体。

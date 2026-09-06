# FluxA 后端单一登录流程

## 目标

让 App 通过 Knowvia 后端完成 FluxA 登录，避免客户端直接访问 FluxA New API 或接触上游 access token。

## 架构与数据流

1. App 选择 `paid` 或 `free`，将站点、用户名、密码一次性 POST 到 Knowvia `/v1/auth/fluxa`。
2. 后端根据站点选择固定且经过 HTTPS origin 校验的 FluxA 地址，POST `/api/user/login`。
3. 后端在内存中使用上游 token 请求 `/api/user/self`，验证并取得 FluxA 身份。
4. 后端按站点隔离的 provider 创建/更新 Knowvia 用户身份并签发现有 session payload。
5. 上游 token 不返回客户端、不落库、不写日志；客户端只持久化 Knowvia session。

## 接口变更

- `/v1/auth/fluxa` 请求字段改为 `site`、`username`、`password`。
- 响应保持现有 Knowvia session 结构。
- 客户端移除直连 FluxA 的请求和 access-token exchange 二段调用。
- 服务端继续拒绝非法站点、非 HTTPS/非纯 origin 配置、重定向和不安全上游响应。

## 错误处理

- FluxA 账密无效映射为 401。
- FluxA 要求 2FA 时返回可识别的业务错误，提示用户在对应站点完成验证。
- 上游超时、网络错误或格式错误映射为安全的服务不可用错误。
- 不在错误消息、响应或日志中包含密码和 access token。

## 验证

- Go 单元测试覆盖登录请求、站点路由、2FA/无效凭据、token 不泄露和会话签发。
- TypeScript 测试覆盖客户端仅调用单一后端依赖。
- 运行 Go 全量测试、App 全量测试和 Android Debug 构建。

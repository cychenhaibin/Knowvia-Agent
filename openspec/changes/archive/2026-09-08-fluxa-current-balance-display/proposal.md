## Why

资料页当前显示的是硬编码积分，无法反映 FluxA 中转站账户的实际可用额度。用户需要看到按照中转站货币设置转换后的当前余额，并在用户分组旁快速识别该金额。

## What Changes

- 服务端安全代理 FluxA 的 `/api/status` 和 `/api/user/self`，使用已加密保存的上游登录 Token 获取 `quota` 与货币配置。
- 新增受 Knowvia 会话认证保护的余额 API，向移动端返回 lower-camel 格式的原始额度和汇率配置。
- 移动端按 `USD`、`CNY`、`CUSTOM`、`TOKENS` 规则将 `quota` 格式化为展示金额。
- 资料页以实时余额替换硬编码积分，并在用户分组标签后追加相同的余额标签。
- 请求失败或配置无效时不展示余额，不阻塞资料页的其它功能。

## Capabilities

### New Capabilities

- `fluxa-current-balance`: 安全读取 FluxA 账户原始额度和货币配置，并在移动端转换、展示当前余额。

### Modified Capabilities

- 无。

## Impact

- 服务端 FluxA 凭据代理与 HTTP 路由。
- 移动端 API 类型、余额格式化函数、资料页与认证查询缓存。
- Go HTTP/认证测试与 Expo/TypeScript 回归测试。

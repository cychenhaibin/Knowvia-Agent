---
comet_change: fluxa-current-balance-display
role: technical-design
canonical_spec: openspec
---

# FluxA 当前余额展示技术设计

## 架构

FluxA 上游凭据只保留在服务端。余额服务先无认证读取 `/api/status`，再使用解密后的上游 Bearer Token 读取 `/api/user/self`，并验证成功信封、必填字段、数值有限性、`quotaPerUnit > 0` 和展示类型。对移动端只暴露 Knowvia 认证的 `/v1/fluxa/balance`，并使用明确的 lower-camel DTO。

移动端把端点返回值交给纯格式化器。USD 使用 `quota / quotaPerUnit`；CNY 乘 `usdExchangeRate`；CUSTOM 乘 `customCurrencyExchangeRate` 并使用自定义符号；TOKENS 直接格式化整数。配置或请求不可用时格式化器返回空值。

## 展示与缓存

资料页仅在存在登录 Token、用户 ID 与 FluxA 站点时加载余额。查询键为 `['fluxa-balance', userId, fluxaSite]`，登出清除该前缀。格式化余额传给积分行并作为独立 pill 紧随用户分组；没有用户分组时仍显示余额 pill。

## 错误与安全

服务端将上游 401/403 转为重新登录，将其它上游与解析问题转为安全不可用错误；不会回传上游 Token、响应正文或来源地址。客户端不因余额失败阻塞资料页，而是隐藏余额。

## 验证

- Go：上游两请求鉴权差异、字段校验、错误映射、paid/free 回退和 response DTO。
- TypeScript：四种换算、无效输入、API Bearer 头、资料页查询隔离与标签位置。
- 全量：`go test ./...` 及 `app` 的 `npm test`。

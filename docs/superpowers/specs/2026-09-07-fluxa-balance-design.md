# FluxA 当前余额展示设计

## 目标

在 FluxA 登录用户的资料页中，将现有硬编码积分替换为中转站的当前余额，并在用户分组标签的右侧显示同一金额标签。

## 范围

- 仅修改移动端 `app`。
- 仅为带有 `fluxaSite` 的已登录用户请求和展示余额。
- 保留资料页现有布局与用户分组标签；不修改登录、后端代理或模型分组功能。

## 数据流

1. 资料页在存在 FluxA 站点、用户 ID 和访问令牌时，通过 React Query 并行请求中转站的 `/api/status` 与 `/api/user/self`。
2. `/api/status` 提供额度基数、展示类型及汇率；`/api/user/self` 提供原始 `quota`。
3. 纯格式化函数按配置转换 `quota`：
   - `USD`：`quota / quota_per_unit`
   - `CNY`：`quota / quota_per_unit * usd_exchange_rate`
   - `CUSTOM`：`quota / quota_per_unit * custom_currency_exchange_rate`
   - `TOKENS`：直接显示 `quota`
4. 格式化后的文本同时用于资料页的积分行和分组右侧的新标签。

## API 与类型

- 为 FluxA 余额响应增加明确的 TypeScript 类型。
- 余额 API 使用当前登录 Token 的 `Authorization: Bearer <token>` 访问 `/api/user/self`；`/api/status` 无需认证。
- API 基址沿用当前 FluxA 会话关联的服务端约定，以避免客户端保存或暴露 FluxA 凭据。

## 界面与异常处理

- 分组标签存在时，在它的右侧追加一个余额标签；无分组时仍显示余额标签。
- 请求中显示无侵入的占位文本，不阻塞资料页。
- 请求失败、缺少配置或遇到无效数值时，不显示余额标签，积分行显示默认占位，且不影响其它资料页功能。
- 金额最多保留两位小数；`TOKENS` 按整数分组显示。`CUSTOM` 使用服务端给出的货币符号，缺失时使用通用标识。

## 测试

- 为四种展示类型及无效配置覆盖格式化单元测试。
- 为余额 API 覆盖 URL 与认证头行为。
- 为资料页增加静态回归测试，确保余额标签位于用户分组的同一行，并且查询键按用户和站点隔离。

## 非目标

- 不改写中转站额度、消费历史或充值流程。
- 不在本次新增余额详情页或后台刷新机制。

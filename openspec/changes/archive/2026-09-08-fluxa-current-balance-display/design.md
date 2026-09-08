## Context

Knowvia 的移动端仅持有 Knowvia 会话 Token，而 FluxA 上游 Token 已由服务端加密保存。资料页目前显示硬编码积分 `2860`，并在用户名后显示可选用户分组，二者都不能反映 FluxA 的真实余额。

## Goals / Non-Goals

**Goals:**
- 通过既有的服务端凭据边界安全读取 FluxA 状态和用户额度。
- 按 FluxA 指定规则将原始 `quota` 格式化为余额。
- 在资料页积分行和用户分组标签后显示同一实时余额。

**Non-Goals:**
- 不提供充值、消费记录、额度写入或独立余额详情页。
- 不把上游 FluxA Token 或来源地址发送给移动端。

## Decisions

- 服务端新增 `/v1/fluxa/balance`，而非移动端直连 FluxA。这样复用加密凭据、受控上游 Origin、超时和安全错误映射；直接连接会暴露 Token。
- 上游 `/api/status` 无认证，`/api/user/self` 带 FluxA Bearer Token。服务端仅返回下游所需的 lower-camel 数据字段。
- 将换算实现为移动端纯函数：`USD = quota / quotaPerUnit`，`CNY` 乘 `usdExchangeRate`，`CUSTOM` 乘 `customCurrencyExchangeRate`，`TOKENS` 直接显示原始 `quota`。无效或缺失配置返回无展示值。
- React Query 缓存键包含用户 ID 和 FluxA 站点，并在登出时清除，避免跨账号显示陈旧余额。

## Risks / Trade-offs

- [FluxA 返回字段缺失或失效] → 服务端做存在性和有限数值校验，移动端隐藏余额。
- [上游 Token 失效] → 映射为安全的重新登录提示，不泄漏凭据或响应正文。
- [服务端与移动端字段不一致] → 端点使用明确 DTO，并用 Go 和 TypeScript 回归测试覆盖字段和值。

## Migration Plan

1. 部署服务端余额代理及端点。
2. 发布移动端格式化与资料页展示。
3. 若出现上游故障，回滚客户端展示即可；端点不改变既有 API。

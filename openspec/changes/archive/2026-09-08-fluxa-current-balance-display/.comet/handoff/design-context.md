# Comet Design Handoff

- Change: fluxa-current-balance-display
- Phase: design
- Mode: compact
- Context hash: d068935c749a49ab776841d84dd7b824e3225f22a5b453a82b23bd4617d0580a

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/fluxa-current-balance-display/proposal.md

- Source: openspec/changes/fluxa-current-balance-display/proposal.md
- Lines: 1-27
- SHA256: 347f814e2699d99277e943b452fd6f30b760162a71c2b92f16beda04950a77b0

```md
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

```

## openspec/changes/fluxa-current-balance-display/design.md

- Source: openspec/changes/fluxa-current-balance-display/design.md
- Lines: 1-33
- SHA256: 0d137645c8ba4d9044c1abc690cdaf40385ddd4ae202e059a1913b59353c0172

```md
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

```

## openspec/changes/fluxa-current-balance-display/tasks.md

- Source: openspec/changes/fluxa-current-balance-display/tasks.md
- Lines: 1-17
- SHA256: f5290e41eb380a5f830c3f39bcec8002d1dc37bda2154ff1bf1c417d25a463a4

```md
## 1. 服务端余额代理

- [ ] 1.1 通过已加密的 FluxA 上游凭据获取状态配置和用户 `quota`，并校验必填数值与展示类型。
- [ ] 1.2 发布受 Knowvia 会话保护的余额端点，使用 lower-camel 响应 DTO，并覆盖站点回退和安全错误映射。
- [ ] 1.3 为上游解析、空值拒绝、认证错误和 HTTP 响应字段添加 Go 回归测试。

## 2. 移动端换算与数据访问

- [ ] 2.1 定义 FluxA 余额 DTO，并实现仅携带 Knowvia Bearer Token 的余额 API 调用。
- [ ] 2.2 实现 USD、CNY、CUSTOM、TOKENS 的纯余额格式化函数，以及无效输入降级。
- [ ] 2.3 为四种展示类型、无效配置和 API 鉴权头添加 TypeScript 回归测试。

## 3. 资料页展示

- [ ] 3.1 在资料页按用户与 FluxA 站点隔离余额查询，并在登出时清除其缓存。
- [ ] 3.2 以格式化余额替换硬编码积分，并在用户分组标签后追加余额标签。
- [ ] 3.3 覆盖资料页查询键、标签位置与失败隐藏行为，运行移动端及服务端完整验证。

```

## openspec/changes/fluxa-current-balance-display/specs/fluxa-current-balance/spec.md

- Source: openspec/changes/fluxa-current-balance-display/specs/fluxa-current-balance/spec.md
- Lines: 1-38
- SHA256: 365d3155b0b276338cfdca3a564c0e80c97a75d6fc40b5daa959bb635db4460c

```md
## ADDED Requirements

### Requirement: 安全获取 FluxA 当前余额
系统 SHALL 使用服务器保存的 FluxA 上游凭据请求 `/api/status` 和 `/api/user/self`，并通过受 Knowvia Bearer 会话认证保护的余额端点返回 `quota` 与货币配置。上游 Token 不得出现在移动端请求、响应或错误消息中。

#### Scenario: 已连接的 FluxA 用户读取余额
- **WHEN** 已认证的 FluxA 用户请求余额端点
- **THEN** 系统 SHALL 返回 lower-camel 格式的 `quota`、`quotaPerUnit`、`quotaDisplayType`、`usdExchangeRate`、`customCurrencySymbol` 和 `customCurrencyExchangeRate`

#### Scenario: 上游凭据失效
- **WHEN** FluxA 上游返回 401 或 403
- **THEN** 系统 SHALL 返回安全的重新认证响应且不得暴露上游 Token

### Requirement: 按配置转换额度显示
系统 SHALL 使用 `/api/status` 的显示类型和汇率转换 `quota`：USD 为 `quota / quotaPerUnit`，CNY 在此基础上乘 `usdExchangeRate`，CUSTOM 乘 `customCurrencyExchangeRate`，TOKENS 直接显示 `quota`。

#### Scenario: 人民币余额
- **WHEN** `quotaDisplayType` 为 `CNY` 且 quota 为 7400000、quotaPerUnit 为 500000、usdExchangeRate 为 7.2
- **THEN** 系统 SHALL 显示 `¥106.56`

#### Scenario: Token 余额
- **WHEN** `quotaDisplayType` 为 `TOKENS`
- **THEN** 系统 SHALL 显示未转换的 `quota` 整数

#### Scenario: 配置无效
- **WHEN** 必需配置缺失、为 null 或不是有限数值
- **THEN** 系统 SHALL 不显示余额且不得阻塞资料页

### Requirement: 资料页显示当前余额
系统 SHALL 用格式化后的 FluxA 当前余额替换硬编码积分，并在用户分组标签之后显示一个余额标签；用户无分组时仍可显示余额标签。

#### Scenario: 成功加载余额
- **WHEN** 资料页获取并转换余额成功
- **THEN** 积分行和用户分组后的标签 SHALL 显示相同的格式化金额

#### Scenario: 余额请求失败
- **WHEN** 余额请求失败
- **THEN** 资料页 SHALL 保持可用并隐藏余额标签

```

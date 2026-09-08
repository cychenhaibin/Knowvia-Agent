## ADDED Requirements

### Requirement: 安全获取 FluxA 当前余额
系统 SHALL 使用服务器保存的 FluxA 上游凭据请求 `/api/status` 和 `/api/user/self`，并通过受 Knowvia Bearer 会话认证保护的余额端点返回 `quota` 与货币配置。上游 Token 不得出现在移动端请求、响应或错误消息中。

#### Scenario: 已连接的 FluxA 用户读取余额
- **WHEN** 已认证的 FluxA 用户请求余额端点
- **THEN** 系统 SHALL 返回 lower-camel 格式的 `quota`、`quotaPerUnit`、`quotaDisplayType`、`usdExchangeRate`、`customCurrencySymbol`、`customCurrencyExchangeRate` 和 `group`

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

#### Scenario: 已配置的套餐分组
- **WHEN** 余额查询成功且 `group` 为 `default`
- **THEN** 资料页 SHALL 显示“免费版”且不显示等级标签

#### Scenario: 订阅套餐分组
- **WHEN** 余额查询成功且 `group` 为 `vip`、`svip` 或 `ssvip`
- **THEN** 资料页 SHALL 显示“订阅版”，并在其旁显示对应的等级标签

#### Scenario: 套餐分组不可用
- **WHEN** `group` 缺失、为 null、非字符串、空白、未配置映射，或余额请求失败
- **THEN** 资料页 SHALL 不显示套餐标题或等级标签

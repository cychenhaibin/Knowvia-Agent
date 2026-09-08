# FluxA 资料页套餐分组展示设计

## 目标

用 FluxA `/api/user/self` 返回的 `group` 替换资料页套餐卡片的硬编码“免费版”文案。若该字段不可用，套餐标题保持为空。

## 数据流

1. 服务端余额抓取器在既有、带上游 Bearer Token 的 `/api/user/self` 请求中读取可选字符串 `group`。
2. `GET /v1/fluxa/balance` 在既有 lower-camel DTO 中增加 `group`；不会新增移动端到 FluxA 的请求，也不会下发上游 Token 或来源地址。
3. 移动端 `FluxABalance` DTO 接收 `group`。资料页只在余额查询成功且 `group.trim()` 非空时，将它传给套餐卡片；否则不显示套餐标题。

## 错误处理

- `group` 缺失、`null`、非字符串或纯空白不应使余额查询失败。
- 余额请求失败时，既有余额与积分隐藏逻辑继续生效，套餐标题同样为空。
- 其他必填余额字段的安全校验和错误映射不变。

## 验证

- Go 测试验证 `/api/user/self` 的 `group` 被安全映射为响应的 `group`，且缺失 group 时为空字符串。
- TypeScript 测试验证套餐标题使用 group，空 group 与失败状态均不渲染标题。

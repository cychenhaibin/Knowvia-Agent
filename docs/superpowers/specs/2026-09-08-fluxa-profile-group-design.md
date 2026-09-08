# FluxA 资料页套餐分组展示设计

## 目标

根据 FluxA `/api/user/self` 返回的 `group` 配置资料页套餐卡片：`default` 显示“免费版”；`vip`、`svip`、`ssvip` 显示“订阅版”，并在标题旁显示对应等级标签。未知或不可用分组不显示套餐信息。

## 数据流

1. 服务端余额抓取器在既有、带上游 Bearer Token 的 `/api/user/self` 请求中读取可选字符串 `group`。
2. `GET /v1/fluxa/balance` 在既有 lower-camel DTO 中增加 `group`；不会新增移动端到 FluxA 的请求，也不会下发上游 Token 或来源地址。
3. 移动端 `FluxABalance` DTO 接收 `group`。资料页将已成功查询的 `group.trim()` 交给代码内的分组展示映射：映射同时定义本地化套餐标题键和可选等级标签，使未来新增 group 只需新增一项配置。
4. `default` 映射为本地化“免费版”且无等级标签；`vip`、`svip`、`ssvip` 映射为本地化“订阅版”，标签分别为 `vip`、`svip`、`ssvip`。未命中映射时不渲染套餐标题或等级标签。

## 错误处理

- `group` 缺失、`null`、非字符串或纯空白不应使余额查询失败。
- 余额请求失败时，既有余额与积分隐藏逻辑继续生效，套餐标题和等级标签同样为空。
- 其他必填余额字段的安全校验和错误映射不变。

## 验证

- Go 测试验证 `/api/user/self` 的 `group` 被安全映射为响应的 `group`，且缺失 group 时为空字符串。
- TypeScript 测试验证分组映射、套餐标题和等级标签；空 group、未知 group 与失败状态均不渲染套餐信息。

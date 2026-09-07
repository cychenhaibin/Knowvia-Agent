# Brainstorm Summary

- Change: fluxa-current-balance-display
- Date: 2026-09-08

## 确认的技术方案

服务端使用加密保存的 FluxA 上游 Token 请求 `/api/status` 与 `/api/user/self`，并由受 Knowvia 会话认证的 `/v1/fluxa/balance` 返回 lower-camel DTO。移动端用纯函数处理 USD、CNY、CUSTOM、TOKENS 四类 quota 换算；资料页复用格式化值替换硬编码积分，并在用户分组后显示余额标签。查询键按用户与站点隔离，登出时清除。

## 关键取舍与风险

服务端代理避免向客户端暴露上游 Token，代价是增加一个内部端点。上游字段缺失、null 或非有限数值通过服务端严格校验转为安全失败；前端在失败时隐藏余额，不影响资料页。

## 测试策略

Go 测试覆盖上游请求头、空值/无效字段、站点回退和 lower-camel 响应。TypeScript 测试覆盖全部换算类型、无效输入、Bearer 请求、缓存隔离和资料页标签位置。

## Spec Patch

无。

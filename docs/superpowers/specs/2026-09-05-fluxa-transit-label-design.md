# FluxA 中转站登录文案

## 目标

将登录入口与 FluxA 登录页标题统一显示为“FluxA中转站登录”。

## 范围

- 更新 `login.fluxaLogin`：登录入口按钮文案。
- 更新 `login.fluxaTitle`：FluxA 登录页标题。
- 不修改站点选择、账号输入、登录请求、回调或会话逻辑。

## 验证

新增/更新翻译资源测试，断言两个键的中文值均为“FluxA中转站登录”；运行相关测试与 TypeScript 检查。

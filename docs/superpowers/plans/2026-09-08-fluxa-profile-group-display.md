---
change: fluxa-current-balance-display
design-doc: docs/superpowers/specs/2026-09-08-fluxa-profile-group-design.md
base-ref: c7c6c68d70b84917306b7912f5c56b09af5a2669
---

# FluxA 资料页套餐分组展示实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 FluxA `/api/user/self` 的 `group` 通过可扩展代码映射配置资料页套餐标题和等级标签。

**Architecture:** 余额抓取器从已有自认证的 self 响应中容错读取 group，余额端点下发 lower-camel `group`。移动端复用同一余额查询，并以单一代码内映射将 group 解析为套餐标题和可选等级标签；不额外发起请求。

**Tech Stack:** Go、`encoding/json`、React Native、TypeScript、Node 测试运行器。

## Global Constraints

- 上游 Token、响应正文和来源地址不得返回移动端。
- `group` 是可选展示字段：缺失、null、非字符串或纯空白均映射为空字符串，不能使余额请求失败。
- `default` 映射为“免费版”且无等级标签；`vip`、`svip`、`ssvip` 映射为“订阅版”且标签分别为原 group。
- 余额请求失败、group 为空或未配置映射时，套餐标题与等级标签均不渲染。
- 保持现有 quota、货币换算、查询键、鉴权和错误映射行为不变。

### Task 1: 在余额端点传递可选 FluxA group

**Files:**
- Modify: `server/internal/auth/fluxa_balance.go`
- Modify: `server/internal/auth/fluxa_balance_test.go`
- Modify: `server/internal/adapters/httpapi/fluxa_balance_handler.go`
- Modify: `server/internal/adapters/httpapi/fluxa_balance_handler_test.go`

**Interfaces:** `FluxABalance` 增加 `Group string`；HTTP 成功 DTO 增加 `group`。

- [x] **Task 1 / Step 1: 编写失败的 Go 回归测试**

在上游 self 成功响应加入 `"group":"vip"`，断言抓取器和 HTTP JSON DTO 都返回 `vip`；添加 `group:null` 与 `group:123` 用例，断言成功且 `Group == ""`。

- [x] **Task 1 / Step 2: 观察 RED**

Run: `env -u GOROOT go test ./internal/auth ./internal/adapters/httpapi -run 'TestFluxABalance' -count=1`

Expected: FAIL，因为当前领域和 DTO 没有 group。

- [x] **Task 1 / Step 3: 最小实现**

将 self 的 group 保存为 `json.RawMessage`；只在 JSON 字符串且 `strings.TrimSpace` 非空时赋给 `FluxABalance.Group`。在 `fluxABalanceResponse` 以 `json:"group"` 序列化该值。不要把非字符串 group 视为余额错误。

- [x] **Task 1 / Step 4: 观察 GREEN 并提交**

Run: `env -u GOROOT go test ./internal/auth ./internal/adapters/httpapi -run 'TestFluxABalance' -count=1`

Commit: `feat: expose FluxA account group with balance`

### Task 2: 使用可扩展 group 映射配置资料页套餐

**Files:**
- Modify: `app/types/api.ts`
- Modify: `app/modules/profile/screens/ProfileScreen.tsx`
- Modify: `app/tests/fluxa-flow.test.ts`

**Interfaces:** `FluxABalance` 增加 `group: string`；代码内单一映射返回套餐标题和可选等级标签；`PlanCard` 接受空字符串标题和可选标签，空值时不渲染相应元素。

- [ ] **Task 2 / Step 1: 编写失败的 TypeScript 回归测试**

断言 DTO 包含 `group: string`；映射将 `default` 解析为“免费版”且无标签，将 `vip`、`svip`、`ssvip` 解析为“订阅版”及对应标签；资料页将成功查询的 `group.trim()` 经映射传给 `PlanCard`；未知、空和失败状态不渲染套餐信息。

- [ ] **Task 2 / Step 2: 观察 RED**

Run: `npm test -- fluxa-flow.test.ts`

Expected: FAIL，因为 DTO 没有 group，也没有 group 到套餐标题和标签的映射。

- [ ] **Task 2 / Step 3: 最小实现**

将 `group` 加入 DTO。定义单一可扩展映射：`default` 为“免费版”，`vip`、`svip`、`ssvip` 为“订阅版”并带原 group 标签。资料页仅在 `!balanceQuery.isError && balanceQuery.data` 时查询该映射；`PlanCard` 对空标题或空标签不渲染对应元素。

- [ ] **Task 2 / Step 4: 观察 GREEN 并提交**

Run: `npm test -- fluxa-flow.test.ts && npx tsc --noEmit`

Commit: `feat: map FluxA group to profile plan`

## 自检

- 两个任务覆盖 group 的上游解析、下游 DTO、可扩展套餐映射、空值与未知值降级和资料页渲染。
- 未使用占位符；服务端字段名、移动端字段名均为 `group`。

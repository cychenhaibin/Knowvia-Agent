# FluxA 中转站登录文案 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在简体中文界面中，将 FluxA 入口与登录页标题统一为“FluxA中转站登录”。

**Architecture:** 登录入口与标题均经 `t()` 读取 `getDictionary('zh-Hans')` 返回消息表中的独立键。仅更新这两个翻译值，因此路由、站点选择、New API 请求和会话交换均保持不变。

**Tech Stack:** Expo/React Native、TypeScript、Node 内置测试运行器。

## Global Constraints

- `getDictionary('zh-Hans')` 的 `login.fluxaLogin` 和 `login.fluxaTitle` 必须严格为 `FluxA中转站登录`。
- 不修改其他语言翻译或登录行为。

---

### Task 1: 简体中文 FluxA 登录文案

**Files:**
- Modify: `app/i18n/messages.ts:592-593`
- Modify: `app/tests/fluxa-flow.test.ts`

**Interfaces:**
- Consumes: `getDictionary('zh-Hans')` 中的 `login.fluxaLogin` 与 `login.fluxaTitle`。
- Produces: 两个 UI 翻译键均返回 `FluxA中转站登录`。

- [ ] **Step 1: 写入失败测试**

在 `app/tests/fluxa-flow.test.ts` 的 imports 后添加：

```ts
import {getDictionary} from '../i18n/messages';

test('Chinese FluxA labels identify the service as the transit station', () => {
  const messages = getDictionary('zh-Hans');
  assert.equal(messages['login.fluxaLogin'], 'FluxA中转站登录');
  assert.equal(messages['login.fluxaTitle'], 'FluxA中转站登录');
});
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm test -- --test-name-pattern="Chinese FluxA labels"`

Expected: FAIL；当前两个消息值分别为 `FluxA 登录` 和 `使用 FluxA 登录`。

- [ ] **Step 3: 实施最小改动**

将 `app/i18n/messages.ts` 的简体中文消息替换为：

```ts
'login.fluxaLogin': 'FluxA中转站登录',
'login.fluxaTitle': 'FluxA中转站登录',
```

- [ ] **Step 4: 运行相关测试并确认通过**

Run: `npm test -- --test-name-pattern="Chinese FluxA labels"`

Expected: PASS；该测试的两个断言均通过。

- [ ] **Step 5: 运行完整客户端测试**

Run: `npm test`

Expected: exit code 0。

- [ ] **Step 6: 提交改动**

```bash
git add app/i18n/messages.ts app/tests/fluxa-flow.test.ts
git commit -m "fix: label FluxA login as transit station"
```

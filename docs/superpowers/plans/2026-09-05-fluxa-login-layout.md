# FluxA 登录页布局与站点名称 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除 FluxA 账号区的大外框、统一输入框的可用宽度，并将免费站称为公益站。

**Architecture:** `FluxALoginScreen` 只保留输入区的间距和禁用透明度，以消除装饰容器；输入区外层以 `w-full` 强制同一可用宽度。站点显示继续通过 `login.fluxaSite.free` 翻译键取得，内部站点值保持为 `free`。

**Tech Stack:** Expo/React Native、NativeWind、TypeScript、Node 内置测试运行器。

## Global Constraints

- 不得改变 `FluxASite`、路由、登录 API 或会话逻辑。
- 简体中文 `login.fluxaSite.free` 必须为 `FluxA 公益站`。
- 账号区不得使用 `rounded-[18px] p-3` 大容器；账号和密码字段共享 `w-full` 宽度容器。

---

### Task 1: 布局和公益站文案

**Files:**
- Modify: `app/modules/auth/screens/FluxALoginScreen.tsx:118-148`
- Modify: `app/i18n/messages.ts:596`
- Modify: `app/tests/fluxa-flow.test.ts`

**Interfaces:**
- Consumes: `getDictionary('zh-Hans')` 和 FluxA 页面输入区 JSX。
- Produces: `FluxA 公益站` 标签、无外层大框且宽度统一的输入区。

- [ ] **Step 1: 写入失败测试**

在 `app/tests/fluxa-flow.test.ts` 中导入 `readFileSync`，并添加：

```ts
test('FluxA login uses full-width fields without a credential card and names the free site public', () => {
  const messages = getDictionary('zh-Hans');
  const screen = readFileSync('modules/auth/screens/FluxALoginScreen.tsx', 'utf8');

  assert.equal(messages['login.fluxaSite.free'], 'FluxA 公益站');
  assert.match(screen, /className="w-full gap-4"/);
  assert.doesNotMatch(screen, /className="gap-4 rounded-\[18px\] p-3"/);
});
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm test -- --test-name-pattern="FluxA login uses full-width"`

Expected: FAIL；当前公益站翻译为 `FluxA 免费版`，输入区仍使用圆角容器。

- [ ] **Step 3: 实施最小改动**

将输入区外层从 `className="gap-4 rounded-[18px] p-3"` 改为 `className="w-full gap-4"`，仅保留 `opacity` 样式；将简体中文 `login.fluxaSite.free` 的值改为 `FluxA 公益站`。

- [ ] **Step 4: 运行相关测试并确认通过**

Run: `npm test -- --test-name-pattern="FluxA login uses full-width"`

Expected: PASS。

- [ ] **Step 5: 运行完整客户端测试**

Run: `npm test`

Expected: exit code 0。

- [ ] **Step 6: 提交改动**

```bash
git add app/modules/auth/screens/FluxALoginScreen.tsx app/i18n/messages.ts app/tests/fluxa-flow.test.ts
git commit -m "fix: simplify FluxA credential layout"
```

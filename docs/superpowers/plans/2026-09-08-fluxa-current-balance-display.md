---
change: fluxa-current-balance-display
design-doc: docs/superpowers/specs/2026-09-08-fluxa-current-balance-display-design.md
base-ref: 129f821fdea32f76755157cb0861e8e56f1d7aa4
---

# FluxA 当前余额展示实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在资料页安全展示已登录 FluxA 用户的当前、按上游配置换算后的余额，并替换硬编码积分值。

**Architecture:** 服务端在既有加密凭据边界内读取 FluxA `/api/status` 与 `/api/user/self`，只通过受 Knowvia 会话认证保护的 `GET /v1/fluxa/balance` 下发 lower-camel DTO。移动端以纯函数完成换算，将结果用一个按用户与站点隔离的 React Query 查询提供给资料页；失败或无效数据不影响页面其余内容。

**Tech Stack:** Go、chi、`net/http`；React Native/Expo、TypeScript、TanStack React Query、Node 内置测试运行器。

## 全局约束

- 上游 FluxA Token、上游响应正文和来源地址不得出现在移动端请求、端点响应或错误消息中。
- `/api/status` 必须无 `Authorization` 请求；`/api/user/self` 必须使用解密后的 `Bearer <FluxA token>`。
- 服务端只接受成功信封、有限 `quota`、有限且大于零的 `quota_per_unit`，以及当前展示类型所需的有限汇率。
- 仅支持 `USD`、`CNY`、`CUSTOM`、`TOKENS`；未知类型、空值和非有限数值均映射为安全不可用。
- USD 为 `quota / quotaPerUnit`，CNY 再乘 `usdExchangeRate`，CUSTOM 再乘 `customCurrencyExchangeRate` 并加非空自定义符号，TOKENS 直接显示原始 `quota` 整数。
- 余额查询键必须为 `['fluxa-balance', userId, fluxaSite]`，并在登出时清除 `['fluxa-balance']` 前缀。
- 仅在存在 Knowvia access token、用户 ID 和 FluxA 站点时请求；余额失败时隐藏资料页余额 pill 和积分值，不能阻塞资料页。

---

## 文件结构

- `server/internal/auth/ports.go`：声明 `FluxABalanceFetcher` 端口，并将其加入服务依赖。
- `server/internal/auth/service.go`：保存余额抓取器，并在生产依赖未显式注入时构造默认实现。
- `server/internal/auth/fluxa_balance.go`：定义领域余额值、解密凭据、访问两个上游端点、验证信封与字段、映射安全领域错误。
- `server/internal/auth/fluxa_balance_test.go`：使用 `httptest.Server` 验证上游请求差异、解析和字段拒绝。
- `server/internal/adapters/httpapi/router.go`：挂载余额路由。
- `server/internal/adapters/httpapi/fluxa_balance_routes.go`：定义会话保护的 `GET /fluxa/balance` 路由。
- `server/internal/adapters/httpapi/fluxa_balance_handler.go`：选择 paid/free 凭据、将领域余额映射为 lower-camel HTTP DTO 与安全状态码。
- `server/internal/adapters/httpapi/fluxa_balance_handler_test.go`：验证授权、站点回退、错误映射和 DTO 字段。
- `app/types/api.ts`：声明下游 `FluxABalance` 和联合展示类型。
- `app/lib/api.ts`：增加仅带 Knowvia Bearer token 的 `api.getFluxABalance`。
- `app/lib/fluxaBalance.ts`：提供无副作用的 `formatFluxABalance`。
- `app/modules/profile/screens/ProfileScreen.tsx`：加载余额、格式化并在积分行与用户分组之后渲染。
- `app/store/auth.ts`：登出时清除余额缓存。
- `app/tests/fluxa-flow.test.ts`：覆盖移动端 API、格式化、查询隔离、展示位置和失败隐藏的回归测试。

### Task 1: 建立服务端 FluxA 余额代理

**Files:**
- Modify: `server/internal/auth/ports.go`
- Modify: `server/internal/auth/service.go`
- Create: `server/internal/auth/fluxa_balance.go`
- Create: `server/internal/auth/fluxa_balance_test.go`

**Interfaces:**
- Consumes: `FluxACredentialStore.GetFluxACredential(context.Context, string, FluxASite)`, `FluxACredentialCipher.Decrypt(string, []byte)`、既有 `fluxACredentialAdditionalData`、`fluxAOrigin` 与非重定向 FluxA HTTP client。
- Produces: `type FluxABalance struct { Quota float64; QuotaPerUnit float64; DisplayType string; USDExchangeRate float64; CustomCurrencySymbol string; CustomCurrencyExchangeRate float64 }`、`type FluxABalanceFetcher interface { Balance(context.Context, FluxASite, string) (FluxABalance, error) }`，以及 `func (s *Service) GetFluxABalance(context.Context, string, FluxASite) (FluxABalance, error)`。

- [x] **Step 1: 编写失败的上游请求与数据校验测试**

在 `fluxa_balance_test.go` 建立 `httptest.Server`。`/api/status` 返回成功 CNY 配置，断言其 `Authorization` 为空；`/api/user/self` 返回 `quota: 7400000`，断言其头为 `Bearer upstream-token`。调用 `Balance(context.Background(), FluxASitePaid, "upstream-token")` 并断言得到 `Quota=7400000`、`QuotaPerUnit=500000`、`DisplayType="CNY"`、`USDExchangeRate=7.2`。

```go
func TestFluxABalanceFetcherRequestsStatusAndSelf(t *testing.T) {
    // httptest handler 对 /api/status 与 /api/user/self 分别验证认证头并返回成功信封。
    got, err := newFluxABalanceFetcher(server.URL, server.URL, server.Client()).
        Balance(context.Background(), FluxASitePaid, "upstream-token")
    if err != nil { t.Fatalf("Balance: %v", err) }
    if got.Quota != 7_400_000 || got.QuotaPerUnit != 500_000 || got.DisplayType != "CNY" || got.USDExchangeRate != 7.2 {
        t.Fatalf("balance = %#v", got)
    }
}
```

同一文件添加表驱动失败用例：`success:false` 信封、`quota_per_unit` 为 `0`、缺失/`null` quota、CNY 缺失/`null` USD 汇率、CUSTOM 缺失/`null` 自定义汇率、未知展示类型，以及上游 401/403。前六类断言 `ErrFluxAUnavailable`，401/403 断言 `ErrFluxAReauthenticationRequired`；还要验证 USD、CUSTOM、TOKENS 的有效状态配置均可被接受。

- [x] **Step 2: 运行新测试并确认其先失败**

Run: `go test ./internal/auth -run 'TestFluxABalance' -count=1`

Working directory: `server`

Expected: FAIL，因为余额端口、抓取器与领域服务方法尚未定义。

- [x] **Step 3: 实现最小服务端代理与凭据边界**

在 `ports.go` 与 `service.go` 增加并注入 `FluxABalanceFetcher`，使用现有 paid/free origin 和 HTTP client 创建默认实现。实现 `GetFluxABalance`：验证站点与 user ID、读取恰好对应 `(userID, site)` 的凭据、拒绝缺失/不匹配/空密文、以 `fluxACredentialAdditionalData(userID, site)` 解密，然后只把解密 Token 传给抓取器。

```go
func (s *Service) GetFluxABalance(ctx context.Context, userID string, site FluxASite) (FluxABalance, error) {
    if _, err := fluxAProvider(site); err != nil { return FluxABalance{}, err }
    credential, err := s.fluxACredentials.GetFluxACredential(ctx, userID, site)
    // 将未连接映射为 ErrFluxANotConnected；所有存储、校验、解密失败映射为 ErrFluxAUnavailable。
    token, err := s.fluxACipher.Decrypt(credential.TokenCiphertext, fluxACredentialAdditionalData(userID, site))
    if err != nil || strings.TrimSpace(token) == "" { return FluxABalance{}, ErrFluxAUnavailable }
    return s.fluxABalance.Balance(ctx, site, token)
}
```

在抓取器中先无认证 `GET origin + "/api/status"`，再对 `GET origin + "/api/user/self"` 设置 Bearer 头；两者都要求 HTTP 200、最大 1 MiB 响应、仅一个 JSON 值、`{"success":true,"data":...}` 信封。用指针字段区分缺失/null；`math.IsNaN`/`math.IsInf` 必须拒绝。根据展示类型只要求所需汇率：USD/TOKENS 不要求汇率，CNY 要 `usd_exchange_rate`，CUSTOM 要 `custom_currency_exchange_rate`。只返回 `FluxABalance`，不保留或记录 token/上游 body。

- [x] **Step 4: 运行服务端单元回归并确认通过**

Run: `go test ./internal/auth -run 'TestFluxABalance|TestListFluxAModel' -count=1`

Working directory: `server`

Expected: PASS。

- [x] **Step 5: 提交该独立交付**

```bash
git add server/internal/auth/ports.go server/internal/auth/service.go server/internal/auth/fluxa_balance.go server/internal/auth/fluxa_balance_test.go
git commit -m "feat: proxy FluxA current balance"
```

### Task 2: 发布受会话保护的余额端点

**Files:**
- Modify: `server/internal/adapters/httpapi/router.go`
- Create: `server/internal/adapters/httpapi/fluxa_balance_routes.go`
- Create: `server/internal/adapters/httpapi/fluxa_balance_handler.go`
- Create: `server/internal/adapters/httpapi/fluxa_balance_handler_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Service.GetFluxABalance(ctx, userID, site)`、`currentUser(r.Context())`、`requireAuth`、`writeJSON` 与既有安全错误写入器。
- Produces: `GET /v1/fluxa/balance`，成功时严格返回 `{quota, quotaPerUnit, quotaDisplayType, usdExchangeRate, customCurrencySymbol, customCurrencyExchangeRate}`。

- [x] **Step 1: 编写失败的 HTTP 边界测试**

使用内存 store、测试 JWT、加密凭据和可注入的静态抓取器，分别写出以下断言：普通 Knowvia 用户请求得到 403 与 `FluxA account is not connected`；抓取器返回 `ErrFluxAReauthenticationRequired` 时得到 401 与 `sign in to FluxA again`，响应正文不含 `upstream-token`；成功响应只有六个 lower-camel 字段且值与领域对象相同；仅有 free 凭据时 paid 查找后回退 free；paid 成功时不得查询 free；`ErrFluxAUnavailable` 映射 503。

```go
func TestFluxABalanceSerializesCamelCaseResponseFields(t *testing.T) {
    router, token := newFluxABalanceRouteWithCredentialSites(t, staticFluxABalanceFetcher{
        balance: auth.FluxABalance{Quota: 12.5, QuotaPerUnit: 100, DisplayType: "USD"},
    }, auth.FluxASitePaid)
    response := serveFluxABalanceRequest(router, token)
    if response.Code != http.StatusOK { t.Fatalf("status = %d: %s", response.Code, response.Body.String()) }
    var payload map[string]json.RawMessage
    if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
    if _, ok := payload["quotaPerUnit"]; !ok { t.Fatalf("response = %s", response.Body.String()) }
    if _, ok := payload["quota_per_unit"]; ok { t.Fatalf("snake_case response = %s", response.Body.String()) }
}
```

- [x] **Step 2: 运行路由测试并确认其先失败**

Run: `go test ./internal/adapters/httpapi -run 'TestFluxABalance' -count=1`

Working directory: `server`

Expected: FAIL，因为 `/v1/fluxa/balance` 尚未注册，且 handler/DTO 不存在。

- [x] **Step 3: 实现路由、站点回退与安全 DTO**

在 router 的 `/v1` 组注册 `registerFluxABalanceRoutes`；路由实现必须是：

```go
func (h *Handler) registerFluxABalanceRoutes(r chi.Router) {
    r.Get("/fluxa/balance", h.requireAuth(h.fluxABalance))
}
```

handler 先以 `FluxASitePaid` 调用服务，只有得到 `ErrFluxANotConnected` 才改查 `FluxASiteFree`。定义不复用领域 JSON tag 的 `fluxABalanceResponse`，每个字段都用明确 lower-camel tag。将未连接映射 403，重新认证映射 401，不可用映射 503，其他内部错误映射既有安全 500；绝不序列化原始 error、Token、上游 body 或 origin。

- [x] **Step 4: 运行 HTTP 回归测试并确认通过**

Run: `go test ./internal/adapters/httpapi -run 'TestFluxABalance|TestFluxAModelGroups' -count=1`

Working directory: `server`

Expected: PASS。

- [x] **Step 5: 提交该独立交付**

```bash
git add server/internal/adapters/httpapi/router.go server/internal/adapters/httpapi/fluxa_balance_routes.go server/internal/adapters/httpapi/fluxa_balance_handler.go server/internal/adapters/httpapi/fluxa_balance_handler_test.go
git commit -m "feat: expose FluxA balance endpoint"
```

### Task 3: 定义移动端 DTO、认证 API 与纯格式化器

**Files:**
- Modify: `app/types/api.ts`
- Modify: `app/lib/api.ts`
- Create: `app/lib/fluxaBalance.ts`
- Modify: `app/tests/fluxa-flow.test.ts`

**Interfaces:**
- Consumes: Task 2 的 lower-camel JSON、`request<FluxABalance>(path, init, token)`。
- Produces: `type FluxAQuotaDisplayType = 'USD' | 'CNY' | 'CUSTOM' | 'TOKENS'`、`interface FluxABalance`、`api.getFluxABalance(token: string): Promise<FluxABalance>` 和 `formatFluxABalance(balance: FluxABalance, locale?: string): string | null`。

- [x] **Step 1: 编写失败的格式化与客户端认证测试**

在 `fluxa-flow.test.ts` 添加四个精确输出断言：`7400000 / 500000` 的 USD/en-US 为 `$14.80`；CNY/zh-CN、汇率 `7.2` 为 `¥106.56`；CUSTOM、符号 `K`、汇率 `2` 为 `K29.6`；TOKENS/en-US 为 `7,400,000`。再为 `quotaPerUnit: 0`、空 CUSTOM symbol、所需汇率缺失/非有限值、未知展示类型断言 `null`。对 USD/TOKENS，故意传入不相关汇率的 `NaN`，断言仍能格式化，保证校验只依赖当前类型所需字段。

```ts
test('formats FluxA CNY quota using the configured USD exchange rate', () => {
  assert.equal(
    formatFluxABalance({quota: 7_400_000, quotaPerUnit: 500_000, quotaDisplayType: 'CNY', usdExchangeRate: 7.2, customCurrencySymbol: '', customCurrencyExchangeRate: 1}, 'zh-CN'),
    '¥106.56',
  );
});
```

复用现有 `loadApiForTest()` 的 fetch mock，调用 `getFluxABalance('knowvia-token')`；断言 URL 恰为 `https://knowvia.example/v1/fluxa/balance`，`Authorization` 恰为 `Bearer knowvia-token`，且没有 FluxA Token 或 origin header。

- [x] **Step 2: 运行移动端测试并确认其先失败**

Run: `npm test -- fluxa-flow.test.ts`

Working directory: `app`

Expected: FAIL，因为 DTO、`getFluxABalance` 和格式化器尚不存在。

- [x] **Step 3: 实现下游契约与最小换算函数**

在 `types/api.ts` 声明字段与端点一致的 lower-camel `FluxABalance`。在 `api.ts` 中只通过现有 request helper 实现：

```ts
getFluxABalance: (token: string) => request<FluxABalance>('/fluxa/balance', {}, token),
```

实现格式化器时先验证 `quota >= 0`、`quotaPerUnit > 0` 且均为有限数；USD/CNY/CUSTOM 使用 `Intl.NumberFormat(locale, {maximumFractionDigits: 2})`，其中 USD/CNY 分别使用 currency 样式与 USD/CNY 符号，CUSTOM 把 `customCurrencySymbol.trim()` 后的符号直接前缀到数字。TOKENS 使用 `{maximumFractionDigits: 0}`，直接格式化 `quota`，不做单位换算。每个分支只校验自身必需汇率；任何 `Intl` 异常返回 `null`。

- [x] **Step 4: 运行格式化器和 API 回归并确认通过**

Run: `npm test -- fluxa-flow.test.ts`

Working directory: `app`

Expected: PASS。

- [x] **Step 5: 提交该独立交付**

```bash
git add app/types/api.ts app/lib/api.ts app/lib/fluxaBalance.ts app/tests/fluxa-flow.test.ts
git commit -m "feat: format FluxA current balance"
```

### Task 4: 在资料页展示余额并在登出时隔离缓存

**Files:**
- Modify: `app/modules/profile/screens/ProfileScreen.tsx`
- Modify: `app/store/auth.ts`
- Modify: `app/tests/fluxa-flow.test.ts`

**Interfaces:**
- Consumes: Task 3 的 `api.getFluxABalance(accessToken)`、`formatFluxABalance(balance, locale)`、`useQuery`、`useAuthStore` 与 `queryClient`。
- Produces: 同一 `formattedBalance: string | null` 用于 PlanCard 积分行与用户名区域的独立余额 pill；logout 清除该查询前缀。

- [x] **Step 1: 编写失败的资料页结构与缓存回归测试**

以源文本回归测试锁定查询与 JSX 顺序：`ProfileScreen.tsx` 必须包含 `queryKey: ['fluxa-balance', user?.id, user?.fluxaSite]`、`api.getFluxABalance(accessToken!)`、`enabled: Boolean(accessToken && user?.id && user?.fluxaSite)` 和 `formatFluxABalance`。断言 JSX 中 `user.fluxaGroup` pill 在 `formattedBalance` pill 之前；断言 `PlanCard` 接收格式化余额以替换硬编码 `2860`。断言 `auth.ts` 在 `logout` 中有 `queryClient.removeQueries({queryKey: ['fluxa-balance']})`。最后断言仅在 `formattedBalance` 非空时渲染 balance pill，证明请求失败/无效值不展示。

```ts
test('profile balance query is scoped to the active FluxA account and site', () => {
  const screen = readFileSync('modules/profile/screens/ProfileScreen.tsx', 'utf8');
  assert.match(screen, /queryKey:\s*\['fluxa-balance',\s*user\?\.id,\s*user\?\.fluxaSite\]/);
  assert.match(screen, /enabled:\s*Boolean\(accessToken && user\?\.id && user\?\.fluxaSite\)/);
  assert.match(screen, /api\.getFluxABalance\(accessToken!\)/);
});
```

- [x] **Step 2: 运行资料页测试并确认其先失败**

Run: `npm test -- fluxa-flow.test.ts`

Working directory: `app`

Expected: FAIL，因为资料页尚无余额查询、余额 pill、PlanCard 可选值或 logout 清理逻辑。

- [x] **Step 3: 实现非阻塞查询和两个展示位置**

在 `ProfileScreen` 引入 `useQuery`、`api`、`formatFluxABalance` 与 `accessToken`。创建查询并用当前 i18n locale（将 `zh-Hans` 映射为 `zh-CN`，其余使用现有语言所对应的 BCP-47 locale）格式化其 `data`；不要让 `error` 分支抛出或替换页面内容。

```tsx
const balanceQuery = useQuery({
  queryKey: ['fluxa-balance', user?.id, user?.fluxaSite],
  queryFn: () => api.getFluxABalance(accessToken!),
  enabled: Boolean(accessToken && user?.id && user?.fluxaSite),
});
const formattedBalance = balanceQuery.data ? formatFluxABalance(balanceQuery.data, locale) : null;
```

将 `PlanCard` 扩展为接收可选 `creditsValue?: string | null`，有值时显示该值、无值时不渲染数值，彻底删除硬编码 `2860`。在用户名行保持现有 group pill；紧随其后仅在 `formattedBalance` 存在时添加独立圆角余额 pill，因此没有分组也仍可出现余额。`logout` 中在已有 model-groups 清理旁增加余额查询前缀清理。

- [x] **Step 4: 运行移动端定向和全量验证并确认通过**

Run: `npm test -- fluxa-flow.test.ts && npm test`

Working directory: `app`

Expected: PASS。

- [x] **Step 5: 运行跨端全量验证并提交最终交付**

Run: `go test ./...`

Working directory: `server`

Expected: PASS。

```bash
git add app/modules/profile/screens/ProfileScreen.tsx app/store/auth.ts app/tests/fluxa-flow.test.ts
git commit -m "feat: show FluxA balance in profile"
```

## 计划自检

- 规格覆盖：Task 1 覆盖上游双请求、凭据解密、字段验证和安全错误；Task 2 覆盖 Knowvia 认证、paid/free 回退与 lower-camel DTO；Task 3 覆盖四种换算、无效输入和移动端 Bearer 边界；Task 4 覆盖资料页两个位置、查询隔离、登出清理和失败隐藏。
- 占位符扫描：每项实现步骤均给出具体文件、签名、断言和命令，没有未决实现项或未定义接口。
- 类型一致性：服务端领域 `FluxABalance` 由 Task 1 产生、Task 2 映射为 lower-camel DTO；移动端 `FluxABalance` 在 Task 3 对应该 DTO，并由 Task 4 的 API 查询和格式化器消费。

Plan complete and saved to `docs/superpowers/plans/2026-09-08-fluxa-current-balance-display.md`. Two execution options:

1. **Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?

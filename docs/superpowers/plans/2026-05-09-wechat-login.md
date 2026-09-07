# WeChat Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Android WeChat login through the native SDK and Go `/auth/wechat` session exchange.

**Architecture:** Android sends a WeChat `snsapi_userinfo` auth request and returns only the authorization `code` to JavaScript. The Go server exchanges that code using `QQA_WECHAT_APP_ID` and `QQA_WECHAT_APP_SECRET`, maps WeChat identity into the existing `auth_identities` model, and issues the standard session payload.

**Tech Stack:** Expo React Native, Kotlin React Native bridge, Tencent WeChat OpenSDK, Go `net/http`, existing Go auth/session stores.

---

### Task 1: Server Auth Model and Tests

**Files:**
- Modify: `server/internal/domain/auth.go`
- Modify: `server/internal/auth/service.go`
- Modify: `server/internal/auth/service_external.go`
- Create: `server/internal/auth/wechat.go`
- Modify: `server/internal/auth/service_test.go`

- [ ] Add provider `AuthProviderWeChat`.
- [ ] Add `VerifiedWeChatIdentity`, `WeChatCodeExchanger`, and service constructor wiring.
- [ ] Write a failing `TestLoginWithWeChatCreatesUserAndIdentity`.
- [ ] Implement `LoginWithWeChat` by reusing `resolveOrCreateExternalUser`, updating user profile fields, upserting auth identity, and issuing a session.
- [ ] Run: `go test ./internal/auth -run 'TestLoginWithWeChat|TestLoginWithGoogle|TestLoginWithMicrosoft'`.

### Task 2: WeChat HTTP Client

**Files:**
- Modify: `server/internal/config/config.go`
- Modify: `server/.env.example`
- Modify: `server/internal/auth/wechat.go`

- [ ] Add `QQA_WECHAT_APP_ID` and `QQA_WECHAT_APP_SECRET` config fields.
- [ ] Implement `NewWeChatCodeExchanger(appID, appSecret)` returning `nil` when credentials are incomplete.
- [ ] Implement code exchange against `https://api.weixin.qq.com/sns/oauth2/access_token`.
- [ ] Implement userinfo fetch against `https://api.weixin.qq.com/sns/userinfo`.
- [ ] Treat WeChat `errcode` payloads, missing `openid`, and missing stable subject as invalid or unavailable auth errors.
- [ ] Run: `go test ./internal/auth`.

### Task 3: Server HTTP Endpoint

**Files:**
- Modify: `server/internal/adapters/httpapi/auth_handler.go`
- Modify: `server/internal/adapters/httpapi/auth_routes.go`
- Create: `server/internal/adapters/httpapi/wechat_auth_test.go`
- Modify: `app/lib/api.ts`

- [ ] Write a failing handler test for `POST /v1/auth/wechat` returning a session payload.
- [ ] Write a failing handler test for an empty `code` returning validation error.
- [ ] Add `loginWithWeChat(code)` to the app API client.
- [ ] Implement the handler and register `r.Post("/auth/wechat", h.loginWithWeChat)`.
- [ ] Run: `go test ./internal/adapters/httpapi -run WeChat`.

### Task 4: Android Native WeChat Bridge

**Files:**
- Modify: `app/android/app/build.gradle`
- Modify: `app/android/app/src/main/AndroidManifest.xml`
- Modify: `app/android/app/src/main/java/com/anonymous/quickqueagent/MainApplication.kt`
- Create: `app/android/app/src/main/java/com/anonymous/quickqueagent/QuickQueWeChatAuthModule.kt`
- Create: `app/android/app/src/main/java/com/anonymous/quickqueagent/QuickQueWeChatAuthPackage.kt`
- Create: `app/android/app/src/main/java/com/anonymous/quickqueagent/wxapi/WXEntryActivity.kt`

- [ ] Add `implementation("com.tencent.mm.opensdk:wechat-sdk-android:6.8.24")`.
- [ ] Add the WeChat callback activity under `com.anonymous.quickqueagent.wxapi`.
- [ ] Register `QuickQueWeChatAuthPackage` in `MainApplication`.
- [ ] Implement a module named `QuickQueWeChatAuth` with `signIn(appId)` and `clearAuthState()`.
- [ ] Use `SendAuth.Req` with scope `snsapi_userinfo` and a random state value.

### Task 5: Chinese Login Page Wiring

**Files:**
- Create: `app/lib/wechat-auth.ts`
- Modify: `app/store/auth.ts`
- Modify: `app/modules/auth/screens/LoginScreen.tsx`

- [ ] Add `signInWithWeChat(appId)` that calls the native module on Android.
- [ ] Add logout cleanup through `clearWeChatAuthState`.
- [ ] Update the Chinese login screen button to call WeChat native auth, post `code` to `/auth/wechat`, store the returned session, and navigate to `/(tabs)/runs`.
- [ ] Keep `LoginScreen-en.tsx` unchanged.

### Task 6: Verification

**Files:**
- Verify all modified files.

- [ ] Run: `go test ./internal/auth ./internal/adapters/httpapi`.
- [ ] Run app package validation or TypeScript check from `app/` using available scripts.
- [ ] Inspect `git diff --stat` and confirm no AppSecret is committed to client code.

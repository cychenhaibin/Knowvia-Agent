# WeChat Login Design

## Goal

Add WeChat login for the Android app through the native WeChat SDK, with the Go API exchanging authorization codes for WeChat identity data and issuing the existing QuickQue session payload.

## Scope

- Implement WeChat login in the Chinese login screen only: `app/modules/auth/screens/LoginScreen.tsx`.
- Add an Android native React Native module that sends a `snsapi_userinfo` authorization request and resolves with the returned `code`.
- Add `POST /v1/auth/wechat` on the Go server. The endpoint accepts `code`, exchanges it with WeChat using server-side credentials, fetches user info, and returns the same session payload used by Google and Microsoft login.
- Store the WeChat AppID in client-accessible Android native configuration, but keep AppSecret server-side only through environment variables.

## Architecture

Android owns only the user authorization hop. It calls the WeChat SDK with `appid=wx070caa369d489329`, receives a one-time `code`, and passes that code to JavaScript. JavaScript posts the code to the Go API.

The Go auth service owns trusted exchange and account binding. It reads `QQA_WECHAT_APP_ID` and `QQA_WECHAT_APP_SECRET`, calls WeChat OAuth token and userinfo APIs, maps the stable provider subject to `unionid` when present or `openid` otherwise, persists an `auth_identities` row with provider `wechat`, and issues the existing JWT/refresh session.

## Error Handling

- If Android has no current activity, no WeChat install, send failure, cancel, or deny, the native module rejects with a specific code.
- If server credentials are missing, `/auth/wechat` returns `503`.
- If WeChat rejects the code or the identity payload has no stable subject, `/auth/wechat` returns `401`.
- Other WeChat network or payload failures return `503` so the client can show a retryable sign-in failure.

## Testing

- Add auth service tests proving WeChat login creates a user and identity.
- Add HTTP handler tests proving `/auth/wechat` returns a session payload and validates missing codes.
- Run focused Go tests for `internal/auth` and `internal/adapters/httpapi`.
- Run TypeScript type checking if the app has a local check command available; otherwise run the closest package validation command that exists.

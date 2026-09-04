# FluxA Login Design

## Goal

Replace the app's email/password login entry with a FluxA login flow backed by
the two deployed New API sites. A user must choose a site before entering
credentials. A successful New API login must create a normal Knowvia session.

The two supported sites are:

- Paid: `https://fluxa.camila.qzz.io`
- Free/public-benefit: `https://free.camila.qzz.io`

Accounts from the two sites are independent, even when they have the same
username or email address.

## User Experience

### Existing login screen

The existing social sign-in choices remain unchanged. The current email login
entry is replaced by a `FluxA Login` entry.

Pressing `FluxA Login` navigates to the dedicated native FluxA login screen.
That screen starts with no selected site and immediately opens an app-style
bottom sheet or selection modal with two choices:

- `Paid site` — `fluxa.camila.qzz.io`
- `Free site` — `free.camila.qzz.io`

The UI stores only the fixed site identifier (`paid` or `free`), not an
arbitrary URL.

### FluxA login screen

The screen contains:

- A site selector at the top.
- Username and password fields.
- A primary login button.
- An inline or toast-style error area consistent with the existing app.

The user must explicitly select a site. Until a site is selected, both
credential fields and the login button are disabled and visually muted behind
the site-selection modal. Closing the modal without choosing leaves the screen
in this disabled state; tapping the site selector opens it again. Selecting a
site closes the modal and enables credential entry. Changing the selected site
clears the password so credentials are not accidentally sent to a different
service.

The page submits a username and password. It does not add registration,
password recovery, passkey, or third-party OAuth controls. If New API requests
two-factor authentication, the first version reports that the account requires
2FA and asks the user to complete login on the site; native 2FA is outside this
change.

## Architecture

### App responsibilities

The app owns the site-selection and credential-entry UI. It maps the two site
identifiers to compile-time HTTPS origins and sends credentials directly to the
selected New API origin:

`POST {origin}/api/user/login`

Request body:

```json
{
  "username": "...",
  "password": "..."
}
```

The app parses the New API response. A normal successful response contains an
`access_token` and a user object. A response with `require_2fa: true` is not a
complete login and is handled as described above.

The app immediately exchanges the short-lived New API access token for a
Knowvia session:

`POST /v1/auth/fluxa`

Request body:

```json
{
  "site": "paid",
  "accessToken": "..."
}
```

The New API token is kept only in memory for the exchange. It is not written to
Expo SecureStore, logs, error text, or analytics. On success, the app stores the
returned Knowvia `SessionPayload` through the existing auth store and replaces
the route with the runs tab.

### Knowvia server responsibilities

The server owns trust and local identity creation. It accepts only the fixed
site identifiers `paid` and `free`. Each maps to a hard-coded or server-owned
configuration value for the corresponding HTTPS origin. Client-provided URLs
are rejected, preventing SSRF and untrusted identity-provider selection.

For a login exchange, the server calls:

`GET {origin}/api/user/self`

with `Authorization: Bearer {New API access token}`. The server applies a short
HTTP timeout, requires a successful New API response, and validates that the
returned user has a positive numeric ID and a non-empty username. The response
body and token are not logged.

The local auth provider is site-specific:

- `fluxa_paid`
- `fluxa_free`

The New API numeric user ID, serialized as a decimal string, is the provider
subject. Consequently, `(fluxa_paid, 42)` and `(fluxa_free, 42)` are distinct
identities and distinct Knowvia users.

After verification, the server reuses the existing external-identity flow:

1. Look up a user by `(provider, provider subject)`.
2. If absent, create a Knowvia user and its auth identity.
3. If present, update safe profile fields such as display name, email, and
   avatar when supplied by New API.
4. Ensure the user's default chat-model configuration exists.
5. Issue the existing Knowvia access and refresh token pair.

Username generation must tolerate collisions because the same username can
appear on both sites or already belong to a password/social account. The local
username is an internal unique value derived from the site and New API user ID;
the New API username is used as the display name when no better display name is
available. Users are never merged by username or email.

### Interfaces and boundaries

The server-side New API verifier is an interface with one operation:

```text
Verify(ctx, site, accessToken) -> FluxAIdentity | error
```

The production implementation performs the HTTPS request. Auth service tests
use a stub implementation, keeping network behavior separate from identity and
session behavior. HTTP-client tests cover request URL, bearer-token placement,
timeouts, response validation, and secret-safe errors.

## Data Flow

1. User presses `FluxA Login` and the app opens the native FluxA login screen.
2. The new screen opens the site-selection modal while credential inputs remain
   disabled.
3. User chooses paid or free site.
4. The login screen enables credential entry for that site.
5. User enters username and password and submits.
6. App calls the selected New API `/api/user/login` endpoint.
7. New API returns a short-lived access token and user session data.
8. App sends only the site identifier and access token to Knowvia
   `/v1/auth/fluxa`.
9. Knowvia validates the token against the selected site's `/api/user/self`.
10. Knowvia resolves or creates the site-scoped local identity and issues its
    own session.
11. App stores the Knowvia session and enters the runs tab.

## Error Handling

- No site selected: fields and submit button remain disabled; no request is
  sent.
- Invalid username/password: show the message returned by New API when safe,
  otherwise show a localized generic login failure.
- 2FA required: explain that native 2FA is not supported in this version and do
  not exchange an incomplete login.
- New API unavailable or timed out: show a site-unavailable message and allow
  retry without clearing the username.
- Knowvia exchange rejected: discard the New API access token and show a
  generic identity-verification failure.
- Unsupported site identifier: return a validation error without making an
  outbound request.
- Invalid/expired New API token: return unauthorized without creating or
  updating a local user.

Errors must never include credentials, bearer tokens, full upstream response
bodies, or internal stack traces.

## Localization

Add translation keys for the FluxA login entry, screen title, site selector,
paid/free site labels, disabled-site hint, site-unavailable error, verification
error, and 2FA-required error. All language dictionaries currently supported by
the app receive values; English text may be used as the fallback where the
existing localization layer supports fallback behavior.

## Testing

### App

- The existing email-login entry is no longer rendered.
- Pressing `FluxA Login` opens the native login screen and its site selector.
- Paid and free choices select the correct fixed site identifier.
- Credential inputs and submit are disabled before site selection.
- Selecting a site enables the fields; changing sites clears the password.
- Login uses the correct fixed New API origin.
- A normal New API response triggers the Knowvia exchange and existing session
  storage.
- Invalid credentials, network failure, incomplete 2FA, and exchange failure
  produce user-facing errors without persisting the New API token.

### Server

- Paid and free identities with the same New API user ID resolve to different
  Knowvia users.
- A repeated login for the same site and user ID reuses the same Knowvia user.
- Unknown sites are rejected before any network call.
- Invalid and expired upstream tokens return unauthorized and create no user.
- Malformed `/api/user/self` payloads are rejected.
- The verifier calls only the configured HTTPS host and sends the access token
  only as a bearer header.
- Existing password, Google, and Microsoft login tests remain green.

## Configuration and Operations

The initial site allowlist is fixed to the two approved domains. If environment
configuration is used, it may override only the origin associated with a known
site identifier; it must not permit a client-supplied host.

Deployment requires outbound HTTPS access from the Knowvia server to both New
API sites. No FluxA client secret or OAuth setup is required.

## Out of Scope

- FluxA registration and password recovery.
- Native New API 2FA, passkey, or third-party OAuth login.
- Linking or merging paid-site and free-site identities.
- Importing a FluxA API key, balance, models, or billing information into
  Knowvia.
- Replacing the Knowvia access/refresh token format.

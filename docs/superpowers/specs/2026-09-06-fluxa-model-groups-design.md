# FluxA Model Groups Design

## Goal

For users who sign in through FluxA, expose the account's FluxA group and the
models available to that group in a new Settings entry. The entry is not shown
for any other login method. Display the supplied RetroArch Simple Icons SVG at
the FluxA login entry.

## Chosen Approach

Knowvia's backend owns FluxA API access after sign-in. The app never receives
or persists a FluxA access token. This keeps the credential out of client
storage and makes future FluxA-backed features use one authenticated backend
boundary.

## Authentication and Credential Storage

1. The app continues to submit the selected FluxA site, username, and password
   to Knowvia's FluxA login endpoint over HTTPS.
2. Knowvia authenticates against the configured FluxA origin, verifies the
   resulting access token, and establishes the normal Knowvia session.
3. The returned FluxA access token is encrypted with AES-GCM before storage.
   The encryption key comes only from a required deployment environment
   variable. Tokens are neither included in API responses nor logged.
4. A credential record is keyed by Knowvia user and FluxA site. Signing in
   again replaces the stored token for that user/site.
5. If FluxA returns an invalid-token response later, Knowvia returns a safe
   re-authentication error. The app asks the user to sign in to FluxA again;
   it does not silently fall back to a different login type.

## FluxA Model Group API

Add an authenticated endpoint:

```
GET /v1/fluxa/model-groups
```

The endpoint is available only to a current user with a stored FluxA
credential. It sends that credential only to the fixed, site-specific FluxA
origin and obtains the account group from `/api/user/self` and its accessible
models from `/api/models`. The response contains only normalized group names
and model display/identifier fields. It never exposes upstream access tokens,
passwords, or unbounded upstream response bodies.

The backend groups the model list under the account group returned by FluxA.
If the upstream payload identifies multiple groups, each model is returned
under every matching group; models without an explicit group are returned
under the account's default group. Empty groups are represented with an empty
model list rather than omitted.

## Session and UI

The login session response includes an optional `fluxaSite` (`paid` or
`free`). The app persists it with the normal Knowvia session. Only a session
with this field renders the new Profile/Settings row named “模型分组”.

Selecting that row opens a read-only Model Groups screen. It loads the new
Knowvia endpoint and displays a section per group, followed by the models in
that section. The screen includes loading, empty, retry, and token-expired
states. Existing custom-model configuration remains unchanged.

The FluxA login entry uses a checked-in version of the supplied RetroArch SVG
rather than a remote image URL. The local icon follows the active theme color
and works offline.

## Error Handling

- Non-FluxA users cannot call the model-groups endpoint and never see its UI
  entry.
- A missing encrypted credential returns a safe forbidden/not-connected
  response.
- Upstream unavailability returns a retryable service-unavailable response.
- Invalid or expired upstream credentials return a re-login-required response.
- Malformed upstream group/model payloads return a safe upstream-data error;
  raw payloads are not propagated.

## Tests

- FluxA credential login stores an encrypted token and does not expose it in
  the session response.
- The model-groups endpoint sends its token only to the configured FluxA
  origin, normalizes the upstream group/model response, and handles expired
  tokens safely.
- Non-FluxA sessions cannot retrieve groups.
- The profile row appears only for a session with `fluxaSite`.
- The model-groups screen renders group models and its loading/error states.
- The FluxA login entry renders the local icon.

## Out of Scope

- Selecting a FluxA model as Knowvia's active chat model.
- Editing FluxA groups or models.
- Persisting FluxA usernames or passwords.
- Linking a FluxA identity to an existing non-FluxA identity.

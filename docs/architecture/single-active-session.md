# Single active session: frontend note

For every portal. An account can be signed in on one device or browser at a time. Signing in somewhere new signs the previous place out.

## What the old device sees

| Where | Response | Do |
|-------|----------|----|
| Any API call | **401** | Clear tokens and go to login. Do not retry. |
| Token refresh `POST /oauth2/token` | **400** `invalid_grant` | Treat as signed out |
| Login button (`/oauth2/authorize`) | Login page, even if the browser was signed in before | Nothing special |

When the gateway update ships, the 401 will have a body:

```json
{ "code": "SESSION_REPLACED" }
```

Show: "You were signed out because your account signed in on another device." For `{"code": "LOGOUT"}`, no body, or any other code, use the normal signed-out handling. Until then the 401 has no body, so the reason can't be shown.

## When the gateway cannot check sign-out

Also with the gateway update, any API call can return:

```
503  Retry-After: 1
{ "code": "REVOCATION_CHECK_UNAVAILABLE" }
```

The gateway could not reach the store that records sign-outs, so it refused the call to be safe. The user is **not** signed out.

- Retry the same call after about a second, up to 3 times.
- Do **not** clear tokens or send the user to login.
- If it still fails, show the generic "temporarily unavailable" error.

## Tabs in the same browser

Share tokens between tabs (for example through `localStorage`). If a second tab runs its own login flow, the first tab's tokens are revoked and its next call gets 401.

## 2FA users

The other device is signed out as soon as the password step succeeds, before OTP. The OTP flow is unchanged.

## After this release

Users already signed in are asked for their password once, the next time the portal sends them to login.

## Do not infer

- A 503 `REVOCATION_CHECK_UNAVAILABLE` does not mean the user was signed out.
- A refresh `invalid_grant` does not mean the session was replaced. It can also mean expired. Do not show the "another device" message for it.
- The JWT has a new `sid` claim. It is opaque: do not parse or display it.

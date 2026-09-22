# Hekato-Go Admin HTTP API

Reference for building the React dashboard. Derived from `proxy/handler.go` (`handleAdminAPI`), `proxy/admin_apikeys.go`, `providers/{kiro,codebuddy,grok}/routes.go`, and the existing vanilla UI in `web/app.js`.

- Base path: `/admin/api`
- All responses: `Content-Type: application/json; charset=utf-8`
- Unknown route: `404 {"error":"Not Found"}`
- Generic error shape everywhere: `{"error": string}` (some auth-poll endpoints add `"success": false`)
- Timestamps are Unix **seconds** unless noted (export uses milliseconds).
- Field names are exactly the Go JSON tags (camelCase).

---

## 1. Auth / Setup

### Authentication mechanism

Every request except `GET /setup/status` and `POST /setup` must carry the admin password in **one** of:

- header `X-Admin-Password: <password>` (what the current UI uses), or
- cookie `admin_password=<password>`

Comparison is constant-time. Failures:

| Condition | Status | Body |
|---|---|---|
| Instance not yet configured (no password set) | 401 | `{"error":"Setup required","setupRequired":"true"}` (note: string `"true"`) |
| Wrong/missing password | 401 | `{"error":"Unauthorized"}` |

There is no session/login endpoint. The UI "logs in" by calling `GET /status` with the header and treating 200 as success. Password is kept client-side (sessionStorage or localStorage with a 72h expiry, "remember me").

### GET /setup/status
Unauthenticated. Tells the client whether first-run setup is done.

Response: `{"configured": bool}`

### POST /setup
Unauthenticated; only works while unconfigured.

Request: `{"password": string}` (required, trimmed length >= 8, body capped at 64 KiB)

Response: `{"success": true}`

Errors: `400 Invalid JSON`, `400 Password must be at least 8 characters`, `409 Already configured` (also 409 on `config.CompleteSetup` refusal).

After success the UI immediately uses the new password as the admin header.

---

## 2. Status & Stats

### GET /status
Used for login probe, dashboard header stats, and the "Service Statistics" modal.

Response:
```json
{
  "version": "1.1.2",
  "accounts": 5,
  "available": 4,
  "totalRequests": 1200,
  "successRequests": 1180,
  "failedRequests": 20,
  "totalTokens": 3456789,
  "totalCredits": 12.5,
  "uptime": 86400
}
```
`accounts` = pool size, `available` = pool accounts currently usable, `uptime` in seconds.

### GET /stats
Same as `/status` minus `version`/`accounts`/`available`: `totalRequests, successRequests, failedRequests, totalTokens, totalCredits, uptime`.

### POST /stats/reset
Zeroes all counters (memory + persisted). No body. Response `{"success": true}`.

### GET /version
Response `{"version": "1.1.2"}`. (Update check in the UI is client-side: it fetches `https://raw.githubusercontent.com/billyriantono/Hekato-Go/main/version.json` -> `{version, download, changelog}` and compares.)

---

## 3. Accounts

### Account object (list view, `GET /accounts`)
Sensitive fields are omitted; use `/accounts/{id}/full` for tokens.

| field | type | notes |
|---|---|---|
| id | string | |
| email | string | label for CodeBuddy/grok keys |
| userId | string | |
| nickname | string | |
| authMethod | string | `idc` \| `social` \| `external_idp` \| `codebuddy` \| `codebuddy-cn` \| `grok` |
| provider | string | `BuilderId`, `Google`, `Github`, `AzureAD` (forced when authMethod=external_idp), `CodeBuddy`, `CodeBuddy CN`, `grok`, … |
| region | string | e.g. `us-east-1`, `global`, `cn` |
| enabled | bool | |
| banStatus | string | `""`/`ACTIVE` normal; otherwise banned/suspended |
| banReason | string | |
| banTime | int64 | unix s |
| expiresAt | int64 | access-token expiry, unix s |
| hasToken | bool | accessToken non-empty |
| machineId | string | UUID |
| weight | int | 0-1 normal, 2+ priority |
| overageStatus | string | `ENABLED` \| `DISABLED` \| `""` |
| overageCapability | string | `OVERAGE_CAPABLE` or other |
| overageCap | float | USD |
| overageRate | float | USD |
| currentOverages | float | USD |
| overageCheckedAt | int64 | unix s |
| proxyURL | string | per-account proxy override |
| relayURL | string | per-account relay override |
| hasRelaySecret | bool | |
| subscriptionType | string | raw upstream (`FREE`, `PRO`, `PRO_PLUS`, `POWER`…) |
| subscriptionTitle | string | |
| daysRemaining | int | |
| usageCurrent / usageLimit / usagePercent | float | main quota |
| nextResetDate | string | |
| lastRefresh | int64 | unix s |
| trialUsageCurrent / trialUsageLimit / trialUsagePercent | float | |
| trialStatus | string | |
| trialExpiresAt | int64 | |
| requestCount | int64 | runtime (pool) |
| errorCount | int64 | runtime |
| totalTokens | int64 | runtime |
| totalCredits | float | runtime |
| lastUsed | int64 | runtime, unix s |

### GET /accounts
Response: JSON **array** of account objects (not wrapped).

### GET /accounts/{id}/full
Same fields as above **plus** `accessToken, refreshToken, clientId, clientSecret` (no `hasToken`). Used by "Copy JSON". Error `404 {"error":"Account not found"}`.

### POST /accounts
Raw add (decodes the full `config.Account` struct; the UI does not use this, it uses the auth-flow endpoints). Request: any `config.Account` JSON; `id` auto-generated if missing, `region` defaults `us-east-1`. Response `{"success": true, "id": string}`. Errors 400/500.

### PUT /accounts/{id}
Partial update; only these keys are honored:

| field | type | validation |
|---|---|---|
| enabled | bool | |
| nickname | string | |
| machineId | string | UI validates UUID or 32-hex |
| weight | number | cast to int |
| proxyURL | string | must start `http://`, `https://`, `socks5://`, `socks5h://` or be `""`; non-empty clears relayURL/relaySecret |
| relayURL | string | must start `http://`/`https://` or `""`; non-empty clears proxyURL; `""` clears relaySecret |
| relaySecret | string | only applied when non-empty (blank keeps existing) |

Response `{"success": true}`. Errors: `400 Invalid JSON`, `400 {"error":"invalid proxyURL"}`, `400 {"error":"invalid relayURL"}`, `404 Account not found`, 500.
Side effect: disabled->enabled with a token triggers async model-cache refresh.

### DELETE /accounts/{id}
Response `{"success": true}`; 500 on error.

### POST /accounts/batch
Request: `{"ids": string[] (required, non-empty), "action": "enable" | "disable" | "refresh"}`

Responses:
- enable/disable: `{"success": true, "count": n}` (enable also resets banStatus to `ACTIVE`)
- refresh: `{"success": true, "refreshed": n, "failed": n}` (refreshes token if refreshToken present, then account info)

Errors: `400 Invalid JSON`, `400 No account IDs provided`, `400 Invalid action: x`.

Note: there is **no** batch delete / batch model-refresh endpoint; the UI loops `DELETE /accounts/{id}` and `POST /accounts/{id}/models/refresh` per id.

### POST /accounts/{id}/refresh
Refreshes token (if near expiry or on 401/403) then upstream account/usage info. No body.

Response: `{"success": true, "info": AccountInfo}` where `AccountInfo` has PascalCase keys (no json tags): `Email, UserId, SubscriptionType, SubscriptionTitle, DaysRemaining, UsageCurrent, UsageLimit, UsagePercent, NextResetDate, LastRefresh, TrialUsageCurrent, TrialUsageLimit, TrialUsagePercent, TrialStatus, TrialExpiresAt`.
If the account was found suspended: `{"success": true, "message": "Account status updated"}`.
Errors: 404, `500 Token refresh failed: …`, 500 other. UI just re-fetches `/accounts` afterwards.

### POST /accounts/{id}/test
Sends a tiny real chat request ("say ok") through the account.

Request (optional): `{"model": string}` default `claude-sonnet-4` (thinking suffix honored).
Response: `{"success": true, "reply": string, "model": string}`
Errors: 404, `500 Token refresh failed: …`, 500 upstream error message.

### GET /accounts/{id}/models
Live-fetches the model list from upstream and updates routing cache.

Response: `{"success": true, "models": ModelInfo[]}`
```json
{"modelId":"claude-sonnet-4","modelName":"Claude Sonnet 4","description":"","supportedInputTypes":["TEXT"],"rateMultiplier":1,"tokenLimits":{"maxInputTokens":200000,"maxOutputTokens":8192}}
```
`tokenLimits` may be `null`. Errors 404 / 500.

### GET /accounts/{id}/models/cached
Response `{"success": true, "models": string[]}` (model IDs only, from cache; empty array if none).

### POST /accounts/{id}/models/refresh
Response `{"success": true, "count": n}`; 404 / 500.

### POST /accounts/models/refresh
Refresh model cache for all enabled accounts. Response `{"success": true, "refreshed": <total cached models>, "failed": 0}`.

### GET /accounts/{id}/overage
Pulls upstream Overages status (AWS Q) and persists it.

Response:
```json
{"success":true,"overageStatus":"ENABLED","overageCapability":"OVERAGE_CAPABLE","subscriptionTitle":"Pro","overageCap":50,"overageRate":0.04,"currentOverages":1.2,"overageCheckedAt":1758500000}
```
Errors: 404, `502 {"error": upstream}`.

### POST /accounts/{id}/overage
Request `{"enabled": bool}`. Response same shape as GET. Errors 400/404/502.

### POST /export
Request (optional): `{"ids": string[]}`; empty/invalid body = export all.
Response is a Kiro-Account-Manager-compatible document (includes secrets):
```json
{
  "version":"1.1.2","exportedAt":1758500000000,
  "accounts":[{
    "id":"…","email":"…","nickname":"…","idp":"BuilderId","userId":"…","machineId":"…",
    "credentials":{"accessToken":"…","csrfToken":"","refreshToken":"…","clientId":"…","clientSecret":"…","region":"us-east-1","expiresAt":1758500000000,"authMethod":"IdC","provider":"BuilderId"},
    "subscription":{"type":"Pro","title":"…"},
    "usage":{"current":0,"limit":0,"percentUsed":0,"lastUpdated":1758500000000},
    "tags":[],"status":"active","createdAt":1758500000000,"lastUsedAt":1758500000000
  }],
  "groups":[],"tags":[]
}
```
`subscription.type` in `Free | Pro | Pro_Plus`; `credentials.expiresAt` is **ms**; `authMethod` `idc` is written as `IdC`.

### GET /generate-machine-id
Response `{"machineId": "<uuid>"}`.

---

## 4. Account auth flows (per provider)

All return the created account on success and require the admin header. Each `start` returns a `sessionId`; polling endpoints take it back. Poll errors are `400 {"success": false, "error": "…"}` (session expired/denied/unknown).

### 4.1 Kiro — generic credential import
`POST /auth/credentials` (core handler; produces idc / social / external_idp accounts, or CodeBuddy when `apiKey` given). Body capped at 1 MiB.

Request fields (all strings, all optional unless noted):

| field | notes |
|---|---|
| refreshToken | **required** for Kiro paths |
| accessToken | optional; for external_idp with a valid JWT `exp` it is trusted on import (no refresh round-trip) |
| clientId, clientSecret | idc needs both; external_idp needs clientId only |
| authMethod | `social`/`google`/`github` -> social; `idc`/`builderid`/`enterprise` -> idc; `external_idp`/`azuread`/`azure`/`entra`/`entra-id`/`entra_id`/`microsoft`/`m365`/`office365`/`external` -> external_idp; empty -> idc if clientId else social |
| provider | display provider (`Google`, `Github`, `BuilderId`, `AzureAD`…); forced `AzureAD` for external_idp |
| region | default `us-east-1` |
| tokenEndpoint, issuerUrl, scopes | external_idp; derived from `userId`/accessToken JWT when omitted; validated against an allow-list |
| id, email, profileArn, userId | identity preservation when pasting a full record; `id` reused only if not colliding |
| apiKey, label, variant | CodeBuddy shortcut (same semantics as `/auth/codebuddy`) |

Response: `{"success": true, "account": {"id": string, "email": string}}` (CodeBuddy branch also returns `authMethod`, `provider`).

Errors (400): `Invalid JSON`, `refreshToken is required`, `apiKey is required`, `external_idp requires clientId and tokenEndpoint (or userId/accessToken to derive it)`, `external IdP endpoint rejected: …`, `external IdP issuer rejected: …`, `Token refresh failed: …`; 500 on persist failure.

UI flows that hit this endpoint:
- **Kiro Local Cache**: user pastes/uploads `kiro-auth-token.json` (+ `{hash}.json` for IdC). Payload `{refreshToken, accessToken, clientId, clientSecret, region, authMethod: 'idc'|'social', provider: 'BuilderId'|'Enterprise'|'Google'|'Github'}`.
- **Credentials JSON**: accepts single object, array, `{accounts:[…]}` Kiro Account Manager export, or line format `x----x----refreshToken----clientId----clientSecret`. One request per item; client normalizes authMethod (`EXTERNAL_IDP` alias list or presence of `tokenEndpoint` -> external_idp).
- **Kiro Web Cookie**: `{refreshToken, accessToken:'', clientId:'', clientSecret:'', authMethod:'social', provider:'Google'|'Github'}`.

After any successful add the UI calls `POST /accounts/{id}/refresh` then reloads the list.

### 4.2 Kiro — AWS Builder ID (device code)
1. `POST /auth/builderid/start` body `{"region": string}` (optional)
   -> `{"sessionId", "userCode", "verificationUri", "interval": int}`
2. Show `userCode` + open `verificationUri`.
3. Loop: `POST /auth/builderid/poll` body `{"sessionId"}` every `interval` seconds
   - pending: `{"success": true, "completed": false, "status": "pending"|"slow_down", "interval": int}` (use the returned interval)
   - done: `{"success": true, "completed": true, "account": {"id","email"}}` (account authMethod `idc`, provider `BuilderId`)
   - failure: `400 {"success": false, "error"}`; 500 on persist.

### 4.3 Kiro — IAM Identity Center (enterprise SSO, manual callback)
1. `POST /auth/iam-sso/start` body `{"startUrl": string (required), "region": string}`
   -> `{"sessionId", "authorizeUrl", "expiresIn": int}`; errors `400 startUrl is required`, 500.
2. User opens `authorizeUrl`, completes login, copies the final callback URL.
3. `POST /auth/iam-sso/complete` body `{"sessionId", "callbackUrl"}`
   -> `{"success": true, "account": {"id","email"}}` (authMethod `idc`); errors 400 (exchange failed), 500.

### 4.4 Kiro — Hosted portal SSO (Microsoft 365 / Entra ID, Google, GitHub)
Redirect lands on `127.0.0.1:3128` on the proxy host, so the browser must be on the same machine or the operator relays the URL manually.

1. `POST /auth/kiro-sso/start` body `{"region": string}` (optional, UI sends `{}`)
   -> `{"sessionId", "signInUrl", "interval": 2}`
2. Open `signInUrl` in a browser.
3. Loop `POST /auth/kiro-sso/poll` body `{"sessionId"}` every 2 s
   - `{"success": true, "completed": false, "status": "pending"}`
   - `{"success": true, "completed": true, "account": {"id","email","authMethod"}}` (authMethod `external_idp` for Azure tenant, else `social`)
   - `400 {"success": false, "error"}`
4. Cross-machine relay (optional, can be called twice): `POST /auth/kiro-sso/relay` body `{"sessionId", "url": "<pasted redirect URL>"}`
   -> `{"success": true, "done": bool, "authorizeUrl": string}`. If `authorizeUrl` non-empty, the operator must open it next (enterprise leg 2). When `done` is true, the next poll completes. Errors `400 sessionId and url are required`, `400 {"success": false, "error"}`.
5. On cancel/close: `POST /auth/kiro-sso/cancel` body `{"sessionId"}` -> `{"success": true}` (frees the loopback port).

### 4.5 Kiro — SSO token (x-amz-sso_authn cookie), batch
`POST /auth/sso-token` body `{"bearerToken": string (required; multiple tokens separated by newline), "region": string}`

Response `{"success": true, "accounts": [{"id","email"}], "errors": string[]}`; all-failed -> `500 {"success": false, "error": "e1; e2"}`; `400 bearerToken is required`. Accounts are authMethod `idc`.

### 4.6 CodeBuddy — API key
`POST /auth/codebuddy` body `{"apiKey": string (required), "label": string, "variant": "global"|"cn"|"china", "region": string}`
CN detection: variant `cn`/`china` or region `cn`/contains `china` -> provider `CodeBuddy CN`, authMethod `codebuddy-cn`, region `cn`; otherwise provider `CodeBuddy`, authMethod `codebuddy`, region `global`.

Response `{"success": true, "account": {"id","email","authMethod","provider"}}`; errors `400 apiKey is required`, 500. Model cache is refreshed synchronously.

### 4.7 Grok (xAI)
Device code:
1. `POST /auth/grok/start` (empty body) -> `{"sessionId","userCode","verificationUri","interval"}`
2. Loop `POST /auth/grok/poll` `{"sessionId"}`: pending `{"success":true,"completed":false,"status":"pending"|"slow_down","interval"}`; done `{"success":true,"completed":true,"account":{"id","email"}}`; `400 {"success":false,"error"}`.

Import: `POST /auth/grok/import` — body is raw grok token export: single object, JSON array, or NDJSON (16 MiB cap). Entry shape:
```json
{"email":"…","password":"…","tokens":{"access_token":"…","refresh_token":"…","expires_at":"RFC3339","expires_in":3600,"email":"…","client_id":"…","id_token":"…"}}
```
Response `{"success": true, "imported": n, "accounts": [{"id","email"}], "errors": string[]}`; `400` on parse error / `no grok account entries found`; all-failed `500 {"success":false,"error"}`.

---

## 5. API Keys (`proxy/admin_apikeys.go`)

### ApiKey view object
| field | type | notes |
|---|---|---|
| id | string | |
| name | string | omitempty |
| keyMasked | string | e.g. `sk-ab…yz` |
| enabled | bool | |
| migrated | bool | omitempty; legacy single key migrated from old config |
| createdAt | int64 | unix s |
| lastUsedAt | int64 | omitempty |
| tokenLimit | int64 | 0 = unlimited (omitempty) |
| creditLimit | float | 0 = unlimited (omitempty) |
| rpmLimit | int64 | requests/min, 0 = unlimited (omitempty) |
| concurrencyLimit | int64 | 0 = unlimited (omitempty) |
| tokensUsed | int64 | |
| creditsUsed | float | |
| requestsCount | int64 | |

### GET /api-keys
Response `{"apiKeys": ApiKeyView[]}`.

### GET /api-keys/{id}
Response: `ApiKeyView`; `404 {"error":"API key not found"}`.

### POST /api-keys
Request: `{"name"?: string, "key"?: string (auto-generated if empty), "enabled"?: bool (default true), "tokenLimit"?: int, "creditLimit"?: number, "rpmLimit"?: int, "concurrencyLimit"?: int}`
Response: `{"success": true, "id": string, "key": "<cleartext, shown once>", "apiKey": ApiKeyView}`; `400 Invalid JSON` / `400 {"error": validation}`.

### PUT /api-keys/{id}
Request: any subset of `name, key, enabled, tokenLimit, creditLimit, rpmLimit, concurrencyLimit` (pointer semantics: absent = unchanged).
Response `{"success": true, "apiKey": ApiKeyView}`; 404, 400, 500.

### DELETE /api-keys/{id}
Response `{"success": true}`; 500.

### POST /api-keys/{id}/reset-usage
Response `{"success": true, "apiKey": ApiKeyView}`; `404 {"error": …}`.

---

## 6. Settings

### 6.1 General — GET /settings
Response:
```json
{"apiKey":"sk-…","requireApiKey":true,"port":8080,"host":"0.0.0.0","allowOverUsage":false}
```
`apiKey` is the legacy single key (kept for backward compat; the UI now manages keys in section 5).

### POST /settings
Request (all optional, patch semantics): `{"apiKey"?: string, "requireApiKey"?: bool, "password"?: string, "allowOverUsage"?: bool}`
- `requireApiKey`: whether `/v1/*` requires `Authorization: Bearer`.
- `password`: change admin password (UI: "Change Password"; the client must then use the new header value).
- `allowOverUsage`: rebuilds pool immediately.
Response `{"success": true}`; 400 / 500.

### 6.2 Thinking — GET /thinking
Response `{"suffix": string, "openaiFormat": string, "claudeFormat": string}`; formats in `reasoning_content | thinking | think`.

### POST /thinking
Request `{"suffix": string, "openaiFormat": string, "claudeFormat": string}` (formats validated; empty allowed).
Response `{"success": true}`; `400 Invalid openaiFormat, must be: reasoning_content, thinking, or think` (same for claudeFormat); 500.

### 6.3 Kiro endpoint — GET /endpoint
Response `{"preferredEndpoint": "auto"|"kiro"|"codewhisperer"|"amazonq", "endpointFallback": bool}`.

### POST /endpoint
Request `{"preferredEndpoint": string (required, one of the four), "endpointFallback"?: bool}`.
Response `{"success": true}`; `400 Invalid endpoint, must be: auto, kiro, codewhisperer, or amazonq`; 500.

### 6.4 Outbound proxy — GET /proxy
Response `{"proxyURL": string, "useRelay": bool, "proxyPool": string[]}`.
Modes are mutually exclusive: `useRelay=true` means the egress relay is active (and proxyURL is cleared); otherwise proxyURL (`""` = direct). `proxyPool` is a list of proxy URLs that accounts without their own proxy/relay get pinned to.

### POST /proxy
Request `{"proxyURL": string, "useRelay": bool, "proxyPool"?: string[]}`
- `proxyPool` (if present) is trimmed/filtered; each entry must start with `http://`, `https://`, `socks5://`, `socks5h://` else `400 proxy pool entry must start with …: <entry>`.
- `useRelay: true` requires a saved relay URL else `400 configure a Relay URL in the Egress Relay section first`; proxyURL ignored.
- `useRelay: false`: `proxyURL` validated with same scheme rule (`400 proxyURL must start with …`), `""` = direct.
Response `{"success": true}`; 500.
UI builds `proxyURL` as `scheme://[user[:pass]@]host:port` from type/host/port/username/password fields.

### 6.5 Prompt filter — GET /prompt-filter
Response:
```json
{"filterClaudeCode":true,"filterEnvNoise":false,"filterStripBoundaries":false,
 "rules":[{"id":"r1","name":"…","type":"regex"|"lines-containing","match":"…","replace":"…","enabled":true}]}
```
`replace` only for `regex` (empty = delete match).

### POST /prompt-filter
Request: any subset of `filterClaudeCode, filterEnvNoise, filterStripBoundaries` (bool) and `rules` (full array replaces). Response `{"success": true}`; 400 / 500.

---

## 7. Logs

### GET /logs
Response `{"logs": RequestLog[]}` (ring buffer, max 500, newest last per handler order).
```json
{"time":1758500000,"endpoint":"claude","model":"claude-sonnet-4","accountId":"…","status":"success","error":"","errorType":"","tokens":1234,"credits":0.5,"duration":812}
```
`endpoint` in `claude | openai | responses`; `status` in `success | error`; `errorType` categories used by the UI: `quota, auth, suspended, overage, profile, unknown`; `duration` ms.

### DELETE /logs
Response `{"success": true}`.

---

## 8. Egress Relay

### GET /relay
Response `{"relayUrl": string, "hasSecret": bool}` (secret never echoed).

### POST /relay
Request `{"relayUrl": string, "relaySecret": string}`
- blank `relaySecret` keeps the stored one; `relayUrl: ""` disables relay and clears the secret.
- `relayUrl` must start `http://`/`https://` else `400 relayUrl must start with http:// or https://`.
Response `{"success": true}`; 500.

### POST /relay/test
Request (optional) `{"relayUrl": string, "relaySecret": string}` — blank falls back to saved values.
Response is **always HTTP 200**:
- ok: `{"ok": true, "status": <upstream http status>, "detail": "ping ok" | "forwarded to upstream"}`
- fail: `{"ok": false, "status": n, "detail": reason, "error": reason}` where reason is `wrong secret (relay returned 401 unauthorized)` or `target host not on the relay allow-list`
- unreachable: `{"ok": false, "error": "relay unreachable: …"}`
Only `400 {"error":"no relay configured to test"}` is non-200.

### GET /relay/source?platform=cloudflare|vercel|deno
Returns deployable relay code with the shared secret baked in (generates + persists a secret if none exists).
Response `{"platform": string, "filename": string, "language": string, "code": string, "secret": string}`; `400 unknown platform (want cloudflare|vercel|deno)`; 500.

---

## 9. Misc / non-admin endpoints the UI shows
Rendered as copyable "API Endpoints" (base = window origin): `POST /v1/messages` (Claude), `POST /v1/chat/completions` (OpenAI), `POST /v1/responses`, `GET /v1/models`, `GET /v1/stats`. The "View model list" modal fetches `GET /v1/models` directly (no admin header) and reads `data[]`.

Static: `GET /admin/` serves `web/index.html`; `GET /admin/<file>` serves `web/<file>`; locales at `/admin/locales/{en,zh}.json`.

---

## 10. Translation files (`web/locales/en.json`, `zh.json`)
Flat JSON, dotted keys (no nesting), `{0}`/`{count}` placeholders. Prefixes:

`app.*`, `login.*`, `setup.*`, `status.*`, `footer.*`, `stats.*`, `tabs.*` (accounts/settings/api/logs), `accounts.*` (incl. `accounts.testLog.*`), `batch.*`, `filter.*`, `detail.*`, `modal.*` (provider/method cards), `builderid.*`, `iam.*`, `kirosso.*`, `sso.*`, `local.*`, `credentials.*`, `cookie.*`, `codebuddy.*`, `grok.*`, `auth.*`, `subscription.*`, `settings.*`, `apiKeys.*`, `promptFilter.*`, `relay.*` (incl. `relay.steps.*`), `logs.*`, `errors.*`, `api.*`, `export.*`, `update.*`, `models.*`, `privacy.*`, `theme.*`, `lang.*`, `time.*`, `common.*`, `aria.*`.

---

## 11. Existing UI feature inventory (parity checklist)

**Shell**
- Login page: password field with show/hide, "Remember password" (localStorage, 72h expiry) vs session; login = `GET /status` probe.
- First-run setup box (shown when `GET /setup/status` -> `configured:false`): password + confirm, min 8, `POST /setup`, auto-login.
- Header: version badge (`GET /version`), "Check for updates" (GitHub version.json -> toast or update modal with download/changelog), theme toggle (system/light/dark), language toggle (EN/中文), privacy mode (mask emails), logout.
- Stat tiles (from `/status`): accounts, requests, success, failed, tokens, credits, capacity/traffic/reliability.
- Tabs: Accounts, Settings, API, Logs. Footer with status + resources/GitHub link.

**Accounts tab**
- Toolbar: Add Account, Export, Refresh Models (all: `POST /accounts/models/refresh`), search (email/nickname), status filter (all/enabled/disabled/banned).
- Row checkboxes + select all; batch bar: Batch Enable / Disable / Refresh (`/accounts/batch`), Refresh Models (loop per id), Batch Delete (loop `DELETE`).
- Account card: email/nickname, provider/auth badge, region, subscription badge (Free/Pro/Pro+/Power), status badge (active/disabled/expired/no token/banned/suspended), weight badge, overage ON/OFF badge, main quota + trial quota bars, trial expiry, token expiry countdown, requests/tokens/credits stats, per-row actions: Refresh (`/accounts/{id}/refresh`), Test, Details, Enable/Disable toggle (`PUT enabled`), Delete (confirm), Copy JSON (`/accounts/{id}/full`).
- Account Test modal: model select populated from `/accounts/{id}/models/cached` (fallback text), run `POST /accounts/{id}/test`, running test log panel (start/success/failed lines with duration, shows Global Proxy / Egress Relay mode), clear/hide log.
- Account Details modal: basic info (email, userId, auth method, region); Machine ID input + Generate (`/generate-machine-id`) + Save (`PUT machineId`); Request Weight + Save; Overages block: switch (`POST /accounts/{id}/overage`), "Pull from AWS" (`GET`), cap/rate/current/last synced, not-capable notice; Account Outbound: inherit / account proxy (`proxyURL`) / account relay (`relayURL` + `relaySecret`) with format validation + Save; subscription (type, days remaining, token expiry); usage (main, trial, trial status/expiry, reset date); statistics (requests, errors, tokens, credits, credit multiplier); Available Models: Load (`GET /accounts/{id}/models`) and Refresh Route Cache (`POST …/models/refresh`).
- Export modal: checklist of accounts (select all/deselect), Download JSON / Show JSON / Copy JSON via `POST /export {ids}`.

**Add Account modal (multi-step picker)**
- Step 1 choose provider: Kiro / AWS, CodeBuddy, Grok (xAI).
- Kiro methods: AWS Builder ID (device code + poll), IAM Identity Center (start URL + region -> authorize URL open/copy -> paste callback URL -> complete), Enterprise SSO – Microsoft 365 (hosted portal: open sign-in URL, poll, "browser on a different machine" relay URL paste + submit, cancel), SSO Token (textarea, batch one per line, region), Kiro Local Cache (login channel select BuilderId/Enterprise/Google/GitHub, paste or upload `kiro-auth-token.json` and `{hash}.json`, Windows/macOS paths shown), Credentials JSON (textarea: object/array/KAM export/line format; auth method hint), Kiro Web Cookie (RefreshToken + provider GitHub/Google, how-to steps).
- CodeBuddy: API Key form (label, region Global/China, key).
- Grok: Device Code Login (code + verification URL open/copy, waiting, poll) and Import Tokens (JSON textarea).
- Back / Cancel navigation between steps; closing cancels in-flight kiro-sso session.

**Settings tab (sections)**
- API Keys list (`/api-keys`): cards with name, masked key, Migrated/Disabled badges, tokens/credits/requests usage, RPM/concurrency limits, unlimited labels; actions: enable toggle (`PUT enabled`), Edit, Delete (confirm), Reset Usage (confirm); Add Key modal (name, key value or auto, enabled, token limit, credit limit, RPM, max concurrent; "0 = unlimited"); New API Key modal showing cleartext once with copy. "Enable API Key Verification" toggle (`POST /settings requireApiKey`) with warning when no enabled key.
- Usage Control: Allow Over-Usage toggle + Save (`POST /settings allowOverUsage`).
- Thinking Mode: trigger suffix, OpenAI format select, Claude format select (+ "no tag" option), Save (`/thinking`).
- Kiro Endpoint: preferred endpoint select (Auto/Kiro IDE/CodeWhisperer/AmazonQ), fallback toggle, Save (`/endpoint`).
- Outbound Proxy: type (Direct/SOCKS5/HTTP/Egress Relay), host/port, username/password, Proxy Pool textarea (one per line), Save (`/proxy`).
- Egress Relay: relay URL, shared secret (blank keeps stored, "secret stored" hint), Save (`/relay`), Test (`/relay/test` -> ok/fail toast), Deploy Relay buttons Cloudflare / Vercel / Deno (`/relay/source` -> info modal with code, filename, copy, step instructions).
- System Prompt Filter: built-in toggles (Claude Code replace, env noise, strip boundaries), custom rules list (type regex/lines-containing, name, match, replace, enabled, remove), add regex / add line-filter rule, Save (`/prompt-filter`).
- Admin Password: new password + Change Password (`POST /settings password`).
- Statistics: Reset Statistics (confirm, `POST /stats/reset`).

**API tab**
- Endpoint cards (Claude, OpenAI, OpenAI Responses, Models, Stats) with copy buttons; "View model list" modal (fetch `/v1/models`, search box, count); "View statistics" modal (`/status`: version, accounts, available, requests, success, failed, tokens, credits, uptime).

**Logs tab**
- Summary (total/success/errors), filter (all/success/errors), Refresh, Auto-refresh toggle, Clear (confirm, `DELETE /logs`), table: time, status, endpoint, model, account, tokens, duration, detail (error type + message).

**Cross-cutting**
- Toast notifications (success/error/warning/primary), confirm dialog, info modal, custom select enhancement, dialogs close on backdrop/X.

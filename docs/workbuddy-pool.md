# WorkBuddy pool

WorkBuddy accounts use the separate `workbuddy` pool, alongside `codebuddy`
(Global) and `codebuddy-cn` (China). They continue to share the CodeBuddy
provider adapter, while WorkBuddy requests use the existing WorkBuddy endpoint.

Existing accounts identified as WorkBuddy by their provider, authentication
method or region are grouped automatically when the updated gateway starts.
No account re-import or credential migration is needed. This change does not
reclassify Global CodeBuddy accounts as WorkBuddy.

Use `workbuddy/<model-id>` to pin a request to WorkBuddy. Custom pool prefixes,
pool test models, pool-qualified model listings, API-key allowlists such as
`workbuddy/*`, and auto-routing entries recognize the new pool.

## Existing configuration

- `codebuddy/<model-id>` and the `codebuddy/*` allowlist no longer include
  WorkBuddy accounts. Add `workbuddy/*` where access to both pools is intended.
- Custom prefixes and pool test models previously configured for `codebuddy`
  remain assigned to that pool. Configure WorkBuddy's values separately.
- Bare model IDs keep the existing unpinned routing behavior.
- `workbuddy:<pattern>` blacklist entries target only the WorkBuddy pool.
  `codebuddy:<pattern>` retains the existing provider-wide blacklist semantics,
  covering all three CodeBuddy adapter variants, as it already did for China.
- The `workbuddy` prefix is reserved even when no WorkBuddy account exists.
  Rename any old custom prefix for another pool that used this name before
  adopting the new reserved prefix.

Restart or deploy the updated gateway for the new grouping to appear in the
running dashboard. No frontend rebuild is required for this server-computed
pool key.

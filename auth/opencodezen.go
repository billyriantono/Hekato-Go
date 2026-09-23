package auth

// OpenCode Zen uses bearer-token authentication with no refresh flow. Accounts
// store their API key in Account.AccessToken; the import path
// (providers/opencodezen/routes.go) creates accounts with AuthMethod
// "opencode_zen". Free-tier accounts store the literal "public" token; the
// upstream recognises it and serves the free model catalog.
//
// This file exists to mirror the per-provider package layout. There is no
// refresh-token grant to implement — OpenCode Zen keys do not expire and
// cannot be rotated through any public API. Operators rotate keys manually
// via the admin panel (Delete + Re-import).
package auth

// OpenCode Go uses bearer-token authentication with no refresh flow. Accounts
// store their API key in Account.AccessToken; the import path
// (providers/opencodego/routes.go) creates accounts with AuthMethod
// "opencode_go". Free-tier accounts store the literal "public" token; the
// upstream recognises it and serves the free model catalog (subject to the
// same fingerprint gate as OpenCode Zen).
//
// This file exists to mirror the per-provider package layout. There is no
// refresh-token grant to implement — operators rotate keys manually via the
// admin panel (Delete + Re-import).
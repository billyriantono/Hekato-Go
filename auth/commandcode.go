package auth

// CommandCode uses a long-lived bearer API key (normally prefixed `user_`).
// Accounts store it in config.Account.AccessToken; there is no OAuth or token
// refresh endpoint to implement. The import route lives beside the provider in
// providers/commandcode/routes.go, matching other API-key providers.

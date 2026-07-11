package proxy

import "kiro-go/config"

// CallUpstreamAPI routes an already-normalized internal chat payload to the
// selected account's provider. Prefer the typed helpers below when the original
// client request is still available, so provider-specific converters can run.
func CallUpstreamAPI(account *config.Account, payload *KiroPayload, callback *KiroStreamCallback) error {
	if isCodeBuddyAccount(account) {
		return callCodeBuddyChatAPI(account, payload, callback)
	}
	return CallKiroAPI(account, payload, callback)
}

// CallClaudeUpstreamAPI converts a Claude request with the selected provider's
// converter, then dispatches to that provider.
func CallClaudeUpstreamAPI(account *config.Account, req *ClaudeRequest, thinking bool, callback *KiroStreamCallback) error {
	if isCodeBuddyAccount(account) {
		return callCodeBuddyChatRequestAPI(account, ClaudeToCodeBuddy(req, thinking), callback)
	}
	return CallKiroAPI(account, ClaudeToKiro(req, thinking), callback)
}

// CallOpenAIUpstreamAPI converts an OpenAI Chat/Responses request with the
// selected provider's converter, then dispatches to that provider.
func CallOpenAIUpstreamAPI(account *config.Account, req *OpenAIRequest, thinking bool, callback *KiroStreamCallback) error {
	if isCodeBuddyAccount(account) {
		return callCodeBuddyChatRequestAPI(account, OpenAIToCodeBuddy(req, thinking), callback)
	}
	return CallKiroAPI(account, OpenAIToKiro(req, thinking), callback)
}

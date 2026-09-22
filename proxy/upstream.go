package proxy

import "hekato-go/config"

func callUpstreamFromClaude(account *config.Account, req *ClaudeRequest, thinking bool, callback *StreamCallback) error {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return err
	}
	if adapter.chatFromClaude == nil {
		return unsupportedProviderCapability(adapter.kind, "Claude requests")
	}
	return adapter.chatFromClaude(account, req, thinking, callback)
}

func callUpstreamFromOpenAI(account *config.Account, req *OpenAIRequest, thinking bool, callback *StreamCallback) error {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return err
	}
	if adapter.chatFromOpenAI == nil {
		return unsupportedProviderCapability(adapter.kind, "OpenAI requests")
	}
	return adapter.chatFromOpenAI(account, req, thinking, callback)
}

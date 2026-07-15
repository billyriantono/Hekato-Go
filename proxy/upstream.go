package proxy

import "kiro-go/config"

func CallClaudeUpstreamAPI(account *config.Account, req *ClaudeRequest, thinking bool, callback *KiroStreamCallback) error {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return err
	}
	if adapter.claudeChat == nil {
		return unsupportedProviderCapability(adapter.kind, "Claude requests")
	}
	return adapter.claudeChat(account, req, thinking, callback)
}

func CallOpenAIUpstreamAPI(account *config.Account, req *OpenAIRequest, thinking bool, callback *KiroStreamCallback) error {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return err
	}
	if adapter.openAIChat == nil {
		return unsupportedProviderCapability(adapter.kind, "OpenAI requests")
	}
	return adapter.openAIChat(account, req, thinking, callback)
}

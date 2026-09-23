package proxy

import (
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/pool"
	"strings"
)

// isModelUnsupportedError matches upstream replies that mean "this account does
// not serve that model" (CodeBuddy code 11102, generic wording elsewhere).
func isModelUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "\"code\":11102") ||
		strings.Contains(msg, "service info not found") ||
		strings.Contains(msg, "model not found") ||
		strings.Contains(msg, "unsupported model") ||
		strings.Contains(msg, "does not support model") ||
		strings.Contains(msg, "model_not_found")
}

// noteModelUnsupported stops routing model to account after upstream rejected it.
func noteModelUnsupported(account *config.Account, model string, err error) {
	if account == nil || !isModelUnsupportedError(err) {
		return
	}
	pool.GetPool().DenyModel(account.ID, model)
	logger.Warnf("[Routing] %s does not serve %s; excluded until its model list is refreshed", account.Email, model)
}

func callUpstreamFromClaude(account *config.Account, req *ClaudeRequest, thinking bool, callback *StreamCallback) error {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return err
	}
	if adapter.chatFromClaude == nil {
		return unsupportedProviderCapability(adapter.kind, "Claude requests")
	}
	err = adapter.chatFromClaude(account, req, thinking, callback)
	noteModelUnsupported(account, req.Model, err)
	return err
}

func callUpstreamFromOpenAI(account *config.Account, req *OpenAIRequest, thinking bool, callback *StreamCallback) error {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return err
	}
	if adapter.chatFromOpenAI == nil {
		return unsupportedProviderCapability(adapter.kind, "OpenAI requests")
	}
	err = adapter.chatFromOpenAI(account, req, thinking, callback)
	noteModelUnsupported(account, req.Model, err)
	return err
}

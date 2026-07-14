// Type and function aliases bridging the proxy core to the shared providers
// package after the per-provider package split. They keep the streaming
// handlers and translators source-compatible; new code should reference
// kiro-go/providers directly.
package proxy

import (
	"kiro-go/providers"
	"kiro-go/providers/kiro"
)

type (
	KiroPayload                  = providers.KiroPayload
	KiroUserInputMessage         = providers.KiroUserInputMessage
	UserInputMessageContext      = providers.UserInputMessageContext
	KiroToolWrapper              = providers.KiroToolWrapper
	InputSchema                  = providers.InputSchema
	KiroToolResult               = providers.KiroToolResult
	KiroResultContent            = providers.KiroResultContent
	KiroImage                    = providers.KiroImage
	KiroHistoryMessage           = providers.KiroHistoryMessage
	KiroAssistantResponseMessage = providers.KiroAssistantResponseMessage
	KiroToolUse                  = providers.KiroToolUse
	InferenceConfig              = providers.InferenceConfig
	KiroStreamCallback           = providers.KiroStreamCallback
	ModelInfo                    = providers.ModelInfo
	OpenAIRequest                = providers.OpenAIRequest
	OpenAIMessage                = providers.OpenAIMessage
	OpenAITool                   = providers.OpenAITool
	ToolCall                     = providers.ToolCall
	ResponsesRequest             = providers.ResponsesRequest
	ResponsesObject              = providers.ResponsesObject
	ResponseOutputItem           = providers.ResponseOutputItem
	ResponseContentPart          = providers.ResponseContentPart
	ResponsesUsage               = providers.ResponsesUsage
	ResponsesError               = providers.ResponsesError
	UpstreamError                = providers.UpstreamError
	OverageSnapshot              = kiro.OverageSnapshot
)

var (
	GetClientForAccount     = providers.GetClientForAccount
	GetRestClientForAccount = providers.GetRestClientForAccount
	InitKiroHttpClient      = providers.InitKiroHttpClient
	upstreamError           = providers.Errorf
	readTokenNumber         = providers.ReadTokenNumber

	CallKiroAPI            = kiro.CallAPI
	FetchOverageStatus     = kiro.FetchOverageStatus
	SetOverageStatus       = kiro.SetOverageStatus
	PersistOverageSnapshot = kiro.PersistOverageSnapshot
)

package proxy

import (
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/pool"
	"strings"
	"sync"
	"time"
)

// accountSlots holds one semaphore per account with MaxConcurrency > 0. A cap
// change swaps in a fresh channel; in-flight holders release into the old one.
var accountSlots = struct {
	sync.Mutex
	m map[string]chan struct{}
}{m: map[string]chan struct{}{}}

// accountSlotWait bounds how long a request queues for a saturated account.
// ponytail: not tied to client disconnect (no ctx reaches here); a dropped
// client still holds its queue place until a slot frees or this expires.
var accountSlotWait = 2 * time.Minute

func accountSlotChan(account *config.Account) chan struct{} {
	accountSlots.Lock()
	defer accountSlots.Unlock()
	ch := accountSlots.m[account.ID]
	if cap(ch) != account.MaxConcurrency {
		ch = make(chan struct{}, account.MaxConcurrency)
		accountSlots.m[account.ID] = ch
	}
	return ch
}

// accountHasFreeSlot reports whether a request on account would start without queueing.
func accountHasFreeSlot(account *config.Account) bool {
	if account.MaxConcurrency <= 0 {
		return true
	}
	ch := accountSlotChan(account)
	return len(ch) < cap(ch)
}

// acquireAccountSlot waits for a free in-flight slot on account. The returned
// release must be called once the upstream call finishes.
func acquireAccountSlot(account *config.Account) (func(), error) {
	if account.MaxConcurrency <= 0 {
		return func() {}, nil
	}
	ch := accountSlotChan(account)
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	default:
	}
	t := time.NewTimer(accountSlotWait)
	defer t.Stop()
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-t.C:
		return nil, fmt.Errorf("account %s busy: max concurrency %d reached", account.Email, account.MaxConcurrency)
	}
}

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
	release, err := acquireAccountSlot(account)
	if err != nil {
		return err
	}
	defer release()
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
	release, err := acquireAccountSlot(account)
	if err != nil {
		return err
	}
	defer release()
	err = adapter.chatFromOpenAI(account, req, thinking, callback)
	noteModelUnsupported(account, req.Model, err)
	return err
}

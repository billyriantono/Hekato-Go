package providers

import "fmt"

// UpstreamError carries the HTTP status of a failed provider call so failover
// can classify on the status code instead of sniffing error strings, whose
// wording differs per provider. The message keeps each call site's original
// format so logs and string-based fallbacks stay unchanged.
type UpstreamError struct {
	Status int
	Msg    string
}

func (e *UpstreamError) Error() string { return e.Msg }

// Errorf builds an UpstreamError with the given status and message.
func Errorf(status int, format string, args ...any) *UpstreamError {
	return &UpstreamError{Status: status, Msg: fmt.Sprintf(format, args...)}
}

package proxy

import (
	"regexp"
	"strconv"
	"strings"
)

// getContextWindowSize returns the context window size (in tokens) for a model.
//
// Per Kiro's ListAvailableModels, the 1M-token context window applies to
// Claude 4.6 and newer (sonnet-4.6, opus-4.6, opus-4.7, opus-4.8, and future
// 4.x releases), while 4.5 and earlier (opus-4.5, sonnet-4.5, sonnet-4,
// haiku-4.5) use a 200K window. This value is used to convert the upstream
// contextUsagePercentage into an absolute input-token count that clients rely
// on to decide when to compact; an undersized window under-reports tokens and
// prevents clients from compacting in time.
func getContextWindowSize(model string) int {
	if isLargeContextModel(model) {
		return 1_000_000
	}
	return 200_000
}

// largeContextMinor matches "claude-<family>-<major>.<minor>" (dot or dash form)
// and is used to classify 1M-window models by version.
var claudeVersionExtractor = regexp.MustCompile(`claude-(?:opus|sonnet|haiku)-(\d+)[.-](\d+)`)

func isLargeContextModel(model string) bool {
	m := strings.ToLower(model)
	if match := claudeVersionExtractor.FindStringSubmatch(m); match != nil {
		major, errMaj := strconv.Atoi(match[1])
		minor, errMin := strconv.Atoi(match[2])
		if errMaj == nil && errMin == nil {
			// 1M window for Claude >= 4.6 (4.6, 4.7, 4.8, ...) and any major >= 5.
			if major > 4 {
				return true
			}
			if major == 4 && minor >= 6 {
				return true
			}
			return false
		}
	}
	// Fallback substring checks for non-standard identifiers.
	for _, tag := range []string{"4.6", "4-6", "4.7", "4-7", "4.8", "4-8", "4.9", "4-9"} {
		if strings.Contains(m, tag) {
			return true
		}
	}
	return false
}

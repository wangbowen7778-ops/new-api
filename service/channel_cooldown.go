package service

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// quotaExhaustedKeywords are the fragments upstreams use when a 429 reports an
// empty quota instead of a brief rate limit. A quota refusal does not clear
// within a burst, so the channel has to stay out of selection for longer.
var quotaExhaustedKeywords = []string{"quota", "insufficient", "billing", "credit", "balance"}

// Upstream429Cooldown reports how long a channel stays out of selection after
// an upstream 429. The gateway's own concurrency 429 never reaches this: it
// clears as soon as an in-flight request finishes, and its retry loop already
// waits for a slot instead of parking the channel.
func Upstream429Cooldown(err *types.NewAPIError) time.Duration {
	if err == nil {
		return common.UpstreamRateLimitCooldown
	}
	// A configured policy wins over the built-in keyword classification. Zero
	// means the operator asked for no cooldown at all (retry_no_cooldown): the
	// channel returns to the pool immediately and only this request moves on.
	if policy, ok := operation_setting.MatchChannel429Policy(err.Error()); ok {
		if policy.Action == operation_setting.Channel429ActionRetryNoCooldown {
			return 0
		}
		return operation_setting.Channel429PolicyCooldown(policy)
	}
	message := strings.ToLower(err.Error())
	for _, keyword := range quotaExhaustedKeywords {
		if strings.Contains(message, keyword) {
			return common.UpstreamQuotaCooldown
		}
	}
	return common.UpstreamRateLimitCooldown
}

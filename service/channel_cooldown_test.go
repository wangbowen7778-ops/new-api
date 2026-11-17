package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

// The cooldown length has to follow what the 429 actually reports: an exhausted
// quota will not clear within a burst, a rate limit will.
func TestUpstream429CooldownClassifiesTheRefusal(t *testing.T) {
	quota := types.NewErrorWithStatusCode(
		errors.New("You have exceeded the 5-hour usage quota. It will reset at 2026-10-19 18:28:17 +0800 CST."),
		types.ErrorCodeBadResponseStatusCode, 429,
	)
	require.Equal(t, common.UpstreamQuotaCooldown, Upstream429Cooldown(quota))

	rateLimited := types.NewErrorWithStatusCode(
		errors.New("Rate limit reached for requests, please retry later."),
		types.ErrorCodeBadResponseStatusCode, 429,
	)
	require.Equal(t, common.UpstreamRateLimitCooldown, Upstream429Cooldown(rateLimited))
}

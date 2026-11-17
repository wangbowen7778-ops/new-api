package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestUpdateChannel429PoliciesValidates(t *testing.T) {
	t.Cleanup(func() { _ = UpdateChannel429Policies("") })

	require.NoError(t, UpdateChannel429Policies(`[
		{"match": ["token rate limit"], "action": "retry", "cooldown_base": 10, "cooldown_max": 120},
		{"match": ["credit balance"], "action": "cooldown_only", "cooldown_base": 60, "cooldown_max": 300},
		{"match": ["inflight batchsize"], "action": "retry_no_cooldown"},
		{"match": ["region queued"], "action": "wait_retry", "wait_max_seconds": 45}
	]`))
	require.Len(t, channel429Policies, 4)

	for _, bad := range []string{
		`not json`,
		`{"match": []}`,

		`[{"match": ["x"], "action": "unknown", "cooldown_base": 1, "cooldown_max": 2}]`,
		`[{"match": ["x"], "action": "retry", "cooldown_base": 0, "cooldown_max": 2}]`,
		`[{"match": ["x"], "action": "retry", "cooldown_base": 10, "cooldown_max": 5}]`,
		`[{"match": ["x"], "action": "wait_retry"}]`,
	} {
		require.Error(t, UpdateChannel429Policies(bad), "must reject: %s", bad)
	}

	// Empty string clears the mapping back to built-in defaults.
	require.NoError(t, UpdateChannel429Policies("  "))
	require.Empty(t, channel429Policies)
}

func TestMatchChannel429Policy(t *testing.T) {
	t.Cleanup(func() { _ = UpdateChannel429Policies("") })
	require.NoError(t, UpdateChannel429Policies(`[
		{"match": ["token rate limit"], "action": "retry", "cooldown_base": 10, "cooldown_max": 120},
		{"match": ["credit balance"], "action": "cooldown_only", "cooldown_base": 60, "cooldown_max": 300},
		{"match": ["inflight batchsize"], "action": "retry_no_cooldown"},
		{"match": ["region queued"], "action": "wait_retry", "wait_max_seconds": 45}
	]`))

	policy, ok := MatchChannel429Policy(
		"your requests to kimi-k3 in eastus2 have exceeded token rate limit.")
	require.True(t, ok)
	require.Equal(t, "retry", policy.Action)

	policy, ok = MatchChannel429Policy("your credit balance is too low")
	require.True(t, ok)
	require.Equal(t, "cooldown_only", policy.Action)

	policy, ok = MatchChannel429Policy("inflight batchsize limit exceeded")
	require.True(t, ok)
	require.Equal(t, "retry_no_cooldown", policy.Action)

	policy, ok = MatchChannel429Policy("upstream says region queued; retry after 40s")
	require.True(t, ok)
	require.Equal(t, "wait_retry", policy.Action)
	require.Equal(t, 45, policy.WaitMaxSeconds)

	_, ok = MatchChannel429Policy("some other refusal")
	require.False(t, ok)
}

func TestChannel429PolicyCooldownClamps(t *testing.T) {
	policy := Channel429Policy{
		Action: "retry", CooldownBaseSeconds: 600, CooldownMaxSeconds: 100000,
	}
	require.Equal(t, Channel429PolicyCooldown(policy), common.UpstreamQuotaMaxCooldown,
		"a typo must not park a channel beyond the built-in cap")
}

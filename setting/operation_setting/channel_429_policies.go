package operation_setting

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Channel429Policy is one configured strategy for an upstream 429 whose body
// matches the given keywords. Action is "retry" (switch channel after the
// cooldown), "retry_no_cooldown" (switch immediately, channel stays hot),
// "cooldown_only" (park the channel, fail the request fast), or "wait_retry"
// (sleep on the same channel for the upstream's retry-after, capped).
type Channel429Policy struct {
	Match               []string `json:"match"`
	Action              string   `json:"action"`
	CooldownBaseSeconds int      `json:"cooldown_base"`
	CooldownMaxSeconds  int      `json:"cooldown_max"`
	WaitMaxSeconds      int      `json:"wait_max_seconds"`
}

const (
	Channel429ActionRetry           = "retry"
	Channel429ActionRetryNoCooldown = "retry_no_cooldown"
	Channel429ActionCooldownOnly    = "cooldown_only"
	Channel429ActionWaitRetry       = "wait_retry"
	Channel429PoliciesJSONKey       = "Channel429Policies"
)

// channel429Policies is the parsed mapping. Nil means no user policies are
// configured and the built-in defaults apply.
var channel429Policies []Channel429Policy

// GetChannel429PoliciesJSON returns the configured mapping as the raw JSON the
// operator wrote, so the options table round-trips exactly what was set.
func GetChannel429PoliciesJSON() string {
	if len(channel429Policies) == 0 {
		return ""
	}
	data, err := json.Marshal(channel429Policies)
	if err != nil {
		return ""
	}
	return string(data)
}

// UpdateChannel429Policies validates and installs a new mapping. An empty
// string clears the mapping, restoring the built-in defaults.
func UpdateChannel429Policies(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed == "" {
		channel429Policies = nil
		return nil
	}
	var policies []Channel429Policy
	if err := json.Unmarshal([]byte(trimmed), &policies); err != nil {
		return fmt.Errorf("invalid Channel429Policies JSON: %w", err)
	}
	for i, policy := range policies {
		if len(policy.Match) == 0 {
			return fmt.Errorf("Channel429Policies[%d]: match must list at least one keyword", i)
		}
		valid := policy.Action == Channel429ActionRetry ||
			policy.Action == Channel429ActionRetryNoCooldown ||
			policy.Action == Channel429ActionCooldownOnly ||
			policy.Action == Channel429ActionWaitRetry
		if !valid {
			return fmt.Errorf("Channel429Policies[%d]: action must be %q, %q, %q or %q",
				i, Channel429ActionRetry, Channel429ActionRetryNoCooldown,
				Channel429ActionCooldownOnly, Channel429ActionWaitRetry)
		}
		if policy.Action == Channel429ActionWaitRetry && policy.WaitMaxSeconds <= 0 {
			return fmt.Errorf("Channel429Policies[%d]: wait_retry requires wait_max_seconds > 0", i)
		}
		if policy.Action != Channel429ActionRetryNoCooldown &&
			policy.Action != Channel429ActionWaitRetry && policy.CooldownBaseSeconds <= 0 {
			return fmt.Errorf("Channel429Policies[%d]: cooldown_base must be positive", i)
		}
		if policy.CooldownMaxSeconds < policy.CooldownBaseSeconds {
			return fmt.Errorf("Channel429Policies[%d]: cooldown_max must be >= cooldown_base", i)
		}
	}
	channel429Policies = policies
	common.SysLog(common.LogText("channel 429 policies updated: %d entries", len(policies)))
	return nil
}

// DebugDump429Policies exposes the live mapping for runtime diagnostics.
func DebugDump429Policies() string {
	if len(channel429Policies) == 0 {
		return "<empty>"
	}
	parts := make([]string, 0, len(channel429Policies))
	for _, p := range channel429Policies {
		parts = append(parts, fmt.Sprintf("[%s -> %s base=%d max=%d wait=%d]",
			strings.Join(p.Match, "|"), p.Action, p.CooldownBaseSeconds, p.CooldownMaxSeconds, p.WaitMaxSeconds))
	}
	return strings.Join(parts, " ")
}

// MatchChannel429Policy returns the first policy whose keywords appear in the
// (lower-cased) upstream error message.
func MatchChannel429Policy(message string) (Channel429Policy, bool) {
	if len(channel429Policies) == 0 {
		return Channel429Policy{}, false
	}
	lower := strings.ToLower(message)
	for _, policy := range channel429Policies {
		for _, keyword := range policy.Match {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				return policy, true
			}
		}
	}
	return Channel429Policy{}, false
}

// Channel429PolicyCooldown converts the matched policy's window into a
// duration, clamped by the built-in quota cap so a typo cannot park a channel
// for hours.
func Channel429PolicyCooldown(policy Channel429Policy) time.Duration {
	base := time.Duration(policy.CooldownBaseSeconds) * time.Second
	limit := time.Duration(policy.CooldownMaxSeconds) * time.Second
	if limit > common.UpstreamQuotaMaxCooldown {
		limit = common.UpstreamQuotaMaxCooldown
	}
	if base > limit {
		base = limit
	}
	return base
}

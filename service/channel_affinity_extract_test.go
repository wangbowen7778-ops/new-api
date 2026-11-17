package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// The affinity key has to differ for different prompts. This mirrors what
// extractChannelAffinityValue does: take the first non-empty gjson hit, then
// hash it. If two different prompts produced the same key, every request of a
// user would land on one channel regardless of content.
func TestAffinityKeyDiffersForDifferentContent(t *testing.T) {
	paths := []string{
		`messages.#(role=="user").content`,
		`messages.0.content`,
		`input.#(role=="user").content`,
		`input.0.content`,
		`prompt`,
	}

	bodies := []string{
		`{"model":"kimi-k3","messages":[{"role":"system","content":"sys"},{"role":"user","content":"alpha prompt"}]}`,
		`{"model":"kimi-k3","messages":[{"role":"system","content":"sys"},{"role":"user","content":"beta prompt"}]}`,
		`{"model":"kimi-k3","input":[{"role":"user","content":"gamma prompt"}]}`,
		`{"model":"kimi-k3","prompt":"delta prompt"}`,
		`{"model":"kimi-k3","messages":[{"role":"user","content":"epsilon prompt"}]}`,
	}

	keys := make(map[string]string, len(bodies))
	for _, body := range bodies {
		raw := ""
		for _, path := range paths {
			res := gjson.GetBytes([]byte(body), path)
			if !res.Exists() {
				continue
			}
			switch res.Type {
			case gjson.String, gjson.Number, gjson.True, gjson.False:
				raw = res.String()
			default:
				raw = res.Raw
			}
			if raw != "" {
				break
			}
		}
		require.NotEmpty(t, raw, "no affinity source matched body: %s", body)

		key := common.Sha1([]byte(raw))[:16]
		keys[key] = raw
	}

	require.Len(t, keys, len(bodies), "different prompts must produce different affinity keys: %v", keys)
}

// Same content must produce the same key, otherwise a session could not stick.
func TestAffinityKeyStableForSameContent(t *testing.T) {
	body := `{"model":"kimi-k3","messages":[{"role":"user","content":"stable prompt"}]}`
	first := common.Sha1([]byte(gjson.Get(body, `messages.#(role=="user").content`).String()))[:16]
	second := common.Sha1([]byte(gjson.Get(body, `messages.#(role=="user").content`).String()))[:16]
	require.Equal(t, first, second)
}

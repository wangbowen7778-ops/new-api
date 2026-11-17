package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const affinityHashTestPrompt = `{"model":"gpt-4.1","messages":[{"role":"system","content":"You are a helpful assistant with a long shared preamble."}]}`

func newAffinityBodyContextForTest(t *testing.T, body string) *gin.Context {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	return ctx
}

func withAffinityRulesForTest(t *testing.T, rules []operation_setting.ChannelAffinityRule) {
	t.Helper()
	setting := operation_setting.GetChannelAffinitySetting()
	original := *setting
	t.Cleanup(func() { *setting = original })
	setting.Enabled = true
	setting.Rules = rules
}

func affinityCacheKeyForTest(t *testing.T, body string, modelName string) string {
	t.Helper()
	ctx := newAffinityBodyContextForTest(t, body)
	GetPreferredChannelByAffinity(ctx, modelName, "default")
	key, _, ok := getChannelAffinityContext(ctx)
	require.True(t, ok, "rule should match the request")
	return key
}

func affinityContentRule(source operation_setting.ChannelAffinityKeySource) []operation_setting.ChannelAffinityRule {
	return []operation_setting.ChannelAffinityRule{{
		Name:              "kv-test",
		ModelRegex:        []string{"^gpt-.*$"},
		PathRegex:         []string{"/v1/chat/completions"},
		KeySources:        []operation_setting.ChannelAffinityKeySource{source},
		TTLSeconds:        600,
		IncludeUsingGroup: true,
		IncludeModelName:  true,
		IncludeRuleName:   true,
	}}
}

// A content-derived source without hashing keeps the prompt verbatim in the
// cache key, which is what the hash flag exists to avoid.
func TestGetPreferredChannelByAffinity_ContentKeySourceKeepsTextWhenNotHashed(t *testing.T) {
	withAffinityRulesForTest(t, affinityContentRule(operation_setting.ChannelAffinityKeySource{
		Type: "gjson",
		Path: "messages.0.content",
	}))

	key := affinityCacheKeyForTest(t, affinityHashTestPrompt, "gpt-4.1")
	require.Contains(t, key, "You are a helpful assistant with a long shared preamble.")
}

func TestGetPreferredChannelByAffinity_HashesContentKeySource(t *testing.T) {
	withAffinityRulesForTest(t, affinityContentRule(operation_setting.ChannelAffinityKeySource{
		Type: "gjson",
		Path: "messages.0.content",
		Hash: true,
	}))

	first := affinityCacheKeyForTest(t, affinityHashTestPrompt, "gpt-4.1")
	second := affinityCacheKeyForTest(t, affinityHashTestPrompt, "gpt-4.1")

	require.Equal(t, first, second, "identical prompts must map to one cache key")
	require.NotContains(t, first, "helpful assistant", "request content must not reach the cache key")

	prefix := channelAffinityCacheNamespace + ":kv-test:gpt-4.1:default:"
	digest := strings.TrimPrefix(first, prefix)
	require.NotEqual(t, first, digest, "cache key should follow the rule and model/group suffixes")
	require.Len(t, digest, 16, "hashed keys keep a fixed-length digest")
}

func TestGetPreferredChannelByAffinity_HashedSourcesStayDistinct(t *testing.T) {
	withAffinityRulesForTest(t, affinityContentRule(operation_setting.ChannelAffinityKeySource{
		Type: "gjson",
		Path: "messages.0.content",
		Hash: true,
	}))

	first := affinityCacheKeyForTest(t, affinityHashTestPrompt, "gpt-4.1")
	second := affinityCacheKeyForTest(t,
		`{"model":"gpt-4.1","messages":[{"role":"system","content":"You are a terse reviewer."}]}`,
		"gpt-4.1",
	)
	require.NotEqual(t, first, second, "different prompts must not share a channel binding")
}

// value_regex runs against the extracted text, so a rule can restrict which
// prompts participate before the value is hashed.
func TestGetPreferredChannelByAffinity_HashAppliesAfterValueRegex(t *testing.T) {
	rules := affinityContentRule(operation_setting.ChannelAffinityKeySource{
		Type: "gjson",
		Path: "messages.0.content",
		Hash: true,
	})
	rules[0].ValueRegex = "^You are a helpful"

	withAffinityRulesForTest(t, rules)
	require.NotEmpty(t, affinityCacheKeyForTest(t, affinityHashTestPrompt, "gpt-4.1"))

	ctx := newAffinityBodyContextForTest(t,
		`{"model":"gpt-4.1","messages":[{"role":"system","content":"unrelated prompt"}]}`,
	)
	GetPreferredChannelByAffinity(ctx, "gpt-4.1", "default")
	_, _, ok := getChannelAffinityContext(ctx)
	require.False(t, ok, "a non-matching value must not create a binding")
}

// A conversation keyed on the first user message keeps every later turn of the
// same conversation on one channel while letting other conversations spread.
func TestGetPreferredChannelByAffinity_FirstUserMessageKey(t *testing.T) {
	rules := affinityContentRule(operation_setting.ChannelAffinityKeySource{
		Type: "gjson",
		Path: `messages.#(role=="user").content`,
		Hash: true,
	})
	rules[0].ModelRegex = []string{"^kimi-"}
	withAffinityRulesForTest(t, rules)

	firstTurn := affinityCacheKeyForTest(t,
		`{"model":"kimi-k3","messages":[{"role":"system","content":"fixed"},{"role":"user","content":"你好，帮我写个函数"}]}`,
		"kimi-k3",
	)
	secondTurn := affinityCacheKeyForTest(t,
		`{"model":"kimi-k3","messages":[{"role":"system","content":"fixed"},{"role":"user","content":"你好，帮我写个函数"},{"role":"assistant","content":"好的"},{"role":"user","content":"再加个参数"}]}`,
		"kimi-k3",
	)
	require.Equal(t, firstTurn, secondTurn, "later turns of one conversation must keep the same binding")

	otherConversation := affinityCacheKeyForTest(t,
		`{"model":"kimi-k3","messages":[{"role":"system","content":"fixed"},{"role":"user","content":"帮我写个 SQL"}]}`,
		"kimi-k3",
	)
	require.NotEqual(t, firstTurn, otherConversation, "different conversations must be able to spread")
}

// The setting reaches the gateway as JSON, so the flag has to survive that
// round trip to have any effect on a live instance.
func TestChannelAffinityKeySourceHashSurvivesSettingsJSON(t *testing.T) {
	rule := affinityContentRule(operation_setting.ChannelAffinityKeySource{
		Type: "gjson",
		Path: "messages.0.content",
		Hash: true,
	})[0]

	raw, err := json.Marshal([]operation_setting.ChannelAffinityRule{rule})
	require.NoError(t, err)

	var decoded []operation_setting.ChannelAffinityRule
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.True(t, decoded[0].KeySources[0].Hash)
}

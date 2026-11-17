package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Two prompts with different content must build separate bindings. If they
// shared one, every request of the deployment would stick to one channel even
// though the content differs.
func TestAffinityBindsDifferentContentToDifferentChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rule := operation_setting.ChannelAffinityRule{
		Name:       "default-by-user",
		ModelRegex: []string{"^.*$"},
		PathRegex:  []string{"/v1/chat/completions"},
		KeySources: []operation_setting.ChannelAffinityKeySource{
			{Type: "gjson", Path: `messages.#(role=="user").content`, Hash: true},
			{Type: "context_int", Key: "token_id"},
		},
		IncludeRuleName:   true,
		IncludeModelName:  true,
		IncludeUsingGroup: true,
		TTLSeconds:        600,
	}

	setting := operation_setting.GetChannelAffinitySetting()
	previousRules := setting.Rules
	previousEnabled := setting.Enabled
	setting.Rules = []operation_setting.ChannelAffinityRule{rule}
	setting.Enabled = true
	t.Cleanup(func() {
		setting.Rules = previousRules
		setting.Enabled = previousEnabled
	})

	build := func(content string) *gin.Context {
		body := fmt.Sprintf(`{"model":"kimi-k3","messages":[{"role":"user","content":%q}]}`, content)
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set(common.KeyRequestBody, []byte(body))
		return ctx
	}

	// First request per prompt: no binding yet, then record the chosen channel.
	ctxA := build("alpha prompt")
	_, found := GetPreferredChannelByAffinity(ctxA, "kimi-k3", "0.2")
	require.False(t, found, "first request cannot have a binding")
	RecordChannelAffinity(ctxA, 5001)

	ctxB := build("beta prompt")
	_, found = GetPreferredChannelByAffinity(ctxB, "kimi-k3", "0.2")
	require.False(t, found)
	RecordChannelAffinity(ctxB, 5002)

	t.Cleanup(func() {
		ClearCurrentChannelAffinityCache(build("alpha prompt"))
		ClearCurrentChannelAffinityCache(build("beta prompt"))
	})

	// Second request per prompt: each must return to its own channel.
	idA, foundA := GetPreferredChannelByAffinity(build("alpha prompt"), "kimi-k3", "0.2")
	require.True(t, foundA, "alpha prompt must hit its binding")
	require.Equal(t, 5001, idA)

	idB, foundB := GetPreferredChannelByAffinity(build("beta prompt"), "kimi-k3", "0.2")
	require.True(t, foundB, "beta prompt must hit its binding")
	require.Equal(t, 5002, idB, "different content must not share one channel")
}

// The fallback source (token id) must keep separate keys per token, otherwise
// every API key of one account would share a channel.
func TestAffinityFallbackTokenIdSeparatesKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rule := operation_setting.ChannelAffinityRule{
		Name:       "fallback",
		ModelRegex: []string{"^.*$"},
		PathRegex:  []string{"/v1/chat/completions"},
		KeySources: []operation_setting.ChannelAffinityKeySource{
			{Type: "gjson", Path: `messages.#(role=="user").content`, Hash: true},
			{Type: "context_int", Key: "token_id"},
		},
		IncludeRuleName:   true,
		IncludeModelName:  true,
		IncludeUsingGroup: true,
		TTLSeconds:        600,
	}

	setting := operation_setting.GetChannelAffinitySetting()
	previousRules := setting.Rules
	previousEnabled := setting.Enabled
	setting.Rules = []operation_setting.ChannelAffinityRule{rule}
	setting.Enabled = true
	t.Cleanup(func() {
		setting.Rules = previousRules
		setting.Enabled = previousEnabled
	})

	// No messages field: content source misses, token id decides.
	build := func(tokenID int) *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set(common.KeyRequestBody, []byte(`{"model":"kimi-k3"}`))
		ctx.Set("token_id", tokenID)
		return ctx
	}

	ctx1 := build(11)
	_, found := GetPreferredChannelByAffinity(ctx1, "kimi-k3", "0.2")
	require.False(t, found)
	RecordChannelAffinity(ctx1, 6001)

	ctx2 := build(22)
	_, found = GetPreferredChannelByAffinity(ctx2, "kimi-k3", "0.2")
	require.False(t, found, "a different token must not inherit the other binding")
	RecordChannelAffinity(ctx2, 6002)

	t.Cleanup(func() {
		ClearCurrentChannelAffinityCache(build(11))
		ClearCurrentChannelAffinityCache(build(22))
	})

	id1, ok1 := GetPreferredChannelByAffinity(build(11), "kimi-k3", "0.2")
	require.True(t, ok1)
	require.Equal(t, 6001, id1)

	id2, ok2 := GetPreferredChannelByAffinity(build(22), "kimi-k3", "0.2")
	require.True(t, ok2)
	require.Equal(t, 6002, id2, "different tokens must keep separate bindings")
}

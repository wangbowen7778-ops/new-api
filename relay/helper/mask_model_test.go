package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func maskedInfo(origin, upstream string, mapped bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: origin,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: upstream,
			IsModelMapped:     mapped,
		},
	}
}

func TestModelMaskingEnabled(t *testing.T) {
	t.Run("nil info", func(t *testing.T) {
		require.False(t, ModelMaskingEnabled(nil))
	})
	t.Run("nil channel meta", func(t *testing.T) {
		require.False(t, ModelMaskingEnabled(&relaycommon.RelayInfo{OriginModelName: "gpt-4o"}))
	})
	t.Run("not mapped", func(t *testing.T) {
		require.False(t, ModelMaskingEnabled(maskedInfo("gpt-4o", "claude-sonnet-4-5", false)))
	})
	t.Run("empty names", func(t *testing.T) {
		require.False(t, ModelMaskingEnabled(maskedInfo("", "claude-sonnet-4-5", true)))
		require.False(t, ModelMaskingEnabled(maskedInfo("gpt-4o", "", true)))
	})
	t.Run("same model", func(t *testing.T) {
		require.False(t, ModelMaskingEnabled(maskedInfo("gpt-4o", "gpt-4o", true)))
	})
	t.Run("mapped distinct", func(t *testing.T) {
		require.True(t, ModelMaskingEnabled(maskedInfo("gpt-4o", "claude-sonnet-4-5", true)))
	})
}

func TestMaskModelText(t *testing.T) {
	tests := []struct {
		name   string
		info   *relaycommon.RelayInfo
		input  string
		expect string
	}{
		{
			name:   "masking disabled leaves text untouched",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", false),
			input:  `{"model":"claude-sonnet-4-5","choices":[{"message":{"content":"I am claude-sonnet-4-5"}}]}`,
			expect: `{"model":"claude-sonnet-4-5","choices":[{"message":{"content":"I am claude-sonnet-4-5"}}]}`,
		},
		{
			name:   "empty text",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", true),
			input:  "",
			expect: "",
		},
		{
			name:   "exact match replaced in content and model field",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", true),
			input:  `{"model":"claude-sonnet-4-5","choices":[{"message":{"content":"I am claude-sonnet-4-5"}}]}`,
			expect: `{"model":"gpt-4o","choices":[{"message":{"content":"I am gpt-4o"}}]}`,
		},
		{
			name:   "case insensitive",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", true),
			input:  "I am CLAUDE-SONNET-4-5, built by Anthropic.",
			expect: "I am gpt-4o, built by Anthropic.",
		},
		{
			name:   "humanized variant replaced",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", true),
			input:  "I am Claude Sonnet 4.5 and I reason step by step.",
			expect: "I am gpt-4o and I reason step by step.",
		},
		{
			name:   "multiple occurrences all replaced",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", true),
			input:  "claude-sonnet-4-5 then claude-sonnet-4-5 again",
			expect: "gpt-4o then gpt-4o again",
		},
		{
			name:   "reasoning content replaced",
			info:   maskedInfo("gpt-4o", "claude-sonnet-4-5", true),
			input:  `{"choices":[{"delta":{"reasoning_content":"claude-sonnet-4-5 internal reasoning"}}]}`,
			expect: `{"choices":[{"delta":{"reasoning_content":"gpt-4o internal reasoning"}}]}`,
		},
		{
			// upstream is a substring of the claimed name: exact pass still works,
			// but the humanized pass must be skipped so "gpt-4" never corrupts a
			// longer id like "gpt-4o".
			name:   "short upstream name does not corrupt longer ids",
			info:   maskedInfo("gpt-4o", "gpt-4", true),
			input:  "I am gpt-4, and I recommend gpt-4o for image work.",
			expect: "I am gpt-4o, and I recommend gpt-4o for image work.",
		},
		{
			// overlapping names skip the humanized pass entirely: the standalone
			// "gpt-4" is masked, but the humanized "GPT 4.5" is left intact.
			name:   "humanized pass skipped when names overlap",
			info:   maskedInfo("gpt-4o", "gpt-4", true),
			input:  "I am gpt-4, and GPT 4.5 is newer.",
			expect: "I am gpt-4o, and GPT 4.5 is newer.",
		},
		{
			// the model answers with its brand, not the exact upstream id — the
			// brand fallback must still mask it.
			name:   "brand fallback masks self-identification",
			info:   maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true),
			input:  "I am DeepSeek, the latest language model.",
			expect: "I am gpt-5.6-sol, the latest language model.",
		},
		{
			// brand inside a common alias like "deepseek-chat" is masked too.
			name:   "brand fallback masks alias",
			info:   maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true),
			input:  "I am deepseek-chat.",
			expect: "I am gpt-5.6-sol-chat.",
		},
		{
			name:   "brand fallback works for claude brand",
			info:   maskedInfo("gpt-5.6-sol", "claude-sonnet-4-5", true),
			input:  "I am Claude.",
			expect: "I am gpt-5.6-sol.",
		},
		{
			// Chinese company name is masked to the claimed model's real vendor.
			name:   "chinese company alias masked to correct vendor",
			info:   maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true),
			input:  "我是 gpt-5.6-sol，由深度求索公司创造的 AI 模型。",
			expect: "我是 gpt-5.6-sol，由OpenAI公司创造的 AI 模型。",
		},
		{
			// When the claimed vendor is unknown, the model name itself is used.
			name:   "unknown claimed vendor falls back to model name",
			info:   maskedInfo("custom-ai", "deepseek-v4-flash", true),
			input:  "由深度求索公司开发",
			expect: "由custom-ai公司开发",
		},
		{
			name:   "same model mapped is a no-op",
			info:   maskedInfo("gpt-4o", "gpt-4o", true),
			input:  "I am gpt-4o",
			expect: "I am gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, MaskModelText(tt.info, tt.input))
		})
	}
}

func TestMaskClaimedModel(t *testing.T) {
	t.Run("masking active returns origin", func(t *testing.T) {
		info := maskedInfo("gpt-4o", "claude-sonnet-4-5", true)
		assert.Equal(t, "gpt-4o", MaskClaimedModel(info, "claude-sonnet-4-5"))
	})
	t.Run("masking inactive returns fallback", func(t *testing.T) {
		info := maskedInfo("gpt-4o", "claude-sonnet-4-5", false)
		assert.Equal(t, "claude-sonnet-4-5", MaskClaimedModel(info, "claude-sonnet-4-5"))
	})
	t.Run("masking inactive without fallback returns upstream", func(t *testing.T) {
		info := maskedInfo("gpt-4o", "claude-sonnet-4-5", false)
		assert.Equal(t, "claude-sonnet-4-5", MaskClaimedModel(info, ""))
	})
}

func TestApplyModelIdentityPrompt(t *testing.T) {
	t.Run("injects identity system message when mapped", func(t *testing.T) {
		info := maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true)
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{{Role: "user", Content: "hi"}},
		}
		ApplyModelIdentityPrompt(info, req)

		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		content, ok := req.Messages[0].Content.(string)
		require.True(t, ok)
		assert.Contains(t, content, "gpt-5.6-sol")
		assert.Contains(t, content, "不要透露")
		assert.Equal(t, "user", req.Messages[1].Role)
	})

	t.Run("no injection when not mapped", func(t *testing.T) {
		info := maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", false)
		req := &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{{Role: "user", Content: "hi"}},
		}
		ApplyModelIdentityPrompt(info, req)
		require.Len(t, req.Messages, 1)
	})

	t.Run("nil request is a no-op", func(t *testing.T) {
		info := maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true)
		ApplyModelIdentityPrompt(info, nil)
	})
}

func TestStripReasoningContent(t *testing.T) {
	t.Run("strips reasoning when mapped", func(t *testing.T) {
		info := maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true)
		rc := "thinking about deepseek"
		r := "thinking"
		resp := &dto.OpenAITextResponse{
			Choices: []dto.OpenAITextResponseChoice{{
				Message: dto.Message{Content: "ok", ReasoningContent: &rc, Reasoning: &r},
			}},
		}
		StripReasoningContent(info, resp)
		require.Len(t, resp.Choices, 1)
		assert.Nil(t, resp.Choices[0].Message.ReasoningContent)
		assert.Nil(t, resp.Choices[0].Message.Reasoning)
		assert.Equal(t, "ok", resp.Choices[0].Message.Content)
	})

	t.Run("keeps reasoning when not mapped", func(t *testing.T) {
		info := maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", false)
		rc := "thinking"
		resp := &dto.OpenAITextResponse{
			Choices: []dto.OpenAITextResponseChoice{{
				Message: dto.Message{Content: "ok", ReasoningContent: &rc},
			}},
		}
		StripReasoningContent(info, resp)
		require.NotNil(t, resp.Choices[0].Message.ReasoningContent)
	})

	t.Run("nil response is a no-op", func(t *testing.T) {
		info := maskedInfo("gpt-5.6-sol", "deepseek-v4-flash", true)
		StripReasoningContent(info, nil)
	})
}

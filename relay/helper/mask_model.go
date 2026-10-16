package helper

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ModelMaskingEnabled reports whether a response must claim the user-requested
// (mapped) model name instead of the real upstream model. It is active only
// when the channel's model mapping actually rewrote the request model
// (info.IsModelMapped) and the two names differ. Every write point that feeds
// the client passes through this gate first, so an unmapped channel is byte
// for byte unaffected.
func ModelMaskingEnabled(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil || !info.IsModelMapped {
		return false
	}
	if info.UpstreamModelName == "" || info.OriginModelName == "" {
		return false
	}
	return !strings.EqualFold(info.UpstreamModelName, info.OriginModelName)
}

// ResponseModelRewriteEnabled reports whether the channel opted in to
// rewriting the response's model field to the model the client requested. It is
// independent of model-mapping disguise: a channel may rewrite the model field
// without also masking content, reasoning, or vendor names. No-op when the
// client did not name a model.
func ResponseModelRewriteEnabled(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil || !info.ChannelSetting.RewriteResponseModel {
		return false
	}
	return info.OriginModelName != ""
}

// responseModelJSONPaths are the response-envelope locations a model name may
// occupy: OpenAI/Claude top-level, Gemini native modelVersion, and the nested
// message/response objects of Claude and OpenAI Responses stream events.
var responseModelJSONPaths = []string{"model", "modelVersion", "message.model", "response.model"}

// RewriteResponseModelField rewrites only the model field of an already
// serialized response body or stream frame to the client-requested model name,
// leaving every other byte untouched. A field absent from the payload is not
// added, and non-JSON input (e.g. a "[DONE]" sentinel) is returned unchanged.
// No-op unless the channel enabled RewriteResponseModel.
func RewriteResponseModelField(info *relaycommon.RelayInfo, text string) string {
	if !ResponseModelRewriteEnabled(info) || text == "" {
		return text
	}
	claimed := info.OriginModelName
	for _, path := range responseModelJSONPaths {
		if !gjson.Get(text, path).Exists() {
			continue
		}
		patched, err := sjson.Set(text, path, claimed)
		if err != nil {
			continue
		}
		text = patched
	}
	return text
}

// MaskResponseText applies the structural model-field rewrite (channel
// RewriteResponseModel switch) and, when model-mapping disguise is active, the
// upstream-model text masking. A channel with neither active gets its input
// back byte for byte. Every envelope write point routes through this so the two
// behaviors compose in one place.
func MaskResponseText(info *relaycommon.RelayInfo, text string) string {
	return MaskModelText(info, RewriteResponseModelField(info, text))
}

// MaskModelText replaces occurrences of the real upstream model name in
// generated text — final content, reasoning/thinking content, and any JSON
// envelope carrying the model field — with the user-requested mapped model
// name, so a channel with model mapping configured never leaks the actual
// model identity through its output. The caller passes already-marshaled
// response text; replacement preserves the surrounding bytes exactly.
//
// Two passes are applied, both word-boundary aware so a short name can never
// corrupt a longer one (e.g. "gpt-4" inside "gpt-4o"):
//   - an exact (case-insensitive) match on the upstream model id, and
//   - a humanized variant ("claude-sonnet-4-5" written as "Claude Sonnet 4.5"),
//     only when neither model name is a substring of the other, because with
//     overlapping names the humanized form of the shorter one becomes ambiguous.
//
// A third, brand-level pass replaces the distinctive family prefix of the real
// model (e.g. "deepseek" from "deepseek-v4-flash") with the claimed name, so a
// model answering "I am DeepSeek" or "deepseek-chat" — rather than the exact
// upstream id — still cannot leak its identity. It is skipped when the brand is
// too short/common or when it overlaps the claimed name. Known Chinese/company
// aliases of the brand are masked too (e.g. "深度求索"), so the model cannot
// reveal its real vendor either.
//
// Streaming note: a model name split across two chunks is not buffered across
// chunks; real model ids are emitted atomically and this is treated as a known
// limitation.
func MaskModelText(info *relaycommon.RelayInfo, text string) string {
	if !ModelMaskingEnabled(info) || text == "" {
		return text
	}

	real := info.UpstreamModelName
	claimed := info.OriginModelName

	text = replaceBoundaryFold(text, real, claimed)

	if human := humanizeModelName(real); human != real && !modelNamesOverlap(real, claimed) {
		text = replaceHumanized(text, human, claimed)
	}

	if brand := modelBrand(real); brand != "" && !strings.EqualFold(brand, claimed) {
		if len(brand) >= 4 && !modelNamesOverlap(brand, claimed) {
			text = replaceBoundaryFold(text, brand, claimed)
		}
		// Chinese/company aliases are masked to the claimed model's real vendor,
		// so "由深度求索公司创造" becomes "由OpenAI公司创造" instead of a fabricated
		// "由gpt-5.6-sol公司创造".
		company := claimedCompany(claimed)
		for _, alias := range modelBrandAliases[brand] {
			if alias == "" || strings.EqualFold(alias, company) || modelNamesOverlap(alias, company) {
				continue
			}
			text = replaceBoundaryFold(text, alias, company)
		}
	}

	return text
}

// MaskClaimedModel returns the model name a response envelope should present to
// the user: the requested (mapped) name while model mapping disguise is active
// or the channel opted into rewriting the response model, otherwise the
// caller-supplied fallback (which is normally the real upstream model).
func MaskClaimedModel(info *relaycommon.RelayInfo, fallback string) string {
	if ModelMaskingEnabled(info) || ResponseModelRewriteEnabled(info) {
		return info.OriginModelName
	}
	if fallback != "" {
		return fallback
	}
	return info.UpstreamModelName
}

// StripReasoningContent removes the reasoning_content / reasoning fields from a
// non-streaming OpenAI chat response, so the model's real thinking — which can
// still reference the true identity — never reaches the client. Applied only
// while model mapping disguise is active.
func StripReasoningContent(info *relaycommon.RelayInfo, resp *dto.OpenAITextResponse) {
	if resp == nil || !ModelMaskingEnabled(info) {
		return
	}
	for i := range resp.Choices {
		resp.Choices[i].Message.ReasoningContent = nil
		resp.Choices[i].Message.Reasoning = nil
	}
}

// MaskChatCompletionsStreamResponse masks one OpenAI chat stream chunk in
// place: the model field (while model mapping disguise or the response-model
// rewrite is active), and the content/reasoning text of every choice delta
// (masking only).
func MaskChatCompletionsStreamResponse(info *relaycommon.RelayInfo, resp *dto.ChatCompletionsStreamResponse) {
	if resp == nil {
		return
	}
	if ModelMaskingEnabled(info) || ResponseModelRewriteEnabled(info) {
		resp.Model = MaskClaimedModel(info, resp.Model)
	}
	if !ModelMaskingEnabled(info) {
		return
	}
	for i := range resp.Choices {
		delta := &resp.Choices[i].Delta
		if delta.Content != nil {
			*delta.Content = MaskModelText(info, *delta.Content)
		}
		if delta.ReasoningContent != nil {
			*delta.ReasoningContent = MaskModelText(info, *delta.ReasoningContent)
		}
		if delta.Reasoning != nil {
			*delta.Reasoning = MaskModelText(info, *delta.Reasoning)
		}
	}
}

// MaskClaudeResponse masks a Claude response in place: the model field (while
// model mapping disguise or the response-model rewrite is active) and, under
// masking, the text/thinking of content blocks, the delta, and the message
// envelope.
func MaskClaudeResponse(info *relaycommon.RelayInfo, resp *dto.ClaudeResponse) {
	if resp == nil {
		return
	}
	if ModelMaskingEnabled(info) || ResponseModelRewriteEnabled(info) {
		resp.Model = MaskClaimedModel(info, resp.Model)
	}
	if !ModelMaskingEnabled(info) {
		return
	}
	for i := range resp.Content {
		maskClaudeMediaMessage(info, &resp.Content[i])
	}
	maskClaudeMediaMessage(info, resp.Delta)
	maskClaudeMediaMessage(info, resp.Message)
}

func maskClaudeMediaMessage(info *relaycommon.RelayInfo, block *dto.ClaudeMediaMessage) {
	if block == nil {
		return
	}
	if block.Text != nil {
		*block.Text = MaskModelText(info, *block.Text)
	}
	if block.Thinking != nil {
		*block.Thinking = MaskModelText(info, *block.Thinking)
	}
}

// replaceBoundaryFold replaces every case-insensitive occurrence of old in s
// that sits between two non-alphanumeric characters (word boundary) with new.
// The boundary rule keeps "gpt-4" from being replaced inside "gpt-4o" or
// "gpt-4.5", while still matching "gpt-4." at the end of a sentence.
func replaceBoundaryFold(s, old, new string) string {
	if s == "" || old == "" {
		return s
	}
	lowerS := strings.ToLower(s)
	lowerOld := strings.ToLower(old)

	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i <= len(s)-len(old) {
		if strings.HasPrefix(lowerS[i:], lowerOld) && atWordBoundary(s, i, len(old)) {
			b.WriteString(new)
			i += len(old)
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	b.WriteString(s[i:])
	return b.String()
}

// replaceHumanized replaces the humanized form of a model id (spaces where the
// id had separators) with the claimed name, treating any of " -_.:" as a
// separator and matching at word boundaries.
func replaceHumanized(s, human, new string) string {
	if s == "" || human == "" {
		return s
	}
	lowerS := strings.ToLower(s)

	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+len(human) <= len(s) && matchHumanizedAt(lowerS, i, human) && atWordBoundary(s, i, len(human)) {
			b.WriteString(new)
			i += len(human)
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// matchHumanizedAt reports whether lowerS[i:] starts with the humanized model
// name, where each space in the pattern matches exactly one separator or space
// byte (" -_.:"). All pattern bytes are ASCII, so byte comparisons are safe.
func matchHumanizedAt(lowerS string, i int, human string) bool {
	for pos := 0; pos < len(human); pos++ {
		ch := human[pos]
		if ch == ' ' {
			switch lowerS[i+pos] {
			case ' ', '-', '_', '.', ':':
			default:
				return false
			}
		} else if lowerS[i+pos] != ch {
			return false
		}
	}
	return true
}

// humanizeModelName turns separators commonly found in model ids into spaces,
// so "claude-sonnet-4-5" can also be matched when a model writes its name in a
// humanized form such as "Claude Sonnet 4.5".
func humanizeModelName(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', '.', ':':
			return ' '
		}
		return r
	}, name)
}

// modelBrand extracts the distinctive family prefix of a model id, e.g.
// "deepseek" from "deepseek-v4-flash" or "gemini" from "gemini-2.5-flash".
// Returns the whole name when it has no separator.
func modelBrand(name string) string {
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '-', '_', '.', ':':
			return name[:i]
		}
	}
	return name
}

// modelBrandAliases lists common Chinese company / product names that a model of
// the given brand may use to self-identify, beyond the brand prefix itself.
// These are masked with the claimed model's vendor so the real vendor cannot
// leak (e.g. "深度求索" for deepseek). Keys are modelBrand() outputs.
var modelBrandAliases = map[string][]string{
	"deepseek":   {"深度求索"},
	"google":     {"谷歌"},
	"gemini":     {"谷歌"},
	"qwen":       {"通义", "千问"},
	"aliyun":     {"阿里巴巴", "通义"},
	"ali":        {"阿里巴巴"},
	"baidu":      {"百度"},
	"zhipu":      {"智谱"},
	"kimi":       {"月之暗面"},
	"moonshot":   {"月之暗面"},
	"baichuan":   {"百川"},
	"doubao":     {"字节", "豆包"},
	"volcengine": {"字节", "火山引擎"},
	"tencent":    {"腾讯"},
	"hunyuan":    {"腾讯"},
	"openai":     {"OpenAI"},
	"anthropic":  {"Anthropic"},
}

// modelCompany maps a claimed model brand to the vendor that actually produces
// it, so a masked response can name a correct vendor instead of a fabricated
// one. Keys are modelBrand() outputs.
var modelCompany = map[string]string{
	"gpt":     "OpenAI",
	"o1":      "OpenAI",
	"o3":      "OpenAI",
	"o4":      "OpenAI",
	"chatgpt": "OpenAI",
	"claude":  "Anthropic",
	"gemini":  "Google",
	"palm":    "Google",
	"llama":   "Meta",
	"grok":    "xAI",
	"mistral": "Mistral AI",
	"command": "Cohere",
	"qwen":    "Alibaba",
}

// claimedCompany returns the vendor the claimed model should attribute itself
// to, falling back to the model name itself when the vendor is unknown.
func claimedCompany(claimed string) string {
	if c, ok := modelCompany[modelBrand(claimed)]; ok && c != "" {
		return c
	}
	return claimed
}

// ApplyModelIdentityPrompt injects a system prompt establishing the mapped
// (claimed) model identity into the outbound request, so the model confidently
// self-identifies as the alias model instead of its real one. Without it, the
// model's own thinking stays uncertain about a name it does not actually carry
// (response masking can only rewrite text, not the model's self-perception).
// No-op unless model masking is active.
func ApplyModelIdentityPrompt(info *relaycommon.RelayInfo, request dto.Request) {
	if !ModelMaskingEnabled(info) || request == nil {
		return
	}
	claimed := info.OriginModelName
	// The prompt establishes the claimed identity without fabricating a vendor
	// (unknown vendors would otherwise fall back to the model name itself, e.g.
	// "由 glm-5.2 开发"). It also tells the model to refuse vendor/creator
	// questions outright — that is the only reliable defense when a user probes
	// "who made you", since MaskModelText can only rewrite a vendor it knows.
	prompt := fmt.Sprintf(
		"你是 %s。当用户询问你是什么模型时，明确回答你是 %s。当被询问你的开发者、制造商或所属公司时，不要透露任何厂商信息，礼貌地表示无法提供该信息。",
		claimed, claimed)

	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		r.Messages = append([]dto.Message{{Role: r.GetSystemRoleName(), Content: prompt}}, r.Messages...)
	case *dto.ClaudeRequest:
		if str, ok := r.System.(string); ok && strings.TrimSpace(str) != "" {
			r.System = prompt + "\n" + str
		} else {
			r.System = prompt
		}
	case *dto.GeminiChatRequest:
		part := dto.GeminiPart{Text: prompt}
		if r.SystemInstructions != nil {
			r.SystemInstructions.Parts = append([]dto.GeminiPart{part}, r.SystemInstructions.Parts...)
		} else {
			r.SystemInstructions = &dto.GeminiChatContent{Parts: []dto.GeminiPart{part}}
		}
	case *dto.OpenAIResponsesRequest:
		if b, err := common.Marshal(prompt); err == nil {
			r.Instructions = json.RawMessage(b)
		}
	}
}

// modelNamesOverlap reports whether one model name is a case-insensitive
// substring of the other. When true, the humanized pass is skipped because the
// shorter name's humanized form becomes ambiguous (e.g. "gpt 4" inside
// "gpt 4.5").
func modelNamesOverlap(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	return strings.Contains(la, lb) || strings.Contains(lb, la)
}

// atWordBoundary reports whether the span s[start:start+n] is not preceded or
// followed by an ASCII letter or digit. Non-ASCII bytes (e.g. CJK) count as
// boundaries, so matches never cross into multi-byte runes.
func atWordBoundary(s string, start, n int) bool {
	if start > 0 && isAsciiLetterOrDigit(s[start-1]) {
		return false
	}
	if start+n < len(s) && isAsciiLetterOrDigit(s[start+n]) {
		return false
	}
	return true
}

func isAsciiLetterOrDigit(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

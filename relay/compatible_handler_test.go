package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TextHelper hoists message-level tools into the parsed copy, but pass-through
// sends the client's raw bytes. The channel switch must therefore hoist the raw
// body too, or the upstream receives message-level declarations it cannot read.
func TestTextHelperHoistsMessageToolsBeforePassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(gin.ReleaseMode) })
	service.InitHttpClient()
	database := setupRelayChannelDB(t)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Token{}))
	previousLogEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = previousLogEnabled })

	const body = `{
		"model":"kimi-k3",
		"stream":false,
		"tool_choice":"required",
		"prompt_cache_options":{"mode":"implicit","ttl":"1h"},
		"messages":[
			{"role":"system","content":"You are Kimi."},
			{"role":"system","tools":[{"type":"function","function":{"name":"get_current_time","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}]},
			{"role":"user","content":"What time is it in Beijing?"}
		]
	}`

	for _, tc := range []struct {
		name               string
		hoist, passThrough bool
		wantTools          int
		wantMessages       int
	}{
		{name: "hoist plus pass-through rewrites the raw body", hoist: true, passThrough: true, wantTools: 1, wantMessages: 2},
		{name: "pass-through alone still sends the raw body", hoist: false, passThrough: true, wantTools: 0, wantMessages: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan []byte, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				received <- raw
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"chat_1","object":"chat.completion","model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)
			}))
			t.Cleanup(upstream.Close)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "kimi-k3")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test")
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
				HoistMessageTools:      tc.hoist,
				PassThroughBodyEnabled: tc.passThrough,
			})

			request := &dto.GeneralOpenAIRequest{}
			require.NoError(t, common.UnmarshalJsonStr(body, request))
			info := &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeChatCompletions,
				RelayFormat:     relaytypes.RelayFormatOpenAI,
				OriginModelName: "kimi-k3",
				RequestURLPath:  "/v1/chat/completions",
				Request:         request,
			}

			apiErr := TextHelper(c, info)

			require.Nil(t, apiErr)
			upstreamBody := <-received
			assert.Len(t, gjson.GetBytes(upstreamBody, "tools").Array(), tc.wantTools)
			assert.Len(t, gjson.GetBytes(upstreamBody, "messages").Array(), tc.wantMessages)
			assert.Equal(t, "required", gjson.GetBytes(upstreamBody, "tool_choice").String())
			// Pass-through must not rebuild the body: fields the request struct
			// does not model survive untouched.
			assert.Equal(t, "implicit", gjson.GetBytes(upstreamBody, "prompt_cache_options.mode").String())
			assert.Equal(t, "1h", gjson.GetBytes(upstreamBody, "prompt_cache_options.ttl").String())

			if tc.hoist {
				assert.Equal(t, "get_current_time", gjson.GetBytes(upstreamBody, "tools.0.function.name").String())
				assert.False(t, gjson.GetBytes(upstreamBody, "messages.0.tools").Exists())
				assert.False(t, gjson.GetBytes(upstreamBody, "messages.1.tools").Exists())
				assert.Equal(t, "What time is it in Beijing?", gjson.GetBytes(upstreamBody, "messages.1.content").String())
			} else {
				assert.Equal(t, "get_current_time", gjson.GetBytes(upstreamBody, "messages.1.tools.0.function.name").String())
			}
		})
	}
}

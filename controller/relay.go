package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relay/requestplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	case relayconstant.RelayModeAlphaSearch:
		err = relay.AlphaSearchHelper(c, info)
	case relayconstant.RelayModeToolsSearch, relayconstant.RelayModeToolsFetch:
		err = relay.DeepSeekToolsHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	var relayInfo *relaycommon.RelayInfo
	var ingressRecord []relaycommon.RequestPluginRecord
	// Registered first so that it runs last: the request plugins' report
	// hooks receive the request once the client has the whole response and
	// the request has settled.
	defer func() {
		requestplugin.Report(c, relayFormat, relayInfo, ingressRecord, newAPIError)
	}()

	defer func() {
		if newAPIError != nil {
			service.RecordRequestPolicyTermination(c, newAPIError)
			logger.LogError(c, common.LogText("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			upstreamURLs := common.UpstreamAddresses(c)
			if relayFormat != types.RelayFormatOpenAIRealtime && c.Writer.Written() &&
				strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream") {
				writeRelayStreamError(c, relayFormat, newAPIError)
				return
			}
			if relayFormat != types.RelayFormatOpenAIRealtime {
				c.Header("Content-Type", "application/json; charset=utf-8")
			}
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError(upstreamURLs...))
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(upstreamURLs...),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(upstreamURLs...),
				})
			}
		}
	}()

	// Ingress takes the request plugin plan and may patch the stored body
	// before it is parsed. A Realtime session runs its instances on its
	// events instead.
	var ingressSteps []relaycommon.RequestPluginStep
	if relayFormat == types.RelayFormatOpenAIRealtime {
		requestplugin.StartRealtime(c)
	} else if ingressSteps, ingressRecord, newAPIError = requestplugin.Ingress(c, relayFormat); newAPIError != nil {
		// The request reached no channel; its error log shows what the
		// plugins did.
		ingress := &relaycommon.RelayInfo{}
		ingress.SetRequestPluginIngress(ingressSteps, ingressRecord)
		service.RecordRelayErrorLog(c, 0, newAPIError, ingress)
		return
	}

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		}
		return
	}

	relayInfo, err = relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}
	relayInfo.SetRequestPluginIngress(ingressSteps, ingressRecord)

	defer func() {
		recovered := recover()
		resultErr := newAPIError
		if recovered != nil {
			resultErr = types.NewError(fmt.Errorf("relay panic: %v", recovered), types.ErrorCodeBadResponse)
		}
		if relayFormat != types.RelayFormatOpenAIRealtime {
			perfmetrics.RecordRelayResult(c.Request.Context(), relayInfo, resultErr)
		}
		if recovered != nil {
			panic(recovered)
		}
	}()

	generalSettings := operation_setting.GetGeneralSetting()
	if relayFormat == types.RelayFormatClaude && relayInfo.IsStream && generalSettings.PingIntervalEnabled {
		stopKeepAlive := helper.StartStreamKeepAlive(c, time.Duration(generalSettings.PingIntervalSeconds)*time.Second)
		defer stopKeepAlive()
	}

	if newAPIError = relay.PrepareRequestBilling(c, relayInfo); newAPIError != nil {
		return
	}
	defer func() {
		newAPIError = relay.RefundFailedRequestBilling(c, relayInfo, newAPIError)
	}()

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	// A failing upstream attempt switches to another channel of the same model,
	// overrunning the retry budget (common.RetryTimes) the same way the
	// concurrency-429 probe loop does. The budget is one probe per priority
	// tier, cycling the tier index so every tier is probed, plus — for a
	// concurrency 429 specifically — one probe per
	// ConcurrencyHighPriorityWaitWindow interval at the top tier (fast-but-small
	// channels free slots continuously, so waiting for one beats running on a
	// slow tier). The request settles once the budget is exhausted. -1 marks the
	// budget as not yet initialized; 0 means exhausted.
	switchBudget := int64(-1)
	switchProbe := 0
	tierCount := 0
	// waitRetryUsed grants at most one same-channel wait per request: after
	// that a still-429ing channel falls through to the normal switch path.
	waitRetryUsed := false
	for ; retryParam.GetRetry() <= common.RetryTimes || switchBudget > 0; retryParam.IncreaseRetry() {
		if retryParam.GetRetry() > common.RetryTimes && switchBudget > 0 {
			switchBudget--
		}
		relayInfo.StreamStatus = nil
		relayInfo.PerformanceBusinessRejection = false
		relayInfo.PerformanceOutputTokens = 0
		relayInfo.RetryIndex = retryParam.GetRetry()
		if retryParam.GetRetry() > 0 {
			time.Sleep(common.ConcurrencyRetryInterval)
		}
		// A retry must not draw the channel that just failed again: without
		// this the request can spend its whole switch budget on the same
		// channel and hand the upstream error back to the caller.
		if retryParam.GetRetry() > 0 {
			retryParam.ExcludeChannelIDs = retryParam.ExcludeChannelIDs[:0]
			for _, usedChannel := range c.GetStringSlice("use_channel") {
				if channelID, parseErr := strconv.Atoi(usedChannel); parseErr == nil {
					retryParam.ExcludeChannelIDs = append(retryParam.ExcludeChannelIDs, channelID)
				}
			}
		}
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			// Keep the upstream error the request already holds: running out of
			// untried channels is the normal end of the refusal switch loop, and
			// replacing it with "no available channel" would hide the reason.
			if newAPIError == nil {
				newAPIError = channelErr
			}
			break
		}
		service.AppendUsedChannel(c, channel.Id)
		if billingErr := service.PrepareTieredBillingForSelectedGroup(c, relayInfo); billingErr != nil {
			newAPIError = billingErr
			break
		}

		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		runAttempt := func() *types.NewAPIError {
			channelSettings, _ := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
			// Affinity-bound requests ride past the per-channel concurrency cap:
			// the binding exists so the upstream cache stays warm, and denying
			// them would shed exactly the traffic that benefits from it. The cap
			// keeps gating arrivals without a binding.
			limit := int64(channelSettings.MaxConcurrentRequests)
			if service.AffinitySelectedForRequest(c) {
				limit = 0
			}
			release, ok := service.AcquireChannelConcurrency(channel.Id, limit)
			if !ok {
				// Status 429 keeps the retry loop switching to another channel;
				// the code must stay a non-"channel:" error so the channel is
				// never auto-disabled, the message must not leak channel info,
				// and the error log is skipped so ordinary users never see the
				// internal retry/probe trail in their log page.
				return types.NewErrorWithStatusCode(
					errors.New(i18n.T(c, i18n.MsgChannelConcurrencyLimit)),
					types.ErrorCodeConcurrencyLimit,
					http.StatusTooManyRequests,
					types.ErrOptionWithNoRecordErrorLog(),
				)
			}
			defer release()

			// The request plugins' reply checks read the answer this attempt
			// writes; without a reply instance nothing changes.
			finishReply := requestplugin.Reply(c, relayFormat, relayInfo)
			defer finishReply()
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				return relay.WssHelper(c, relayInfo)
			case types.RelayFormatClaude:
				return relay.ClaudeHelper(c, relayInfo)
			case types.RelayFormatGemini:
				return geminiRelayHandler(c, relayInfo)
			default:
				return relayHandler(c, relayInfo)
			}
		}
		newAPIError = runAttempt()

		if newAPIError == nil {
			service.MarkRequestPolicySuccess(c, relayInfo.StreamStatus)
			relayInfo.LastError = nil
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		decision := service.DecideRelayRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry())
		// wait_retry policy: an upstream 429 with a wait hint parks THIS request
		// on the same channel for min(retry_after, policy ceiling) and retries
		// it, so the session's warm cache is not shed to another channel. One
		// wait per request; past that the normal switch path takes over.
		if newAPIError.StatusCode == http.StatusTooManyRequests && !waitRetryUsed {
			matched, matchedOK := operation_setting.MatchChannel429Policy(newAPIError.Error())
			logger.LogWarn(c, common.LogText("429 policy probe: matched=%v action=%q dump=%s err=%.140s",
				matchedOK, matched.Action, operation_setting.DebugDump429Policies(), newAPIError.Error()))
			if matchedOK && matched.Action == operation_setting.Channel429ActionWaitRetry {
				waitRetryUsed = true
				wait := matched.WaitMaxSeconds
				if hint := newAPIError.RetryAfterSeconds; hint > 0 && hint < wait {
					wait = hint
				}
				logger.LogWarn(c, common.LogText(
					"upstream 429 with wait_retry policy: waiting %ds before retrying channel #%d",
					wait, channel.Id))
				select {
				case <-time.After(time.Duration(wait) * time.Second):
				case <-c.Request.Context().Done():
					return
				}
				// Withdraw this channel from the used list so the next
				// selection may return to it.
				current := fmt.Sprintf("%d", channel.Id)
				used := c.GetStringSlice("use_channel")
				kept := used[:0:0]
				for _, u := range used {
					if u != current {
						kept = append(kept, u)
					}
				}
				c.Set("use_channel", kept)
				retryParam.SetRetry(retryParam.GetRetry() - 1) // cancel this loop's IncreaseRetry
				if attemptErr := runAttempt(); attemptErr == nil {
					service.MarkRequestPolicySuccess(c, relayInfo.StreamStatus)
					relayInfo.LastError = nil
					return
				} else {
					newAPIError = attemptErr
				}
				continue // still failing: normal decision/cooldown/switch path
			}
		}
		// Park the channel so concurrent requests stop probing a failure that
		// will not clear on its own. Each attempt against a broken channel costs
		// the client the full failure latency before the retry moves on, so
		// without the park every later request pays that cost again — multiplied
		// by however many channels are in the same broken state.
		upstream429 := newAPIError.StatusCode == http.StatusTooManyRequests && newAPIError.GetErrorCode() != types.ErrorCodeConcurrencyLimit
		if newAPIError.GetErrorCode() != types.ErrorCodeConcurrencyLimit {
			switch {
			case upstream429:
				// A configured retry_no_cooldown policy reports zero: the
				// channel goes straight back into the pool and only this
				// request switches away.
				if base := service.Upstream429Cooldown(newAPIError); base > 0 {
					limit := common.UpstreamRateLimitMaxCooldown
					if base >= common.UpstreamQuotaCooldown {
						limit = common.UpstreamQuotaMaxCooldown
					}
					model.MarkChannelCooldownBackoff(channel.Id, base, limit)
				}
			case newAPIError.StatusCode >= 500 && newAPIError.StatusCode <= 599:
				model.MarkChannelCooldownBackoff(channel.Id, common.UpstreamServerErrorCooldown, common.UpstreamServerErrorMaxCooldown)
			case newAPIError.StatusCode == http.StatusPaymentRequired:
				// An empty balance behaves like an exhausted quota: no point
				// re-probing it until an operator tops the account up.
				model.MarkChannelCooldownBackoff(channel.Id, common.UpstreamQuotaCooldown, common.UpstreamQuotaMaxCooldown)
			case newAPIError.StatusCode == http.StatusForbidden:
				model.MarkChannelCooldownBackoff(channel.Id, common.UpstreamForbiddenCooldown, common.UpstreamForbiddenMaxCooldown)
			case newAPIError.StatusCode <= 0:
				// No upstream status at all: the connection failed before a
				// response existed, so the channel is unreachable.
				model.MarkChannelCooldownBackoff(channel.Id, common.UpstreamServerErrorCooldown, common.UpstreamServerErrorMaxCooldown)
			}
		}
		// Every switch-worthy failure has to reach a different channel, so
		// selection ignores priority tiers and treats ExcludeChannelIDs as
		// strict. Without this an exhausted top tier falls back to the channel
		// that just failed, the post-selection filter rejects it, and the
		// request dies on the first error while other channels stay healthy.
		// The concurrency 429 is excluded: it keeps its own wait-window logic
		// and deliberately revisits the same tier.
		if decision.Action == "retry" && newAPIError.GetErrorCode() != types.ErrorCodeConcurrencyLimit {
			retryParam.SwitchAcrossTiers = true
		}
		if newAPIError.GetErrorCode() == types.ErrorCodeConcurrencyLimit {
			// A saturated channel is healthy but full: park it only briefly so a
			// concurrent burst stops re-selecting it, while the wait-window
			// logic below can still return to it once slots free up.
			model.MarkChannelCooldown(channel.Id, common.ChannelConcurrencyCooldown)
			// Probe budget = wait-window probes at the top tier + one probe
			// per lower tier. While the window lasts, the tier index stays at
			// 0 so freed slots on fast high-priority channels win the probe;
			// once it expires, the index cycles across the remaining tiers so
			// every tier gets probed before the request settles with a 429.
			if switchBudget < 0 {
				tierCount = model.MaxConcurrencyRetryProbes(
					common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
					relayInfo.OriginModelName,
					service.GetChannelConstraints(c).Filters,
				)
				if tierCount <= 0 {
					tierCount = 1
				}
				switchBudget = int64(tierCount)
				if windowProbes := int64(common.ConcurrencyHighPriorityWaitWindow / common.ConcurrencyRetryInterval); windowProbes > 0 {
					switchBudget += windowProbes
				}
			}
			probeIndex := int64(0)
			if switchBudget > 0 {
				probeIndex = switchBudget - 1
			}
			windowProbes := int64(common.ConcurrencyHighPriorityWaitWindow / common.ConcurrencyRetryInterval)
			var tier int
			if windowProbes > 0 && probeIndex >= windowProbes {
				tier = int((probeIndex - windowProbes) % int64(tierCount))
			}
			retryParam.TierOverride = &tier
			retryParam.PriorityFloor = 0
			decision = service.PolicyDecision{Action: "retry", Reason: "channel_concurrency_limit", Source: "system"}
		} else if decision.Action == "retry" && (retryParam.GetRetry() >= common.RetryTimes || upstream429) {
			// Any other switch-worthy failure (any non-2xx upstream status or
			// channel error) keeps moving to another channel of the same model
			// past the retry budget, the same way the concurrency probe does:
			// one probe per priority tier, cycling the tier index so every
			// candidate tier is revisited before the request settles on the
			// error.
			if switchBudget < 0 {
				tierCount = max(model.MaxConcurrencyRetryProbes(
					common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
					relayInfo.OriginModelName,
					service.GetChannelConstraints(c).Filters,
				), 1)
				switchBudget = int64(tierCount)
				// A cross-tier switch walks channels, not tiers, so its budget
				// is one probe per candidate channel. Charging it to the tier
				// count would settle on the first failure as soon as all
				// channels share a priority.
				if retryParam.SwitchAcrossTiers {
					switchBudget = int64(max(model.RetryChannelCount(
						common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
						relayInfo.OriginModelName,
						service.GetChannelConstraints(c).Filters,
					), tierCount))
				}
			}
			if retryParam.SwitchAcrossTiers {
				// Cross-tier switching ignores the priority ladder, so the tier
				// override would only re-pin the attempt that the switch is
				// trying to escape.
				retryParam.TierOverride = nil
				retryParam.PriorityFloor = 0
			} else {
				tier := switchProbe % tierCount
				switchProbe++
				retryParam.TierOverride = &tier
				retryParam.PriorityFloor = 0
			}
		} else {
			// A real failure moved the attempt counter down the tiers; keep
			// probing from the reached tier until a concurrency probe resets
			// the floor or the switch budget runs out.
			retryParam.TierOverride = nil
			retryParam.PriorityFloor = retryParam.GetRetry()
		}
		service.RecordPolicyFailure(c, channel.Id, newAPIError, decision)
		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError, relayInfo)

		if decision.Action != "retry" {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := common.LogText("retry: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
}

func writeRelayStreamError(c *gin.Context, format types.RelayFormat, apiError *types.NewAPIError) {
	// 心跳提交响应头后，最终错误继续通过同一个流返回。
	status, _ := common.GetContextKeyType[*relaycommon.StreamStatus](c, constant.ContextKeyResponseStreamStatus)
	if status == nil {
		status = relaycommon.NewStreamStatus()
		common.SetContextKey(c, constant.ContextKeyResponseStreamStatus, status)
	}
	status.MarkFailed(string(apiError.GetErrorCode()), string(apiError.GetErrorType()), apiError.StatusCode)
	upstreamURLs := common.UpstreamAddresses(c)
	if format == types.RelayFormatClaude {
		_ = helper.ClaudeData(c, dto.ClaudeResponse{Type: "error", Error: apiError.ToClaudeError(upstreamURLs...)})
		return
	}
	_ = helper.ObjectData(c, gin.H{"error": apiError.ToOpenAIError(upstreamURLs...)})
	helper.Done(c)
}

// CountClaudeTokens implements Anthropic's token-counting utility endpoint.
// It deliberately skips upstream generation and billing; callers use this
// endpoint to size prompts before creating a Message.
func CountClaudeTokens(c *gin.Context) {
	request, err := helper.GetAndValidateClaudeRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": common.MessageWithRequestId(err.Error(), c.GetString(common.RequestIdKey)),
			},
		})
		return
	}

	info := relaycommon.GenRelayInfoClaude(c, request)
	inputTokens, err := service.CountRequestToken(c, request.GetTokenCountMeta(), info)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": common.MessageWithRequestId(err.Error(), c.GetString(common.RequestIdKey)),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"input_tokens": inputTokens})
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime", "responses"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	// The nil-ChannelMeta branch is the first attempt: the distributor already
	// picked a channel (affinity/pin aware). InitChannelMeta marks it as done
	// so later attempts always re-select — even when the handler never ran
	// (e.g. a concurrency-limit 429 rejects the attempt before the handler
	// initializes the meta itself), otherwise every retry would be pinned to
	// the same saturated channel.
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		channel := &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}
		service.RequestPolicy(c).BeginAttempt(channel, info.UsingGroup)
		info.InitChannelMeta(c)
		return channel, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryGetChannelFailed, map[string]any{"Group": selectGroup, "Model": info.OriginModelName, "Error": err.Error()})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryNoAvailableChannel, map[string]any{"Group": selectGroup, "Model": info.OriginModelName})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	service.RequestPolicy(c).BeginAttempt(channel, selectGroup)
	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	info.InitChannelMeta(c)
	return channel, nil
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo) {
	service.ProcessChannelError(c, channelError, err, relayInfo)
}

func RelayMidjourney(c *gin.Context) {
	policy := service.RequestPolicy(c)
	defer func() {
		if policy.Attempts > 0 && !policy.Successful {
			service.RecordRequestPolicyTermination(c, types.NewErrorWithStatusCode(errors.New("Midjourney submission failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway, types.ErrOptionWithSkipRetry()))
		}
	}()
	var relayInfo *relaycommon.RelayInfo
	var reportErr *types.NewAPIError
	// Registered first so that it runs last: the request plugins' report
	// hooks receive a submission once the client has the response.
	defer func() {
		requestplugin.Report(c, types.RelayFormatMjProxy, relayInfo, nil, reportErr)
	}()
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *taskdto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	default:
		if reportErr = midjourneyRequestPlugins(c, relayInfo); reportErr != nil {
			code := constant.MjRequestError
			if reportErr.StatusCode >= http.StatusInternalServerError {
				code = constant.MjErrorUnknown
			}
			c.JSON(reportErr.StatusCode, gin.H{
				"description": common.MessageWithRequestId(reportErr.Error(), c.GetString(common.RequestIdKey)),
				"type":        string(reportErr.GetErrorCode()),
				"code":        code,
			})
			return
		}
		if relayInfo.RelayMode == relayconstant.RelayModeSwapFace {
			mjErr = relay.RelaySwapFace(c, relayInfo)
		} else {
			mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
		}
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		policy.Successful = false
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = i18n.T(c, i18n.MsgRelayGroupSaturated)
			statusCode = http.StatusTooManyRequests
		}
		reportErr = types.NewErrorWithStatusCode(fmt.Errorf("%s %s", mjErr.Description, mjErr.Result), types.ErrorCodeBadResponseStatusCode, statusCode)
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, common.LogText("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

// midjourneyRequestPlugins runs the request plugins of a Midjourney
// submission on its client body before the body is read: the request's
// ingress, then the late ingress of its channel. A Midjourney submission
// has one attempt, so the late record stays with the request's. A stop is
// written to the error log.
func midjourneyRequestPlugins(c *gin.Context, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	_, steps, record, apiErr := requestplugin.SubmitIngress(c, types.RelayFormatMjProxy, common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
	relayInfo.SetRequestPluginIngress(steps, record)
	channelID := 0
	if apiErr == nil {
		relayInfo.InitChannelMeta(c)
		channelID = relayInfo.ChannelId
		_, apiErr = requestplugin.TaskAttempt(c, relayInfo)
		relayInfo.SetRequestPluginIngress(relayInfo.RequestPluginSteps, relayInfo.RequestPluginRecord)
	}
	if apiErr != nil {
		service.RecordRelayErrorLog(c, channelID, apiErr, relayInfo)
	}
	return apiErr
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	// The web fallback may already have applied static-asset cache headers.
	// A missing API or asset can appear after an upgrade; never cache its 404.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

// RelayTaskPluginEndpoint keeps unclaimed shared-endpoint traffic on its
// existing handler while claimed requests enter the generation-pinned
// host-owned protocol bridge.
func RelayTaskPluginEndpoint(c *gin.Context, fallback gin.HandlerFunc) {
	pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint)
	if !exists {
		fallback(c)
		return
	}
	pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint)
	if !ok || pinned.Plugin == nil || pinned.Generation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Task protocol request failed",
				"type":    "new_api_error",
				"code":    "task_protocol_error",
			},
		})
		return
	}
	switch pinned.Protocol {
	case "openai_responses":
		serveTaskPluginProtocol(c, pinned, defaultPluginProtocolBridgeDeps())
	case pluginruntime.ProtocolOpenAIImage:
		serveTaskPluginImageProtocol(c, pinned, defaultPluginProtocolBridgeDeps())
	default:
		fallback(c)
	}
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &taskdto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

type taskSubmissionOutcome struct {
	Result    *relay.TaskSubmitResult
	Task      *model.Task
	RelayInfo *relaycommon.RelayInfo
}

func RelayTask(c *gin.Context) {
	var relayInfo *relaycommon.RelayInfo
	var taskErr *taskdto.TaskError
	// Registered first so that it runs last: the request plugins' report
	// hooks receive the submission once the client has the response.
	defer func() {
		requestplugin.Report(c, types.RelayFormatTask, relayInfo, nil, taskReportError(taskErr))
	}()
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		respondTaskSubmissionError(c, &taskdto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if action := c.GetString("task_action"); action != "" {
		relayInfo.Action = action
	}

	if taskErr = relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	if taskErr = relay.ApplyOriginTaskAffinity(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}

	var outcome *taskSubmissionOutcome
	outcome, taskErr = executeTaskSubmission(c, relayInfo)
	if taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	presentTaskSubmission(c, outcome)
}

// taskReportError is the error a task submission's client received, as the
// request plugins' report record shows it; nil when it succeeded.
func taskReportError(taskErr *taskdto.TaskError) *types.NewAPIError {
	if taskErr == nil {
		return nil
	}
	err := taskErr.Error
	if err == nil {
		err = errors.New(taskErr.Message)
	}
	return types.NewErrorWithStatusCode(err, types.ErrorCode(taskErr.Code), taskErr.StatusCode)
}

// executeTaskSubmission owns the retry, billing, and persistence lifecycle.
// It deliberately performs no client response writes so JSON and protocol
// presenters share the same durable task barrier. Its cancellation semantics
// come from c.Request.Context: native task endpoints use the client context,
// while the Responses bridge supplies an independently bounded context.
func executeTaskSubmission(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*taskSubmissionOutcome, *taskdto.TaskError) {
	return executeTaskSubmissionWith(c, relayInfo, relay.RelayTaskSubmit)
}

type taskSubmitAttempt func(*gin.Context, *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError)

func executeTaskSubmissionWith(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	submit taskSubmitAttempt,
) (*taskSubmissionOutcome, *taskdto.TaskError) {
	policy := service.RequestPolicy(c)
	diagnostics := newTaskPluginSubmitDiagnostics(c)
	diagnostics.start(relayInfo)
	var result *relay.TaskSubmitResult
	var taskErr *taskdto.TaskError
	durable := false
	stage := "start"
	defer func() {
		// The client of a relayed event stream already holds the vendor's
		// work, so the submission is billed instead of refunded.
		if !durable && relayInfo.Billing != nil && !c.Writer.Written() {
			diagnostics.refund(stage)
			relayInfo.Billing.Refund(c)
		}
	}()
	stage = "before_attempt"
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_attempt", 0)
		return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
	}

	// The request plugins' ingress runs once, on the client body, before the
	// submission is validated, priced or reserved; a changed body is decoded
	// again.
	stage = "request_plugins"
	var format types.RelayFormat = types.RelayFormatTask
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && (pinned.Protocol == string(types.RelayFormatOpenAIResponses) || pinned.Protocol == pluginruntime.ProtocolOpenAIImage) {
			format = types.RelayFormat(pinned.Protocol)
		}
	}
	ingressModel := relayInfo.OriginModelName
	if ingressModel == "" {
		ingressModel = common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	}
	changed, ingressSteps, ingressRecord, ingressErr := requestplugin.SubmitIngress(c, format, ingressModel)
	relayInfo.SetRequestPluginIngress(ingressSteps, ingressRecord)
	if ingressErr != nil {
		// The submission reached no channel and reserved nothing; its error
		// log shows what the plugins did.
		service.RecordRelayErrorLog(c, 0, ingressErr, relayInfo)
		taskErr = requestplugin.TaskError(ingressErr)
		diagnostics.failed(stage, "request_plugin", taskErr, false)
		return nil, taskErr
	}
	if changed {
		if taskErr = middleware.RefreshTaskPluginRequest(c); taskErr != nil {
			diagnostics.failed(stage, "request_decode", taskErr, false)
			return nil, taskErr
		}
	}

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		stage = "select_channel"
		if requestErr := c.Request.Context().Err(); requestErr != nil {
			diagnostics.cancelled("before_attempt", retryParam.GetRetry()+1)
			taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
			break
		}
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			policy.BeginAttempt(channel, relayInfo.UsingGroup)
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", channelErr.StatusCode)
				break
			}
		}
		diagnostics.attempt(retryParam.GetRetry()+1, channel, relayInfo.LockedChannel != nil)

		service.AppendUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			stage = "read_body"
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		stage = "submit"
		result, taskErr = submit(c, relayInfo)
		if requestErr := c.Request.Context().Err(); requestErr != nil {
			diagnostics.cancelled("after_submit", retryParam.GetRetry()+1)
			taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
			break
		}
		if taskErr == nil {
			diagnostics.attemptSucceeded(retryParam.GetRetry()+1, result)
			break
		}

		taskAPIError := taskSubmissionAPIError(taskErr)
		relayInfo.LastError = taskAPIError
		decision := decideTaskRetry(c, taskErr, common.RetryTimes-retryParam.GetRetry())
		service.RecordPolicyFailure(c, channel.Id, taskAPIError, decision)
		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				taskAPIError,
				relayInfo)
		} else if requestplugin.Stopped(taskErr) {
			// A request plugin stopped the attempt; its error log shows what
			// the plugins did.
			service.RecordRelayErrorLog(c, channel.Id, taskReportError(taskErr), relayInfo)
		}

		willRetry := decision.Action == "retry"
		diagnostics.attemptFailed(retryParam.GetRetry()+1, channel, taskErr, willRetry)
		if !willRetry {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := common.LogText("retry: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	if taskErr != nil {
		diagnostics.failed(stage, "task_error", taskErr, false)
		return nil, taskErr
	}
	if result == nil {
		taskErr = service.TaskErrorWrapperLocal(errors.New("task submission returned no result"), "task_submit_failed", http.StatusInternalServerError)
		diagnostics.failed("submit", "missing_result", taskErr, false)
		return nil, taskErr
	}
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_reserve", retryParam.GetRetry()+1)
		return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
	}

	// Reserve any submit-time upward billing adjustment before persistence.
	// This keeps insertion failures fully refundable while ensuring settlement
	// after the barrier normally has a zero positive delta. A relayed event
	// stream is billed even when this reserve fails: settlement then charges
	// past the available quota, as it does for any usage above its reservation.
	if relayInfo.Billing != nil {
		stage = "reserve"
		diagnostics.reserve("reserve_start", result.Quota)
		if reserveErr := relayInfo.Billing.Reserve(result.Quota); reserveErr != nil {
			common.SysError(common.LogText("reserve adjusted task billing error: %s", reserveErr.Error()))
			if !c.Writer.Written() {
				taskErr = service.TaskErrorWrapperLocal(errors.New("insufficient quota for adjusted task cost"), string(types.ErrorCodeInsufficientUserQuota), http.StatusForbidden)
				diagnostics.failed("reserve", "insufficient_quota", taskErr, false)
				return nil, taskErr
			}
		} else {
			diagnostics.reserve("reserve_complete", result.Quota)
		}
	}
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_insert", retryParam.GetRetry()+1)
		return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
	}

	stage = "insert"
	task := model.InitTask(result.Platform, relayInfo)
	task.PrivateData.Execution = service.TaskExecutionSnapshotFromContext(c)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.NodeName = common.NodeName
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:       relayInfo.PriceData.ModelPrice,
		GroupRatio:       relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:       relayInfo.PriceData.ModelRatio,
		OtherRatios:      relayInfo.PriceData.OtherRatios(),
		OriginModelName:  relayInfo.OriginModelName,
		PerCallBilling:   common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		PreConsumedQuota: result.Quota,
		TieredSnapshot:   relayInfo.TieredBillingSnapshot,
	}
	if requestValue, exists := c.Get("task_request"); exists {
		if requestBody, ok := requestValue.(map[string]any); ok {
			task.PrivateData.RequestSnapshot = service.BuildTaskRequestSnapshot(c.Request.URL.Path, requestBody)
		}
	}
	task.Quota = result.Quota
	task.Data = result.TaskData
	if len(result.PluginState) > 0 {
		task.PrivateData.PluginState = result.PluginState
	}
	// A request plugin instance that may report the task gets it once, at
	// its final state.
	task.PrivateData.RequestPlugins = requestplugin.TaskReport(c, relayInfo)
	task.Action = relayInfo.Action
	if immediate := result.Immediate; immediate != nil {
		task.Status = model.TaskStatus(immediate.Status)
		task.Progress = immediate.Progress
		if immediate.Status == model.TaskStatusSuccess || immediate.Status == model.TaskStatusFailure {
			task.FinishTime = time.Now().Unix()
		}
		if immediate.Status == model.TaskStatusFailure {
			task.FailReason = immediate.Reason
		}
		if immediate.Url != "" {
			task.PrivateData.ResultURL = immediate.Url
		} else if immediate.Status == model.TaskStatusSuccess {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
	}
	// A native submit route may declare retainResult: false. It applies only
	// to immediate terminal results: the client receives the complete response
	// once, the upstream snapshot is never persisted, and the task is not
	// retrievable afterwards. An asynchronous result on such a route keeps its
	// snapshot because polling and retrieval need it. The OpenAI Images
	// protocol delivers its images inline in the same HTTP response, so its
	// immediate results follow the same rule; an asynchronous image task is
	// expected there and is polled inside the request.
	var insertOmits []string
	immediateTerminal := result.Immediate != nil && (result.Immediate.Status == model.TaskStatusSuccess || result.Immediate.Status == model.TaskStatusFailure)
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedRoute); exists {
		pinned, ok := pinnedValue.(pluginruntime.PinnedRoute)
		if ok && pinned.Route.RetainResult != nil && !*pinned.Route.RetainResult {
			if immediateTerminal {
				task.PrivateData.ResultDiscarded = true
				insertOmits = append(insertOmits, "data")
			} else {
				logger.LogWarn(c, common.LogText("task plugin route %s %s declares retainResult: false but returned an asynchronous result; retaining task %s", pinned.Route.Method, pinned.Route.Path, task.TaskID))
			}
		}
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists && immediateTerminal {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && pinned.Protocol == pluginruntime.ProtocolOpenAIImage {
			task.PrivateData.ResultDiscarded = true
			insertOmits = append(insertOmits, "data")
		}
	}
	diagnostics.insertStart(task)
	if insertErr := task.InsertWithContext(c.Request.Context(), insertOmits...); insertErr != nil {
		common.SysError(common.LogText("insert task error: %s", insertErr.Error()))
		taskErr = service.TaskErrorWrapperLocal(errors.New("failed to persist task"), "task_insert_failed", http.StatusInternalServerError)
		diagnostics.failed("insert", "database_error", taskErr, false)
		// The client of a relayed event stream already holds the vendor's
		// work, so it is settled and logged without a task row.
		if c.Writer.Written() {
			if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
				common.SysError(common.LogText("settle task billing error: %s", settleErr.Error()))
				return nil, taskErr
			}
			service.LogTaskConsumption(c, relayInfo, task)
		}
		return nil, taskErr
	}
	durable = true
	requestplugin.TaskCreated(c)
	if immediateTerminal {
		// Reported once settled, whatever the settlement did.
		defer requestplugin.ReportTask(task)
	}
	stage = "settle"
	diagnostics.durable(task)
	diagnostics.settleStart(task, result.Quota)

	if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
		common.SysError(common.LogText("settle task billing error: %s", settleErr.Error()))
		taskErr = service.TaskErrorWrapperLocal(errors.New("failed to settle task billing"), "task_billing_settlement_failed", http.StatusInternalServerError)
		diagnostics.failed("settle", "billing_error", taskErr, true)
		return nil, taskErr
	}
	perfmetrics.RecordImmediateTaskResult(relayInfo, result.Immediate)
	if task.Status != model.TaskStatusFailure {
		service.MarkRequestPolicySuccess(c, nil)
	} else {
		policy.AddEvent(service.PolicyEvent{Decision: service.PolicyDecision{Action: "stop", Reason: "task_failed", Source: "upstream"}})
	}
	service.LogTaskConsumption(c, relayInfo, task)
	diagnostics.complete(task, result.Quota)

	return &taskSubmissionOutcome{Result: result, Task: task, RelayInfo: relayInfo}, nil
}

func presentTaskSubmission(c *gin.Context, outcome *taskSubmissionOutcome) {
	diagnostics := newTaskPluginSubmitDiagnostics(c)
	// A native route that relayed the vendor's event stream has answered.
	if c.Writer.Written() {
		diagnostics.present(outcome.Task, "native_stream")
		return
	}
	otherRatios := outcome.RelayInfo.PriceData.OtherRatios()
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	if ratiosJSON, err := common.Marshal(otherRatios); err == nil {
		c.Header("X-New-Api-Other-Ratios", string(ratiosJSON))
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedRoute); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedRoute); ok && pinned.Plugin != nil && pinned.Route.Render != "" {
			view, err := service.BuildTaskPluginView(outcome.Task)
			requestValue, _ := c.Get(pluginruntime.ContextKeyRouteRequest)
			requestContext, _ := requestValue.(pluginruntime.RouteRequestContext)
			if err == nil {
				viewValue, valueErr := taskPluginProtocolJSONValue(view)
				if valueErr == nil {
					if body, callErr := pinned.Plugin.Engine.CallPath(c.Request.Context(), "native", []string{pinned.Route.Render}, requestContext.JSValue(), viewValue); callErr == nil {
						if replyErr := middleware.WriteNativeReply(c, http.StatusOK, body); replyErr == nil {
							diagnostics.present(outcome.Task, "native_presenter")
							return
						} else {
							logger.LogError(c, common.LogText("task plugin native submit presenter failed: %s", replyErr.Error()))
						}
					} else {
						logger.LogError(c, common.LogText("task plugin native submit presenter failed: %s", callErr.Error()))
					}
				} else {
					logger.LogError(c, common.LogText("encode task plugin native submit view failed: %s", valueErr.Error()))
				}
			} else {
				logger.LogError(c, common.LogText("build task plugin native submit view failed: %s", err.Error()))
			}
		}
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && pinned.Protocol == "openai_video" && pinned.Operation.Name == "create" {
			diagnostics.present(outcome.Task, "openai_video_create")
			c.JSON(http.StatusOK, taskSubmittedVideoBody(outcome.Task, outcome.RelayInfo.OriginModelName))
			return
		}
	}
	diagnostics.present(outcome.Task, "host_fallback")
	c.JSON(http.StatusOK, taskSubmittedVideoBody(outcome.Task, outcome.RelayInfo.OriginModelName))
}

// taskSubmittedVideoBody 统一提交响应的形状：不同渠道（百炼、第三方中转）
// 返回同一组字段，usage 是预扣口径，与查询接口的结算口径使用同一套键。
func taskSubmittedVideoBody(task *model.Task, modelName string) map[string]any {
	if modelName == "" {
		modelName = task.Properties.OriginModelName
	}
	createdAt := task.CreatedAt
	if createdAt == 0 {
		createdAt = task.SubmitTime
	}
	return map[string]any{
		"id":         task.TaskID,
		"object":     "video",
		"model":      modelName,
		"status":     "queued",
		"progress":   0,
		"created_at": createdAt,
		"usage":      service.TaskVideoUsage(task),
	}
}

func respondTaskSubmissionError(c *gin.Context, taskErr *taskdto.TaskError) {
	service.RecordRequestPolicyTermination(c, taskSubmissionAPIError(taskErr))
	newTaskPluginSubmitDiagnostics(c).presentError(taskErr)
	// The client of a relayed event stream already has the vendor's events,
	// including any error event; nothing can follow them.
	if c.Writer.Written() {
		return
	}
	if middleware.RespondTaskPluginError(c, taskErr) {
		return
	}
	respondTaskError(c, taskErr)
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *taskdto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = i18n.T(c, i18n.MsgRelayGroupUpstreamSaturated)
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

// taskSubmissionAPIError adapts a task error for the shared relay error paths.
// TaskError.Error is nil for many local rejections, so fall back to the message.
func taskSubmissionAPIError(taskErr *taskdto.TaskError) *types.NewAPIError {
	err := taskErr.Error
	if err == nil {
		err = errors.New(taskErr.Message)
	}
	return types.NewOpenAIError(err, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)
}

// decideTaskRetry is the single retry decision for task submissions. The
// reason is recorded in the request policy decision events of the log details.
func decideTaskRetry(c *gin.Context, taskErr *taskdto.TaskError, retryTimes int) service.PolicyDecision {
	stop := service.PolicyDecision{Action: "stop", Source: "system"}
	retry := service.PolicyDecision{Action: "retry", Reason: "retry_status_matched", Source: "system"}
	switch {
	case taskErr == nil:
		stop.Reason = "request_completed"
	case taskErr.NoRetry:
		stop.Reason = "task_accepted"
	case requestplugin.Stopped(taskErr):
		stop.Reason, stop.Source = "local_rejection", "plugin"
	case service.ShouldSkipRetryAfterChannelAffinityFailure(c):
		stop.Reason, stop.Source = "strict_session", "session_rule"
		if source := service.RequestPolicy(c).SessionModeSource; source != "" {
			stop.Source = source
		}
	case retryTimes <= 0:
		stop.Reason, stop.Source = "attempt_budget_exhausted", "global"
	case service.GetChannelConstraints(c).SuppressesRetry():
		stop.Reason, stop.Source = "pinned_channel", "channel_constraint"
	case taskErr.StatusCode == http.StatusTooManyRequests, taskErr.StatusCode == 307:
		return retry
	case taskErr.StatusCode/100 == 5:
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			stop.Reason = "system_retry_exclusion"
			break
		}
		return retry
	case taskErr.StatusCode == http.StatusBadRequest, taskErr.StatusCode == 408:
		// azure处理超时不重试
		stop.Reason = "status_not_retryable"
	case taskErr.LocalError:
		stop.Reason = "local_rejection"
	case taskErr.StatusCode/100 == 2:
		stop.Reason = "system_retry_exclusion"
	default:
		return retry
	}
	return stop
}

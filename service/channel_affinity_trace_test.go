package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Trace which channel each request of a multi-turn conversation lands on, per
// key, with messages accumulating like a real client session. This prints every
// pick so "did it stay on one channel" is directly visible.
func TestAffinityTraceMultiTurnPerKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB := model.DB
	previousMemoryCache := common.MemoryCacheEnabled
	model.DB = db
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCache
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	const (
		group   = "0.2倍率"
		mname   = "kimi-k3"
		total   = 28
		turns   = 6
	)
	priority := int64(0)
	emptySetting := ""
	for i := 0; i < total; i++ {
		weight := uint(10)
		channel := &model.Channel{
			Id: 800000 + i, Type: 1, Status: common.ChannelStatusEnabled,
			Name: fmt.Sprintf("ch-%02d", i), Models: mname, Group: group,
			Priority: &priority, Weight: &weight, Setting: &emptySetting,
		}
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group: group, Model: mname, ChannelId: channel.Id, Enabled: true,
			Priority: &priority, Weight: weight,
		}).Error)
	}

	model.InitChannelCache()

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
	affinitySetting := operation_setting.GetChannelAffinitySetting()
	previousRules := affinitySetting.Rules
	previousEnabled := affinitySetting.Enabled
	affinitySetting.Rules = []operation_setting.ChannelAffinityRule{rule}
	affinitySetting.Enabled = true
	t.Cleanup(func() {
		affinitySetting.Rules = previousRules
		affinitySetting.Enabled = previousEnabled
	})

	build := func(firstUser string, turn int) *gin.Context {
		messages := []map[string]string{{"role": "user", "content": firstUser}}
		for t := 0; t < turn; t++ {
			messages = append(messages,
				map[string]string{"role": "assistant", "content": fmt.Sprintf("reply %d", t)},
				map[string]string{"role": "user", "content": fmt.Sprintf("follow-up %d", t)})
		}
		payload, marshalErr := json.Marshal(map[string]any{
			"model": mname, "messages": messages,
		})
		require.NoError(t, marshalErr)

		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set(common.KeyRequestBody, payload)
		return ctx
	}

	keys := []struct {
		tokenID  int
		firstMsg string
	}{
		{11, "用户的第一个问题：杭州天气"},
		{22, "客户的第一个问题：写一首秋天的诗"},
		{33, "另一个人的第一个问题：解释量子纠缠"},
	}

	for _, k := range keys {
		line := ""
		for turn := 0; turn < turns; turn++ {
			ctx := build(k.firstMsg, turn)
			ctx.Set("token_id", k.tokenID)

			zero := 0
			retry := &RetryParam{Ctx: ctx, TokenGroup: group, ModelName: mname, Retry: &zero}
			channel, _, selectErr := SelectChannelForRequest(ctx, mname, retry)
			require.Nil(t, selectErr)
			require.NotNil(t, channel)

			RecordChannelAffinity(ctx, channel.Id)
			line += fmt.Sprintf("turn%d=#%d(%s) ", turn, channel.Id, channel.Name)
		}
		t.Logf("token_id=%d first_msg=%q -> %s", k.tokenID, k.firstMsg, line)
	}
}

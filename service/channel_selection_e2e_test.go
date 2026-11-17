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

// End-to-end reproduction of the deployed setup: 28 channels in one group,
// abilities enabled, content-hash affinity, then 30 different prompts through
// the real select path. If the deployment really pins one channel, this test
// should show the same thing.
func TestSelectChannelSpreadsAcrossRealCachePath(t *testing.T) {
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
		group = "0.2倍率"
		mname = "kimi-k3"
		total = 28
	)
	priority := int64(0)
	emptySetting := ""
	for i := 0; i < total; i++ {
		weight := uint(10)
		channel := &model.Channel{
			Id:       700000 + i,
			Type:     1,
			Status:   common.ChannelStatusEnabled,
			Name:     fmt.Sprintf("ch-%02d", i),
			Models:   mname,
			Group:    group,
			Priority: &priority,
			Weight:   &weight,
			Setting:  &emptySetting,
		}
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group:     group,
			Model:     mname,
			ChannelId: channel.Id,
			Enabled:   true,
			Priority:  &priority,
			Weight:    weight,
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

	selection := func(content string, tokenID int) (*model.Channel, *ChannelSelectError) {
		payload, marshalErr := json.Marshal(map[string]any{
			"model": mname,
			"messages": []map[string]string{
				{"role": "user", "content": content},
			},
		})
		require.NoError(t, marshalErr)

		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set(common.KeyRequestBody, payload)
		ctx.Set("token_id", tokenID)

		zero := 0
		retry := &RetryParam{Ctx: ctx, TokenGroup: group, ModelName: mname, Retry: &zero}
		channel, _, selectErr := SelectChannelForRequest(ctx, mname, retry)
		return channel, selectErr
	}

	// 30 distinct prompts, distinct tokens: nothing here shares an affinity key.
	hits := make(map[int]int, total)
	for i := 0; i < 30; i++ {
		channel, selectErr := selection(fmt.Sprintf("unique prompt number %d about topic %d", i, i*7), 9000+i)
		require.Nil(t, selectErr)
		require.NotNil(t, channel)
		hits[channel.Id]++
	}

	t.Logf("distribution over %d distinct channels of %d: %v", len(hits), total, hits)
	require.Greater(t, len(hits), 5,
		"30 distinct prompts must not collapse onto one channel, got %v", hits)
}

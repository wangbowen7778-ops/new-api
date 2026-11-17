package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVideoFilterProbe(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	for i := 0; i < 6; i++ {
		setting := `{"video_parsing_enabled": false}`
		if i < 2 {
			setting = `{"video_parsing_enabled": true}`
		}
		prio := int64(0)
		w := uint(10)
		ch := &Channel{Id: 9601 + i, Type: 1, Status: common.ChannelStatusEnabled,
			Models: "kimi-k3", Group: "default", Priority: &prio, Weight: &w, Setting: &setting}
		require.NoError(t, db.Create(ch).Error)
		require.NoError(t, db.Create(&Ability{Group: "default", Model: "kimi-k3",
			ChannelId: ch.Id, Enabled: true, Priority: &prio, Weight: w}).Error)
	}

	ch1, err := GetChannelById(9601, true)
	require.NoError(t, err)
	ch5, err := GetChannelById(9605, true)
	require.NoError(t, err)

	t.Logf("ch9601 raw Setting = %q (nil=%v)", func() string { if ch1.Setting != nil { return *ch1.Setting }; return "<nil>" }(), ch1.Setting == nil)
	t.Logf("ch9601 VideoParsingEnabled = %v", ch1.GetSetting().VideoParsingEnabled)
	t.Logf("ch9605 VideoParsingEnabled = %v", ch5.GetSetting().VideoParsingEnabled)

	videoFilters := []dto.ChannelFilter{{Kind: dto.FilterRequestContainsVideo}}
	ok1, _ := ChannelSatisfiesFilters(ch1, "kimi-k3", videoFilters)
	ok5, _ := ChannelSatisfiesFilters(ch5, "kimi-k3", videoFilters)
	t.Logf("FilterRequestContainsVideo: ch9601 ok=%v, ch9605 ok=%v", ok1, ok5)
	require.True(t, ok1, "video-enabled channel must pass the video filter")
	require.False(t, ok5, "non-video channel must be rejected by the video filter")

	got, err := GetChannel("default", "kimi-k3", 0, videoFilters)
	require.NoError(t, err)
	t.Logf("GetChannel(video filter) picked = #%d (must be 9601/9602)", got.Id)
	require.Contains(t, []int{9601, 9602}, got.Id, "video request must land on a video channel")
}

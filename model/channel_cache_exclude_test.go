package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// A retry must not draw the channel that just failed, and it must still settle
// on a channel once every candidate has been tried.
func TestGetRandomSatisfiedChannelSkipsExcludedChannels(t *testing.T) {
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = false })

	weight := uint(1)
	first := &Channel{Id: 910001, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: "m", Group: "g", Weight: &weight}
	second := &Channel{Id: 910002, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: "m", Group: "g", Weight: &weight}

	channelSyncLock.Lock()
	previousGroups := group2model2channels
	previousChannels := channelsIDM
	group2model2channels = map[string]map[string][]int{"g": {"m": {910001, 910002}}}
	channelsIDM = map[int]*Channel{910001: first, 910002: second}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = previousGroups
		channelsIDM = previousChannels
		channelSyncLock.Unlock()
	})

	exclude := func(ids ...int) []dto.ChannelFilter {
		return []dto.ChannelFilter{{Kind: dto.FilterExcludeChannels, ChannelIDs: ids}}
	}

	for range 20 {
		channel, err := GetRandomSatisfiedChannel("g", "m", 0, exclude(910001))
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.Equal(t, 910002, channel.Id, "the tried channel must not be drawn again")
	}

	for range 20 {
		channel, err := GetRandomSatisfiedChannel("g", "m", 0, exclude(910001, 910002))
		require.NoError(t, err)
		require.NotNil(t, channel, "excluding every candidate falls back to the full list")
	}

	require.Equal(t, 2, RetryChannelCount("g", "m", nil), "every candidate channel counts once")
	require.Equal(t, 1, MaxConcurrencyRetryProbes("g", "m", nil), "both channels share one priority tier")
	require.Equal(t, 0, RetryChannelCount("g", "missing", nil))
}

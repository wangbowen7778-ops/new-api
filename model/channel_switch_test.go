package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// installSwitchTestChannels publishes a cache holding channels/ids for one
// (group, model) pair and restores the previous cache afterwards.
func installSwitchTestChannels(t *testing.T, group string, model string, ids []int, channels []*Channel) {
	t.Helper()
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = false })

	byID := make(map[int]*Channel, len(ids))
	for i, id := range ids {
		byID[id] = channels[i]
	}

	channelSyncLock.Lock()
	previousGroups := group2model2channels
	previousChannels := channelsIDM
	group2model2channels = map[string]map[string][]int{group: {model: ids}}
	channelsIDM = byID
	channelSyncLock.Unlock()

	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = previousGroups
		channelsIDM = previousChannels
		channelSyncLock.Unlock()
	})
}

func switchTestChannel(id int, priority int64) *Channel {
	weight := uint(1)
	group := "g"
	models := "m"
	return &Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Status:   common.ChannelStatusEnabled,
		Name:     "switch-" + string(rune('a'+id%26)),
		Models:   models,
		Group:    group,
		Priority: &priority,
		Weight:   &weight,
	}
}

func excludeFilter(ids ...int) []dto.ChannelFilter {
	return []dto.ChannelFilter{{Kind: dto.FilterExcludeChannels, ChannelIDs: ids}}
}

// A refusal must move the request across priority tiers. The channel that just
// answered 429 usually sits alone in the top tier, so an attempt that stays
// inside one tier ends up back on that same channel.
func TestPickChannelForSwitchVisitsEveryPriorityTier(t *testing.T) {
	ids := []int{920001, 920002, 920003}
	installSwitchTestChannels(t, "g", "m", ids, []*Channel{
		switchTestChannel(920001, 10),
		switchTestChannel(920002, 5),
		switchTestChannel(920003, 0),
	})

	require.Equal(t, 3, RetryChannelCount("g", "m", nil), "a refusal budget covers every channel")
	require.Equal(t, 3, MaxConcurrencyRetryProbes("g", "m", nil), "three priorities means three tiers")

	tried := make([]int, 0, len(ids))
	for range len(ids) {
		channel, err := PickChannelForSwitch("g", "m", excludeFilter(tried...), nil)
		require.NoError(t, err)
		require.NotNil(t, channel, "every not-yet-tried channel must stay reachable, tried=%v", tried)
		require.NotContains(t, tried, channel.Id, "the refused channel must not be drawn again")
		tried = append(tried, channel.Id)
	}

	require.ElementsMatch(t, ids, tried, "the switch must visit every channel of the group exactly once")

	channel, err := PickChannelForSwitch("g", "m", excludeFilter(tried...), nil)
	require.NoError(t, err)
	require.Nil(t, channel, "an exhausted switch reports no channel instead of falling back to a tried one")
}

// The tiered selector re-admits a channel it was told to skip once the
// exclusion empties the candidate list: that fallback exists so an ordinary
// failure never turns into "no available channel". A refusal switch cannot use
// it, or the request spends its budget re-asking the channel that just said no.
func TestGetRandomSatisfiedChannelFallsBackWhenAllExcluded(t *testing.T) {
	ids := []int{920011, 920012}
	installSwitchTestChannels(t, "g", "m", ids, []*Channel{
		switchTestChannel(920011, 10),
		switchTestChannel(920012, 0),
	})

	for range 10 {
		channel, err := GetRandomSatisfiedChannel("g", "m", 0, excludeFilter(920011, 920012))
		require.NoError(t, err)
		require.NotNil(t, channel, "the tiered selector falls back to the full list")
		require.Contains(t, ids, channel.Id)
	}

	for range 10 {
		channel, err := PickChannelForSwitch("g", "m", excludeFilter(920011, 920012), nil)
		require.NoError(t, err)
		require.Nil(t, channel, "the switch reports exhaustion instead of re-admitting a tried channel")
	}
}

// Channels sharing one priority must still be walked one by one.
func TestPickChannelForSwitchWalksSiblingsOfOneTier(t *testing.T) {
	ids := []int{920021, 920022, 920023, 920024}
	installSwitchTestChannels(t, "g", "m", ids, []*Channel{
		switchTestChannel(920021, 0),
		switchTestChannel(920022, 0),
		switchTestChannel(920023, 0),
		switchTestChannel(920024, 0),
	})

	require.Equal(t, 1, MaxConcurrencyRetryProbes("g", "m", nil))

	tried := make([]int, 0, len(ids))
	for range len(ids) {
		channel, err := PickChannelForSwitch("g", "m", excludeFilter(tried...), nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.NotContains(t, tried, channel.Id)
		tried = append(tried, channel.Id)
	}
	require.ElementsMatch(t, ids, tried)
}

// Each channel carries its own concurrency cap, so a switch has to pass over a
// saturated candidate instead of walking into its cap and stalling there.
func TestPickChannelForSwitchSkipsSaturatedChannels(t *testing.T) {
	ids := []int{920031, 920032}
	installSwitchTestChannels(t, "g", "m", ids, []*Channel{
		switchTestChannel(920031, 0),
		switchTestChannel(920032, 0),
	})

	saturated := func(ch *Channel) bool { return ch.Id == 920031 }
	for range 20 {
		channel, err := PickChannelForSwitch("g", "m", nil, saturated)
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.Equal(t, 920032, channel.Id, "the saturated channel must lose the draw")
	}

	// Every candidate saturated: waiting for a slot must still be possible,
	// otherwise the request would report "no available channel" instead.
	allSaturated := func(*Channel) bool { return true }
	channel, err := PickChannelForSwitch("g", "m", nil, allSaturated)
	require.NoError(t, err)
	require.NotNil(t, channel, "one candidate is still returned when all are saturated")
	require.Contains(t, ids, channel.Id)
}

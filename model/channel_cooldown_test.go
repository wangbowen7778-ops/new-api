package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestMarkChannelCooldownKeepsTheLongerPark(t *testing.T) {
	ClearChannelCooldown(930001)
	t.Cleanup(func() { ClearChannelCooldown(930001) })

	MarkChannelCooldown(930001, time.Minute)
	MarkChannelCooldown(930001, time.Millisecond)

	require.True(t, ChannelCooling(930001), "a shorter park must not shorten a longer one")
}

func TestChannelCooldownExpires(t *testing.T) {
	ClearChannelCooldown(930002)
	t.Cleanup(func() { ClearChannelCooldown(930002) })

	MarkChannelCooldown(930002, 2*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	require.False(t, ChannelCooling(930002))
}

// A parked channel is skipped by selection, but the request still has to settle
// on something when every candidate is parked.
func TestGetRandomSatisfiedChannelSkipsCoolingChannels(t *testing.T) {
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = false
		ClearChannelCooldown(920001)
		ClearChannelCooldown(920002)
	})

	weight := uint(1)
	first := &Channel{Id: 920001, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: "m", Group: "g", Weight: &weight}
	second := &Channel{Id: 920002, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: "m", Group: "g", Weight: &weight}

	channelSyncLock.Lock()
	previousGroups := group2model2channels
	previousChannels := channelsIDM
	group2model2channels = map[string]map[string][]int{"g": {"m": {920001, 920002}}}
	channelsIDM = map[int]*Channel{920001: first, 920002: second}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = previousGroups
		channelsIDM = previousChannels
		channelSyncLock.Unlock()
	})

	MarkChannelCooldown(920001, time.Minute)
	for range 20 {
		channel, err := GetRandomSatisfiedChannel("g", "m", 0, nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.Equal(t, 920002, channel.Id, "a parked channel must be skipped while another is available")
	}

	MarkChannelCooldown(920002, time.Minute)
	channel, err := GetRandomSatisfiedChannel("g", "m", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, channel, "every candidate parked still yields one channel to settle on")
}

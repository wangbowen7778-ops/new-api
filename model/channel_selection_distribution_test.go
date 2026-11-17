package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// Twenty channels sharing one priority, one group and one weight must spread
// evenly. If this fails, the selection code is the cause of the "everything
// lands on one channel" report; if it passes, the candidate list itself must be
// smaller than expected on that deployment.
func TestSelectionSpreadsAcrossEqualChannels(t *testing.T) {
	const total = 20
	ids := make([]int, 0, total)
	channels := make([]*Channel, 0, total)
	for i := 0; i < total; i++ {
		id := 940000 + i
		ids = append(ids, id)
		channels = append(channels, switchTestChannel(id, 0))
	}
	installSwitchTestChannels(t, "g", "m", ids, channels)

	const pulls = 2000
	hits := make(map[int]int, total)
	for i := 0; i < pulls; i++ {
		channel, err := GetRandomSatisfiedChannel("g", "m", 0, nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		hits[channel.Id]++
	}

	require.Len(t, hits, total, "every channel must be reachable: got %d distinct of %d", len(hits), total)

	expected := pulls / total
	for id, count := range hits {
		require.InDelta(t, expected, count, float64(expected),
			"channel %d got %d of %d pulls, expected around %d", id, count, pulls, expected)
	}

	// Print the distribution so a failing run is self-explanatory.
	line := ""
	for _, id := range ids {
		line += fmt.Sprintf("%d:%d ", id, hits[id])
	}
	t.Log("distribution:", line)
}

// Different priorities must still spread inside the top tier when that tier has
// several channels.
func TestSelectionSpreadsInsideTopTier(t *testing.T) {
	ids := make([]int, 0, 8)
	channels := make([]*Channel, 0, 8)
	for i := 0; i < 4; i++ {
		id := 941000 + i
		ids = append(ids, id)
		channels = append(channels, switchTestChannel(id, 100)) // top tier
	}
	for i := 0; i < 4; i++ {
		id := 941100 + i
		ids = append(ids, id)
		channels = append(channels, switchTestChannel(id, 0)) // backup tier
	}
	installSwitchTestChannels(t, "g", "m", ids, channels)

	hits := make(map[int]int, 8)
	for i := 0; i < 400; i++ {
		channel, err := GetRandomSatisfiedChannel("g", "m", 0, nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		hits[channel.Id]++
	}

	for i := 0; i < 4; i++ {
		require.Positive(t, hits[941000+i], "top-tier channel 94100%d must be selectable", i)
	}
	for i := 0; i < 4; i++ {
		require.Zero(t, hits[941100+i], "backup tier channel 94110%d must not be drawn at retry 0", i)
	}
}

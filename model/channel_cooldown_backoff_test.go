package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A window limit lasts minutes, so the park has to grow with every consecutive
// refusal instead of probing the same channel every few seconds.
func TestCooldownBackoffDoublesThenCaps(t *testing.T) {
	base := 3 * time.Second
	limit := 60 * time.Second

	require.Equal(t, base, cooldownBackoffDuration(base, limit, 1))
	require.Equal(t, 2*base, cooldownBackoffDuration(base, limit, 2))
	require.Equal(t, 4*base, cooldownBackoffDuration(base, limit, 3))
	require.Equal(t, 8*base, cooldownBackoffDuration(base, limit, 4))

	require.Equal(t, 16*base, cooldownBackoffDuration(base, limit, 5), "3s->48s before the cap")

	for _, strikes := range []int{6, 7, 10, 50} {
		require.Equal(t, limit, cooldownBackoffDuration(base, limit, strikes),
			"strikes=%d must cap at the limit", strikes)
	}

	require.Equal(t, base, cooldownBackoffDuration(base, 0, 1), "limit 0 means uncapped")
}

// Consecutive refusals extend the park, and the streak survives an expired
// park so a channel stuck in a long window keeps backing off.
func TestMarkChannelCooldownBackoffExtendsOnRepeat(t *testing.T) {
	const channelID = 930001
	ClearChannelCooldown(channelID)
	t.Cleanup(func() { ClearChannelCooldown(channelID) })

	base := 20 * time.Millisecond
	limit := 200 * time.Millisecond

	MarkChannelCooldownBackoff(channelID, base, limit)
	first := loadCooldownEntry(channelID)
	require.Equal(t, 1, first.strikes)

	MarkChannelCooldownBackoff(channelID, base, limit)
	second := loadCooldownEntry(channelID)
	require.Equal(t, 2, second.strikes, "a second refusal must extend the streak")
	require.True(t, second.until.After(first.until), "the park must grow")

	// A success ends the streak.
	NoteChannelHealthy(channelID)
	require.False(t, ChannelCooling(channelID))

	MarkChannelCooldownBackoff(channelID, base, limit)
	afterSuccess := loadCooldownEntry(channelID)
	require.Equal(t, 1, afterSuccess.strikes, "a success must reset the doubling")
}

// The strike window resets after a quiet period so an occasional refusal does
// not accumulate into the longest park.
func TestChannelCooldownStrikeResetsAfterQuietWindow(t *testing.T) {
	const channelID = 930002
	ClearChannelCooldown(channelID)
	t.Cleanup(func() { ClearChannelCooldown(channelID) })

	channelCooldowns.Store(channelID, channelCooldownEntry{
		strikes: 6,
		lastAt:  time.Now().Add(-channelCooldownStrikeReset - time.Minute),
	})

	MarkChannelCooldownBackoff(channelID, 10*time.Millisecond, time.Second)
	require.Equal(t, 1, loadCooldownEntry(channelID).strikes,
		"a refusal after the quiet window restarts the streak")
}

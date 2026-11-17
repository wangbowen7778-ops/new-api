package model

import (
	"sort"
	"sync"
	"time"
)

// channelCooldowns records, per channel, how long it stays out of selection
// after an upstream refusal. It deliberately lives in process memory: a
// cooldown only has to stop this instance from re-probing a channel that just
// answered 429, and every instance learns that from its own responses.
var channelCooldowns sync.Map // channel id -> channelCooldownEntry

// channelCooldownEntry keeps the park deadline plus how many consecutive
// refusals produced it. A window limit can last minutes, so a fixed 3-second
// park would send one request at it every 3 seconds forever; doubling the park
// per consecutive refusal pushes the probes out until the window reopens.
type channelCooldownEntry struct {
	until   time.Time
	strikes int
	lastAt  time.Time
}

// channelCooldownStrikeReset clears the streak when a channel has been quiet
// this long, so an occasional refusal does not accumulate into a long park.
const channelCooldownStrikeReset = 10 * time.Minute

func loadCooldownEntry(channelID int) channelCooldownEntry {
	current, ok := channelCooldowns.Load(channelID)
	if !ok {
		return channelCooldownEntry{}
	}
	entry, ok := current.(channelCooldownEntry)
	if !ok {
		return channelCooldownEntry{}
	}
	return entry
}

// cooldownBackoffDuration returns base doubled once per extra strike, capped at
// limit. limit <= 0 means uncapped.
func cooldownBackoffDuration(base, limit time.Duration, strikes int) time.Duration {
	if strikes < 1 {
		strikes = 1
	}
	duration := base
	for i := 1; i < strikes; i++ {
		if limit > 0 && duration >= limit {
			break
		}
		duration *= 2
	}
	if limit > 0 && duration > limit {
		duration = limit
	}
	return duration
}

// MarkChannelCooldownBackoff parks a channel for base, doubling it for every
// consecutive refusal up to limit. It never shortens an existing park.
func MarkChannelCooldownBackoff(channelID int, base, limit time.Duration) {
	if channelID <= 0 || base <= 0 {
		return
	}
	now := time.Now()
	entry := loadCooldownEntry(channelID)
	if entry.strikes <= 0 || now.Sub(entry.lastAt) > channelCooldownStrikeReset {
		entry.strikes = 1
	} else {
		entry.strikes++
	}
	entry.lastAt = now

	until := now.Add(cooldownBackoffDuration(base, limit, entry.strikes))
	if until.After(entry.until) {
		entry.until = until
	}
	channelCooldowns.Store(channelID, entry)
}

// MarkChannelCooldown parks a channel until now+d, replacing any shorter park.
// It is used where the caller has no doubling strategy (the concurrency 429).
func MarkChannelCooldown(channelID int, d time.Duration) {
	if channelID <= 0 || d <= 0 {
		return
	}
	now := time.Now()
	entry := loadCooldownEntry(channelID)
	until := now.Add(d)
	if until.After(entry.until) {
		entry.until = until
	}
	entry.lastAt = now
	channelCooldowns.Store(channelID, entry)
}

// NoteChannelHealthy drops the cooldown record after a successful request, so
// the next refusal starts the doubling from base again.
func NoteChannelHealthy(channelID int) {
	if channelID <= 0 {
		return
	}
	channelCooldowns.Delete(channelID)
}

// ChannelCooling reports whether the channel is still parked after a refusal.
func ChannelCooling(channelID int) bool {
	entry := loadCooldownEntry(channelID)
	return !entry.until.IsZero() && time.Now().Before(entry.until)
}

// ClearChannelCooldown forgets a park, used when an administrator changes the
// channel and therefore expects it to be selectable again.
func ClearChannelCooldown(channelID int) {
	channelCooldowns.Delete(channelID)
}

// dropCoolingChannels removes parked candidates, but never lets the pool
// collapse. A burst of refusals can park most channels at once; keeping only
// the few unparked ones would funnel every request — including other users'
// — onto them. When that happens the channels closest to finishing their park
// are kept too, so at least a third of the candidates stay selectable.
func dropCoolingChannels(ids []int) []int {
	if len(ids) < 2 {
		return ids
	}
	now := time.Now()
	type parked struct {
		id    int
		until time.Time
	}
	kept := make([]int, 0, len(ids))
	resting := make([]parked, 0, len(ids))
	for _, id := range ids {
		entry := loadCooldownEntry(id)
		if entry.until.IsZero() || !now.Before(entry.until) {
			kept = append(kept, id)
			continue
		}
		resting = append(resting, parked{id: id, until: entry.until})
	}
	if len(resting) == 0 {
		return kept
	}
	if len(kept) == 0 {
		// Everything is parked: the caller still needs a channel to settle on.
		return ids
	}

	floor := (len(ids) + 2) / 3
	if len(kept) >= floor {
		return kept
	}
	sort.Slice(resting, func(i, j int) bool { return resting[i].until.Before(resting[j].until) })
	need := floor - len(kept)
	if need > len(resting) {
		need = len(resting)
	}
	for i := 0; i < need; i++ {
		kept = append(kept, resting[i].id)
	}
	return kept
}

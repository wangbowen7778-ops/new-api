package service

import (
	"sync"

	"github.com/QuantumNous/new-api/model"
)

// channelConcurrency tracks in-flight relay requests per channel. The limit is
// enforced per node: on multi-node deployments each node enforces the cap
// independently.
const channelConcurrencyStripes = 64

var channelConcurrency struct {
	mu    [channelConcurrencyStripes]sync.Mutex
	count map[int]int64
}

func init() {
	channelConcurrency.count = make(map[int]int64)
}

func channelConcurrencyStripe(id int) int {
	if id < 0 {
		id = -id
	}
	return id % channelConcurrencyStripes
}

// ChannelConcurrencyRoom reports whether the channel still has room under its
// per-node in-flight cap. limit <= 0 means unlimited and always has room.
func ChannelConcurrencyRoom(id int, limit int) bool {
	if limit <= 0 {
		return true
	}
	stripe := channelConcurrencyStripe(id)
	channelConcurrency.mu[stripe].Lock()
	defer channelConcurrency.mu[stripe].Unlock()
	return channelConcurrency.count[id] < int64(limit)
}

// saturatedChannelsSkip marks channels whose in-flight budget is already spent
// right now. Each channel carries its own cap, so a switch has to consult the
// candidate's own limit rather than the limit of the channel that just failed.
func saturatedChannelsSkip(channel *model.Channel) bool {
	return !ChannelConcurrencyRoom(channel.Id, channel.GetSetting().MaxConcurrentRequests)
}

// AcquireChannelConcurrency registers an in-flight request for the channel when
// its concurrency budget allows it. limit <= 0 means unlimited and always
// succeeds. When ok is false the budget is exhausted and nothing was
// registered. The returned release func is idempotent.
func AcquireChannelConcurrency(id int, limit int64) (release func(), ok bool) {
	if limit <= 0 {
		return func() {}, true
	}
	stripe := channelConcurrencyStripe(id)
	channelConcurrency.mu[stripe].Lock()
	defer channelConcurrency.mu[stripe].Unlock()
	if channelConcurrency.count[id] >= limit {
		return nil, false
	}
	channelConcurrency.count[id]++
	var once sync.Once
	return func() {
		once.Do(func() {
			channelConcurrency.mu[stripe].Lock()
			defer channelConcurrency.mu[stripe].Unlock()
			if channelConcurrency.count[id] > 0 {
				channelConcurrency.count[id]--
			}
			if channelConcurrency.count[id] == 0 {
				delete(channelConcurrency.count, id)
			}
		})
	}, true
}

package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
// channel2advancedCustomConfig caches parsed Advanced Custom (type 58) configs so
// path-aware selection avoids re-parsing JSON per request. Refreshed on full sync.
var channel2advancedCustomConfig map[int]*kitdto.AdvancedCustomConfig
var channelSyncLock sync.RWMutex

// channelCacheReloadLock runs one channel cache reload at a time, so a reload
// that read the database earlier never publishes over a later one.
var channelCacheReloadLock sync.Mutex

func InitChannelCache() {
	channelCacheReloadLock.Lock()
	defer channelCacheReloadLock.Unlock()
	if !common.MemoryCacheEnabled {
		InvalidatePricingCache()
		rebuildTaskAliasView()
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	newChannel2advancedCustomConfig := make(map[int]*kitdto.AdvancedCustomConfig)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
				newChannel2advancedCustomConfig[channel.Id] = config
			}
		}
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.SplitSeq(channel.Group, ",")
		for group := range groups {
			models := channel.GetModels()
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channel2advancedCustomConfig = newChannel2advancedCustomConfig
	channelSyncLock.Unlock()
	// Lock ordering: InvalidatePricingCache acquires updatePricingLock, and
	// GetPricing (holding updatePricingLock) nests channelSyncLock.RLock via
	// loadPricingAdvancedCustomConfigs. channelSyncLock MUST be released before
	// invalidating the pricing cache, otherwise the reversed order deadlocks.
	InvalidatePricingCache()
	rebuildTaskAliasView()
	common.SysLog(common.LogText("channels synced from database"))
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog(common.LogText("syncing channels from database"))
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(
	group string,
	model string,
	retry int,
	filters []dto.ChannelFilter,
) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannel(group, model, retry, filters)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// A retry excludes the channels it already attempted. When that exclusion
	// removes every candidate the request has nowhere left to go, so the
	// exclusion is dropped: the caller then settles on the upstream error it
	// already holds instead of replacing it with "no available channel".
	candidatesFor := func(modelName string) []int {
		channels, emptiedBy := filterCandidateIDs(group2model2channels[group][modelName], model, filters)
		if emptiedBy != dto.FilterExcludeChannels {
			return channels
		}
		remaining := make([]dto.ChannelFilter, 0, len(filters))
		for _, filter := range filters {
			if filter.Kind != dto.FilterExcludeChannels {
				remaining = append(remaining, filter)
			}
		}
		channels, _ = filterCandidateIDs(group2model2channels[group][modelName], model, remaining)
		return channels
	}

	// First, try to find channels with the exact model name.
	channels := candidatesFor(model)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		channels = candidatesFor(ratio_setting.RoutingMatchModelName(model))
	}

	// A channel parked after an upstream 429 is skipped while it cools down.
	// The skip is a preference, not a predicate: when every candidate is
	// parked the request still needs one to settle the refusal on.
	if kept := dropCoolingChannels(channels); len(kept) > 0 {
		channels = kept
	}

	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("database consistency error: channel #%d does not exist, please contact the administrator", channels[0])
	}

	candidates := make([]channelTier, 0, len(channels))
	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok {
			return nil, fmt.Errorf("database consistency error: channel #%d does not exist, please contact the administrator", channelId)
		}
		candidates = append(candidates, channelTier{priority: channel.GetPriority(), video: channel.GetSetting().VideoParsingEnabled})
	}
	tiers := buildPriorityTiers(candidates, requestsNonVideoPreference(filters))

	if retry >= len(tiers) {
		retry = len(tiers) - 1
	}
	targetTier := tiers[retry]

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetTier.priority && targetTier.matchesChannel(channel.GetSetting().VideoParsingEnabled) {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("database consistency error: channel #%d does not exist, please contact the administrator", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetTier.priority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

// pickWeightedChannel draws one channel from channels with the same weighted
// smoothing the priority-tier selection uses, so switching off a refusal keeps
// the configured traffic split between otherwise-equivalent channels.
func pickWeightedChannel(channels []*Channel) (*Channel, error) {
	if len(channels) == 0 {
		return nil, nil
	}
	var sumWeight = 0
	for _, channel := range channels {
		sumWeight += channel.GetWeight()
	}
	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = len(channels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(channels) < 10 {
		smoothingFactor = 100
	}
	totalWeight := sumWeight * smoothingFactor
	randomWeight := rand.Intn(totalWeight)
	for _, channel := range channels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	return nil, errors.New("channel not found")
}

// switchCandidatesLocked resolves the candidate ids a switch may still reach.
// exhausted reports that the list is empty only because every candidate was
// already attempted by this request. Caller must hold channelSyncLock.
func switchCandidatesLocked(group string, model string, filters []dto.ChannelFilter) (ids []int, exhausted bool) {
	lookup := func(modelName string) ([]int, bool) {
		candidates, emptiedBy := filterCandidateIDs(group2model2channels[group][modelName], model, filters)
		if emptiedBy == dto.FilterExcludeChannels {
			return nil, true
		}
		return candidates, false
	}

	ids, exhausted = lookup(model)
	if exhausted {
		return nil, true
	}
	if len(ids) == 0 {
		if normalized := ratio_setting.RoutingMatchModelName(model); normalized != "" && normalized != model {
			if ids, exhausted = lookup(normalized); exhausted {
				return nil, true
			}
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	if kept := dropCoolingChannels(ids); len(kept) > 0 {
		ids = kept
	}
	return ids, false
}

// PickChannelForSwitch selects a channel for a retry that has to move off the
// channel which just refused the request. Unlike GetRandomSatisfiedChannel it
// ignores priority tiers and treats the exclusion filter as strict:
//
//   - Tiers pin an attempt to a single priority level, so a top tier holding
//     only the channel that just answered 429 is exhausted and the attempt
//     falls back to that same channel.
//   - The exclusion filter is where "already attempted by this request" lives.
//     Falling back to the unfiltered list re-admits the refused channel and
//     spends the remaining budget re-learning the same refusal.
//
// skip, when non-nil, marks channels that cannot take another in-flight
// request right now (each channel carries its own concurrency cap). Those are
// passed over while any other candidate remains, because landing on a
// saturated channel turns the refusal switch into a concurrency wait; when
// every candidate is saturated they stay eligible, since waiting for a slot
// beats reporting no channel at all.
//
// It returns (nil, nil) once every candidate has been attempted so the caller
// can settle on the upstream error it already holds.
func PickChannelForSwitch(group string, model string, filters []dto.ChannelFilter, skip func(*Channel) bool) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return pickChannelForSwitchFromDB(group, model, filters, skip)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	ids, exhausted := switchCandidatesLocked(group, model, filters)
	if exhausted || len(ids) == 0 {
		return nil, nil
	}
	channels := make([]*Channel, 0, len(ids))
	for _, id := range ids {
		if channel, ok := channelsIDM[id]; ok {
			channels = append(channels, channel)
		}
	}
	return pickWeightedChannel(dropSaturatedChannels(channels, skip))
}

// dropSaturatedChannels moves channels flagged by skip out of the draw, keeping
// them only when nothing else is left.
func dropSaturatedChannels(channels []*Channel, skip func(*Channel) bool) []*Channel {
	if skip == nil || len(channels) < 2 {
		return channels
	}
	roomy := make([]*Channel, 0, len(channels))
	for _, channel := range channels {
		if !skip(channel) {
			roomy = append(roomy, channel)
		}
	}
	if len(roomy) == 0 {
		return channels
	}
	return roomy
}

func pickChannelForSwitchFromDB(group string, model string, filters []dto.ChannelFilter, skip func(*Channel) bool) (*Channel, error) {
	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).Find(&abilities).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}
	abilities, channelsByID := filterAbilitiesByConstraintsWithChannels(abilities, model, filters)
	if len(abilities) == 0 {
		return nil, nil
	}
	channels := make([]*Channel, 0, len(abilities))
	for _, ability := range abilities {
		if channel := channelsByID[ability.ChannelId]; channel != nil {
			channels = append(channels, channel)
		}
	}
	return pickWeightedChannel(dropSaturatedChannels(channels, skip))
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("channel #%d no longer exists", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("channel #%d no longer exists", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	if channel == nil {
		channelSyncLock.Unlock()
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	// An administrator touched the channel, so it is expected to serve again
	// even if a previous refusal parked it.
	ClearChannelCooldown(channel.Id)
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	if channel2advancedCustomConfig == nil {
		channel2advancedCustomConfig = make(map[int]*kitdto.AdvancedCustomConfig)
	}
	delete(channel2advancedCustomConfig, channel.Id)
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			channel2advancedCustomConfig[channel.Id] = config
		}
	}
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
	// Lock ordering: do NOT hold channelSyncLock while calling
	// InvalidatePricingCache. GetPricing acquires updatePricingLock first and then
	// channelSyncLock.RLock (via loadPricingAdvancedCustomConfigs); acquiring
	// updatePricingLock while holding channelSyncLock would be an AB-BA deadlock.
	channelSyncLock.Unlock()
	InvalidatePricingCache()
}

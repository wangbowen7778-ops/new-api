package model

import (
	"slices"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	return slices.Contains(list, channelID)
}

// requestsNonVideoPreference reports whether the request carries the non-video
// ordering preference. The filter is a selection hint, never a predicate:
// filterEvalOrder does not include it, so video channels stay selectable and
// ChannelSatisfiesFilters keeps passing them.
func requestsNonVideoPreference(filters []dto.ChannelFilter) bool {
	for _, filter := range filters {
		if filter.Kind == dto.FilterNonVideoPreferred {
			return true
		}
	}
	return false
}

// channelTier identifies one selectable tier: a priority, plus the lane it
// belongs to. anyLane marks a tier that is not lane-specific, so a single
// non-video-preferred request keeps the historical one-tier-per-priority
// grouping.
type channelTier struct {
	priority int64
	video    bool
	anyLane  bool
}

func (t channelTier) matchesChannel(video bool) bool {
	return t.anyLane || t.video == video
}

// buildPriorityTiers returns the selection tiers in order. Without the
// non-video preference (video requests, or requests that carry no
// classification) there is one lane-agnostic tier per distinct priority,
// descending.
//
// With the preference set (a text/image request), the non-video lane comes
// first and the video lane after it: every non-video tier descends, then every
// video-only tier descends. A priority holding both kinds yields two tiers —
// its non-video channels ahead of its video ones — so a video channel is only
// selected once no non-video channel is reachable at any priority. The retry
// index walks this list, which is what makes a retry reach a video channel
// only after every non-video tier has been probed.
func buildPriorityTiers(candidates []channelTier, preferNonVideo bool) []channelTier {
	if !preferNonVideo {
		priorities := make([]int64, 0, len(candidates))
		seen := make(map[int64]bool, len(candidates))
		for _, candidate := range candidates {
			if seen[candidate.priority] {
				continue
			}
			seen[candidate.priority] = true
			priorities = append(priorities, candidate.priority)
		}
		sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
		tiers := make([]channelTier, 0, len(priorities))
		for _, priority := range priorities {
			tiers = append(tiers, channelTier{priority: priority, anyLane: true})
		}
		return tiers
	}

	nonVideo := make([]int64, 0, len(candidates))
	videoOnly := make([]int64, 0, len(candidates))
	seenNonVideo := make(map[int64]bool, len(candidates))
	seenVideo := make(map[int64]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.video {
			if !seenVideo[candidate.priority] {
				seenVideo[candidate.priority] = true
				videoOnly = append(videoOnly, candidate.priority)
			}
			continue
		}
		if !seenNonVideo[candidate.priority] {
			seenNonVideo[candidate.priority] = true
			nonVideo = append(nonVideo, candidate.priority)
		}
	}
	sort.Slice(nonVideo, func(i, j int) bool { return nonVideo[i] > nonVideo[j] })
	sort.Slice(videoOnly, func(i, j int) bool { return videoOnly[i] > videoOnly[j] })

	tiers := make([]channelTier, 0, len(nonVideo)+len(videoOnly))
	for _, priority := range nonVideo {
		tiers = append(tiers, channelTier{priority: priority})
	}
	for _, priority := range videoOnly {
		tiers = append(tiers, channelTier{priority: priority, video: true})
	}
	return tiers
}

// satisfiedCandidates lists the enabled channels serving (group, modelName)
// after the request's channel filters, together with the non-video ordering
// preference the filters carry.
func satisfiedCandidates(group string, modelName string, filters []dto.ChannelFilter) ([]channelTier, bool) {
	if group == "" || modelName == "" {
		return nil, false
	}
	if !common.MemoryCacheEnabled {
		var abilities []Ability
		if err := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).Find(&abilities).Error; err != nil {
			return nil, false
		}
		abilities, channelsByID := filterAbilitiesByConstraintsWithChannels(abilities, modelName, filters)
		candidates := make([]channelTier, 0, len(abilities))
		for _, ability := range abilities {
			priority := int64(0)
			if ability.Priority != nil {
				priority = *ability.Priority
			}
			channel := channelsByID[ability.ChannelId]
			candidates = append(candidates, channelTier{
				priority: priority,
				video:    channel != nil && channel.GetSetting().VideoParsingEnabled,
			})
		}
		return candidates, channelsByID != nil && requestsNonVideoPreference(filters)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return nil, false
	}
	ids := group2model2channels[group][modelName]
	if len(ids) == 0 {
		normalized := ratio_setting.RoutingMatchModelName(modelName)
		if normalized != "" && normalized != modelName {
			ids = group2model2channels[group][normalized]
		}
	}
	ids, _ = filterCandidateIDs(ids, modelName, filters)
	candidates := make([]channelTier, 0, len(ids))
	for _, id := range ids {
		channel, ok := channelsIDM[id]
		if !ok {
			continue
		}
		candidates = append(candidates, channelTier{priority: channel.GetPriority(), video: channel.GetSetting().VideoParsingEnabled})
	}
	return candidates, requestsNonVideoPreference(filters)
}

// MaxConcurrencyRetryProbes counts the distinct priority tiers among the
// enabled channels serving (group, modelName) after the request's channel
// filters, so the concurrency-wait loop can cycle its tier index across all
// of them. The ordering of buildPriorityTiers makes that index walk the
// preferred lane first, so a non-video request probes every non-video tier
// before it reaches a video-only one.
func MaxConcurrencyRetryProbes(group string, modelName string, filters []dto.ChannelFilter) int {
	candidates, preferNonVideo := satisfiedCandidates(group, modelName, filters)
	return len(buildPriorityTiers(candidates, preferNonVideo))
}

// RetryChannelCount counts the channels a retry can reach for (group,
// modelName) after the request's filters. A 429 never served the request, so
// its switch budget is one probe per channel and every channel is tried once
// before the request settles on the error.
func RetryChannelCount(group string, modelName string, filters []dto.ChannelFilter) int {
	candidates, _ := satisfiedCandidates(group, modelName, filters)
	return len(candidates)
}

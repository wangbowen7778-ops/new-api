package model

import (
	"slices"

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

// MaxConcurrencyRetryProbes counts the distinct priority tiers among the
// enabled channels serving (group, modelName) after the request's channel
// filters, so the concurrency-wait loop can cycle its tier index across all
// of them.
func MaxConcurrencyRetryProbes(group string, modelName string, filters []dto.ChannelFilter) int {
	if group == "" || modelName == "" {
		return 0
	}
	if !common.MemoryCacheEnabled {
		var priorities []int64
		DB.Model(&Ability{}).
			Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).
			Distinct("priority").
			Pluck("priority", &priorities)
		return len(priorities)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return 0
	}
	ids := group2model2channels[group][modelName]
	if len(ids) == 0 {
		normalized := ratio_setting.RoutingMatchModelName(modelName)
		if normalized != "" && normalized != modelName {
			ids = group2model2channels[group][normalized]
		}
	}
	ids, _ = filterCandidateIDs(ids, modelName, filters)
	seen := make(map[int64]struct{}, 4)
	for _, id := range ids {
		channel, ok := channelsIDM[id]
		if !ok {
			continue
		}
		seen[channel.GetPriority()] = struct{}{}
	}
	return len(seen)
}

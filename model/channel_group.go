package model

import (
	"github.com/QuantumNous/new-api/common"
)

// GetEnabledChannelsByGroup 返回该分组下所有已启用渠道，按优先级与 ID 排序。
//
// 素材类接口（/v1/fleet/media、/v1/catalog/models）不带 model 字段，无法用
// GetRandomSatisfiedChannel 选择渠道，只能按分组挑一个。优先用 abilities 表
// 里该分组真实存在的渠道，避免把分组没授权的渠道也选进来。
func GetEnabledChannelsByGroup(group string) ([]*Channel, error) {
	if group == "" {
		return nil, nil
	}
	var ids []int
	err := DB.Table("abilities").
		Where(commonGroupCol+" = ? and enabled = ?", group, true).
		Distinct("channel_id").
		Pluck("channel_id", &ids).Error
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var channels []*Channel
	if err := DB.Where("id IN ? AND status = ?", ids, common.ChannelStatusEnabled).
		Order("priority DESC, id ASC").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// GetAllEnabledChannels 返回全部启用渠道，按优先级与 ID 排序。
// 素材类接口在令牌分组没有匹配渠道时的兜底：素材不绑定模型，
// 分组过滤落空时仍应能取到可用渠道。
func GetAllEnabledChannels() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("status = ?", common.ChannelStatusEnabled).
		Order("priority DESC, id ASC").
		Find(&channels).Error
	return channels, err
}

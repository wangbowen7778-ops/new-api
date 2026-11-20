package jsplugin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// validateTaskMaterialBindings 校验 input.media[].asset_id 是否属于本次选中的渠道。
//
// 上游的资产 ID 只在上传它的那个调用身份下有效。如果同一个模型挂了多个渠道，
// 资产可能被上传到 A 渠道却在 B 渠道引用，上游只会回一句“资产不存在”之类的模糊
// 错误。这里提前拦住并给出可定位的提示。
func validateTaskMaterialBindings(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if info == nil || info.ChannelId <= 0 {
		return nil
	}
	value, exists := c.Get("task_request")
	if !exists {
		return nil
	}
	for _, materialId := range collectMaterialAssetIDs(value) {
		material, err := model.GetTaskMaterialByMaterialId(materialId)
		if err != nil || material == nil {
			// 查不到记录说明不是本网关登记的资产，交给上游判定。
			continue
		}
		if material.ChannelId > 0 && material.ChannelId != info.ChannelId {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("asset %s was registered on channel #%d but this request is routed to channel #%d; re-upload the asset on the same channel",
					materialId, material.ChannelId, info.ChannelId),
				"material_channel_mismatch", http.StatusBadRequest)
		}
	}
	for _, mediaId := range collectFleetMediaIDs(value) {
		material, err := model.GetTaskMaterialByMaterialId(mediaId)
		if err != nil || material == nil {
			continue
		}
		if material.ChannelId > 0 && material.ChannelId != info.ChannelId {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("media %s was uploaded on channel #%d but this request is routed to channel #%d; re-upload it on the same channel",
					mediaId, material.ChannelId, info.ChannelId),
				"material_channel_mismatch", http.StatusBadRequest)
		}
	}
	return nil
}

// collectMaterialAssetIDs 从请求体里取出 input.media[].asset_id。
func collectMaterialAssetIDs(body any) []string {
	requestBody, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	input, ok := requestBody["input"].(map[string]any)
	if !ok {
		return nil
	}
	media, ok := input["media"].([]any)
	if !ok {
		return nil
	}
	return mediaIDs(media)
}

// collectFleetMediaIDs 从请求体里取出 parameters.media[].id（共享车素材引用）。
func collectFleetMediaIDs(body any) []string {
	requestBody, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	parameters, ok := requestBody["parameters"].(map[string]any)
	if !ok {
		return nil
	}
	media, ok := parameters["media"].([]any)
	if !ok {
		return nil
	}
	return mediaIDs(media)
}

func mediaIDs(media []any) []string {
	ids := make([]string, 0, len(media))
	for _, item := range media {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, ok := entry["id"].(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

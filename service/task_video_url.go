package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// taskUpstreamVideoURLPaths 是对客展示用的上游视频地址在任务原始响应里的常见位置。
// 各上游写法不同（百炼放在 output.video_url，第三方中转放在 metadata.url），
// 这里按优先级逐个取，不做任何改写。
var taskUpstreamVideoURLPaths = [][]string{
	{"url"},
	{"video_url"},
	{"output", "video_url"},
	{"output", "url"},
	{"metadata", "url"},
	{"data", "url"},
	{"result", "url"},
}

// TaskUpstreamVideoURL 返回上游给出的视频直链，用于对客下发可匿名访问的地址。
// 优先级：任务结算落库的 result_url（若它不是网关自身的代理地址）→ 上游响应体里的
// 直链。取不到时返回空串，调用方自行回退。
func TaskUpstreamVideoURL(task *model.Task) string {
	if task == nil {
		return ""
	}
	if result := strings.TrimSpace(task.PrivateData.ResultURL); result != "" && !isGatewayTaskContentURL(result, task.TaskID) {
		return result
	}
	return upstreamURLFromTaskData(task)
}

// isGatewayTaskContentURL 判断地址是否指向本网关的内容接口。
// 网关代理地址对下游来说需要带令牌，不能作为匿名直链下发。
func isGatewayTaskContentURL(raw, taskID string) bool {
	if taskID == "" {
		return false
	}
	return strings.Contains(raw, "/v1/videos/"+taskID+"/content") ||
		strings.Contains(raw, "/v1/tasks/"+taskID+"/artifacts/")
}

func upstreamURLFromTaskData(task *model.Task) string {
	if len(task.Data) == 0 {
		return ""
	}
	var body map[string]any
	if err := common.Unmarshal(task.Data, &body); err != nil {
		return ""
	}
	for _, path := range taskUpstreamVideoURLPaths {
		if value := nestedString(body, path); strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			return value
		}
	}
	return ""
}

func nestedString(source map[string]any, path []string) string {
	current := any(source)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	text, _ := current.(string)
	return strings.TrimSpace(text)
}

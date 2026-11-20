package plugins_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// 用阿里官方文档的三个示例体，验证经 TD 路径后 input/parameters 是否逐字段一致。
func TestTdOfficialSamplesTmp(t *testing.T) {
	plugin := newAlibabaPlugin(t)

	img := func(u string) map[string]any { return map[string]any{"type": "reference_image", "url": u} }

	samples := map[string]map[string]any{
		"reference_mix": {
			"model": "wan3.0-video",
			"prompt": "视频1抱着图3，在图4的椅子上弹奏…",
			"input": map[string]any{"media": []any{
				img("https://x/obj-girl.jpg"),
				map[string]any{"type": "reference_video", "url": "https://x/role2.mp4"},
				img("https://x/object3.png"),
				img("https://x/object4.png"),
				img("https://x/background5.png"),
			}},
			"parameters": map[string]any{"resolution": "480P", "ratio": "adaptive", "duration": float64(5), "prompt_extend": true},
		},
		"first_last_frame": {
			"model": "wan3.0-video",
			"prompt": "清晨太阳刚刚升起…",
			"input": map[string]any{"media": []any{
				map[string]any{"type": "first_frame", "url": "https://x/first.webp"},
				map[string]any{"type": "last_frame", "url": "https://x/last.webp"},
			}},
			"parameters": map[string]any{"resolution": "480P", "ratio": "adaptive", "duration": float64(5), "prompt_extend": true},
		},
		"file_input": {
			"model": "wan3.0-video",
			"prompt": "一支高端智能眼镜产品广告…",
			"input": map[string]any{"media": []any{
				map[string]any{"type": "file", "url": "https://x/glass.pptx"},
			}},
			"parameters": map[string]any{"resolution": "480P", "ratio": "adaptive", "duration": float64(10), "prompt_extend": true},
		},
	}

	for _, name := range []string{"reference_mix", "first_last_frame", "file_input"} {
		request := samples[name]
		body, facts, _ := submitAlibabaRequest(t, plugin, "wan3.0-video", request)
		upInput, _ := body["input"].(map[string]any)
		upParams, _ := body["parameters"].(map[string]any)
		src := request["input"].(map[string]any)

		srcJSON, _ := json.Marshal(src["media"])
		upJSON, _ := json.Marshal(upInput["media"])
		paramsJSON, _ := json.Marshal(upParams)
		factsJSON, _ := json.Marshal(facts)
		fmt.Printf("%-16s media_same=%v prompt_same=%v params=%s facts=%s\n",
			name, string(srcJSON) == string(upJSON), upInput["prompt"] == request["prompt"], paramsJSON, factsJSON)
	}
}

package plugins_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The public TD contract puts input and parameters at the top level, as
// DashScope's own body does, while the converter reads metadata. Both shapes
// must reach DashScope identically; metadata stays authoritative when both are
// present.
func TestAlibabaVideoAcceptsTopLevelInputAndParameters(t *testing.T) {
	plugin := newAlibabaPlugin(t)

	input := map[string]any{
		"media": []any{
			map[string]any{"type": "first_frame", "url": "https://example.com/1.png"},
			map[string]any{"type": "last_frame", "url": "https://example.com/2.png"},
		},
	}
	parameters := map[string]any{"audio": false, "seed": float64(7), "prompt_extend": false}
	base := map[string]any{
		"model":      "wan3.0-video",
		"prompt":     "两个人相视而笑",
		"duration":   float64(5),
		"resolution": "1080P",
		"ratio":      "16:9",
	}
	with := func(extra map[string]any) map[string]any {
		request := map[string]any{}
		for key, value := range base {
			request[key] = value
		}
		for key, value := range extra {
			request[key] = value
		}
		return request
	}

	metadataBody, _, _ := submitAlibabaRequest(t, plugin, "wan3.0-video", with(map[string]any{
		"metadata": map[string]any{"input": input, "parameters": parameters},
	}))
	tdBody, tdFacts, _ := submitAlibabaRequest(t, plugin, "wan3.0-video", with(map[string]any{
		"input":      input,
		"parameters": parameters,
	}))

	assert.Equal(t, metadataBody, tdBody)
	converted := alibabaObject(t, tdBody)
	assert.Equal(t, input["media"], converted["input"].(map[string]any)["media"])
	convertedParameters := converted["parameters"].(map[string]any)
	assert.Equal(t, false, convertedParameters["prompt_extend"])
	assert.Equal(t, false, convertedParameters["audio"])
	assert.Equal(t, float64(7), convertedParameters["seed"])
	assert.Equal(t, map[string]any{"resolution": "1080P", "seconds": float64(5), "output_seconds": float64(5)}, tdFacts)

	// metadata wins when the client sends both shapes.
	both, _, _ := submitAlibabaRequest(t, plugin, "wan3.0-video", with(map[string]any{
		"input":      input,
		"parameters": parameters,
		"metadata":   map[string]any{"parameters": map[string]any{"audio": true}},
	}))
	assert.Equal(t, true, alibabaObject(t, both)["parameters"].(map[string]any)["audio"])

	// Without either shape the request is unchanged.
	promptOnly, _, _ := submitAlibabaRequest(t, plugin, "wan3.0-video", base)
	assert.Equal(t, map[string]any{
		"input":      map[string]any{"prompt": "两个人相视而笑"},
		"model":      "wan3.0-video",
		"parameters": map[string]any{"duration": float64(5), "prompt_extend": true, "ratio": "16:9", "resolution": "1080P"},
	}, alibabaObject(t, promptOnly))
}

// 宿主探测出的参考视频时长（_reference_seconds）要计入 Wan3 的预扣，
// 并拆出输出/输入两段事实；探测字段本身不能进入上游请求体。
func TestAlibabaWanReferenceDurationEntersReservation(t *testing.T) {
	plugin := newAlibabaPlugin(t)
	clip := "https://cdn.example/clip.mp4"
	request := map[string]any{
		"model":              "wan3.0-video",
		"prompt":             "flowers in the breeze",
		"duration":           float64(5),
		"_reference_seconds": float64(5),
		"media":              []any{map[string]any{"type": "reference_video", "url": clip}},
	}
	body, facts, _ := submitAlibabaRequest(t, plugin, "wan3.0-video", request)

	assert.NotContains(t, body, "_reference_seconds")
	assert.Equal(t, float64(10), facts["seconds"])
	assert.Equal(t, float64(5), facts["output_seconds"])
	assert.Equal(t, float64(5), facts["input_video_seconds"])
}

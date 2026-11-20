package plugins_test

import "testing"

func TestSoraResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "sora",
		model:     "sora-2-pro",
		requestBody: map[string]any{
			"model":   "sora-2-pro",
			"input":   "waves at sunset",
			"seconds": 8,
			"size":    "1792x1024",
		},
		wantAction: "text_to_video",
		wantRequest: map[string]any{
			"model":   "sora-2-pro",
			"prompt":  "waves at sunset",
			"seconds": float64(8),
			"size":    "1792x1024",
		},
		// 定制计费按分辨率档位（480P/720P/1080P）取价，usage_facts 多声明一个 resolution。
		wantUsageKeys: []string{"resolution", "seconds", "size"},
		// extractUsage 在提交阶段额外产出 output_seconds（成片秒数，结算阶段再拆输入/输出）。
		wantSubmitUsageKeys: []string{"output_seconds", "resolution", "seconds", "size"},
		wantVendorName:      "sora",
	})
}

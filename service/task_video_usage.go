package service

import "github.com/QuantumNous/new-api/model"

// TaskVideoUsage 返回对客任务响应里固定形状的 usage 块，提交与查询共用。
// 数值取本地计费快照（提交后即预扣口径，结算后即最终口径），与扣费一致。
func TaskVideoUsage(task *model.Task) map[string]any {
	return TaskVideoUsageWithUpstream(task, nil)
}

// TaskVideoUsageWithUpstream 在本地快照缺失时回退到插件渲染出的上游 usage，
// 使历史任务也给出同一组键：百炼用 input_video_duration/output_video_duration，
// 第三方中转用 input_seconds/output_seconds，对客只有一套字段名。
func TaskVideoUsageWithUpstream(task *model.Task, upstream map[string]any) map[string]any {
	if task == nil {
		return emptyTaskVideoUsage("final")
	}
	var facts map[string]any
	if bc := task.PrivateData.BillingContext; bc != nil && bc.TieredSnapshot != nil {
		facts = bc.TieredSnapshot.UsageFacts
	}
	input, hasInput := taskUsageSeconds(facts, "input_video_seconds")
	if !hasInput {
		input, hasInput = taskUsageSeconds(upstream, "input_video_seconds", "input_seconds", "input_video_duration")
	}
	output, hasOutput := taskUsageSeconds(facts, "output_seconds")
	if !hasOutput {
		output, hasOutput = taskUsageSeconds(upstream, "output_video_seconds", "output_seconds", "output_video_duration", "duration")
	}
	total, hasTotal := taskUsageSeconds(facts, "seconds")
	if !hasTotal {
		total, hasTotal = taskUsageSeconds(upstream, "billable_seconds", "seconds", "total_seconds")
	}
	switch {
	case hasInput && hasOutput:
		if !hasTotal || total <= 0 {
			total = input + output
		}
	case hasTotal:
		hasInput = false
		if !hasOutput && total >= input {
			output = total - input
		}
	}
	if !hasInput {
		input = 0
	}
	if !hasOutput {
		output = 0
	}
	semantic := "reserved"
	switch task.Status {
	case model.TaskStatusSuccess, model.TaskStatusFailure:
		semantic = "final"
	}
	if task.Status == model.TaskStatusFailure {
		// 失败任务全额退款，对客用量与实际扣费保持一致。
		input, output, total = 0, 0, 0
	}
	return taskVideoUsageBlock(input, output, total, semantic)
}

func emptyTaskVideoUsage(semantic string) map[string]any {
	return taskVideoUsageBlock(0, 0, 0, semantic)
}

func taskVideoUsageBlock(input, output, total float64, semantic string) map[string]any {
	return map[string]any{
		"input_seconds":        input,
		"output_seconds":       output,
		"input_video_seconds":  input,
		"output_video_seconds": output,
		"billable_seconds":     total,
		"usage_semantic":       semantic,
	}
}

// taskUsageSeconds 按优先级读取第一个可用的秒数，缺失与 0 秒可区分。
func taskUsageSeconds(source map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if source == nil {
			return 0, false
		}
		value, exists := source[key]
		if !exists {
			continue
		}
		if number, ok := taskUsageNumber(value); ok {
			return number, true
		}
	}
	return 0, false
}

func taskUsageNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int64:
		return float64(number), true
	case int:
		return float64(number), true
	default:
		return 0, false
	}
}

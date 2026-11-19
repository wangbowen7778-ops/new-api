package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestTaskUpstreamVideoURLPrefersStoredUpstreamLink(t *testing.T) {
	task := &model.Task{
		TaskID:      "task_upstream",
		PrivateData: model.TaskPrivateData{ResultURL: "https://oss.example/video.mp4?Expires=1"},
	}
	assert.Equal(t, "https://oss.example/video.mp4?Expires=1", TaskUpstreamVideoURL(task))
}

// 中转渠道把网关代理地址写进 result_url，真实直链只在任务响应体里。
func TestTaskUpstreamVideoURLFallsBackToTaskData(t *testing.T) {
	task := &model.Task{
		TaskID: "task_relay",
		PrivateData: model.TaskPrivateData{
			ResultURL: "http://gateway.example/v1/videos/task_relay/content",
		},
		Data: []byte(`{"id":"task_relay","metadata":{"url":"https://oss.example/relay.mp4?Expires=2"},"usage":{"duration":7}}`),
	}
	assert.Equal(t, "https://oss.example/relay.mp4?Expires=2", TaskUpstreamVideoURL(task))
}

func TestTaskUpstreamVideoURLReadsNestedOutputURL(t *testing.T) {
	task := &model.Task{
		TaskID: "task_bailian",
		Data:   []byte(`{"output":{"task_id":"x","task_status":"SUCCEEDED","video_url":"https://oss.example/bailian.mp4?Expires=3"}}`),
	}
	assert.Equal(t, "https://oss.example/bailian.mp4?Expires=3", TaskUpstreamVideoURL(task))
}

func TestTaskUpstreamVideoURLEmptyWhenOnlyGatewayLink(t *testing.T) {
	task := &model.Task{
		TaskID:      "task_none",
		PrivateData: model.TaskPrivateData{ResultURL: "http://gateway.example/v1/videos/task_none/content"},
		Data:        []byte(`{"id":"task_none"}`),
	}
	assert.Empty(t, TaskUpstreamVideoURL(task))
	assert.Empty(t, TaskUpstreamVideoURL(nil))
}

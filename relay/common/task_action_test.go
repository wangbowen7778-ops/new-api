package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestTaskActionFromSubmitRequest(t *testing.T) {
	cases := []struct {
		name string
		req  TaskSubmitReq
		want string
	}{
		{
			name: "plain text",
			req:  TaskSubmitReq{Prompt: "a cat"},
			want: constant.TaskActionTextToVideo,
		},
		{
			name: "single image field",
			req:  TaskSubmitReq{Prompt: "a cat", Image: "https://cdn.example/a.png"},
			want: constant.TaskActionImageToVideo,
		},
		{
			name: "one reference image",
			req: TaskSubmitReq{Input: &TaskSubmitInput{Media: []TaskSubmitMedia{
				{Type: "reference_image"},
			}}},
			want: constant.TaskActionImageToVideo,
		},
		{
			name: "several reference images",
			req: TaskSubmitReq{Input: &TaskSubmitInput{Media: []TaskSubmitMedia{
				{Type: "reference_image"}, {Type: "reference_image"},
			}}},
			want: constant.TaskActionReferenceToVideo,
		},
		{
			name: "single reference video",
			req: TaskSubmitReq{Input: &TaskSubmitInput{Media: []TaskSubmitMedia{
				{Type: "reference_video"},
			}}},
			want: constant.TaskActionReferenceToVideo,
		},
		{
			name: "multi_ref mode without media",
			req:  TaskSubmitReq{Mode: "multi_ref"},
			want: constant.TaskActionReferenceToVideo,
		},
		{
			name: "images and video",
			req: TaskSubmitReq{Input: &TaskSubmitInput{Media: []TaskSubmitMedia{
				{Type: "reference_image"}, {Type: "reference_image"}, {Type: "reference_video"},
			}}},
			want: constant.TaskActionReferenceToVideo,
		},
		{
			name: "metadata input media",
			req: TaskSubmitReq{Metadata: map[string]any{
				"input": map[string]any{
					"media": []any{
						map[string]any{"type": "reference_image"},
						map[string]any{"type": "reference_video"},
					},
				},
			}},
			want: constant.TaskActionReferenceToVideo,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, taskActionFromSubmitRequest(tc.req))
		})
	}
}

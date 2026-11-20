package jsplugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputMediaItemsReadsEveryRequestShape(t *testing.T) {
	video := map[string]any{"type": "reference_video", "url": "https://cdn.example/clip.mp4"}
	other := map[string]any{"type": "reference_video", "url": "https://cdn.example/other.mp4"}

	// OpenAI-style clients put media at the top level of the video request.
	openAI := map[string]any{"model": "wan3.0-video", "media": []any{video}}
	assert.Equal(t, []any{video}, inputMediaItems(openAI))

	// DashScope's own body nests media under input.
	native := map[string]any{"model": "wan3.0-video", "input": map[string]any{"media": []any{video}}}
	assert.Equal(t, []any{video}, inputMediaItems(native))

	// The public TD contract nests media under metadata.input, which the
	// converter treats as authoritative over input.
	metadata := map[string]any{
		"model":    "wan3.0-video",
		"input":    map[string]any{"media": []any{video}},
		"metadata": map[string]any{"input": map[string]any{"media": []any{other}}},
	}
	assert.Equal(t, []any{other}, inputMediaItems(metadata))

	assert.Nil(t, inputMediaItems(map[string]any{"model": "wan3.0-video"}))
	assert.Nil(t, inputMediaItems(map[string]any{"media": "not-an-array"}))
}

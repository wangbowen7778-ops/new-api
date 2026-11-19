package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTaskRequestSnapshotExtractsPromptParametersAndMedia(t *testing.T) {
	body := map[string]any{
		"model":      "wan3.0-video",
		"prompt":     "flowers in the breeze",
		"duration":   float64(5),
		"resolution": "720P",
		"ratio":      "16:9",
		"input": map[string]any{
			"media": []any{
				map[string]any{"type": "reference_video", "url": "https://cdn.example/ref.mp4"},
				map[string]any{"type": "reference_image", "url": "https://cdn.example/frame.png"},
			},
		},
		"_reference_seconds": float64(5.5),
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	assert.Equal(t, "/v1/videos", snapshot.Source)
	assert.Equal(t, "wan3.0-video", snapshot.Model)
	assert.Equal(t, "flowers in the breeze", snapshot.Prompt)
	assert.Equal(t, map[string]any{
		"duration":   float64(5),
		"ratio":      "16:9",
		"resolution": "720P",
	}, snapshot.Parameters)
	require.Len(t, snapshot.Media, 2)
	assert.Equal(t, "video", snapshot.Media[0].Kind)
	assert.Equal(t, "https://cdn.example/ref.mp4", snapshot.Media[0].URL)
	assert.Equal(t, 5.5, snapshot.Media[0].DurationSeconds)
	assert.Equal(t, "image", snapshot.Media[1].Kind)
	assert.Equal(t, "https://cdn.example/frame.png", snapshot.Media[1].URL)
	assert.Zero(t, snapshot.Media[1].DurationSeconds)
}

func TestBuildTaskRequestSnapshotUsesPerMediaDurationsAndMetadataPriority(t *testing.T) {
	body := map[string]any{
		"model":    "wan3.0-video",
		"prompt":   "top level prompt",
		"duration": float64(5),
		"metadata": map[string]any{
			"parameters": map[string]any{"duration": float64(10), "resolution": "480P"},
			"input": map[string]any{
				"media": []any{
					map[string]any{"type": "reference_video", "url": "https://cdn.example/a.mp4"},
					map[string]any{"type": "reference_video", "url": "https://cdn.example/b.mp4"},
				},
			},
		},
		"_reference_durations": []any{
			map[string]any{"url": "https://cdn.example/a.mp4", "seconds": float64(3)},
			map[string]any{"url": "https://cdn.example/b.mp4", "seconds": float64(4)},
		},
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	assert.Equal(t, float64(10), snapshot.Parameters["duration"])
	assert.Equal(t, "480P", snapshot.Parameters["resolution"])
	require.Len(t, snapshot.Media, 2)
	assert.Equal(t, 3.0, snapshot.Media[0].DurationSeconds)
	assert.Equal(t, 4.0, snapshot.Media[1].DurationSeconds)
}

func TestBuildTaskRequestSnapshotKeepsInlineAndOversizedURLsOut(t *testing.T) {
	longURL := "https://cdn.example/large.png?signature=" + strings.Repeat("a", 1100)
	body := map[string]any{
		"model":  "wan3.0-video",
		"prompt": "p",
		"images": []any{"data:image/png;base64,AAAA", longURL},
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Media, 2)
	assert.True(t, snapshot.Media[0].Inline)
	assert.Empty(t, snapshot.Media[0].URL)
	assert.Equal(t, "image", snapshot.Media[0].Kind)
	assert.Equal(t, "https://cdn.example/large.png", snapshot.Media[1].URL)
}

func TestBuildTaskRequestSnapshotCapsMediaAndTruncatesPrompt(t *testing.T) {
	images := make([]any, 0, 24)
	for i := 0; i < 24; i++ {
		images = append(images, "https://cdn.example/image-"+strconv.Itoa(i)+".png")
	}
	body := map[string]any{
		"model":  "wan3.0-video",
		"prompt": strings.Repeat("p", 5000),
		"images": images,
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	assert.Len(t, snapshot.Media, taskRequestSnapshotMaxMedia)
	assert.LessOrEqual(t, len([]rune(snapshot.Prompt)), taskRequestSnapshotMaxPrompt)
	assert.NotEqual(t, strings.Repeat("p", 5000), snapshot.Prompt)
}

func TestBuildTaskRequestSnapshotRecordsUploadedFiles(t *testing.T) {
	body := map[string]any{
		"model":  "wan3.0-video",
		"prompt": "p",
		"_uploaded_files": []any{
			map[string]any{
				"field":        "input_reference",
				"filename":     "frame.png",
				"content_type": "image/png",
			},
			map[string]any{
				"field":        "input_reference",
				"filename":     "clip.mp4",
				"content_type": "video/mp4",
			},
			map[string]any{
				"field":        "audio",
				"filename":     "voice.mp3",
				"content_type": "audio/mpeg",
			},
		},
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Media, 2)
	assert.Equal(t, "image", snapshot.Media[0].Kind)
	assert.Equal(t, "frame.png", snapshot.Media[0].Filename)
	assert.Equal(t, "reference_image", snapshot.Media[0].Type)
	assert.Equal(t, "video", snapshot.Media[1].Kind)
	assert.Equal(t, "clip.mp4", snapshot.Media[1].Filename)
	assert.Equal(t, "reference_video", snapshot.Media[1].Type)
}

func TestBuildTaskRequestSnapshotReturnsNilForEmptyRequest(t *testing.T) {
	assert.Nil(t, BuildTaskRequestSnapshot("", map[string]any{}))
	assert.Nil(t, BuildTaskRequestSnapshot("", nil))
}

func TestBuildTaskRequestSnapshotKeepsAssetReferencesAndExtraParameters(t *testing.T) {
	body := map[string]any{
		"model":  "wan3.0-video",
		"prompt": "p",
		"input": map[string]any{
			"media": []any{
				map[string]any{"type": "reference_image", "asset_id": "material.asset.abc"},
				map[string]any{"type": "reference_audio", "url": "https://cdn.example/voice.mp3"},
			},
			"parameters": map[string]any{
				"negative_prompt": "blur",
				"cfg_scale":       float64(4.5),
			},
		},
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	assert.Equal(t, "blur", snapshot.Parameters["negative_prompt"])
	assert.Equal(t, float64(4.5), snapshot.Parameters["cfg_scale"])
	require.Len(t, snapshot.Media, 1)
	assert.Equal(t, "material.asset.abc", snapshot.Media[0].AssetID)
	assert.Empty(t, snapshot.Media[0].URL)
	assert.Equal(t, "image", snapshot.Media[0].Kind)
}

func TestBuildTaskRequestSnapshotMatchesDurationsAfterURLTrimming(t *testing.T) {
	longURL := "https://cdn.example/ref.mp4?Signature=" + strings.Repeat("a", 1100)
	body := map[string]any{
		"model": "wan3.0-video",
		"input": map[string]any{
			"media": []any{
				map[string]any{"type": "reference_video", "url": longURL},
			},
		},
		"_reference_durations": []any{
			map[string]any{"url": longURL, "seconds": float64(8)},
		},
	}

	snapshot := BuildTaskRequestSnapshot("/v1/videos", body)
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Media, 1)
	assert.Equal(t, "https://cdn.example/ref.mp4", snapshot.Media[0].URL)
	assert.Equal(t, 8.0, snapshot.Media[0].DurationSeconds)
}

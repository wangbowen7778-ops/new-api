package service

import (
	"math"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

const (
	taskRequestSnapshotMaxPrompt     = 4000
	taskRequestSnapshotMaxParameters = 24
	taskRequestSnapshotMaxMedia      = 16
	taskRequestSnapshotMaxURLLen     = 1024
)

// BuildTaskRequestSnapshot 从插件解析后的任务请求体提取可展示的请求素材：
// 提示词、标量参数、图片/视频 URL 以及本地探测到的参考视频时长。
// 只保留元数据，不保存二进制或内联 data URI 内容（内联素材仅标记 Inline）。
func BuildTaskRequestSnapshot(source string, body map[string]any) *model.TaskRequestSnapshot {
	if len(body) == 0 {
		return nil
	}

	parameters := taskRequestParameters(body)
	media := taskRequestMedia(body)
	prompt := taskRequestPrompt(body)

	snapshot := &model.TaskRequestSnapshot{
		Source:     strings.TrimSpace(source),
		Model:      taskRequestString(body["model"]),
		Prompt:     truncateRequestText(prompt, taskRequestSnapshotMaxPrompt),
		Parameters: parameters,
		Media:      media,
	}
	if snapshot.Model == "" {
		for _, object := range []map[string]any{taskRequestObject(body["metadata"]), taskRequestObject(body["input"])} {
			if snapshot.Model == "" {
				snapshot.Model = taskRequestString(object["model"])
			}
		}
	}
	if snapshot.Source == "" && snapshot.Model == "" && snapshot.Prompt == "" && len(parameters) == 0 && len(media) == 0 {
		return nil
	}
	return snapshot
}

func taskRequestPrompt(body map[string]any) string {
	for _, candidate := range []any{
		body["prompt"],
		taskRequestObject(body["input"])["prompt"],
		taskRequestObject(taskRequestObject(body["metadata"])["input"])["prompt"],
	} {
		if prompt := taskRequestString(candidate); prompt != "" {
			return prompt
		}
	}
	return ""
}

// taskRequestParameters 收集展示用标量参数。优先级从低到高：
// 顶层字段、parameters、input.parameters、metadata.parameters（与插件合并顺序一致）。
// 容器内的模型私有扩展字段全部保留，只丢弃非标量值。
func taskRequestParameters(body map[string]any) map[string]any {
	parameters := make(map[string]any, taskRequestSnapshotMaxParameters)
	// 顶层只取视频协议里的常见字段，避免把 model、prompt、素材塞进参数。
	for _, key := range []string{
		"resolution", "duration", "seconds", "size", "ratio", "aspect_ratio", "fps",
		"audio", "watermark", "prompt_extend", "mode", "quality", "seed",
	} {
		if value := body[key]; value != nil {
			parameters[key] = value
		}
	}
	for _, object := range []map[string]any{
		taskRequestObject(body["parameters"]),
		taskRequestObject(taskRequestObject(body["input"])["parameters"]),
		taskRequestObject(taskRequestObject(body["metadata"])["parameters"]),
	} {
		for key, value := range object {
			parameters[key] = value
		}
	}
	result := make(map[string]any, len(parameters))
	for key, value := range parameters {
		if scalar, ok := taskRequestScalar(value); ok {
			result[key] = scalar
		}
	}
	if len(result) == 0 {
		return nil
	}
	return limitRequestParameters(result)
}

// limitRequestParameters 控制快照体积：常用字段优先保留，其余按键名排序截断。
func limitRequestParameters(parameters map[string]any) map[string]any {
	if len(parameters) <= taskRequestSnapshotMaxParameters {
		return parameters
	}
	priority := []string{
		"resolution", "duration", "seconds", "size", "ratio", "aspect_ratio", "fps",
		"audio", "watermark", "prompt_extend", "mode", "quality", "seed",
	}
	limited := make(map[string]any, taskRequestSnapshotMaxParameters)
	for _, key := range priority {
		if value, present := parameters[key]; present && len(limited) < taskRequestSnapshotMaxParameters {
			limited[key] = value
		}
	}
	rest := make([]string, 0, len(parameters))
	for key := range parameters {
		if _, present := limited[key]; !present {
			rest = append(rest, key)
		}
	}
	slices.Sort(rest)
	for _, key := range rest {
		if len(limited) >= taskRequestSnapshotMaxParameters {
			break
		}
		limited[key] = parameters[key]
	}
	return limited
}

// taskRequestMedia 汇总各协议写法里的图片/视频素材，并挂上本地探测时长。
func taskRequestMedia(body map[string]any) []model.TaskRequestMedia {
	media := make([]model.TaskRequestMedia, 0, taskRequestSnapshotMaxMedia)
	seen := make(map[string]struct{}, taskRequestSnapshotMaxMedia)

	add := func(value any, defaultType string) {
		if len(media) >= taskRequestSnapshotMaxMedia {
			return
		}
		item, ok := taskRequestMediaItem(value, defaultType)
		if !ok {
			return
		}
		identity := item.URL
		if identity == "" {
			identity = item.AssetID
		}
		if identity != "" {
			key := item.Kind + "\x00" + identity
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
		}
		media = append(media, item)
	}

	for _, array := range []any{
		taskRequestObject(body["input"])["media"],
		body["media"],
		taskRequestObject(body["parameters"])["media"],
		taskRequestObject(taskRequestObject(body["metadata"])["input"])["media"],
		taskRequestObject(taskRequestObject(body["metadata"])["parameters"])["media"],
		body["images"],
	} {
		for _, value := range taskRequestArray(array) {
			add(value, "")
		}
	}
	for _, key := range []string{"image", "image_url", "img_url", "input_reference", "first_frame_url", "last_frame_url"} {
		add(body[key], "reference_image")
	}
	// multipart 直传的素材没有 URL，只记录字段名与文件名。
	for _, value := range taskRequestArray(body["_uploaded_files"]) {
		entry := taskRequestObject(value)
		filename := taskRequestString(entry["filename"])
		if filename == "" {
			continue
		}
		contentType := strings.ToLower(taskRequestString(entry["content_type"]))
		if strings.HasPrefix(contentType, "audio/") {
			continue
		}
		kind := "image"
		if strings.HasPrefix(contentType, "video/") || mediaURLIsVideo(filename) {
			kind = "video"
		}
		if len(media) >= taskRequestSnapshotMaxMedia {
			break
		}
		media = append(media, model.TaskRequestMedia{
			Type:     taskRequestUploadedMediaType(taskRequestString(entry["field"]), kind),
			Kind:     kind,
			Filename: filename,
		})
	}

	durations := taskReferenceDurations(body)
	totalFallback, hasTotalFallback := durations[""]
	videoWithURL := 0
	for index := range media {
		if media[index].Kind == "video" && media[index].URL != "" {
			videoWithURL++
		}
	}
	for index := range media {
		if media[index].Kind != "video" || media[index].URL == "" {
			continue
		}
		if duration, ok := matchRequestDuration(durations, media[index].URL); ok {
			media[index].DurationSeconds = roundRequestSeconds(duration)
		} else if videoWithURL == 1 && hasTotalFallback {
			media[index].DurationSeconds = roundRequestSeconds(totalFallback)
		}
	}
	return media
}

// matchRequestDuration 先按完整 URL 匹配，再按去掉查询串的地址匹配：
// 快照可能为了控制体积裁掉了签名参数，探测记录仍是原始地址。
func matchRequestDuration(durations map[string]float64, rawURL string) (float64, bool) {
	if duration, ok := durations[rawURL]; ok {
		return duration, true
	}
	normalized := stripURLForDisplay(rawURL)
	if normalized == "" {
		return 0, false
	}
	for recorded, duration := range durations {
		if recorded != "" && stripURLForDisplay(recorded) == normalized {
			return duration, true
		}
	}
	return 0, false
}

func taskRequestMediaItem(value any, defaultType string) (model.TaskRequestMedia, bool) {
	var item model.TaskRequestMedia
	var rawURL, assetID, entryType string
	var duration float64
	switch entry := value.(type) {
	case string:
		rawURL = entry
		entryType = defaultType
	case map[string]any:
		rawURL = taskRequestString(entry["url"])
		if rawURL == "" {
			rawURL = taskRequestString(entry["image_url"])
		}
		if rawURL == "" {
			rawURL = taskRequestString(entry["img_url"])
		}
		if rawURL == "" {
			rawURL = taskRequestString(entry["video_url"])
		}
		assetID = taskRequestString(entry["asset_id"])
		if assetID == "" {
			assetID = taskRequestString(entry["material_id"])
		}
		entryType = taskRequestString(entry["type"])
		if entryType == "" {
			entryType = defaultType
		}
		if number, ok := taskRequestNumber(entry["duration_seconds"]); ok {
			duration = number
		} else if number, ok := taskRequestNumber(entry["duration"]); ok {
			duration = number
		} else if number, ok := taskRequestNumber(entry["seconds"]); ok {
			duration = number
		}
	default:
		return model.TaskRequestMedia{}, false
	}

	rawURL = strings.TrimSpace(rawURL)
	entryType = strings.TrimSpace(entryType)
	if entryType == "" {
		if mediaURLIsVideo(rawURL) {
			entryType = "reference_video"
		} else {
			entryType = "reference_image"
		}
	}
	kind := taskRequestMediaKind(entryType, rawURL)
	if kind == "" {
		return model.TaskRequestMedia{}, false
	}

	item = model.TaskRequestMedia{Type: entryType, Kind: kind, AssetID: assetID}
	if rawURL == "" {
		// 素材库引用（asset_id）只留标识；两者都没有的条目没有展示价值。
		if assetID == "" {
			return model.TaskRequestMedia{}, false
		}
	} else if strings.HasPrefix(rawURL, "data:") || strings.HasPrefix(rawURL, "blob:") {
		item.Inline = true
	} else if len(rawURL) <= taskRequestSnapshotMaxURLLen {
		item.URL = rawURL
	} else if safe := stripURLForDisplay(rawURL); safe != "" && len(safe) <= taskRequestSnapshotMaxURLLen {
		item.URL = safe
	}
	if kind == "video" && duration > 0 {
		item.DurationSeconds = roundRequestSeconds(duration)
	}
	return item, true
}

// taskRequestUploadedMediaType 用上传字段名推断素材角色：OpenAI 的
// input_reference 是参考图，first_frame/last_frame 保留首尾帧语义。
func taskRequestUploadedMediaType(field, kind string) string {
	lower := strings.ToLower(field)
	switch {
	case strings.Contains(lower, "video"):
		return "reference_video"
	case strings.Contains(lower, "frame"):
		return lower
	case kind == "video":
		// 字段名没写视频，但文件本身是视频（如 input_reference 传 mp4）。
		return "reference_video"
	case strings.Contains(lower, "image"), lower == "input_reference", lower == "file":
		return "reference_image"
	default:
		return "reference_image"
	}
}

func taskRequestMediaKind(entryType, rawURL string) string {
	lowerType := strings.ToLower(entryType)
	if strings.Contains(lowerType, "audio") {
		return ""
	}
	if strings.Contains(lowerType, "video") || strings.Contains(lowerType, "clip") || strings.Contains(lowerType, "frame_video") {
		return "video"
	}
	if strings.Contains(lowerType, "image") || strings.Contains(lowerType, "frame") {
		return "image"
	}
	if mediaURLIsVideo(rawURL) {
		return "video"
	}
	if rawURL == "" {
		return ""
	}
	return "image"
}

func mediaURLIsVideo(rawURL string) bool {
	value := strings.ToLower(strings.TrimSpace(rawURL))
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "data:video/") {
		return true
	}
	if parsed, err := url.Parse(value); err == nil {
		value = strings.ToLower(parsed.Path)
	}
	switch path.Ext(value) {
	case ".mp4", ".mov", ".webm", ".mkv", ".avi", ".m4v", ".mpeg", ".mpg":
		return true
	}
	return false
}

// stripURLForDisplay drops the query string of an oversized URL so a signed
// URL never bloats the task record while its path remains identifiable.
func stripURLForDisplay(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func taskReferenceDurations(body map[string]any) map[string]float64 {
	durations := make(map[string]float64)
	for _, value := range taskRequestArray(body["_reference_durations"]) {
		entry := taskRequestObject(value)
		rawURL := taskRequestString(entry["url"])
		number, ok := taskRequestNumber(entry["seconds"])
		if rawURL == "" || !ok || number <= 0 {
			continue
		}
		durations[rawURL] = number
	}
	if len(durations) == 0 {
		if total, ok := taskRequestNumber(body["_reference_seconds"]); ok && total > 0 {
			durations[""] = total
		}
	}
	return durations
}

func roundRequestSeconds(value float64) float64 {
	return math.Round(value*100) / 100
}

func taskRequestString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func taskRequestNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case int32:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint64:
		return float64(number), true
	case jsonNumber:
		value, err := number.Float64()
		return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
	default:
		return 0, false
	}
}

// jsonNumber lets numericValue accept encoding/json numbers without importing
// the encoding in business code.
type jsonNumber interface {
	Float64() (float64, error)
}

func taskRequestScalar(value any) (any, bool) {
	switch scalar := value.(type) {
	case string:
		text := strings.TrimSpace(scalar)
		if text == "" {
			return nil, false
		}
		return truncateRequestText(text, taskRequestSnapshotMaxURLLen), true
	case bool:
		return scalar, true
	case float64:
		if math.IsNaN(scalar) || math.IsInf(scalar, 0) {
			return nil, false
		}
		return scalar, true
	case int, int64, int32, uint, uint64:
		return scalar, true
	default:
		return nil, false
	}
}

func truncateRequestText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	// 按 rune 截断，避免切坏 UTF-8。
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit > 1 {
		return string(runes[:limit-1]) + "…"
	}
	return string(runes[:limit])
}

func taskRequestObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func taskRequestArray(value any) []any {
	switch entries := value.(type) {
	case []any:
		return entries
	case []string:
		result := make([]any, 0, len(entries))
		for _, entry := range entries {
			result = append(result, entry)
		}
		return result
	default:
		return nil
	}
}

package common

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func SanitizeURLForLog(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	if len(query) == 0 {
		return rawURL
	}

	changed := false
	for key := range query {
		if isSensitiveURLQueryKey(key) {
			query.Set(key, "***masked***")
			changed = true
		}
	}
	if !changed {
		return rawURL
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func isSensitiveURLQueryKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "key",
		"api_key",
		"api-key",
		"apikey",
		"x-api-key",
		"access_token",
		"refresh_token",
		"id_token",
		"token",
		"authorization",
		"auth",
		"client_secret",
		"secret",
		"password",
		"passwd",
		"signature",
		"sig",
		"awsaccesskeyid",
		"x-amz-credential",
		"x-amz-security-token",
		"x-amz-signature":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "signature")
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	// TaskSubmitReq 只是各视频协议的交集（model/prompt/mode/size/duration/
	// seconds/metadata）。插件驱动的上游协议差异很大：OpenAI 兼容认 seconds/size，
	// TD 用 duration/resolution/ratio，DashScope 用 parameters/input。如果只把
	// 结构体交给插件，结构体里没声明的字段会在到达插件之前被静默丢弃，
	// 上游就会收到一个缺参数的请求。这里改为以客户端原始 JSON 为底，
	// 再把校验/规范化后的字段覆盖回去。
	if merged, ok := mergeTaskRequestBody(c, requestObj); ok {
		c.Set("task_request", merged)
		return
	}
	c.Set("task_request", requestObj)
}

// mergeTaskRequestBody 取客户端原始请求体（JSON 或表单），叠加规范化后的字段。
// 读不到请求体时返回 false，回落到结构体。
func mergeTaskRequestBody(c *gin.Context, req TaskSubmitReq) (map[string]any, bool) {
	contentType := c.Request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		form := c.Request.PostForm
		multipart := c.Request.MultipartForm
		hasFiles := multipart != nil && len(multipart.File) > 0
		if len(form) == 0 && !hasFiles {
			return nil, false
		}
		body := make(map[string]any, len(form)+1)
		for key, values := range form {
			switch len(values) {
			case 0:
			case 1:
				body[key] = values[0]
			default:
				body[key] = values
			}
		}
		// 上传的文件不出现在 PostForm 里，只记录字段名、文件名和大小，
		// 供任务详情展示本次请求附带的素材（内容本身仍由上游接收）。
		if hasFiles {
			files := make([]any, 0, len(multipart.File))
			for field, headers := range multipart.File {
				for _, header := range headers {
					files = append(files, map[string]any{
						"field":        field,
						"filename":     header.Filename,
						"size":         header.Size,
						"content_type": header.Header.Get("Content-Type"),
					})
				}
			}
			body["_uploaded_files"] = files
		}
		overlayTaskFields(body, req)
		return body, true
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, false
	}
	raw, err := storage.Bytes()
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	var body map[string]any
	if err := common.Unmarshal(raw, &body); err != nil || body == nil {
		return nil, false
	}
	overlayTaskFields(body, req)
	return body, true
}

// overlayTaskFields 把校验/规范化后的字段写回原始请求体，覆盖同名字段。
func overlayTaskFields(body map[string]any, req TaskSubmitReq) {
	if req.Model != "" {
		body["model"] = req.Model
	}
	if req.Prompt != "" {
		body["prompt"] = req.Prompt
	}
	if req.Mode != "" {
		body["mode"] = req.Mode
	}
	if req.Size != "" {
		body["size"] = req.Size
	}
	if req.Duration != 0 {
		body["duration"] = req.Duration
	}
	if req.Seconds != "" {
		body["seconds"] = req.Seconds
	}
	if req.InputReference != "" {
		body["input_reference"] = req.InputReference
	}
	if req.Image != "" {
		body["image"] = req.Image
	}
	if len(req.Images) > 0 {
		body["images"] = req.Images
	}
	if req.Metadata != nil {
		body["metadata"] = req.Metadata
	}
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	switch typed := v.(type) {
	case TaskSubmitReq:
		return typed, nil
	case *TaskSubmitReq:
		if typed != nil {
			return *typed, nil
		}
	case map[string]any:
		// storeTaskRequest 存的是叠加了规范化字段的原始请求体。
		encoded, err := common.Marshal(typed)
		if err == nil {
			var req TaskSubmitReq
			if err = common.Unmarshal(encoded, &req); err == nil {
				return req, nil
			}
		}
	}
	return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

// MaxTaskDurationSeconds caps user-supplied video duration. Duration is used
// as a billing multiplier (OtherRatio "seconds"); an unbounded value could
// overflow quota calculation into a negative charge.
const MaxTaskDurationSeconds = 3600

func validateTaskDurationBounds(req TaskSubmitReq) *dto.TaskError {
	seconds := req.Duration
	if seconds == 0 && req.Seconds != "" {
		seconds, _ = strconv.Atoi(req.Seconds)
	}
	// -1 为自动时长（部分模型如 wan3.0-video 支持），与 1–3600 一并放行；
	// 模型是否真的接受 -1 由上游校验。
	if seconds < -1 || seconds > MaxTaskDurationSeconds {
		return createTaskError(fmt.Errorf("seconds must be -1 (auto) or between 1 and %d", MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest, true)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	if _, err := c.MultipartForm(); err != nil {
		return req, err
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:   formData.Get("prompt"),
		Model:    formData.Get("model"),
		Mode:     formData.Get("mode"),
		Image:    formData.Get("image"),
		Size:     formData.Get("size"),
		Metadata: make(map[string]any),
	}

	if durationStr := formData.Get("seconds"); durationStr != "" {
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	} else if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{strings.TrimSpace(req.Image)}
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskDurationBounds(req); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextToVideo
	if hasInputReference {
		action = constant.TaskActionImageToVideo
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":          true,
		"model":           true,
		"mode":            true,
		"image":           true,
		"images":          true,
		"size":            true,
		"duration":        true,
		"input_reference": true, // Sora 特有字段
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	} else if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		// 为了metadata字段的兼容性，统一UnmarshalBodyReusable
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskDurationBounds(req); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	// 调用方不指定动作时按参考素材推导，避免纯文生视频被记成 image_to_video；
	// 多参考素材与参考视频记为 reference_to_video，与插件侧保持同一口径。
	if strings.TrimSpace(action) == "" {
		action = taskActionFromSubmitRequest(req)
	}

	storeTaskRequest(c, info, action, req)
	return nil
}

// taskActionFromSubmitRequest 依据 input.media / metadata.input.media 与图片
// 字段推导任务动作。动作只用于任务日志与对账展示，不影响计费。
func taskActionFromSubmitRequest(req TaskSubmitReq) string {
	hasVideo, images := false, 0
	collect := func(media []TaskSubmitMedia) {
		for _, item := range media {
			switch item.Type {
			case "reference_video":
				hasVideo = true
			case "reference_image":
				images++
			}
		}
	}
	if req.Input != nil {
		collect(req.Input.Media)
	}
	for _, source := range []any{req.Metadata, relayTaskObject(req.Metadata["input"])} {
		object, ok := source.(map[string]any)
		if !ok {
			continue
		}
		entries, ok := object["media"].([]any)
		if !ok {
			continue
		}
		media := make([]TaskSubmitMedia, 0, len(entries))
		for _, entry := range entries {
			if item, ok := entry.(map[string]any); ok {
				media = append(media, TaskSubmitMedia{Type: relayTaskString(item["type"])})
			}
		}
		collect(media)
	}
	switch {
	case hasVideo || images > 1 || req.Mode == "multi_ref":
		return constant.TaskActionReferenceToVideo
	case images == 1 || len(req.Images) > 0 || strings.TrimSpace(req.Image) != "" || req.InputReference != "":
		return constant.TaskActionImageToVideo
	default:
		return constant.TaskActionTextToVideo
	}
}

func relayTaskObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func relayTaskString(value any) string {
	text, _ := value.(string)
	return text
}

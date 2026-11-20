package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// 上游限制 30MiB，这里留一点余量给 multipart 表单开销。
const maxMaterialRequestBytes = 32 << 20
const maxMaterialResponseBytes = 4 << 20

// 共享车素材视频上限 100MB（image 20MB / audio 15MB / file 20MB），留余量给表单。
const maxFleetMediaBytes = 101 << 20

// 批量查询正文上限：上游要求最多 16 KiB。
const maxFleetQueryBytes = 20 << 10

// RelayMaterialUpload 转发 POST /v1/materials 到渠道上游。
//
// 支持 multipart（file）和 JSON（source_url）两种形态，请求体与 Content-Type
// 原样转发（boundary 不改），响应原样返回。上传成功后在 task_materials 里记下
// 资产与渠道的绑定关系，并把上传的图片在本地留一份副本，换渠道时可以重新入库。
func RelayMaterialUpload(c *gin.Context) {
	group := materialRequestGroup(c)
	contentType := c.GetHeader("Content-Type")

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		materialError(c, http.StatusBadRequest, "read request body failed: "+err.Error(), "invalid_request")
		return
	}
	raw, err := storage.Bytes()
	if err != nil {
		materialError(c, http.StatusBadRequest, "read request body failed: "+err.Error(), "invalid_request")
		return
	}
	if len(raw) == 0 {
		materialError(c, http.StatusBadRequest, "request body is empty", "invalid_request")
		return
	}
	if len(raw) > maxMaterialRequestBytes {
		materialError(c, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("material exceeds %d MiB", maxMaterialRequestBytes>>20), "material_too_large")
		return
	}

	parsed, parseErr := parseMaterialRequest(contentType, raw)
	if parseErr != nil {
		materialError(c, http.StatusBadRequest, parseErr.Error(), "invalid_request")
		return
	}
	modelName := strings.TrimSpace(parsed.fields["model"])
	if modelName == "" {
		materialError(c, http.StatusBadRequest, "model is required", "invalid_request")
		return
	}

	channel, channelErr := selectMaterialChannel(group, modelName)
	if channelErr != nil {
		materialError(c, http.StatusBadRequest, channelErr.Error(), "model_not_found")
		return
	}

	status, respBody, respType, forwardErr := forwardMaterial(c, channel, http.MethodPost,
		materialUpstreamURL(channel, "/v1/materials"), "", raw)
	if forwardErr != nil {
		materialError(c, http.StatusBadGateway, forwardErr.Error(), "upstream_request_failed")
		return
	}

	// 只有 2xx 且带 id 才算登记成功；失败响应原样透出去，由调用方判断。
	materialId := ""
	respObject := "material"
	respStatus := ""
	if status >= 200 && status < 300 {
		var payload map[string]any
		if err := common.Unmarshal(respBody, &payload); err == nil {
			materialId, _ = payload["id"].(string)
			if object, ok := payload["object"].(string); ok && object != "" {
				respObject = object
			}
			respStatus, _ = payload["status"].(string)
		}
	}
	if materialId != "" {
		record := &model.TaskMaterial{
			MaterialId:   materialId,
			Object:       respObject,
			Model:        modelName,
			MaterialType: strings.TrimSpace(parsed.fields["material_type"]),
			UserId:       c.GetInt("id"),
			TokenId:      c.GetInt("token_id"),
			ChannelId:    channel.Id,
			BaseUrl:      materialUpstreamBase(channel),
			SourceUrl:    parsed.fields["source_url"],
			Status:       respStatus,
		}
		if len(parsed.fileBytes) > 0 {
			record.LocalPath, record.Size = saveMaterialCopy(parsed.fileBytes, parsed.fileName, contentType)
		}
		if err := model.UpsertTaskMaterial(record); err != nil {
			common.SysError(common.LogText("failed to record material %s: %v", materialId, err))
		}
	}

	if respType == "" {
		respType = "application/json"
	}
	c.Data(status, respType, respBody)
}

// RelayMaterialGet 转发 GET /v1/materials/{id} 到资产上传时绑定的那个渠道。
func RelayMaterialGet(c *gin.Context) {
	materialId := strings.TrimSpace(c.Param("material_id"))
	if materialId == "" {
		materialError(c, http.StatusBadRequest, "material id is required", "invalid_request")
		return
	}
	group := materialRequestGroup(c)

	var channel *model.Channel
	record, lookupErr := model.GetTaskMaterialByMaterialId(materialId)
	if lookupErr != nil {
		common.SysError(common.LogText("failed to load material %s: %v", materialId, lookupErr))
	}
	if record != nil && record.ChannelId > 0 {
		channel, _ = model.GetChannelById(record.ChannelId, true)
	}
	if channel == nil {
		modelName := strings.TrimSpace(c.Query("model"))
		if modelName == "" {
			materialError(c, http.StatusBadRequest,
				"asset is not known to this gateway; pass ?model= so the channel can be resolved", "invalid_request")
			return
		}
		var channelErr error
		channel, channelErr = selectMaterialChannel(group, modelName)
		if channelErr != nil {
			materialError(c, http.StatusBadRequest, channelErr.Error(), "model_not_found")
			return
		}
	}

	query := ""
	if modelName := strings.TrimSpace(c.Query("model")); modelName != "" {
		query = "?model=" + url.QueryEscape(modelName)
	}
	path := "/v1/materials/" + url.PathEscape(materialId) + query

	status, respBody, respType, forwardErr := forwardMaterial(c, channel, http.MethodGet, materialUpstreamURL(channel, path), "", nil)
	if forwardErr != nil {
		materialError(c, http.StatusBadGateway, forwardErr.Error(), "upstream_request_failed")
		return
	}

	if record != nil && status >= 200 && status < 300 {
		var payload map[string]any
		if err := common.Unmarshal(respBody, &payload); err == nil {
			if value, ok := payload["status"].(string); ok && value != "" && value != record.Status {
				record.Status = value
				if code, ok := payload["error_code"].(string); ok {
					record.ErrorCode = code
				}
				if err := model.UpsertTaskMaterial(record); err != nil {
					common.SysError(common.LogText("failed to update material %s: %v", materialId, err))
				}
			}
		}
	}

	if respType == "" {
		respType = "application/json"
	}
	c.Data(status, respType, respBody)
}

type materialRequest struct {
	fields    map[string]string
	fileName  string
	fileBytes []byte
}

// parseMaterialRequest 解析两种上传形态。multipart 只提取字段与 file 内容用于
// 本地留档，转发仍用原始字节，避免重建 boundary 出错。
func parseMaterialRequest(contentType string, raw []byte) (*materialRequest, error) {
	result := &materialRequest{fields: map[string]string{}}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Type: %w", err)
	}
	switch {
	case strings.HasPrefix(mediaType, "multipart/form-data"):
		boundary := params["boundary"]
		if boundary == "" {
			return nil, fmt.Errorf("multipart boundary is required")
		}
		reader := multipart.NewReader(bytes.NewReader(raw), boundary)
		for {
			part, nextErr := reader.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				return nil, fmt.Errorf("invalid multipart body: %w", nextErr)
			}
			name := part.FormName()
			if part.FileName() != "" {
				data, readErr := io.ReadAll(io.LimitReader(part, maxMaterialRequestBytes+1))
				part.Close()
				if readErr != nil {
					return nil, fmt.Errorf("read uploaded file failed: %w", readErr)
				}
				if len(data) > maxMaterialRequestBytes {
					return nil, fmt.Errorf("uploaded file exceeds %d MiB", maxMaterialRequestBytes>>20)
				}
				if name == "file" || result.fileBytes == nil {
					result.fileBytes = data
					result.fileName = part.FileName()
				}
				continue
			}
			value, readErr := io.ReadAll(io.LimitReader(part, 8<<10))
			part.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read multipart field %q failed: %w", name, readErr)
			}
			result.fields[name] = strings.TrimSpace(string(value))
		}
	case strings.Contains(mediaType, "json") || mediaType == "":
		var payload map[string]any
		if err := common.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("invalid JSON body: %w", err)
		}
		for _, key := range []string{"model", "material_type", "source_url"} {
			if value, ok := payload[key].(string); ok {
				result.fields[key] = strings.TrimSpace(value)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported Content-Type %q", mediaType)
	}
	return result, nil
}

func materialRequestGroup(c *gin.Context) string {
	group := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
	if group == "" {
		group = strings.TrimSpace(c.GetString("group"))
	}
	if group == "" {
		group = "default"
	}
	return group
}

// selectMaterialChannel 用与中转一致的渠道选择逻辑（按分组 + 模型可用性）。
func selectMaterialChannel(group, modelName string) (*model.Channel, error) {
	channel, err := model.GetRandomSatisfiedChannel(group, modelName, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("select channel failed: %w", err)
	}
	if channel == nil {
		return nil, fmt.Errorf("model %q has no available channel in group %q", modelName, group)
	}
	if len(channel.GetKeys()) == 0 {
		return nil, fmt.Errorf("channel #%d has no API key", channel.Id)
	}
	if materialUpstreamBase(channel) == "" {
		return nil, fmt.Errorf("channel #%d has no base URL", channel.Id)
	}
	return channel, nil
}

func materialUpstreamBase(channel *model.Channel) string {
	raw := ""
	if channel != nil && channel.BaseURL != nil {
		raw = strings.TrimSpace(*channel.BaseURL)
	}
	if raw == "" {
		return ""
	}
	base := strings.TrimRight(raw, "/")
	base = strings.TrimSuffix(base, "/v1")
	return base
}

func materialUpstreamURL(channel *model.Channel, path string) string {
	return materialUpstreamBase(channel) + path
}

// forwardMaterial 把请求打到渠道上游。请求头只带必要项：客户端带来的
// Authorization/Cookie 一律不透传，避免把网关令牌发给上游。
func forwardMaterial(c *gin.Context, channel *model.Channel, method, target, contentType string, body []byte) (int, []byte, string, error) {
	return forwardRequestBody(c, channel, method, target, contentType, func() (io.Reader, int64, error) {
		if body == nil {
			return nil, 0, nil
		}
		return bytes.NewReader(body), int64(len(body)), nil
	})
}

// forwardMaterialStreaming 用流式 body 转发请求，大文件不需要读进内存。
func forwardMaterialStreaming(c *gin.Context, channel *model.Channel, method, target, contentType string, storage common.BodyStorage) (int, []byte, string, error) {
	return forwardRequestBody(c, channel, method, target, contentType, func() (io.Reader, int64, error) {
		reader, err := storage.NewReader()
		if err != nil {
			return nil, 0, fmt.Errorf("read request body failed: %w", err)
		}
		return reader, storage.Size(), nil
	})
}

func forwardRequestBody(c *gin.Context, channel *model.Channel, method, target, contentType string, newBody func() (io.Reader, int64, error)) (int, []byte, string, error) {
	setting := channel.GetSetting()
	client, err := service.GetHttpClientWithProxySettings(setting.Proxy, setting)
	if err != nil {
		return 0, nil, "", fmt.Errorf("build http client failed: %w", err)
	}
	body, contentLength, err := newBody()
	if err != nil {
		return 0, nil, "", fmt.Errorf("prepare request body failed: %w", err)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), method, target, body)
	if err != nil {
		return 0, nil, "", fmt.Errorf("build upstream request failed: %w", err)
	}
	if contentLength > 0 {
		req.ContentLength = contentLength
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contract := c.GetHeader("X-TD-Contract"); contract != "" {
		req.Header.Set("X-TD-Contract", contract)
	}
	if accept := c.GetHeader("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return 0, nil, "", fmt.Errorf("channel #%d has no API key", channel.Id)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(keys[0]))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, "", fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxMaterialResponseBytes))
	if readErr != nil {
		return 0, nil, "", fmt.Errorf("read upstream response failed: %w", readErr)
	}
	return resp.StatusCode, respBody, resp.Header.Get("Content-Type"), nil
}

// saveMaterialCopy 把上传的图片按内容哈希存一份本地副本，返回路径与字节数。
func saveMaterialCopy(data []byte, fileName, contentType string) (string, int64) {
	dir := materialStorageDir()
	if dir == "" {
		return "", 0
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		common.SysError(common.LogText("failed to create material dir %s: %v", dir, err))
		return "", 0
	}
	sum := sha256.Sum256(data)
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
			switch mediaType {
			case "image/png":
				ext = ".png"
			case "image/jpeg":
				ext = ".jpg"
			case "image/webp":
				ext = ".webp"
			case "image/bmp":
				ext = ".bmp"
			}
		}
	}
	path := filepath.Join(dir, hex.EncodeToString(sum[:])+ext)
	if _, err := os.Stat(path); err == nil {
		return path, int64(len(data))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		common.SysError(common.LogText("failed to write material copy %s: %v", path, err))
		return "", 0
	}
	return path, int64(len(data))
}

// materialStorageDir 可用 MATERIAL_STORAGE_DIR 覆盖；容器内默认 /data/materials。
func materialStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("MATERIAL_STORAGE_DIR")); dir != "" {
		return dir
	}
	if info, err := os.Stat("/data"); err == nil && info.IsDir() {
		return "/data/materials"
	}
	return "./data/materials"
}

// RelayFleetMediaUpload 转发 POST /v1/fleet/media（共享车素材）到渠道上游。
//
// 该接口没有 model 字段，渠道按令牌分组选择；为让素材始终绑定在同一个调用
// 身份上，优先沿用该用户此前上传同名素材时用过的渠道。文件流式转发，同时在
// 本地留一份副本（流式写入，按内容哈希命名；100MB 视频也会落盘）。
func RelayFleetMediaUpload(c *gin.Context) {
	group := materialRequestGroup(c)
	contentType := c.GetHeader("Content-Type")

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		materialError(c, http.StatusBadRequest, "read request body failed: "+err.Error(), "invalid_request")
		return
	}
	if size := storage.Size(); size == 0 {
		materialError(c, http.StatusBadRequest, "request body is empty", "invalid_request")
		return
	} else if size > maxFleetMediaBytes {
		materialError(c, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("media exceeds %d MiB", maxFleetMediaBytes>>20), "media_too_large")
		return
	}

	fields, _, localPath, fileBytes, parseErr := parseFleetForm(storage, contentType)
	if parseErr != nil {
		materialError(c, http.StatusBadRequest, parseErr.Error(), "invalid_request")
		return
	}

	var channel *model.Channel
	var channelErr error
	modelName := strings.TrimSpace(fields["model"])
	if modelName != "" {
		channel, channelErr = selectMaterialChannel(group, modelName)
	} else {
		channel, channelErr = selectFleetChannel(group, strings.TrimSpace(fields["kind"]), strings.TrimSpace(fields["token"]), c.GetInt("id"))
	}
	if channelErr != nil {
		materialError(c, http.StatusBadRequest, channelErr.Error(), "model_not_found")
		return
	}

	status, respBody, respType, forwardErr := forwardMaterialStreaming(c, channel, http.MethodPost,
		materialUpstreamURL(channel, "/v1/fleet/media"), contentType, storage)
	if forwardErr != nil {
		materialError(c, http.StatusBadGateway, forwardErr.Error(), "upstream_request_failed")
		return
	}

	if status >= 200 && status < 300 {
		var payload map[string]any
		if err := common.Unmarshal(respBody, &payload); err == nil {
			if materialId, _ := payload["id"].(string); materialId != "" {
				record := &model.TaskMaterial{
					MaterialId: materialId,
					Object:     "fleet_media",
					Model:      modelName,
					Kind:       stringOrFallback(payload["kind"], fields["kind"]),
					Token:      stringOrFallback(payload["token"], fields["token"]),
					Seconds:    floatOrZero(payload["seconds"]),
					UserId:     c.GetInt("id"),
					TokenId:    c.GetInt("token_id"),
					ChannelId:  channel.Id,
					BaseUrl:    materialUpstreamBase(channel),
					Status:     "ready",
				}
				record.LocalPath = localPath
				if fileBytes > 0 {
					record.Size = fileBytes
				}
				if err := model.UpsertTaskMaterial(record); err != nil {
					common.SysError(common.LogText("failed to record fleet media %s: %v", materialId, err))
				}
			}
		}
	}

	writeMaterialResponse(c, status, respType, respBody)
}

// RelayCatalogModels 转发 GET /v1/catalog/models，让客户端能查到渠道账号
// 实际可用的模型与能力标记（例如 x_td.material_preparation）。
func RelayCatalogModels(c *gin.Context) {
	group := materialRequestGroup(c)
	channel, channelErr := selectGroupChannel(group)
	if channelErr != nil {
		materialError(c, http.StatusBadRequest, channelErr.Error(), "model_not_found")
		return
	}
	status, respBody, respType, forwardErr := forwardMaterial(c, channel, http.MethodGet,
		materialUpstreamURL(channel, "/v1/catalog/models"), "", nil)
	if forwardErr != nil {
		materialError(c, http.StatusBadGateway, forwardErr.Error(), "upstream_request_failed")
		return
	}
	writeMaterialResponse(c, status, respType, respBody)
}

// parseFleetForm 流式解析共享车素材表单：字段收集进 map，文件部分落盘为本地
// 副本。整个 body 不需要读进内存。
func parseFleetForm(storage common.BodyStorage, contentType string) (map[string]string, string, string, int64, error) {
	fields := map[string]string{}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", "", 0, fmt.Errorf("invalid Content-Type: %w", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", "", 0, fmt.Errorf("multipart boundary is required")
	}
	reader, readerErr := storage.NewReader()
	if readerErr != nil {
		return nil, "", "", 0, fmt.Errorf("read request body failed: %w", readerErr)
	}
	multipartReader := multipart.NewReader(reader, boundary)
	var fileName string
	var localPath string
	var fileBytes int64
	for {
		part, nextErr := multipartReader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, "", "", 0, fmt.Errorf("invalid multipart body: %w", nextErr)
		}
		name := part.FormName()
		if part.FileName() != "" {
			path, written := saveMaterialStream(io.LimitReader(part, maxFleetMediaBytes+1), part.FileName())
			part.Close()
			if written > maxFleetMediaBytes {
				return nil, "", "", 0, fmt.Errorf("media exceeds %d MiB", maxFleetMediaBytes>>20)
			}
			if path != "" {
				fileName = part.FileName()
				fileBytes = written
				localPath = path
			}
			continue
		}
		value, readErr := io.ReadAll(io.LimitReader(part, 8<<10))
		part.Close()
		if readErr != nil {
			return nil, "", "", 0, fmt.Errorf("read multipart field %q failed: %w", name, readErr)
		}
		fields[name] = strings.TrimSpace(string(value))
	}
	return fields, fileName, localPath, fileBytes, nil
}

// selectFleetChannel 共享车素材没有 model，按分组选渠道。优先沿用该用户
// 上传同名同类型素材时用过的渠道，保证素材始终落在同一个调用身份下。
func selectFleetChannel(group, kind, token string, userId int) (*model.Channel, error) {
	if previous, err := model.GetLatestTaskMaterialByToken(token, kind, userId); err == nil && previous != nil && previous.ChannelId > 0 {
		if channel, err := model.GetChannelById(previous.ChannelId, true); err == nil &&
			len(channel.GetKeys()) > 0 && materialUpstreamBase(channel) != "" {
			return channel, nil
		}
	}
	return selectGroupChannel(group)
}

// selectGroupChannel 按分组挑第一个可用渠道（有 Key 且有 Base URL）。
// 分组下没有匹配渠道时回退到全部启用渠道：素材接口不绑定模型，
// 令牌分组与渠道分组对不上时也应能用。
func selectGroupChannel(group string) (*model.Channel, error) {
	channels, err := model.GetEnabledChannelsByGroup(group)
	if err != nil {
		return nil, fmt.Errorf("select channel failed: %w", err)
	}
	if len(channels) == 0 {
		common.SysError(common.LogText("material endpoints: no channel in group %q, falling back to all enabled channels", group))
		channels, err = model.GetAllEnabledChannels()
		if err != nil {
			return nil, fmt.Errorf("select channel failed: %w", err)
		}
	}
	var candidates []*model.Channel
	for _, channel := range channels {
		if len(channel.GetKeys()) == 0 || materialUpstreamBase(channel) == "" {
			continue
		}
		candidates = append(candidates, channel)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no usable channel in group %q", group)
	}
	if len(candidates) > 1 {
		// 素材绑定在渠道的调用身份上，多渠道分组里这个选择是模糊的，
		// 显式记录下来便于排查"素材在另一个渠道"类问题。
		common.SysError(common.LogText("material endpoints: group %q has %d usable channels, using channel #%d", group, len(candidates), candidates[0].Id))
	}
	return candidates[0], nil
}

func stringOrFallback(value any, fallback string) string {
	if text, ok := value.(string); ok && text != "" {
		return text
	}
	return fallback
}

func floatOrZero(value any) float64 {
	if number, ok := value.(float64); ok {
		return number
	}
	return 0
}

// saveMaterialStream 把上传的文件流式写入本地，按内容哈希命名。
// 返回的 written 超过调用方限制时由调用方负责报错，文件随后会被清理。
func saveMaterialStream(r io.Reader, fileName string) (string, int64) {
	dir := materialStorageDir()
	if dir == "" {
		return "", 0
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		common.SysError(common.LogText("failed to create material dir %s: %v", dir, err))
		return "", 0
	}
	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		common.SysError(common.LogText("failed to create temp file in %s: %v", dir, err))
		return "", 0
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), r)
	tmp.Close()
	if copyErr != nil {
		os.Remove(tmp.Name())
		common.SysError(common.LogText("failed to write material copy: %v", copyErr))
		return "", 0
	}
	path := filepath.Join(dir, hex.EncodeToString(hash.Sum(nil))+strings.ToLower(filepath.Ext(fileName)))
	if _, err := os.Stat(path); err == nil {
		os.Remove(tmp.Name())
		return path, written
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		common.SysError(common.LogText("failed to persist material copy %s: %v", path, err))
		return "", 0
	}
	return path, written
}

// RelayFleetCatalog 转发 GET /v1/fleet/catalog（共享车车型与报价清单）。
func RelayFleetCatalog(c *gin.Context) {
	group := materialRequestGroup(c)
	channel, channelErr := selectGroupChannel(group)
	if channelErr != nil {
		materialError(c, http.StatusBadRequest, channelErr.Error(), "model_not_found")
		return
	}
	status, respBody, respType, forwardErr := forwardMaterial(c, channel, http.MethodGet,
		materialUpstreamURL(channel, "/v1/fleet/catalog"), "", nil)
	if forwardErr != nil {
		materialError(c, http.StatusBadGateway, forwardErr.Error(), "upstream_request_failed")
		return
	}
	writeMaterialResponse(c, status, respType, respBody)
}

// RelayFleetVideosQuery 转发 POST /v1/fleet/videos/query（批量查询任务状态，
// 只读，不建单不扣费）。正文 JSON 原样透传。
func RelayFleetVideosQuery(c *gin.Context) {
	group := materialRequestGroup(c)
	contentType := c.GetHeader("Content-Type")
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		materialError(c, http.StatusBadRequest, "read request body failed: "+err.Error(), "invalid_request")
		return
	}
	raw, err := storage.Bytes()
	if err != nil || len(raw) == 0 {
		materialError(c, http.StatusBadRequest, "JSON body is required", "invalid_request")
		return
	}
	if len(raw) > maxFleetQueryBytes {
		materialError(c, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("query body exceeds %d KiB", maxFleetQueryBytes>>10), "invalid_request")
		return
	}
	channel, channelErr := selectGroupChannel(group)
	if channelErr != nil {
		materialError(c, http.StatusBadRequest, channelErr.Error(), "model_not_found")
		return
	}
	if contentType == "" {
		contentType = "application/json"
	}
	status, respBody, respType, forwardErr := forwardMaterial(c, channel, http.MethodPost,
		materialUpstreamURL(channel, "/v1/fleet/videos/query"), contentType, raw)
	if forwardErr != nil {
		materialError(c, http.StatusBadGateway, forwardErr.Error(), "upstream_request_failed")
		return
	}
	writeMaterialResponse(c, status, respType, respBody)
}

func writeMaterialResponse(c *gin.Context, status int, respType string, respBody []byte) {
	if respType == "" {
		respType = "application/json"
	}
	c.Data(status, respType, respBody)
}

func materialError(c *gin.Context, status int, message, code string) {
	c.JSON(status, gin.H{"error": gin.H{"message": message, "type": "invalid_request_error", "code": code}})
}

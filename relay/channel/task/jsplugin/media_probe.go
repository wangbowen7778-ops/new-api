package jsplugin

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
)

// 参考素材的官方限制（与文档一致）。
const (
	mediaProbeTimeout   = 20 * time.Second
	mediaProbeHeadBytes = 128 * 1024
	mediaProbeTailBytes = 512 * 1024
)

// probeAndValidateReferenceVideos 尽力探测请求体里的 reference_video（URL 引用）
// 的时长（MP4/MOV 同为 QuickTime/ISO-BMFF 盒结构，均按 moov/mvhd 解析），
// 并把参考视频总时长注入 task_request 的 _reference_seconds 字段——
// 插件的 extractUsage 会把它并入计费秒数（video_total_seconds 口径），
// buildSubmitRequest 构造上游请求体时会剥离下划线开头的字段。
//
// 探测是尽力而为：格式不支持、URL 不可访问等情况一律放行（不拦截提交），
// 素材合法性由上游校验，失败任务由上游退款。探测成功的部分照常计入计费。
func probeAndValidateReferenceVideos(c *gin.Context) *dto.TaskError {
	value, exists := c.Get("task_request")
	if !exists {
		return nil
	}
	body, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	var urls []string
	for _, item := range inputMediaItems(body) {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := entry["type"].(string); t != "reference_video" {
			continue
		}
		if u, _ := entry["url"].(string); strings.TrimSpace(u) != "" {
			urls = append(urls, strings.TrimSpace(u))
		}
	}
	if len(urls) == 0 {
		return nil
	}

	client := &http.Client{Timeout: mediaProbeTimeout}
	total := 0.0
	durations := make([]any, 0, len(urls))
	for _, raw := range urls {
		safe, err := safeMediaURL(raw)
		if err != nil {
			continue
		}
		dur, perr := mp4DurationSeconds(client, safe)
		if perr != nil {
			continue
		}
		if dur > 0 {
			total += dur
			durations = append(durations, map[string]any{
				"url":     strings.TrimSpace(raw),
				"seconds": math.Round(dur*100) / 100,
			})
		}
	}
	if total > 0 {
		body["_reference_seconds"] = math.Round(total*100) / 100
		body["_reference_durations"] = durations
	}
	return nil
}

// inputMediaItems 按插件合并素材的优先顺序取第一处 material 列表：
// metadata.input.media（TD 写法，权威）→ input.media → 顶层 media。
// 兼容 OpenAI 风格（顶层 media）与百炼 TD 写法，避免只认一种导致漏计费。
func inputMediaItems(body map[string]any) []any {
	input, _ := body["input"].(map[string]any)
	metadata, _ := body["metadata"].(map[string]any)
	metadataInput, _ := metadata["input"].(map[string]any)
	for _, candidate := range []map[string]any{metadataInput, input, body} {
		if media, ok := candidate["media"].([]any); ok && len(media) > 0 {
			return media
		}
	}
	return nil
}

// safeMediaURL 校验素材地址：仅 http(s)、无凭据、解析结果必须是公网地址，
// 防止把网关变成探测内网地址的跳板。
func safeMediaURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("media url 无效")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("media url 仅支持 http(s)")
	}
	if u.User != nil {
		return "", fmt.Errorf("media url 不能携带凭据")
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("media url 域名无法解析")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return "", fmt.Errorf("media url 必须指向公网地址")
		}
	}
	return u.String(), nil
}

// mp4DurationSeconds 通过 HTTP Range 读取 MP4 的 moov/mvhd 解析时长（秒）。
// moov 位于文件头部时一次请求即可；位于尾部时先取文件尾定位 moov 的偏移，
// 再按需取整块。分片 MP4（fMP4）没有全局时长，返回错误。
func mp4DurationSeconds(client *http.Client, rawURL string) (float64, error) {
	head, header, err := httpFetch(client, rawURL, "bytes=0-"+strconv.Itoa(mediaProbeHeadBytes-1), mediaProbeHeadBytes)
	if err != nil {
		return 0, err
	}
	start, size, found, complete := findTopLevelBox(head, "moov")
	if found {
		if complete {
			return mvhdSeconds(head[start : start+size])
		}
		// moov 跨越了头部截断边界，按完整范围重新取
		return fetchMoovByOffset(client, rawURL, int64(start), int64(size))
	}
	total := parseTotalSize(header.Get("Content-Range"))
	if total <= 0 {
		return 0, fmt.Errorf("无法确定素材文件大小")
	}
	tailLen := int64(mediaProbeTailBytes)
	if total < tailLen {
		tailLen = total
	}
	tail, _, err := httpFetch(client, rawURL, fmt.Sprintf("bytes=%d-%d", total-tailLen, total-1), mediaProbeTailBytes)
	if err != nil {
		return 0, err
	}
	tStart, tSize, tFound, tComplete := findMoovInTail(tail, total, tailLen)
	if tFound {
		if tComplete {
			return mvhdSeconds(tail[tStart : tStart+tSize])
		}
		// moov 起点可能在尾部切片之前：换算文件绝对偏移后按需取
		fileOffset := total - tailLen + int64(tStart)
		return fetchMoovByOffset(client, rawURL, fileOffset, int64(tSize))
	}
	return 0, fmt.Errorf("未找到 MP4 元数据（moov）")
}

// fetchMoovByOffset 按文件偏移取完整 moov box。
func fetchMoovByOffset(client *http.Client, rawURL string, start, size int64) (float64, error) {
	if size <= 0 || size > 16<<20 {
		return 0, fmt.Errorf("moov 尺寸异常")
	}
	moov, _, err := httpFetch(client, rawURL, fmt.Sprintf("bytes=%d-%d", start, start+size-1), int(size))
	if err != nil {
		return 0, err
	}
	if len(moov) < int(size) {
		return 0, fmt.Errorf("moov 数据不完整")
	}
	return mvhdSeconds(moov)
}

// findTopLevelBox 在数据头部按 box 结构遍历，返回目标 box 的偏移与大小。
// size==0 表示 box 延伸到数据末尾；size==1 表示使用 64 位 largesize。
func findTopLevelBox(data []byte, boxType string) (start, size int, found, complete bool) {
	pos := 0
	for pos+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		typ := string(data[pos+4 : pos+8])
		header := 8
		if size == 1 {
			if pos+16 > len(data) {
				return 0, 0, false, false
			}
			size = int(binary.BigEndian.Uint64(data[pos+8 : pos+16]))
			header = 16
		}
		if size == 0 {
			size = len(data) - pos
		}
		if size < header {
			return 0, 0, false, false
		}
		if typ == boxType {
			return pos, size, true, pos+size <= len(data)
		}
		pos += size
	}
	return 0, 0, false, false
}

// findMoovInTail 在文件尾部切片中定位 moov box。返回的 start/size 是
// 相对切片的偏移与 box 大小；ok 为 false 时表示尾部未包含完整 moov。
func findMoovInTail(tail []byte, total, tailLen int64) (start, size int, found, complete bool) {
	searchFrom := 0
	for searchFrom+4 <= len(tail) {
		idx := bytes.Index(tail[searchFrom:], []byte("moov"))
		if idx < 0 {
			return 0, 0, false, false
		}
		boxStart := searchFrom + idx - 4
		if boxStart < 0 || boxStart+8 > len(tail) {
			searchFrom = searchFrom + idx + 4
			continue
		}
		size := int(binary.BigEndian.Uint32(tail[boxStart : boxStart+4]))
		if size < 8 || int64(boxStart)+int64(size) > total {
			// 不是合法的 moov box 头，继续找下一个匹配位置
			searchFrom = searchFrom + idx + 4
			continue
		}
		return boxStart, size, true, boxStart+size <= len(tail)
	}
	return 0, 0, false, false
}

// mvhdSeconds 在 moov box 中解析 mvhd，返回时长（秒）。
func mvhdSeconds(moov []byte) (float64, error) {
	pos := 8 // 跳过 moov 自身 header
	for pos+8 <= len(moov) {
		size := int(binary.BigEndian.Uint32(moov[pos : pos+4]))
		typ := string(moov[pos+4 : pos+8])
		if size == 1 {
			if pos+16 > len(moov) {
				return 0, fmt.Errorf("moov 子 box 使用 64 位尺寸但数据不足")
			}
			size = int(binary.BigEndian.Uint64(moov[pos+8 : pos+16]))
		}
		if size < 8 {
			return 0, fmt.Errorf("moov 子 box 尺寸异常")
		}
		if typ == "mvhd" {
			p := pos + 8 // mvhd box payload 起点
			if p+1 > len(moov) {
				return 0, fmt.Errorf("mvhd 数据不足")
			}
			version := moov[p]
			if version == 1 {
				if p+32 > len(moov) {
					return 0, fmt.Errorf("mvhd v1 数据不足")
				}
				timescale := binary.BigEndian.Uint32(moov[p+20 : p+24])
				duration := binary.BigEndian.Uint64(moov[p+24 : p+32])
				return secondsFromTimescale(timescale, duration)
			}
			if p+20 > len(moov) {
				return 0, fmt.Errorf("mvhd v0 数据不足")
			}
			timescale := binary.BigEndian.Uint32(moov[p+12 : p+16])
			duration := binary.BigEndian.Uint32(moov[p+16 : p+20])
			return secondsFromTimescale(timescale, uint64(duration))
		}
		pos += size
	}
	return 0, fmt.Errorf("moov 中未找到 mvhd")
}

func secondsFromTimescale(timescale uint32, duration uint64) (float64, error) {
	if timescale == 0 || duration == 0 {
		return 0, fmt.Errorf("mvhd 时长为 0（可能为分片 MP4）")
	}
	return float64(duration) / float64(timescale), nil
}

func httpFetch(client *http.Client, rawURL, rng string, capBytes int) ([]byte, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("构造探测请求失败")
	}
	req.Header.Set("Range", rng)
	req.Header.Set("User-Agent", "new-api-media-probe")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("下载探测数据失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, nil, fmt.Errorf("探测响应状态 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(capBytes)+1))
	if err != nil {
		return nil, nil, fmt.Errorf("读取探测数据失败: %w", err)
	}
	if len(body) > capBytes {
		body = body[:capBytes]
	}
	return body, resp.Header, nil
}

func parseTotalSize(contentRange string) int64 {
	// 形如 "bytes 0-131071/2468075"
	idx := strings.LastIndex(contentRange, "/")
	if idx < 0 {
		return 0
	}
	total, err := strconv.ParseInt(strings.TrimSpace(contentRange[idx+1:]), 10, 64)
	if err != nil {
		return 0
	}
	return total
}

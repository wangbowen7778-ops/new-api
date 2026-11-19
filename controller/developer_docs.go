package controller

import (
	"embed"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed docs/developer-docs.html
var docsFS embed.FS

// DeveloperDocs 站点开发者文档页（公开访问，无需鉴权）。
//
// 默认使用编译时内嵌的 docs/developer-docs.html；若 /data/developer-docs.html
// 存在（可用 DEVELOPER_DOCS_PATH 覆盖路径），则优先使用它，便于不改代码热更新文档。
func DeveloperDocs(c *gin.Context) {
	if data, ok := loadDeveloperDocsOverride(); ok {
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		return
	}
	data, err := docsFS.ReadFile("docs/developer-docs.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "developer docs unavailable")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func loadDeveloperDocsOverride() ([]byte, bool) {
	path := strings.TrimSpace(os.Getenv("DEVELOPER_DOCS_PATH"))
	if path == "" {
		if info, err := os.Stat("/data"); err == nil && info.IsDir() {
			path = "/data/developer-docs.html"
		}
	}
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

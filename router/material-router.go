package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetMaterialRouter 资产库（人物素材）端点：转发到渠道上游并记录资产与渠道的绑定。
// 只做鉴权，不做 Distribute —— 渠道由 controller 内部按 model 选择，
// 因为 GET 请求没有请求体，Distribute 无法从 body 里取 model。
func SetMaterialRouter(router *gin.Engine) {
	materialRouter := router.Group("/v1")
	materialRouter.Use(middleware.RouteTag("relay"))
	materialRouter.Use(middleware.TokenAuth())
	materialRouter.POST("/materials", controller.RelayMaterialUpload)
	materialRouter.GET("/materials/:material_id", controller.RelayMaterialGet)
	materialRouter.POST("/fleet/media", controller.RelayFleetMediaUpload)
	materialRouter.GET("/fleet/catalog", controller.RelayFleetCatalog)
	materialRouter.POST("/fleet/videos/query", controller.RelayFleetVideosQuery)
	materialRouter.GET("/catalog/models", controller.RelayCatalogModels)
}

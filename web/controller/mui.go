package controller

import (
	"github.com/gin-gonic/gin"
)

type muiController struct {
	BaseController

	inboundController *InboundController
	settingController *SettingController
}

func NewmuiController(g *gin.RouterGroup) *muiController {
	a := &muiController{}
	a.initRouter(g)
	return a
}

func (a *muiController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/mui")
	g.Use(a.checkLogin)

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/setting", a.setting)

	a.inboundController = NewInboundController(g)
	a.settingController = NewSettingController(g)
}

func (a *muiController) index(c *gin.Context) {
	html(c, "index.html", "系统状态", nil)
}

func (a *muiController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "入站列表", nil)
}

func (a *muiController) setting(c *gin.Context) {
	html(c, "setting.html", "设置", nil)
}

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/GeekChomolungma/ChomoExecutor/api/handler"
)

func NewRouter(signalHandler *handler.SignalHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/v1")
	{
		v1.POST("/signal", signalHandler.HandleSignal)
	}

	return r
}

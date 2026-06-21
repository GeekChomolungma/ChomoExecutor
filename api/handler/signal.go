package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/GeekChomolungma/ChomoExecutor/model"
	"github.com/GeekChomolungma/ChomoExecutor/service"
)

type SignalHandler struct {
	executor *service.Executor
	authUID  string
}

func NewSignalHandler(executor *service.Executor, authUID string) *SignalHandler {
	return &SignalHandler{executor: executor, authUID: authUID}
}

// HandleSignal receives a trading signal webhook and forwards it to the executor.
//
// POST /signal
func (h *SignalHandler) HandleSignal(c *gin.Context) {
	var sig model.Signal
	if err := c.ShouldBindJSON(&sig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	if sig.UID != h.authUID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized uid"})
		return
	}

	result, err := h.executor.Execute(c.Request.Context(), &sig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orderId":   result.OrderID,
		"symbol":    result.Symbol,
		"status":    result.Status,
		"filledQty": result.FilledQty,
		"avgPrice":  result.AvgPrice,
	})
}

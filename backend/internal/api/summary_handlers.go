package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *ReadHandlers) GetSummary(c *gin.Context) {
	summary, err := h.Store.GetSummary()
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

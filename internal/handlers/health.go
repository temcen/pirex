package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/services"
)

type HealthHandler struct {
	logger        *logrus.Logger
	healthService *services.HealthService
}

func NewHealthHandler(logger *logrus.Logger, healthService *services.HealthService) *HealthHandler {
	return &HealthHandler{
		logger:        logger,
		healthService: healthService,
	}
}

func (h *HealthHandler) Check(c *gin.Context) {
	status := h.healthService.CheckHealth()

	var httpStatus int
	switch status.Status {
	case "healthy":
		httpStatus = http.StatusOK
	case "degraded":
		httpStatus = http.StatusOK // Still operational
	case "unhealthy":
		httpStatus = http.StatusServiceUnavailable
	default:
		httpStatus = http.StatusInternalServerError
	}

	c.JSON(httpStatus, status)
}

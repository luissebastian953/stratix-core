package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/luissebastian953/stratix-core/internal/ai/service"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

type AIHandler struct {
	svc *service.ParseService
}

func NewAIHandler(svc *service.ParseService) *AIHandler {
	return &AIHandler{svc: svc}
}

func (h *AIHandler) Register(g *echo.Group, rateLimiter echo.MiddlewareFunc) {
	g.POST("/parse", h.parse, rateLimiter)
}

type parseRequest struct {
	Input string `json:"input"`
}

func (h *AIHandler) parse(c echo.Context) error {
	var req parseRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	result, err := h.svc.Parse(c.Request().Context(), req.Input, time.Now().UTC())
	if err != nil {
		var appErr *apperror.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return apperror.Internal(err)
	}

	return c.JSON(http.StatusOK, result)
}

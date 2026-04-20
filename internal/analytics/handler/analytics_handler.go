package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/luissebastian953/stratix-core/internal/analytics/service"
	"github.com/luissebastian953/stratix-core/internal/middleware"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

type AnalyticsHandler struct {
	svc *service.AnalyticsService
}

func NewAnalyticsHandler(svc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

func (h *AnalyticsHandler) Register(g *echo.Group) {
	g.GET("/summary", h.summary)
	g.GET("/distribution", h.distribution)
	g.GET("/activity", h.activity)
}

func (h *AnalyticsHandler) summary(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	summary, err := h.svc.Summary(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, summary)
}

func (h *AnalyticsHandler) distribution(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	counts, err := h.svc.TypeDistribution(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, counts)
}

func (h *AnalyticsHandler) activity(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	days := 30
	if raw := c.QueryParam("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			days = n
		}
	}

	counts, err := h.svc.CompletedPerDay(c.Request().Context(), userID, days)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, counts)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func userIDFromCtx(c echo.Context) (uuid.UUID, error) {
	raw, ok := c.Get(middleware.UserIDKey).(string)
	if !ok || raw == "" {
		return uuid.Nil, apperror.Unauthorized("missing user identity")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperror.Unauthorized("invalid user identity")
	}
	return id, nil
}

func mapError(err error) *apperror.AppError {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperror.Internal(err)
}

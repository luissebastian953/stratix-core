package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/luissebastian953/stratix-core/internal/middleware"
	"github.com/luissebastian953/stratix-core/internal/notifications/service"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) Register(g *echo.Group) {
	g.POST("/devices", h.registerDevice)
	g.DELETE("/devices/:device_id", h.unregisterDevice)
	g.GET("", h.listNotifications)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *NotificationHandler) registerDevice(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	var req RegisterDeviceRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	if err := h.svc.RegisterDevice(c.Request().Context(), userID, req.DeviceID, req.Token, req.Platform); err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "device registered"})
}

func (h *NotificationHandler) unregisterDevice(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	deviceID := c.Param("device_id")
	if deviceID == "" {
		return apperror.BadRequest("device_id is required")
	}

	if err := h.svc.UnregisterDevice(c.Request().Context(), userID, deviceID); err != nil {
		return mapError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *NotificationHandler) listNotifications(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	limit := 50
	if raw := c.QueryParam("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}

	ns, err := h.svc.ListNotifications(c.Request().Context(), userID, limit)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, ns)
}

// ── Request types ─────────────────────────────────────────────────────────────

type RegisterDeviceRequest struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (r *RegisterDeviceRequest) Validate() error {
	if r.DeviceID == "" {
		return errors.New("device_id is required")
	}
	if r.Token == "" {
		return errors.New("token is required")
	}
	if r.Platform != "ios" && r.Platform != "android" {
		return errors.New("platform must be ios or android")
	}
	return nil
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

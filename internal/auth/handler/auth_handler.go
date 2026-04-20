package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/luissebastian953/stratix-core/internal/auth/service"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(g *echo.Group) {
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
}

func (h *AuthHandler) register(c echo.Context) error {
	var req RegisterRequest

	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	pair, err := h.svc.Register(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusCreated, toTokenResponse(pair))
}

func (h *AuthHandler) login(c echo.Context) error {
	var req LoginRequest

	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	pair, err := h.svc.Login(c.Request().Context(), req.Email, req.Password)

	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTokenResponse(pair))
}

func (h *AuthHandler) refresh(c echo.Context) error {
	var req RefreshRequest

	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	pair, err := h.svc.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTokenResponse(pair))
}

func (h *AuthHandler) logout(c echo.Context) error {
	var req LogoutRequest

	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	if err := h.svc.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, MessageResponse{Message: "logged out successfully"})
}

func toTokenResponse(pair *service.TokenPair) TokenResponse {
	return TokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    "Bearer",
	}
}

func mapError(err error) *apperror.AppError {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return apperror.Internal(err)
}

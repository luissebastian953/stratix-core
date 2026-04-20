package handler

import (
	"errors"
	"strings"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *RegisterRequest) Validate() error {
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))

	if r.Email == "" {
		return errors.New("email is required")
	}

	if r.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))

	if r.Email == "" {
		return errors.New("email is required")
	}

	if r.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (r *RefreshRequest) Validate() error {
	r.RefreshToken = strings.TrimSpace(r.RefreshToken)

	if r.RefreshToken == "" {
		return errors.New("refresh_token is required")
	}

	return nil
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (r *LogoutRequest) Validate() error {
	r.RefreshToken = strings.TrimSpace(r.RefreshToken)

	if r.RefreshToken == "" {
		return errors.New("refresh_token is required")
	}

	return nil
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

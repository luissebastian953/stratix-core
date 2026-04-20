package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

type AuthService struct {
	users  domain.UserRepository
	tokens domain.RefreshTokenRepository
	config AuthConfig
}

type AuthConfig struct {
	JWTSecret        string
	AccessExpMinutes int
	RefreshExpDays   int
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func NewAuthService(users domain.UserRepository, tokens domain.RefreshTokenRepository, config AuthConfig) *AuthService {
	return &AuthService{users: users, tokens: tokens, config: config}
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*TokenPair, error) {
	if err := validateEmail(email); err != nil {
		return nil, apperror.BadRequest(err.Error())
	}
	if err := validatePassword(password); err != nil {
		return nil, apperror.BadRequest(err.Error())
	}

	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return nil, apperror.Conflict("email is already registered")
	}
	if !domain.IsNotFound(err) {
		return nil, apperror.Internal(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		if domain.IsConflict(err) {
			return nil, apperror.Conflict("email is already registered")
		}
		return nil, apperror.Internal(err)
	}

	return s.issueAndStorePair(ctx, user.ID)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.Unauthorized("invalid email or password")
		}
		return nil, apperror.Internal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperror.Unauthorized("invalid email or password")
	}

	return s.issueAndStorePair(ctx, user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil {
		return nil, apperror.Unauthorized("invalid or expired refresh token")
	}

	if _, err := s.tokens.Get(ctx, refreshToken); err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.Unauthorized("refresh token has been revoked or expired")
		}
		return nil, apperror.Internal(err)
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return nil, apperror.Unauthorized("invalid token claims")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return nil, apperror.Unauthorized("invalid token subject")
	}

	if _, err := s.users.GetByID(ctx, userID); err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.Unauthorized("user account no longer exists")
		}
		return nil, apperror.Internal(err)
	}

	if err := s.tokens.Revoke(ctx, refreshToken); err != nil {
		return nil, apperror.Internal(err)
	}

	return s.issueAndStorePair(ctx, userID)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return apperror.BadRequest("refresh token is required")
	}
	if err := s.tokens.Revoke(ctx, refreshToken); err != nil {
		if domain.IsNotFound(err) {
			return nil
		}
		return apperror.Internal(err)
	}
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.tokens.RevokeAllForUser(ctx, userID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *AuthService) issueAndStorePair(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	accessExp := time.Duration(s.config.AccessExpMinutes) * time.Minute
	refreshExp := time.Duration(s.config.RefreshExpDays) * 24 * time.Hour

	accessToken, err := s.signToken(userID.String(), "access", accessExp)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	refreshToken, err := s.signToken(userID.String(), "refresh", refreshExp)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	expiresAt := time.Now().UTC().Add(refreshExp)
	if err := s.tokens.Store(ctx, refreshToken, userID, expiresAt); err != nil {
		return nil, apperror.Internal(err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessExp.Seconds()),
	}, nil
}

func (s *AuthService) signToken(subject, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":  subject,
		"exp":  now.Add(expiry).Unix(),
		"iat":  now.Unix(),
		"jti":  uuid.New().String(),
		"type": tokenType,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *AuthService) parseToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}
	return claims, nil
}

func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > 254 {
		return errors.New("email must be 254 characters or fewer")
	}
	atIdx := -1
	for i, c := range email {
		if c == '@' {
			atIdx = i
		}
	}
	if atIdx < 1 || atIdx >= len(email)-2 {
		return errors.New("email must be a valid email address")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(password) > 72 {
		return errors.New("password must be 72 characters or fewer")
	}
	return nil
}

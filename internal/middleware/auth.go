package middleware

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

const UserIDKey = "userID"

func JWT(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr, err := extractBearerToken(c)
			if err != nil {
				return apperror.Unauthorized(err.Error())
			}

			claims, err := parseAndValidate(tokenStr, secret)
			if err != nil {
				return apperror.Unauthorized("invalid or expired token")
			}

			if tokenType, _ := claims["type"].(string); tokenType != "access" {
				return apperror.Unauthorized("access token required")
			}

			sub, err := claims.GetSubject()
			if err != nil || strings.TrimSpace(sub) == "" {
				return apperror.Unauthorized("invalid token subject")
			}

			c.Set(UserIDKey, sub)

			return next(c)
		}
	}
}

// extractBearerToken reads and validates the Authorization header format.
func extractBearerToken(c echo.Context) (string, error) {
	header := c.Request().Header.Get("Authorization")

	if header == "" {
		return "", errors.New("authorization header is required")
	}

	if !strings.HasPrefix(header, "Bearer ") {
		return "", errors.New("authorization header must use Bearer scheme")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))

	if token == "" {
		return "", errors.New("token is empty")
	}

	return token, nil
}

func parseAndValidate(tokenStr, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}

		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok {
		return nil, errors.New("invalid claims")
	}

	return claims, nil
}

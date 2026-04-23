package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luissebastian953/stratix-core/internal/domain"
)

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepository(db *pgxpool.Pool) domain.RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Store(ctx context.Context, token string, userID uuid.UUID, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, expiresAt)
	return err
}

func (r *RefreshTokenRepository) Get(ctx context.Context, token string) (*domain.RefreshToken, error) {
	row := r.db.QueryRow(ctx, `
		SELECT token, user_id, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token      = $1
		  AND revoked    = FALSE
		  AND expires_at > NOW()
		LIMIT 1
	`, token)

	var rt domain.RefreshToken
	if err := row.Scan(&rt.Token, &rt.UserID, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.NotFoundError{Resource: "refresh_token", ID: token}
		}
		return nil, err
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked = TRUE WHERE token = $1
	`, token)
	return err
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked = TRUE
		WHERE user_id = $1 AND revoked = FALSE
	`, userID)
	return err
}

func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM refresh_tokens WHERE expires_at < NOW()
	`)
	return err
}

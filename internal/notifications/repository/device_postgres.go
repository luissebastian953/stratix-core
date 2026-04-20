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

type DeviceRepository struct {
	db *pgxpool.Pool
}

func NewDeviceRepository(db *pgxpool.Pool) domain.DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Upsert(ctx context.Context, device *domain.Device) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO devices (device_id, user_id, token, platform, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (device_id) DO UPDATE
		SET token      = EXCLUDED.token,
		    user_id    = EXCLUDED.user_id,
		    updated_at = EXCLUDED.updated_at
	`, device.DeviceID, device.UserID, device.Token, device.Platform, time.Now().UTC())
	return err
}

func (r *DeviceRepository) GetByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	rows, err := r.db.Query(ctx, `
		SELECT device_id, user_id, token, platform, updated_at
		FROM devices
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*domain.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []*domain.Device{}
	}
	return devices, rows.Err()
}

func (r *DeviceRepository) Delete(ctx context.Context, deviceID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM devices WHERE device_id = $1`, deviceID)
	return err
}

func (r *DeviceRepository) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM devices WHERE user_id = $1`, userID)
	return err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDevice(row scannable) (*domain.Device, error) {
	var d domain.Device
	err := row.Scan(&d.DeviceID, &d.UserID, &d.Token, &d.Platform, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.NotFoundError{Resource: "device", ID: ""}
		}
		return nil, err
	}
	return &d, nil
}

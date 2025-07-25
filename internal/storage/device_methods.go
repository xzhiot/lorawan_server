package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// ========== Device Methods ==========

// CreateDevice creates a new device
func (s *PostgresStore) CreateDevice(ctx context.Context, device *models.Device) error {
	if device.ID == uuid.Nil {
		device.ID = uuid.New()
	}

	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now

	query := `
        INSERT INTO devices (
            dev_eui, created_at, updated_at, tenant_id, join_eui, dev_addr,
            name, description, application_id, device_profile_id, is_disabled,
            app_s_key, nwk_s_enc_key, s_nwk_s_int_key, f_nwk_s_int_key,
            f_cnt_up, n_f_cnt_down, a_f_cnt_down, dr
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
        )`

	_, err := s.getDB().ExecContext(ctx, query,
		device.DevEUI[:], device.CreatedAt, device.UpdatedAt, device.TenantID,
		device.JoinEUI, device.DevAddr, device.Name, device.Description,
		device.ApplicationID, device.DeviceProfileID, device.IsDisabled,
		device.AppSKey, device.NwkSEncKey, device.SNwkSIntKey, device.FNwkSIntKey,
		device.FCntUp, device.NFCntDown, device.AFCntDown, device.DR,
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrDuplicateKey
		}
		return err
	}

	return nil
}

// GetDevice gets a device by DevEUI
func (s *PostgresStore) GetDevice(ctx context.Context, devEUI lorawan.EUI64) (*models.Device, error) {
	query := `
        SELECT dev_eui, created_at, updated_at, tenant_id, join_eui, dev_addr,
               name, description, application_id, device_profile_id, is_disabled,
               last_seen_at, battery_level, battery_level_updated_at,
               app_s_key, nwk_s_enc_key, s_nwk_s_int_key, f_nwk_s_int_key,
               f_cnt_up, n_f_cnt_down, a_f_cnt_down, dr
        FROM devices
        WHERE dev_eui = $1`

	device := &models.Device{}
	var devEUIBytes, joinEUIBytes, devAddrBytes []byte

	err := s.getDB().QueryRowContext(ctx, query, devEUI[:]).Scan(
		&devEUIBytes, &device.CreatedAt, &device.UpdatedAt, &device.TenantID,
		&joinEUIBytes, &devAddrBytes, &device.Name, &device.Description,
		&device.ApplicationID, &device.DeviceProfileID, &device.IsDisabled,
		&device.LastSeenAt, &device.BatteryLevel, &device.BatteryLevelUpdatedAt,
		&device.AppSKey, &device.NwkSEncKey, &device.SNwkSIntKey, &device.FNwkSIntKey,
		&device.FCntUp, &device.NFCntDown, &device.AFCntDown, &device.DR,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	// Convert byte arrays
	copy(device.DevEUI[:], devEUIBytes)
	if joinEUIBytes != nil {
		device.JoinEUI = &models.EUI64{}
		copy((*device.JoinEUI)[:], joinEUIBytes)
	}
	if devAddrBytes != nil {
		device.DevAddr = &models.DevAddr{}
		copy((*device.DevAddr)[:], devAddrBytes)
	}

	return device, nil
}

// GetDeviceByDevAddr gets devices by DevAddr
func (s *PostgresStore) GetDeviceByDevAddr(ctx context.Context, devAddr lorawan.DevAddr) ([]*models.Device, error) {
	query := `
        SELECT dev_eui, created_at, updated_at, tenant_id, join_eui, dev_addr,
               name, description, application_id, device_profile_id, is_disabled,
               f_cnt_up, n_f_cnt_down, a_f_cnt_down
        FROM devices
        WHERE dev_addr = $1 AND is_disabled = false`

	rows, err := s.getDB().QueryContext(ctx, query, devAddr[:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		var devEUIBytes, joinEUIBytes, devAddrBytes []byte

		err := rows.Scan(
			&devEUIBytes, &device.CreatedAt, &device.UpdatedAt, &device.TenantID,
			&joinEUIBytes, &devAddrBytes, &device.Name, &device.Description,
			&device.ApplicationID, &device.DeviceProfileID, &device.IsDisabled,
			&device.FCntUp, &device.NFCntDown, &device.AFCntDown,
		)
		if err != nil {
			return nil, err
		}

		// Convert byte arrays
		copy(device.DevEUI[:], devEUIBytes)
		if joinEUIBytes != nil {
			device.JoinEUI = &models.EUI64{}
			copy((*device.JoinEUI)[:], joinEUIBytes)
		}
		if devAddrBytes != nil {
			device.DevAddr = &models.DevAddr{}
			copy((*device.DevAddr)[:], devAddrBytes)
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// UpdateDevice updates a device
func (s *PostgresStore) UpdateDevice(ctx context.Context, device *models.Device) error {
	device.UpdatedAt = time.Now()
	var devAddrBytes []byte
	if device.DevAddr != nil {
		devAddrBytes = (*device.DevAddr)[:] // 转换为字节数组
	}
	query := `
        UPDATE devices SET
            updated_at = $2, name = $3, description = $4, is_disabled = $5,
            last_seen_at = $6, battery_level = $7, battery_level_updated_at = $8,
            f_cnt_up = $9, n_f_cnt_down = $10, a_f_cnt_down = $11, dr = $12,
            app_s_key = $13, nwk_s_enc_key = $14, s_nwk_s_int_key = $15, f_nwk_s_int_key = $16,
            dev_addr = $17
        WHERE dev_eui = $1`

	result, err := s.getDB().ExecContext(ctx, query,
		device.DevEUI[:], device.UpdatedAt, device.Name, device.Description,
		device.IsDisabled, device.LastSeenAt, device.BatteryLevel,
		device.BatteryLevelUpdatedAt, device.FCntUp, device.NFCntDown,
		device.AFCntDown, device.DR, device.AppSKey, device.NwkSEncKey,
		device.SNwkSIntKey, device.FNwkSIntKey, devAddrBytes,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteDevice deletes a device
func (s *PostgresStore) DeleteDevice(ctx context.Context, devEUI lorawan.EUI64) error {
	result, err := s.getDB().ExecContext(ctx, "DELETE FROM devices WHERE dev_eui = $1", devEUI[:])
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// ListDevices lists devices
func (s *PostgresStore) ListDevices(ctx context.Context, applicationID uuid.UUID, limit, offset int) ([]*models.Device, int64, error) {
	// Get count
	var count int64
	err := s.getDB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM devices WHERE application_id = $1", applicationID,
	).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	// Get rows
	query := `
        SELECT dev_eui, created_at, updated_at, tenant_id, join_eui, dev_addr,
               name, description, application_id, device_profile_id, is_disabled,
               last_seen_at, battery_level, f_cnt_up
        FROM devices
        WHERE application_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`

	rows, err := s.getDB().QueryContext(ctx, query, applicationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		var devEUIBytes, joinEUIBytes, devAddrBytes []byte

		err := rows.Scan(
			&devEUIBytes, &device.CreatedAt, &device.UpdatedAt, &device.TenantID,
			&joinEUIBytes, &devAddrBytes, &device.Name, &device.Description,
			&device.ApplicationID, &device.DeviceProfileID, &device.IsDisabled,
			&device.LastSeenAt, &device.BatteryLevel, &device.FCntUp,
		)
		if err != nil {
			return nil, 0, err
		}

		// Convert byte arrays
		copy(device.DevEUI[:], devEUIBytes)
		if joinEUIBytes != nil {
			device.JoinEUI = &models.EUI64{}
			copy((*device.JoinEUI)[:], joinEUIBytes)
		}
		if devAddrBytes != nil {
			device.DevAddr = &models.DevAddr{}
			copy((*device.DevAddr)[:], devAddrBytes)
		}

		devices = append(devices, device)
	}

	return devices, count, nil
}

// ========== Device Keys Methods ==========

// SetDeviceKeys sets device keys
func (s *PostgresStore) SetDeviceKeys(ctx context.Context, keys *models.DeviceKeys) error {
	keys.UpdatedAt = time.Now()

	query := `
        INSERT INTO device_keys (dev_eui, app_key, nwk_key, updated_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (dev_eui) DO UPDATE SET
            app_key = EXCLUDED.app_key,
            nwk_key = EXCLUDED.nwk_key,
            updated_at = EXCLUDED.updated_at`

	_, err := s.getDB().ExecContext(ctx, query,
		keys.DevEUI[:], keys.AppKey, keys.NwkKey, keys.UpdatedAt,
	)

	return err
}

// GetDeviceKeys gets device keys
func (s *PostgresStore) GetDeviceKeys(ctx context.Context, devEUI lorawan.EUI64) (*models.DeviceKeys, error) {
	query := `
        SELECT dev_eui, app_key, nwk_key, updated_at
        FROM device_keys
        WHERE dev_eui = $1`

	keys := &models.DeviceKeys{}
	var devEUIBytes []byte

	err := s.getDB().QueryRowContext(ctx, query, devEUI[:]).Scan(
		&devEUIBytes, &keys.AppKey, &keys.NwkKey, &keys.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	copy(keys.DevEUI[:], devEUIBytes)

	return keys, nil
}

// DeleteDeviceKeys deletes device keys
func (s *PostgresStore) DeleteDeviceKeys(ctx context.Context, devEUI lorawan.EUI64) error {
	result, err := s.getDB().ExecContext(ctx, "DELETE FROM device_keys WHERE dev_eui = $1", devEUI[:])
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

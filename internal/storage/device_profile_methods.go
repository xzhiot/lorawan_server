package storage

import (
    "context"
    "database/sql"
    "strings"
    "time"
    
    "github.com/google/uuid"
    "github.com/lorawan-server/lorawan-server-pro/internal/models"
)

// ========== Device Profile Methods ==========

// CreateDeviceProfile creates a new device profile
func (s *PostgresStore) CreateDeviceProfile(ctx context.Context, profile *models.DeviceProfile) error {
    if profile.ID == uuid.Nil {
        profile.ID = uuid.New()
    }
    
    now := time.Now()
    profile.CreatedAt = now
    profile.UpdatedAt = now
    
    query := `
        INSERT INTO device_profiles (
            id, created_at, updated_at, tenant_id, name, description,
            mac_version, reg_params_revision, max_eirp, max_duty_cycle,
            rf_region, supports_join, supports_32_bit_f_cnt,
            supports_class_b, class_b_timeout, ping_slot_period,
            ping_slot_dr, ping_slot_freq, supports_class_c,
            class_c_timeout, uplink_interval
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
            $14, $15, $16, $17, $18, $19, $20, $21
        )`
    
    _, err := s.getDB().ExecContext(ctx, query,
        profile.ID, profile.CreatedAt, profile.UpdatedAt, profile.TenantID,
        profile.Name, profile.Description, profile.MACVersion,
        profile.RegParamsRevision, profile.MaxEIRP, profile.MaxDutyCycle,
        profile.RFRegion, profile.SupportsJoin, profile.Supports32BitFCnt,
        profile.SupportsClassB, profile.ClassBTimeout, profile.PingSlotPeriod,
        profile.PingSlotDR, profile.PingSlotFreq, profile.SupportsClassC,
        profile.ClassCTimeout, profile.UplinkInterval,
    )
    
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return ErrDuplicateKey
        }
        return err
    }
    
    return nil
}

// GetDeviceProfile gets a device profile by ID
func (s *PostgresStore) GetDeviceProfile(ctx context.Context, id uuid.UUID) (*models.DeviceProfile, error) {
    query := `
        SELECT id, created_at, updated_at, tenant_id, name, description,
               mac_version, reg_params_revision, max_eirp, max_duty_cycle,
               rf_region, supports_join, supports_32_bit_f_cnt
        FROM device_profiles
        WHERE id = $1`
    
    profile := &models.DeviceProfile{}
    err := s.getDB().QueryRowContext(ctx, query, id).Scan(
        &profile.ID, &profile.CreatedAt, &profile.UpdatedAt, &profile.TenantID,
        &profile.Name, &profile.Description, &profile.MACVersion,
        &profile.RegParamsRevision, &profile.MaxEIRP, &profile.MaxDutyCycle,
        &profile.RFRegion, &profile.SupportsJoin, &profile.Supports32BitFCnt,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    return profile, err
}

// UpdateDeviceProfile updates a device profile
func (s *PostgresStore) UpdateDeviceProfile(ctx context.Context, profile *models.DeviceProfile) error {
    profile.UpdatedAt = time.Now()
    
    query := `
        UPDATE device_profiles SET
            updated_at = $2, name = $3, description = $4
        WHERE id = $1`
    
    result, err := s.getDB().ExecContext(ctx, query,
        profile.ID, profile.UpdatedAt, profile.Name, profile.Description,
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

// DeleteDeviceProfile deletes a device profile
func (s *PostgresStore) DeleteDeviceProfile(ctx context.Context, id uuid.UUID) error {
    result, err := s.getDB().ExecContext(ctx, "DELETE FROM device_profiles WHERE id = $1", id)
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

// ListDeviceProfiles lists device profiles
func (s *PostgresStore) ListDeviceProfiles(ctx context.Context, tenantID *uuid.UUID, limit, offset int) ([]*models.DeviceProfile, int64, error) {
    var args []interface{}
    countQuery := "SELECT COUNT(*) FROM device_profiles"
    query := `SELECT id, created_at, updated_at, tenant_id, name, description,
                     rf_region, supports_join
              FROM device_profiles`
    
    if tenantID != nil {
        countQuery += " WHERE tenant_id = $1"
        query += " WHERE tenant_id = $1"
        args = append(args, *tenantID)
    }
    
    // Get count
    var count int64
    err := s.getDB().QueryRowContext(ctx, countQuery, args...).Scan(&count)
    if err != nil {
        return nil, 0, err
    }
    
    // Get rows
    query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
    args = append(args, limit, offset)
    
    rows, err := s.getDB().QueryContext(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var profiles []*models.DeviceProfile
    for rows.Next() {
        profile := &models.DeviceProfile{}
        err := rows.Scan(
            &profile.ID, &profile.CreatedAt, &profile.UpdatedAt, &profile.TenantID,
            &profile.Name, &profile.Description, &profile.RFRegion, &profile.SupportsJoin,
        )
        if err != nil {
            return nil, 0, err
        }
        profiles = append(profiles, profile)
    }
    
    return profiles, count, nil
}

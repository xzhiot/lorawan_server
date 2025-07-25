package storage

import (
    "context"
    "database/sql"
    "time"
    
    "github.com/lorawan-server/lorawan-server-pro/internal/models"
    "github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// ========== Device Session Methods ==========

// GetDeviceSession gets a device session
func (s *PostgresStore) GetDeviceSession(ctx context.Context, devEUI lorawan.EUI64) (*models.DeviceSession, error) {
    query := `
        SELECT dev_eui, dev_addr, join_eui, app_s_key, f_nwk_s_int_key,
               s_nwk_s_int_key, nwk_s_enc_key, f_cnt_up, n_f_cnt_down,
               a_f_cnt_down, conf_f_cnt, rx1_delay, rx1_dr_offset,
               rx2_dr, rx2_freq, tx_power, dr, adr,
               last_dev_status_request, created_at, updated_at
        FROM device_sessions
        WHERE dev_eui = $1`
    
    session := &models.DeviceSession{}
    var devEUIBytes, devAddrBytes, joinEUIBytes []byte
    
    err := s.getDB().QueryRowContext(ctx, query, devEUI[:]).Scan(
        &devEUIBytes, &devAddrBytes, &joinEUIBytes,
        &session.AppSKey, &session.FNwkSIntKey, &session.SNwkSIntKey,
        &session.NwkSEncKey, &session.FCntUp, &session.NFCntDown,
        &session.AFCntDown, &session.ConfFCnt, &session.RX1Delay,
        &session.RX1DROffset, &session.RX2DR, &session.RX2Freq,
        &session.TXPower, &session.DR, &session.ADR,
        &session.LastDevStatusRequest, &session.CreatedAt, &session.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    if err != nil {
        return nil, err
    }
    
    copy(session.DevEUI[:], devEUIBytes)
    copy(session.DevAddr[:], devAddrBytes)
    copy(session.JoinEUI[:], joinEUIBytes)
    
    return session, nil
}

// SaveDeviceSession saves a device session
func (s *PostgresStore) SaveDeviceSession(ctx context.Context, session *models.DeviceSession) error {
    session.UpdatedAt = time.Now()
    
    query := `
        INSERT INTO device_sessions (
            dev_eui, dev_addr, join_eui, app_s_key, f_nwk_s_int_key,
            s_nwk_s_int_key, nwk_s_enc_key, f_cnt_up, n_f_cnt_down,
            a_f_cnt_down, conf_f_cnt, rx1_delay, rx1_dr_offset,
            rx2_dr, rx2_freq, tx_power, dr, adr,
            last_dev_status_request, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
            $13, $14, $15, $16, $17, $18, $19, $20, $21
        )
        ON CONFLICT (dev_eui) DO UPDATE SET
            dev_addr = EXCLUDED.dev_addr,
            join_eui = EXCLUDED.join_eui,
            app_s_key = EXCLUDED.app_s_key,
            f_nwk_s_int_key = EXCLUDED.f_nwk_s_int_key,
            s_nwk_s_int_key = EXCLUDED.s_nwk_s_int_key,
            nwk_s_enc_key = EXCLUDED.nwk_s_enc_key,
            f_cnt_up = EXCLUDED.f_cnt_up,
            n_f_cnt_down = EXCLUDED.n_f_cnt_down,
            a_f_cnt_down = EXCLUDED.a_f_cnt_down,
            conf_f_cnt = EXCLUDED.conf_f_cnt,
            rx1_delay = EXCLUDED.rx1_delay,
            rx1_dr_offset = EXCLUDED.rx1_dr_offset,
            rx2_dr = EXCLUDED.rx2_dr,
            rx2_freq = EXCLUDED.rx2_freq,
            tx_power = EXCLUDED.tx_power,
            dr = EXCLUDED.dr,
            adr = EXCLUDED.adr,
            last_dev_status_request = EXCLUDED.last_dev_status_request,
            updated_at = EXCLUDED.updated_at`
    
    _, err := s.getDB().ExecContext(ctx, query,
        session.DevEUI[:], session.DevAddr[:], session.JoinEUI[:],
        session.AppSKey, session.FNwkSIntKey, session.SNwkSIntKey,
        session.NwkSEncKey, session.FCntUp, session.NFCntDown,
        session.AFCntDown, session.ConfFCnt, session.RX1Delay,
        session.RX1DROffset, session.RX2DR, session.RX2Freq,
        session.TXPower, session.DR, session.ADR,
        session.LastDevStatusRequest, session.CreatedAt, session.UpdatedAt,
    )
    
    return err
}

// DeleteDeviceSession deletes a device session
func (s *PostgresStore) DeleteDeviceSession(ctx context.Context, devEUI lorawan.EUI64) error {
    result, err := s.getDB().ExecContext(ctx, "DELETE FROM device_sessions WHERE dev_eui = $1", devEUI[:])
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

// GetDeviceSessionByDevAddr gets device sessions by DevAddr
func (s *PostgresStore) GetDeviceSessionByDevAddr(ctx context.Context, devAddr lorawan.DevAddr) ([]*models.DeviceSession, error) {
    query := `
        SELECT dev_eui, dev_addr, join_eui, app_s_key, f_nwk_s_int_key,
               s_nwk_s_int_key, nwk_s_enc_key, f_cnt_up, n_f_cnt_down,
               a_f_cnt_down, conf_f_cnt
        FROM device_sessions
        WHERE dev_addr = $1`
    
    rows, err := s.getDB().QueryContext(ctx, query, devAddr[:])
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var sessions []*models.DeviceSession
    for rows.Next() {
        session := &models.DeviceSession{}
        var devEUIBytes, devAddrBytes, joinEUIBytes []byte
        
        err := rows.Scan(
            &devEUIBytes, &devAddrBytes, &joinEUIBytes,
            &session.AppSKey, &session.FNwkSIntKey, &session.SNwkSIntKey,
            &session.NwkSEncKey, &session.FCntUp, &session.NFCntDown,
            &session.AFCntDown, &session.ConfFCnt,
        )
        if err != nil {
            return nil, err
        }
        
        copy(session.DevEUI[:], devEUIBytes)
        copy(session.DevAddr[:], devAddrBytes)
        copy(session.JoinEUI[:], joinEUIBytes)
        
        sessions = append(sessions, session)
    }
    
    return sessions, nil
}

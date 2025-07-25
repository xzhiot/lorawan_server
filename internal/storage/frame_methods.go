package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// CreateUplinkFrame creates an uplink frame record
func (s *PostgresStore) CreateUplinkFrame(ctx context.Context, frame *models.UplinkFrame) error {
	if frame.ID == uuid.Nil {
		frame.ID = uuid.New()
	}

	if frame.ReceivedAt.IsZero() {
		frame.ReceivedAt = time.Now()
	}

	query := `
        INSERT INTO uplink_frames (
            id, dev_eui, dev_addr, application_id, phy_payload,
            tx_info, rx_info, f_cnt, f_port, dr, adr,
            data, object, confirmed, received_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	_, err := s.getDB().ExecContext(ctx, query,
		frame.ID, frame.DevEUI[:], frame.DevAddr[:], frame.ApplicationID,
		frame.PHYPayload, frame.TXInfo, frame.RXInfo, frame.FCnt, frame.FPort,
		frame.DR, frame.ADR, frame.Data, frame.Object, frame.Confirmed,
		frame.ReceivedAt,
	)

	return err
}

// ListUplinkFrames lists uplink frames for a device
func (s *PostgresStore) ListUplinkFrames(ctx context.Context, devEUI lorawan.EUI64, limit, offset int) ([]*models.UplinkFrame, int64, error) {
	// Get count
	var count int64
	err := s.getDB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM uplink_frames WHERE dev_eui = $1", devEUI[:],
	).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	// Get rows
	query := `
        SELECT id, dev_eui, dev_addr, application_id, phy_payload,
               tx_info, rx_info, f_cnt, f_port, dr, adr,
               data, object, confirmed, received_at
        FROM uplink_frames
        WHERE dev_eui = $1
        ORDER BY received_at DESC
        LIMIT $2 OFFSET $3`

	rows, err := s.getDB().QueryContext(ctx, query, devEUI[:], limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var frames []*models.UplinkFrame
	for rows.Next() {
		frame := &models.UplinkFrame{}
		var devEUIBytes, devAddrBytes []byte

		err := rows.Scan(
			&frame.ID, &devEUIBytes, &devAddrBytes, &frame.ApplicationID,
			&frame.PHYPayload, &frame.TXInfo, &frame.RXInfo, &frame.FCnt,
			&frame.FPort, &frame.DR, &frame.ADR, &frame.Data, &frame.Object,
			&frame.Confirmed, &frame.ReceivedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		copy(frame.DevEUI[:], devEUIBytes)
		copy(frame.DevAddr[:], devAddrBytes)

		frames = append(frames, frame)
	}

	return frames, count, nil
}

// CreateDownlinkFrame creates a downlink frame
func (s *PostgresStore) CreateDownlinkFrame(ctx context.Context, frame *models.DownlinkFrame) error {
	if frame.ID == uuid.Nil {
		frame.ID = uuid.New()
	}

	if frame.CreatedAt.IsZero() {
		frame.CreatedAt = time.Now()
	}

	frame.IsPending = true

	query := `
        INSERT INTO downlink_frames (
            id, dev_eui, application_id, f_port, data, confirmed,
            is_pending, retry_count, created_at, reference
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.getDB().ExecContext(ctx, query,
		frame.ID, frame.DevEUI[:], frame.ApplicationID, frame.FPort,
		frame.Data, frame.Confirmed, frame.IsPending, frame.RetryCount,
		frame.CreatedAt, frame.Reference,
	)

	return err
}

// GetPendingDownlinks gets pending downlink frames
func (s *PostgresStore) GetPendingDownlinks(ctx context.Context, devEUI lorawan.EUI64) ([]*models.DownlinkFrame, error) {
	query := `
        SELECT id, dev_eui, application_id, f_port, data, confirmed,
               is_pending, retry_count, created_at, transmitted_at,
               acked_at, reference
        FROM downlink_frames
        WHERE dev_eui = $1 AND is_pending = true
        ORDER BY created_at ASC`

	rows, err := s.getDB().QueryContext(ctx, query, devEUI[:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var frames []*models.DownlinkFrame
	for rows.Next() {
		frame := &models.DownlinkFrame{}
		var devEUIBytes []byte

		err := rows.Scan(
			&frame.ID, &devEUIBytes, &frame.ApplicationID, &frame.FPort,
			&frame.Data, &frame.Confirmed, &frame.IsPending, &frame.RetryCount,
			&frame.CreatedAt, &frame.TransmittedAt, &frame.AckedAt, &frame.Reference,
		)
		if err != nil {
			return nil, err
		}

		copy(frame.DevEUI[:], devEUIBytes)
		frames = append(frames, frame)
	}

	return frames, nil
}

// UpdateDownlinkFrame updates a downlink frame
func (s *PostgresStore) UpdateDownlinkFrame(ctx context.Context, frame *models.DownlinkFrame) error {
	query := `
        UPDATE downlink_frames SET
            is_pending = $2, retry_count = $3, transmitted_at = $4, acked_at = $5
        WHERE id = $1`

	result, err := s.getDB().ExecContext(ctx, query,
		frame.ID, frame.IsPending, frame.RetryCount,
		frame.TransmittedAt, frame.AckedAt,
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

// DeleteDownlinkFrame deletes a pending downlink frame
func (s *PostgresStore) DeleteDownlinkFrame(ctx context.Context, id uuid.UUID) error {
	query := `
        DELETE FROM downlink_frames
        WHERE id = $1 AND is_pending = true`

	result, err := s.getDB().ExecContext(ctx, query, id)
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

// SaveUplinkFrame 保存上行帧
// SaveUplinkFrame 保存上行帧
func (s *PostgresStore) SaveUplinkFrame(ctx context.Context, frame *models.UplinkFrame) error {
	if frame.ID == uuid.Nil {
		frame.ID = uuid.New()
	}

	if frame.ReceivedAt.IsZero() {
		frame.ReceivedAt = time.Now()
	}

	// ✅ 关键修复：将 map 转换为 JSON
	txInfoJSON, err := json.Marshal(frame.TXInfo)
	if err != nil {
		return fmt.Errorf("marshal tx_info: %w", err)
	}

	rxInfoJSON, err := json.Marshal(frame.RXInfo)
	if err != nil {
		return fmt.Errorf("marshal rx_info: %w", err)
	}

	objectJSON, err := json.Marshal(frame.Object)
	if err != nil {
		return fmt.Errorf("marshal object: %w", err)
	}

	query := `
        INSERT INTO uplink_frames (
            id, dev_eui, dev_addr, application_id, phy_payload,
            tx_info, rx_info, f_cnt, f_port, dr, adr,
            data, object, confirmed, received_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 
            $11, $12, $13, $14, $15
        )`

	// 处理可选的 FPort 字段
	var fPort sql.NullInt16
	if frame.FPort != nil {
		fPort = sql.NullInt16{
			Int16: int16(*frame.FPort),
			Valid: true,
		}
	}

	_, err = s.getDB().ExecContext(ctx, query,
		frame.ID,
		frame.DevEUI[:],
		frame.DevAddr[:],
		frame.ApplicationID,
		frame.PHYPayload,
		txInfoJSON, // ✅ 使用JSON格式
		rxInfoJSON, // ✅ 使用JSON格式
		frame.FCnt,
		fPort, // ✅ 使用 sql.NullInt16
		frame.DR,
		frame.ADR,
		frame.Data,
		objectJSON, // ✅ 使用JSON格式
		frame.Confirmed,
		frame.ReceivedAt,
	)

	return err
}

// GetLastGatewayForDevice 获取设备最后使用的网关
func (s *PostgresStore) GetLastGatewayForDevice(ctx context.Context, devEUI lorawan.EUI64) (string, error) {
	query := `
		SELECT rx_info->0->>'gatewayID' as gateway_id
		FROM uplink_frames
		WHERE dev_eui = $1 
		AND rx_info IS NOT NULL
		AND jsonb_array_length(rx_info) > 0
		ORDER BY received_at DESC
		LIMIT 1`

	var gatewayID sql.NullString
	err := s.getDB().QueryRowContext(ctx, query, devEUI[:]).Scan(&gatewayID)

	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	if !gatewayID.Valid {
		return "", nil
	}

	return gatewayID.String, nil
}

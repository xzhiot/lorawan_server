package storage

import (
    "context"
    "fmt"
    "strings"
    "time"
    
    "github.com/google/uuid"
    "github.com/lorawan-server/lorawan-server-pro/internal/models"
)

// CreateEventLog creates an event log entry
func (s *PostgresStore) CreateEventLog(ctx context.Context, event *models.EventLog) error {
    if event.ID == uuid.Nil {
        event.ID = uuid.New()
    }
    
    if event.CreatedAt.IsZero() {
        event.CreatedAt = time.Now()
    }
    
    query := `
        INSERT INTO event_logs (
            id, created_at, tenant_id, application_id, dev_eui,
            gateway_id, type, level, code, description, details
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
    
    var devEUI, gatewayID []byte
    if event.DevEUI != nil {
        devEUI = (*event.DevEUI)[:]
    }
    if event.GatewayID != nil {
        gatewayID = (*event.GatewayID)[:]
    }
    
    _, err := s.getDB().ExecContext(ctx, query,
        event.ID, event.CreatedAt, event.TenantID, event.ApplicationID,
        devEUI, gatewayID, event.Type, event.Level, event.Code,
        event.Description, event.Details,
    )
    
    return err
}

// ListEventLogs lists event logs with filters
func (s *PostgresStore) ListEventLogs(ctx context.Context, filters EventLogFilters, limit, offset int) ([]*models.EventLog, int64, error) {
    // Build query with filters
    query := "SELECT COUNT(*) FROM event_logs WHERE 1=1"
    args := []interface{}{}
    argCount := 0
    
    if filters.TenantID != nil {
        argCount++
        query += fmt.Sprintf(" AND tenant_id = $%d", argCount)
        args = append(args, *filters.TenantID)
    }
    
    if filters.ApplicationID != nil {
        argCount++
        query += fmt.Sprintf(" AND application_id = $%d", argCount)
        args = append(args, *filters.ApplicationID)
    }
    
    if filters.DevEUI != nil {
        argCount++
        query += fmt.Sprintf(" AND dev_eui = $%d", argCount)
        args = append(args, (*filters.DevEUI)[:])
    }
    
    if filters.GatewayID != nil {
        argCount++
        query += fmt.Sprintf(" AND gateway_id = $%d", argCount)
        args = append(args, (*filters.GatewayID)[:])
    }
    
    if filters.Type != nil {
        argCount++
        query += fmt.Sprintf(" AND type = $%d", argCount)
        args = append(args, *filters.Type)
    }
    
    if filters.Level != nil {
        argCount++
        query += fmt.Sprintf(" AND level = $%d", argCount)
        args = append(args, *filters.Level)
    }
    
    if filters.StartTime != nil {
        argCount++
        query += fmt.Sprintf(" AND created_at >= $%d", argCount)
        args = append(args, *filters.StartTime)
    }
    
    if filters.EndTime != nil {
        argCount++
        query += fmt.Sprintf(" AND created_at <= $%d", argCount)
        args = append(args, *filters.EndTime)
    }
    
    // Get count
    var count int64
    err := s.getDB().QueryRowContext(ctx, query, args...).Scan(&count)
    if err != nil {
        return nil, 0, err
    }
    
    // Get rows
    selectQuery := strings.Replace(query, "SELECT COUNT(*)", 
        "SELECT id, created_at, tenant_id, application_id, dev_eui, gateway_id, type, level, code, description, details", 1)
    
    argCount++
    selectQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argCount)
    args = append(args, limit)
    
    argCount++
    selectQuery += fmt.Sprintf(" OFFSET $%d", argCount)
    args = append(args, offset)
    
    rows, err := s.getDB().QueryContext(ctx, selectQuery, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var events []*models.EventLog
    for rows.Next() {
        event := &models.EventLog{}
        var devEUI, gatewayID []byte
        
        err := rows.Scan(
            &event.ID, &event.CreatedAt, &event.TenantID, &event.ApplicationID,
            &devEUI, &gatewayID, &event.Type, &event.Level, &event.Code,
            &event.Description, &event.Details,
        )
        if err != nil {
            return nil, 0, err
        }
        
        if devEUI != nil {
            event.DevEUI = &models.EUI64{}
            copy((*event.DevEUI)[:], devEUI)
        }
        if gatewayID != nil {
            event.GatewayID = &models.EUI64{}
            copy((*event.GatewayID)[:], gatewayID)
        }
        
        events = append(events, event)
    }
    
    return events, count, nil
}

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

// ========== Gateway Methods ==========

// CreateGateway creates a new gateway
func (s *PostgresStore) CreateGateway(ctx context.Context, gateway *models.Gateway) error {
    if gateway.ID == uuid.Nil {
        gateway.ID = uuid.New()
    }
    
    now := time.Now()
    gateway.CreatedAt = now
    gateway.UpdatedAt = now
    
    query := `
        INSERT INTO gateways (
            gateway_id, created_at, updated_at, tenant_id, name, description,
            location, model, min_frequency, max_frequency, network_server_id,
            gateway_profile_id, tags, metadata
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
        )`
    
    _, err := s.getDB().ExecContext(ctx, query,
        gateway.GatewayID[:], gateway.CreatedAt, gateway.UpdatedAt, gateway.TenantID,
        gateway.Name, gateway.Description, gateway.Location, gateway.Model,
        gateway.MinFrequency, gateway.MaxFrequency, gateway.NetworkServerID,
        gateway.GatewayProfileID, gateway.Tags, gateway.Metadata,
    )
    
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return ErrDuplicateKey
        }
        return err
    }
    
    return nil
}

// GetGateway gets a gateway by ID
func (s *PostgresStore) GetGateway(ctx context.Context, gatewayID lorawan.EUI64) (*models.Gateway, error) {
    query := `
        SELECT gateway_id, created_at, updated_at, tenant_id, name, description,
               location, model, min_frequency, max_frequency, last_seen_at,
               first_seen_at, network_server_id, gateway_profile_id, tags, metadata
        FROM gateways
        WHERE gateway_id = $1`
    
    gateway := &models.Gateway{}
    var gatewayIDBytes []byte
    
    err := s.getDB().QueryRowContext(ctx, query, gatewayID[:]).Scan(
        &gatewayIDBytes, &gateway.CreatedAt, &gateway.UpdatedAt, &gateway.TenantID,
        &gateway.Name, &gateway.Description, &gateway.Location, &gateway.Model,
        &gateway.MinFrequency, &gateway.MaxFrequency, &gateway.LastSeenAt,
        &gateway.FirstSeenAt, &gateway.NetworkServerID, &gateway.GatewayProfileID,
        &gateway.Tags, &gateway.Metadata,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    if err != nil {
        return nil, err
    }
    
    copy(gateway.GatewayID[:], gatewayIDBytes)
    
    return gateway, nil
}

// UpdateGateway updates a gateway
func (s *PostgresStore) UpdateGateway(ctx context.Context, gateway *models.Gateway) error {
    gateway.UpdatedAt = time.Now()
    
    query := `
        UPDATE gateways SET
            updated_at = $2, name = $3, description = $4, location = $5,
            model = $6, min_frequency = $7, max_frequency = $8,
            last_seen_at = $9, first_seen_at = $10, tags = $11, metadata = $12
        WHERE gateway_id = $1`
    
    result, err := s.getDB().ExecContext(ctx, query,
        gateway.GatewayID[:], gateway.UpdatedAt, gateway.Name, gateway.Description,
        gateway.Location, gateway.Model, gateway.MinFrequency, gateway.MaxFrequency,
        gateway.LastSeenAt, gateway.FirstSeenAt, gateway.Tags, gateway.Metadata,
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

// DeleteGateway deletes a gateway
func (s *PostgresStore) DeleteGateway(ctx context.Context, gatewayID lorawan.EUI64) error {
    result, err := s.getDB().ExecContext(ctx, "DELETE FROM gateways WHERE gateway_id = $1", gatewayID[:])
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

// ListGateways lists gateways
func (s *PostgresStore) ListGateways(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.Gateway, int64, error) {
    // Get count
    var count int64
    err := s.getDB().QueryRowContext(ctx,
        "SELECT COUNT(*) FROM gateways WHERE tenant_id = $1", tenantID,
    ).Scan(&count)
    if err != nil {
        return nil, 0, err
    }
    
    // Get rows
    query := `
        SELECT gateway_id, created_at, updated_at, tenant_id, name, description,
               location, last_seen_at, first_seen_at
        FROM gateways
        WHERE tenant_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`
    
    rows, err := s.getDB().QueryContext(ctx, query, tenantID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var gateways []*models.Gateway
    for rows.Next() {
        gateway := &models.Gateway{}
        var gatewayIDBytes []byte
        
        err := rows.Scan(
            &gatewayIDBytes, &gateway.CreatedAt, &gateway.UpdatedAt, &gateway.TenantID,
            &gateway.Name, &gateway.Description, &gateway.Location,
            &gateway.LastSeenAt, &gateway.FirstSeenAt,
        )
        if err != nil {
            return nil, 0, err
        }
        
        copy(gateway.GatewayID[:], gatewayIDBytes)
        gateways = append(gateways, gateway)
    }
    
    return gateways, count, nil
}

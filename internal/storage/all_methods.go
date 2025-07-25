package storage

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    "time"
    
    "github.com/google/uuid"
    "github.com/lorawan-server/lorawan-server-pro/internal/models"
    "github.com/lorawan-server/lorawan-server-pro/pkg/crypto"
)

// ========== User Methods ==========

// CreateUser creates a new user
func (s *PostgresStore) CreateUser(ctx context.Context, user *models.User) error {
    if user.ID == uuid.Nil {
        user.ID = uuid.New()
    }
    
    now := time.Now()
    user.CreatedAt = now
    user.UpdatedAt = now
    
    // Hash password if provided in settings
    if pwd, ok := user.Settings["password"].(string); ok && pwd != "" {
        hash, err := crypto.HashPassword(pwd)
        if err != nil {
            return fmt.Errorf("hash password: %w", err)
        }
        user.PasswordHash = hash
        delete(user.Settings, "password")
    }
    
    query := `
        INSERT INTO users (
            id, created_at, updated_at, email, username, first_name, last_name,
            password_hash, is_admin, is_active, email_verified, tenant_id, settings
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
        )`
    
    _, err := s.getDB().ExecContext(ctx, query,
        user.ID, user.CreatedAt, user.UpdatedAt, user.Email, user.Username,
        user.FirstName, user.LastName, user.PasswordHash, user.IsAdmin,
        user.IsActive, user.EmailVerified, user.TenantID, user.Settings,
    )
    
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return ErrDuplicateKey
        }
        return err
    }
    
    return nil
}

// GetUser gets a user by ID
func (s *PostgresStore) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
    query := `
        SELECT id, created_at, updated_at, email, username, first_name, last_name,
               password_hash, is_admin, is_active, email_verified, email_verified_at,
               last_login_at, tenant_id, settings
        FROM users
        WHERE id = $1`
    
    user := &models.User{}
    err := s.getDB().QueryRowContext(ctx, query, id).Scan(
        &user.ID, &user.CreatedAt, &user.UpdatedAt, &user.Email, &user.Username,
        &user.FirstName, &user.LastName, &user.PasswordHash, &user.IsAdmin,
        &user.IsActive, &user.EmailVerified, &user.EmailVerifiedAt,
        &user.LastLoginAt, &user.TenantID, &user.Settings,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    return user, err
}

// GetUserByEmail gets a user by email
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
    query := `
        SELECT id, created_at, updated_at, email, username, first_name, last_name,
               password_hash, is_admin, is_active, email_verified, email_verified_at,
               last_login_at, tenant_id, settings
        FROM users
        WHERE email = $1`
    
    user := &models.User{}
    err := s.getDB().QueryRowContext(ctx, query, email).Scan(
        &user.ID, &user.CreatedAt, &user.UpdatedAt, &user.Email, &user.Username,
        &user.FirstName, &user.LastName, &user.PasswordHash, &user.IsAdmin,
        &user.IsActive, &user.EmailVerified, &user.EmailVerifiedAt,
        &user.LastLoginAt, &user.TenantID, &user.Settings,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    return user, err
}

// UpdateUser updates a user
func (s *PostgresStore) UpdateUser(ctx context.Context, user *models.User) error {
    user.UpdatedAt = time.Now()
    
    query := `
        UPDATE users SET
            updated_at = $2, email = $3, username = $4, first_name = $5,
            last_name = $6, is_admin = $7, is_active = $8, email_verified = $9,
            email_verified_at = $10, last_login_at = $11, tenant_id = $12, settings = $13
        WHERE id = $1`
    
    result, err := s.getDB().ExecContext(ctx, query,
        user.ID, user.UpdatedAt, user.Email, user.Username, user.FirstName,
        user.LastName, user.IsAdmin, user.IsActive, user.EmailVerified,
        user.EmailVerifiedAt, user.LastLoginAt, user.TenantID, user.Settings,
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

// DeleteUser deletes a user
func (s *PostgresStore) DeleteUser(ctx context.Context, id uuid.UUID) error {
    result, err := s.getDB().ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
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

// ListUsers lists users
func (s *PostgresStore) ListUsers(ctx context.Context, tenantID *uuid.UUID, limit, offset int) ([]*models.User, int64, error) {
    var args []interface{}
    query := `SELECT id, created_at, updated_at, email, username, first_name, last_name,
                     is_admin, is_active, email_verified, last_login_at, tenant_id
              FROM users`
    countQuery := `SELECT COUNT(*) FROM users`
    
    if tenantID != nil {
        query += ` WHERE tenant_id = $1`
        countQuery += ` WHERE tenant_id = $1`
        args = append(args, *tenantID)
    }
    
    // Get count
    var count int64
    err := s.getDB().QueryRowContext(ctx, countQuery, args...).Scan(&count)
    if err != nil {
        return nil, 0, err
    }
    
    // Get rows
    query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d OFFSET %d`, limit, offset)
    
    rows, err := s.getDB().QueryContext(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var users []*models.User
    for rows.Next() {
        user := &models.User{}
        err := rows.Scan(
            &user.ID, &user.CreatedAt, &user.UpdatedAt, &user.Email, &user.Username,
            &user.FirstName, &user.LastName, &user.IsAdmin, &user.IsActive,
            &user.EmailVerified, &user.LastLoginAt, &user.TenantID,
        )
        if err != nil {
            return nil, 0, err
        }
        users = append(users, user)
    }
    
    return users, count, nil
}

// ========== Tenant Methods ==========

// CreateTenant creates a new tenant
func (s *PostgresStore) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
    if tenant.ID == uuid.Nil {
        tenant.ID = uuid.New()
    }
    
    now := time.Now()
    tenant.CreatedAt = now
    tenant.UpdatedAt = now
    tenant.IsActive = true
    
    query := `
        INSERT INTO tenants (
            id, created_at, updated_at, name, description, max_gateway_count,
            max_device_count, max_user_count, can_have_gateways, private_gateways,
            billing_email, billing_plan, is_active
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
        )`
    
    _, err := s.getDB().ExecContext(ctx, query,
        tenant.ID, tenant.CreatedAt, tenant.UpdatedAt, tenant.Name, tenant.Description,
        tenant.MaxGatewayCount, tenant.MaxDeviceCount, tenant.MaxUserCount,
        tenant.CanHaveGateways, tenant.PrivateGateways, tenant.BillingEmail,
        tenant.BillingPlan, tenant.IsActive,
    )
    
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return ErrDuplicateKey
        }
        return err
    }
    
    return nil
}

// GetTenant gets a tenant by ID
func (s *PostgresStore) GetTenant(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
    query := `
        SELECT id, created_at, updated_at, name, description, max_gateway_count,
               max_device_count, max_user_count, can_have_gateways, private_gateways,
               billing_email, billing_plan, is_active, suspended_at
        FROM tenants
        WHERE id = $1`
    
    tenant := &models.Tenant{}
    err := s.getDB().QueryRowContext(ctx, query, id).Scan(
        &tenant.ID, &tenant.CreatedAt, &tenant.UpdatedAt, &tenant.Name, &tenant.Description,
        &tenant.MaxGatewayCount, &tenant.MaxDeviceCount, &tenant.MaxUserCount,
        &tenant.CanHaveGateways, &tenant.PrivateGateways, &tenant.BillingEmail,
        &tenant.BillingPlan, &tenant.IsActive, &tenant.SuspendedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    return tenant, err
}

// UpdateTenant updates a tenant
func (s *PostgresStore) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
    tenant.UpdatedAt = time.Now()
    
    query := `
        UPDATE tenants SET
            updated_at = $2, name = $3, description = $4, max_gateway_count = $5,
            max_device_count = $6, max_user_count = $7, can_have_gateways = $8,
            private_gateways = $9, billing_email = $10, billing_plan = $11,
            is_active = $12, suspended_at = $13
        WHERE id = $1`
    
    result, err := s.getDB().ExecContext(ctx, query,
        tenant.ID, tenant.UpdatedAt, tenant.Name, tenant.Description,
        tenant.MaxGatewayCount, tenant.MaxDeviceCount, tenant.MaxUserCount,
        tenant.CanHaveGateways, tenant.PrivateGateways, tenant.BillingEmail,
        tenant.BillingPlan, tenant.IsActive, tenant.SuspendedAt,
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

// DeleteTenant deletes a tenant
func (s *PostgresStore) DeleteTenant(ctx context.Context, id uuid.UUID) error {
    result, err := s.getDB().ExecContext(ctx, "DELETE FROM tenants WHERE id = $1", id)
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

// ListTenants lists tenants
func (s *PostgresStore) ListTenants(ctx context.Context, limit, offset int) ([]*models.Tenant, int64, error) {
    // Get count
    var count int64
    err := s.getDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants").Scan(&count)
    if err != nil {
        return nil, 0, err
    }
    
    // Get rows
    query := `
        SELECT id, created_at, updated_at, name, description, max_gateway_count,
               max_device_count, max_user_count, can_have_gateways, is_active
        FROM tenants
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2`
    
    rows, err := s.getDB().QueryContext(ctx, query, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var tenants []*models.Tenant
    for rows.Next() {
        tenant := &models.Tenant{}
        err := rows.Scan(
            &tenant.ID, &tenant.CreatedAt, &tenant.UpdatedAt, &tenant.Name,
            &tenant.Description, &tenant.MaxGatewayCount, &tenant.MaxDeviceCount,
            &tenant.MaxUserCount, &tenant.CanHaveGateways, &tenant.IsActive,
        )
        if err != nil {
            return nil, 0, err
        }
        tenants = append(tenants, tenant)
    }
    
    return tenants, count, nil
}

// ========== Application Methods ==========

// CreateApplication creates a new application
func (s *PostgresStore) CreateApplication(ctx context.Context, app *models.Application) error {
    if app.ID == uuid.Nil {
        app.ID = uuid.New()
    }
    
    now := time.Now()
    app.CreatedAt = now
    app.UpdatedAt = now
    
    query := `
        INSERT INTO applications (
            id, created_at, updated_at, tenant_id, name, description,
            http_integration, mqtt_integration, payload_codec,
            payload_decoder, payload_encoder
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
        )`
    
    _, err := s.getDB().ExecContext(ctx, query,
        app.ID, app.CreatedAt, app.UpdatedAt, app.TenantID, app.Name,
        app.Description, app.HTTPIntegration, app.MQTTIntegration,
        app.PayloadCodec, app.PayloadDecoder, app.PayloadEncoder,
    )
    
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return ErrDuplicateKey
        }
        return err
    }
    
    return nil
}

// GetApplication gets an application by ID
func (s *PostgresStore) GetApplication(ctx context.Context, id uuid.UUID) (*models.Application, error) {
    query := `
        SELECT id, created_at, updated_at, tenant_id, name, description,
               http_integration, mqtt_integration, payload_codec,
               payload_decoder, payload_encoder
        FROM applications
        WHERE id = $1`
    
    app := &models.Application{}
    err := s.getDB().QueryRowContext(ctx, query, id).Scan(
        &app.ID, &app.CreatedAt, &app.UpdatedAt, &app.TenantID, &app.Name,
        &app.Description, &app.HTTPIntegration, &app.MQTTIntegration,
        &app.PayloadCodec, &app.PayloadDecoder, &app.PayloadEncoder,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    
    return app, err
}

// UpdateApplication updates an application
func (s *PostgresStore) UpdateApplication(ctx context.Context, app *models.Application) error {
    app.UpdatedAt = time.Now()
    
    query := `
        UPDATE applications SET
            updated_at = $2, name = $3, description = $4,
            http_integration = $5, mqtt_integration = $6,
            payload_codec = $7, payload_decoder = $8, payload_encoder = $9
        WHERE id = $1`
    
    result, err := s.getDB().ExecContext(ctx, query,
        app.ID, app.UpdatedAt, app.Name, app.Description,
        app.HTTPIntegration, app.MQTTIntegration,
        app.PayloadCodec, app.PayloadDecoder, app.PayloadEncoder,
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

// DeleteApplication deletes an application
func (s *PostgresStore) DeleteApplication(ctx context.Context, id uuid.UUID) error {
    result, err := s.getDB().ExecContext(ctx, "DELETE FROM applications WHERE id = $1", id)
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

// ListApplications lists applications
func (s *PostgresStore) ListApplications(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.Application, int64, error) {
    // Get count
    var count int64
    err := s.getDB().QueryRowContext(ctx,
        "SELECT COUNT(*) FROM applications WHERE tenant_id = $1", tenantID,
    ).Scan(&count)
    if err != nil {
        return nil, 0, err
    }
    
    // Get rows
    query := `
        SELECT id, created_at, updated_at, tenant_id, name, description
        FROM applications
        WHERE tenant_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`
    
    rows, err := s.getDB().QueryContext(ctx, query, tenantID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var apps []*models.Application
    for rows.Next() {
        app := &models.Application{}
        err := rows.Scan(
            &app.ID, &app.CreatedAt, &app.UpdatedAt, &app.TenantID,
            &app.Name, &app.Description,
        )
        if err != nil {
            return nil, 0, err
        }
        apps = append(apps, app)
    }
    
    return apps, count, nil
}

// [继续实现其他方法...]
// 由于篇幅限制，这里只展示了部分方法的实现
// 完整实现应包括所有 Store 接口中定义的方法

// TODO: 实现以下方法组：
// - Device methods (CreateDevice, GetDevice, etc.)
// - Gateway methods (CreateGateway, GetGateway, etc.)
// - DeviceProfile methods
// - Frame methods
// - EventLog methods
// - DeviceKeys methods
// - DeviceSession methods

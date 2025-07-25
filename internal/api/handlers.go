package api

import (
    "fmt"
    "encoding/hex"
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/rs/zerolog/log"

    "github.com/lorawan-server/lorawan-server-pro/internal/models"
    "github.com/lorawan-server/lorawan-server-pro/internal/storage"
    "github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// ========== Auth handlers ==========

// HandleLogin handles user login
func (s *RESTServer) HandleLogin(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email    string `json:"email" validate:"required,email"`
        Password string `json:"password" validate:"required"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Get user
    user, err := s.store.GetUserByEmail(r.Context(), req.Email)
    if err != nil {
        s.respondError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    // Verify password
    if !s.auth.VerifyPassword(req.Password, user.PasswordHash) {
        s.respondError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    // Check user status
    if !user.IsActive {
        s.respondError(w, http.StatusForbidden, "account is disabled")
        return
    }

    // Generate tokens
    accessToken, refreshToken, err := s.auth.GenerateTokenPair(user)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, "failed to generate tokens")
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "expires_in":    int(s.config.JWT.AccessTokenTTL.Seconds()),
        "token_type":    "Bearer",
    })
}

// HandleRefresh handles token refresh
func (s *RESTServer) HandleRefresh(w http.ResponseWriter, r *http.Request) {
    var req struct {
        RefreshToken string `json:"refresh_token" validate:"required"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Refresh token
    accessToken, refreshToken, err := s.auth.RefreshToken(req.RefreshToken)
    if err != nil {
        s.respondError(w, http.StatusUnauthorized, "invalid refresh token")
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "expires_in":    int(s.config.JWT.AccessTokenTTL.Seconds()),
        "token_type":    "Bearer",
    })
}

// HandleGetCurrentUser gets current user
func (s *RESTServer) HandleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
    // TODO: Get from context
    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "id":       "22222222-2222-2222-2222-222222222222",
        "email":    "admin@example.com",
        "name":     "Administrator",
        "role":     "admin",
        "is_admin": true,
    })
}

// ========== Tenant handlers ==========

// HandleListTenants lists tenants
func (s *RESTServer) HandleListTenants(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit == 0 {
        limit = 20
    }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    tenants, total, err := s.store.ListTenants(ctx, limit, offset)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "tenants": tenants,
        "total":   total,
    })
}

// HandleCreateTenant creates a tenant
func (s *RESTServer) HandleCreateTenant(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name            string `json:"name" validate:"required,min=3,max=100"`
        Description     string `json:"description"`
        MaxDeviceCount  int    `json:"max_device_count" validate:"min=0"`
        MaxGatewayCount int    `json:"max_gateway_count" validate:"min=0"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    tenant := &models.Tenant{
        Name:            req.Name,
        Description:     req.Description,
        MaxDeviceCount:  req.MaxDeviceCount,
        MaxGatewayCount: req.MaxGatewayCount,
        CanHaveGateways: req.MaxGatewayCount > 0,
    }

    if tenant.MaxDeviceCount == 0 {
        tenant.MaxDeviceCount = 100
    }
    if tenant.MaxGatewayCount == 0 {
        tenant.MaxGatewayCount = 10
    }

    if err := s.store.CreateTenant(r.Context(), tenant); err != nil {
        if err == storage.ErrDuplicateKey {
            s.respondError(w, http.StatusConflict, "tenant already exists")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusCreated, tenant)
}

// HandleGetTenant gets a tenant
func (s *RESTServer) HandleGetTenant(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid tenant id")
        return
    }

    tenant, err := s.store.GetTenant(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "tenant not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, tenant)
}

// HandleUpdateTenant updates a tenant
func (s *RESTServer) HandleUpdateTenant(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid tenant id")
        return
    }

    var req struct {
        Name            string `json:"name" validate:"required,min=3,max=100"`
        Description     string `json:"description"`
        MaxDeviceCount  int    `json:"max_device_count" validate:"min=0"`
        MaxGatewayCount int    `json:"max_gateway_count" validate:"min=0"`
        CanHaveGateways bool   `json:"can_have_gateways"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    tenant, err := s.store.GetTenant(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "tenant not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    tenant.Name = req.Name
    tenant.Description = req.Description
    tenant.MaxDeviceCount = req.MaxDeviceCount
    tenant.MaxGatewayCount = req.MaxGatewayCount
    tenant.CanHaveGateways = req.CanHaveGateways || req.MaxGatewayCount > 0

    if err := s.store.UpdateTenant(ctx, tenant); err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, tenant)
}

// HandleDeleteTenant deletes a tenant
func (s *RESTServer) HandleDeleteTenant(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid tenant id")
        return
    }

    if err := s.store.DeleteTenant(ctx, id); err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "tenant not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// ========== Application handlers ==========

// HandleListApplications lists applications
func (s *RESTServer) HandleListApplications(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // TODO: Get from auth context
    tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit == 0 {
        limit = 20
    }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    apps, total, err := s.store.ListApplications(ctx, tenantID, limit, offset)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "applications": apps,
        "total":        total,
    })
}

// HandleCreateApplication creates an application
func (s *RESTServer) HandleCreateApplication(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name        string `json:"name" validate:"required,min=3,max=100"`
        Description string `json:"description"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    // TODO: Get from auth context
    tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

    app := &models.Application{
        TenantModel: models.TenantModel{
            TenantID: tenantID,
        },
        Name:        req.Name,
        Description: req.Description,
    }

    if err := s.store.CreateApplication(r.Context(), app); err != nil {
        if err == storage.ErrDuplicateKey {
            s.respondError(w, http.StatusConflict, "application already exists")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusCreated, app)
}

// HandleGetApplication gets an application
func (s *RESTServer) HandleGetApplication(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid application id")
        return
    }

    app, err := s.store.GetApplication(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "application not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, app)
}

// HandleUpdateApplication updates an application
func (s *RESTServer) HandleUpdateApplication(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid application id")
        return
    }

    var req struct {
        Name        string `json:"name" validate:"required,min=3,max=100"`
        Description string `json:"description"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    app, err := s.store.GetApplication(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "application not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    app.Name = req.Name
    app.Description = req.Description

    if err := s.store.UpdateApplication(ctx, app); err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, app)
}

// HandleDeleteApplication deletes an application
func (s *RESTServer) HandleDeleteApplication(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid application id")
        return
    }

    if err := s.store.DeleteApplication(ctx, id); err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "application not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// ========== User handlers ==========

// HandleListUsers lists users
func (s *RESTServer) HandleListUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var tenantID *uuid.UUID
    if tid := r.URL.Query().Get("tenant_id"); tid != "" {
        id, err := uuid.Parse(tid)
        if err == nil {
            tenantID = &id
        }
    }

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit == 0 {
        limit = 20
    }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    users, total, err := s.store.ListUsers(ctx, tenantID, limit, offset)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "users": users,
        "total": total,
    })
}

// HandleCreateUser creates a user
func (s *RESTServer) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email     string    `json:"email" validate:"required,email"`
        Password  string    `json:"password" validate:"required,min=6"`
        Username  string    `json:"username,omitempty"`
        FirstName string    `json:"firstName,omitempty"`
        LastName  string    `json:"lastName,omitempty"`
        TenantID  uuid.UUID `json:"tenant_id"`
        IsAdmin   bool      `json:"is_admin"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    if req.Username == "" {
        req.Username = req.Email
    }

    user := &models.User{
        Email:     req.Email,
        Username:  req.Username,
        FirstName: req.FirstName,
        LastName:  req.LastName,
        TenantID:  &req.TenantID,
        IsAdmin:   req.IsAdmin,
        IsActive:  true,
        Settings:  make(models.Variables),
    }

    // Store password temporarily
    user.Settings["password"] = req.Password

    if err := s.store.CreateUser(r.Context(), user); err != nil {
        if err == storage.ErrDuplicateKey {
            s.respondError(w, http.StatusConflict, "user with this email already exists")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Clear sensitive data
    delete(user.Settings, "password")
    user.PasswordHash = ""

    s.respondJSON(w, http.StatusCreated, user)
}

// HandleGetUser gets a user
func (s *RESTServer) HandleGetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid user id")
        return
    }

    user, err := s.store.GetUser(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "user not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Clear password hash
    user.PasswordHash = ""

    s.respondJSON(w, http.StatusOK, user)
}

// HandleUpdateUser updates a user
func (s *RESTServer) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid user id")
        return
    }

    var req struct {
        Username  string `json:"username,omitempty"`
        FirstName string `json:"firstName,omitempty"`
        LastName  string `json:"lastName,omitempty"`
        Email     string `json:"email" validate:"required,email"`
        IsActive  bool   `json:"is_active"`
        IsAdmin   bool   `json:"is_admin"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    user, err := s.store.GetUser(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "user not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Update fields
    if req.Username != "" {
        user.Username = req.Username
    }
    if req.FirstName != "" {
        user.FirstName = req.FirstName
    }
    if req.LastName != "" {
        user.LastName = req.LastName
    }
    user.Email = req.Email
    user.IsActive = req.IsActive
    user.IsAdmin = req.IsAdmin

    if err := s.store.UpdateUser(ctx, user); err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Clear password hash
    user.PasswordHash = ""

    s.respondJSON(w, http.StatusOK, user)
}

// HandleDeleteUser deletes a user
func (s *RESTServer) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid user id")
        return
    }

    if err := s.store.DeleteUser(ctx, id); err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "user not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// ========== Helper methods ==========

// HandleHealth health check
func (s *RESTServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "status": "healthy",
        "time":   time.Now(),
    })
}

// HandleRoot root handler
func (s *RESTServer) HandleRoot(w http.ResponseWriter, r *http.Request) {
    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "service":  "LoRaWAN Application Server",
        "version":  "1.0.0",
        "api_docs": "/api/v1/docs",
        "health":   "/api/v1/health",
        "message":  "Please use the Web UI at port 8098 or access the API endpoints",
    })
}

// respondJSON responds with JSON
func (s *RESTServer) respondJSON(w http.ResponseWriter, status int, payload interface{}) {
    response, err := json.Marshal(payload)
    if err != nil {
        log.Error().Err(err).Msg("Failed to marshal response")
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    w.Write(response)
}

// respondError responds with error
func (s *RESTServer) respondError(w http.ResponseWriter, status int, message string) {
    s.respondJSON(w, status, map[string]string{
        "error": message,
    })
}

// ========== Helper functions ==========

// parseEUI64 parses EUI64
func parseEUI64(s string) (lorawan.EUI64, error) {
    var eui lorawan.EUI64
    if len(s) != 16 {
        return eui, fmt.Errorf("invalid length")
    }

    bytes, err := hex.DecodeString(s)
    if err != nil {
        return eui, err
    }

    copy(eui[:], bytes)
    return eui, nil
}

// parseDevAddr parses DevAddr
func parseDevAddr(s string) (lorawan.DevAddr, error) {
    var addr lorawan.DevAddr
    if len(s) != 8 {
        return addr, fmt.Errorf("invalid length")
    }

    bytes, err := hex.DecodeString(s)
    if err != nil {
        return addr, err
    }

    copy(addr[:], bytes)
    return addr, nil
}

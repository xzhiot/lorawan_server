package api

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    "github.com/lorawan-server/lorawan-server-pro/internal/models"
    "github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

// HandleListGateways lists gateways
func (s *RESTServer) HandleListGateways(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // TODO: Get from auth context
    tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit == 0 {
        limit = 20
    }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    gateways, total, err := s.store.ListGateways(ctx, tenantID, limit, offset)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "gateways": gateways,
        "total":    total,
    })
}

// HandleCreateGateway creates a gateway
func (s *RESTServer) HandleCreateGateway(w http.ResponseWriter, r *http.Request) {
    var req struct {
        GatewayID   string  `json:"gateway_id" validate:"required,len=16"`
        Name        string  `json:"name" validate:"required"`
        Description string  `json:"description"`
        Latitude    float64 `json:"latitude"`
        Longitude   float64 `json:"longitude"`
        Altitude    float64 `json:"altitude"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    gatewayID, err := parseEUI64(req.GatewayID)
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid gateway_id")
        return
    }

    // TODO: Get from auth context
    tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

    gateway := &models.Gateway{
        GatewayID: models.EUI64(gatewayID),
        TenantModel: models.TenantModel{
            TenantID: tenantID,
        },
        Name:        req.Name,
        Description: req.Description,
    }

    // Handle location
    if req.Latitude != 0 || req.Longitude != 0 || req.Altitude != 0 {
        gateway.Location = &models.Location{
            Latitude:  req.Latitude,
            Longitude: req.Longitude,
            Altitude:  req.Altitude,
        }
    }

    if err := s.store.CreateGateway(r.Context(), gateway); err != nil {
        if err == storage.ErrDuplicateKey {
            s.respondError(w, http.StatusConflict, "gateway already exists")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusCreated, gateway)
}

// HandleGetGateway gets a gateway
func (s *RESTServer) HandleGetGateway(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    gatewayIDStr := chi.URLParam(r, "gateway_id")
    gatewayID, err := parseEUI64(gatewayIDStr)
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid gateway_id")
        return
    }

    gateway, err := s.store.GetGateway(ctx, gatewayID)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "gateway not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, gateway)
}

// HandleUpdateGateway updates a gateway
func (s *RESTServer) HandleUpdateGateway(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    gatewayIDStr := chi.URLParam(r, "gateway_id")
    gatewayID, err := parseEUI64(gatewayIDStr)
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid gateway_id")
        return
    }

    var req struct {
        Name        string  `json:"name" validate:"required"`
        Description string  `json:"description"`
        Latitude    float64 `json:"latitude"`
        Longitude   float64 `json:"longitude"`
        Altitude    float64 `json:"altitude"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := s.validator.Validate(req); err != nil {
        s.respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    gateway, err := s.store.GetGateway(ctx, gatewayID)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "gateway not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    gateway.Name = req.Name
    gateway.Description = req.Description

    // Update location
    if req.Latitude != 0 || req.Longitude != 0 || req.Altitude != 0 {
        gateway.Location = &models.Location{
            Latitude:  req.Latitude,
            Longitude: req.Longitude,
            Altitude:  req.Altitude,
        }
    }

    if err := s.store.UpdateGateway(ctx, gateway); err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, gateway)
}

// HandleDeleteGateway deletes a gateway
func (s *RESTServer) HandleDeleteGateway(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    gatewayIDStr := chi.URLParam(r, "gateway_id")
    gatewayID, err := parseEUI64(gatewayIDStr)
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid gateway_id")
        return
    }

    if err := s.store.DeleteGateway(ctx, gatewayID); err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "gateway not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// HandleListDeviceProfiles lists device profiles
func (s *RESTServer) HandleListDeviceProfiles(w http.ResponseWriter, r *http.Request) {
    // TODO: Implement device profiles list
    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "profiles": []map[string]interface{}{
            {
                "id":          "44444444-4444-4444-4444-444444444444",
                "name":        "Default Profile",
                "description": "Default device profile for Class A devices",
            },
        },
        "total": 1,
    })
}

// HandleCreateDeviceProfile creates device profile
func (s *RESTServer) HandleCreateDeviceProfile(w http.ResponseWriter, r *http.Request) {
    s.respondJSON(w, http.StatusCreated, map[string]string{
        "message": "create device profile not implemented",
    })
}

// HandleGetDeviceProfile gets device profile
func (s *RESTServer) HandleGetDeviceProfile(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "invalid profile id")
        return
    }

    profile, err := s.store.GetDeviceProfile(ctx, id)
    if err != nil {
        if err == storage.ErrNotFound {
            s.respondError(w, http.StatusNotFound, "device profile not found")
            return
        }
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, profile)
}

// HandleUpdateDeviceProfile updates device profile
func (s *RESTServer) HandleUpdateDeviceProfile(w http.ResponseWriter, r *http.Request) {
    s.respondJSON(w, http.StatusOK, map[string]string{
        "message": "update device profile not implemented",
    })
}

// HandleDeleteDeviceProfile deletes device profile
func (s *RESTServer) HandleDeleteDeviceProfile(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}

// HandleListEvents lists events
func (s *RESTServer) HandleListEvents(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit == 0 {
        limit = 20
    }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    filters := storage.EventLogFilters{}

    // Parse filters
    if appID := r.URL.Query().Get("application_id"); appID != "" {
        if id, err := uuid.Parse(appID); err == nil {
            filters.ApplicationID = &id
        }
    }

    if devEUIStr := r.URL.Query().Get("dev_eui"); devEUIStr != "" {
        if devEUI, err := parseEUI64(devEUIStr); err == nil {
            filters.DevEUI = &devEUI
        }
    }

    if eventType := r.URL.Query().Get("type"); eventType != "" {
        modelEventType := models.EventType(eventType)
        filters.Type = &modelEventType
    }

    if level := r.URL.Query().Get("level"); level != "" {
        modelEventLevel := models.EventLevel(level)
        filters.Level = &modelEventLevel
    }

    events, total, err := s.store.ListEventLogs(ctx, filters, limit, offset)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "events": events,
        "total":  total,
    })
}

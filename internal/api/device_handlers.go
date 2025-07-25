package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

// HandleListDevices lists devices
func (s *RESTServer) HandleListDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := r.URL.Query().Get("application_id")
	if applicationID == "" {
		s.respondError(w, http.StatusBadRequest, "application_id is required")
		return
	}

	appID, err := uuid.Parse(applicationID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid application_id")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	devices, total, err := s.store.ListDevices(ctx, appID, limit, offset)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
		"total":   total,
	})
}

// HandleCreateDevice creates a device
func (s *RESTServer) HandleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DevEUI          string    `json:"dev_eui" validate:"required,len=16"`
		Name            string    `json:"name" validate:"required"`
		Description     string    `json:"description"`
		ApplicationID   uuid.UUID `json:"application_id" validate:"required"`
		DeviceProfileID uuid.UUID `json:"device_profile_id" validate:"required"`

		// OTAA keys
		AppKey  string `json:"app_key,omitempty" validate:"omitempty,len=32"`
		NwkKey  string `json:"nwk_key,omitempty" validate:"omitempty,len=32"`
		JoinEUI string `json:"join_eui,omitempty" validate:"omitempty,len=16"`

		// ABP params
		DevAddr string `json:"dev_addr,omitempty" validate:"omitempty,len=8"`
		AppSKey string `json:"app_s_key,omitempty" validate:"omitempty,len=32"`
		NwkSKey string `json:"nwk_s_key,omitempty" validate:"omitempty,len=32"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.validator.Validate(req); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Parse DevEUI
	devEUI, err := parseEUI64(req.DevEUI)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid DevEUI")
		return
	}

	// Get application to determine tenant
	app, err := s.store.GetApplication(r.Context(), req.ApplicationID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "application not found")
		return
	}

	device := &models.Device{
		DevEUI:      models.EUI64(devEUI),
		Name:        req.Name,
		Description: req.Description,
		TenantModel: models.TenantModel{
			TenantID: app.TenantID,
		},
		ApplicationID:   req.ApplicationID,
		DeviceProfileID: req.DeviceProfileID,
	}

	// Handle OTAA params
	if req.JoinEUI != "" {
		joinEUI, err := parseEUI64(req.JoinEUI)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, "invalid JoinEUI")
			return
		}
		device.JoinEUI = (*models.EUI64)(&joinEUI)
	}

	// Handle ABP params
	if req.DevAddr != "" {
		devAddr, err := parseDevAddr(req.DevAddr)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, "invalid DevAddr")
			return
		}
		device.DevAddr = (*models.DevAddr)(&devAddr)
		device.AppSKey = &req.AppSKey
		device.NwkSEncKey = &req.NwkSKey
		device.SNwkSIntKey = &req.NwkSKey
		device.FNwkSIntKey = &req.NwkSKey
	}

	// Create device
	if err := s.store.CreateDevice(r.Context(), device); err != nil {
		if err == storage.ErrDuplicateKey {
			s.respondError(w, http.StatusConflict, "device already exists")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Save keys (if OTAA)
	if req.AppKey != "" {
		keys := &models.DeviceKeys{
			DevEUI: device.DevEUI,
			AppKey: req.AppKey,
			NwkKey: req.NwkKey,
		}

		if err := s.store.SetDeviceKeys(r.Context(), keys); err != nil {
			// Rollback: delete device
			s.store.DeleteDevice(r.Context(), devEUI)
			s.respondError(w, http.StatusInternalServerError, "failed to save device keys")
			return
		}
	}

	s.respondJSON(w, http.StatusCreated, device)
}

// HandleGetDevice gets a device
func (s *RESTServer) HandleGetDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	device, err := s.store.GetDevice(ctx, devEUI)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "device not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, device)
}

// HandleUpdateDevice updates a device
func (s *RESTServer) HandleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	var req struct {
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
		IsDisabled  bool   `json:"is_disabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.validator.Validate(req); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	device, err := s.store.GetDevice(ctx, devEUI)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "device not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	device.Name = req.Name
	device.Description = req.Description
	device.IsDisabled = req.IsDisabled

	if err := s.store.UpdateDevice(ctx, device); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, device)
}

// HandleDeleteDevice deletes a device
func (s *RESTServer) HandleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	if err := s.store.DeleteDevice(ctx, devEUI); err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "device not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleActivateDevice activates a device
func (s *RESTServer) HandleActivateDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	var req struct {
		DevAddr string `json:"dev_addr" validate:"required,len=8"`
		AppSKey string `json:"app_s_key" validate:"required,len=32"`
		NwkSKey string `json:"nwk_s_key" validate:"required,len=32"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.validator.Validate(req); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	device, err := s.store.GetDevice(ctx, devEUI)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "device not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	devAddr, err := parseDevAddr(req.DevAddr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid DevAddr")
		return
	}

	device.DevAddr = (*models.DevAddr)(&devAddr)
	device.AppSKey = &req.AppSKey
	device.NwkSEncKey = &req.NwkSKey
	device.SNwkSIntKey = &req.NwkSKey
	device.FNwkSIntKey = &req.NwkSKey
	device.FCntUp = 0
	device.NFCntDown = 0
	device.AFCntDown = 0

	if err := s.store.UpdateDevice(ctx, device); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "device activated successfully",
		"dev_addr": req.DevAddr,
	})
}

// HandleGetDeviceKeys gets device keys
func (s *RESTServer) HandleGetDeviceKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	keys, err := s.store.GetDeviceKeys(ctx, devEUI)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "keys not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"appKey": keys.AppKey,
		"nwkKey": keys.NwkKey,
	})
}

// HandleSetDeviceKeys sets device keys
func (s *RESTServer) HandleSetDeviceKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	var req struct {
		AppKey string `json:"app_key"`
		NwkKey string `json:"nwk_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	keys := &models.DeviceKeys{
		DevEUI: models.EUI64(devEUI),
		AppKey: req.AppKey,
		NwkKey: req.NwkKey,
	}

	if err := s.store.SetDeviceKeys(ctx, keys); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListDeviceDownlinks lists pending downlinks
func (s *RESTServer) HandleListDeviceDownlinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	frames, err := s.store.GetPendingDownlinks(ctx, devEUI)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to API response
	response := make([]map[string]interface{}, len(frames))
	for i, frame := range frames {
		response[i] = map[string]interface{}{
			"id":             frame.ID,
			"fPort":          frame.FPort,
			"data":           hex.EncodeToString(frame.Data),
			"confirmed":      frame.Confirmed,
			"isPending":      frame.IsPending,
			"retryCount":     frame.RetryCount,
			"reference":      frame.Reference,
			"createdAt":      frame.CreatedAt,
			"transmittedAt":  frame.TransmittedAt,
			"acknowledgedAt": frame.AckedAt,
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"downlinks": response,
		"total":     len(response),
	})
}

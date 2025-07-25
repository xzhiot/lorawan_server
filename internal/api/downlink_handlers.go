package api

import (
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage" // Add this import
)

// HandleSendDownlink sends downlink data
func (s *RESTServer) HandleSendDownlink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	var req struct {
		FPort     uint8  `json:"fPort" validate:"required,min=1,max=223"`
		Data      string `json:"data" validate:"required"` // hex encoded
		Confirmed bool   `json:"confirmed"`
		Reference string `json:"reference,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.validator.Validate(req); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Decode data
	data, err := hex.DecodeString(req.Data)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid hex data")
		return
	}

	// Check data length
	if len(data) > 242 {
		s.respondError(w, http.StatusBadRequest, "data too large (max 242 bytes)")
		return
	}

	// Get device info
	device, err := s.store.GetDevice(ctx, devEUI)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "device not found")
		return
	}

	// Create downlink frame
	frame := &models.DownlinkFrame{
		DevEUI:        device.DevEUI,
		ApplicationID: device.ApplicationID,
		FPort:         int(req.FPort),
		Data:          data,
		Confirmed:     req.Confirmed,
		Reference:     req.Reference,
	}

	if err := s.store.CreateDownlinkFrame(ctx, frame); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to queue downlink")
		return
	}

	// Log event
	event := &models.EventLog{
		ApplicationID: &device.ApplicationID,
		DevEUI:        &device.DevEUI,
		Type:          models.EventTypeDownlinkQueued,
		Level:         models.EventLevelInfo,
		Description:   "Downlink queued",
		Details: models.Variables{
			"id":        frame.ID,
			"fPort":     req.FPort,
			"dataSize":  len(data),
			"confirmed": req.Confirmed,
			"reference": req.Reference,
		},
	}
	s.store.CreateEventLog(ctx, event)

	s.respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":      frame.ID,
		"message": "Downlink queued successfully",
		"status":  "pending",
	})
}

// HandleListDownlinks lists pending downlinks
func (s *RESTServer) HandleListDownlinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	// Get pending downlinks
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

// Replace the existing HandleCancelDownlink function in downlink_handlers.go

// HandleCancelDownlink cancels a downlink
func (s *RESTServer) HandleCancelDownlink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	downlinkIDStr := chi.URLParam(r, "id")
	downlinkID, err := uuid.Parse(downlinkIDStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid downlink_id")
		return
	}

	if err := s.store.DeleteDownlinkFrame(ctx, downlinkID); err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "downlink not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
func (s *RESTServer) HandleGetDeviceData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	// Get uplink history
	frames, total, err := s.store.ListUplinkFrames(ctx, devEUI, limit, offset)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to API response
	response := make([]map[string]interface{}, len(frames))
	for i, frame := range frames {
		response[i] = map[string]interface{}{
			"id":         frame.ID,
			"fCnt":       frame.FCnt,
			"fPort":      frame.FPort,
			"data":       hex.EncodeToString(frame.Data),
			"dr":         frame.DR,
			"adr":        frame.ADR,
			"rssi":       frame.GetRSSI(),
			"snr":        frame.GetSNR(),
			"confirmed":  frame.Confirmed,
			"receivedAt": frame.ReceivedAt,
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  response,
		"total": total,
	})
}

// Replace the existing HandleExportDeviceData function:
// HandleExportDeviceData exports device data
func (s *RESTServer) HandleExportDeviceData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	devEUIStr := chi.URLParam(r, "dev_eui")
	devEUI, err := parseEUI64(devEUIStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid dev_eui")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// Get all data (simplified, should support time range)
	frames, _, err := s.store.ListUplinkFrames(ctx, devEUI, 1000, 0)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"device_%s_data.csv\"", devEUIStr))

		// Use CSV writer
		writer := csv.NewWriter(w)
		defer writer.Flush()

		// Write CSV header
		header := []string{
			"Timestamp",
			"Frame Counter",
			"Port",
			"Data (Hex)",
			"RSSI (dBm)",
			"SNR (dB)",
			"Data Rate",
			"ADR",
			"Confirmed",
		}

		if err := writer.Write(header); err != nil {
			return
		}

		// Write data rows
		for _, frame := range frames {
			dataHex := ""
			if frame.Data != nil {
				dataHex = hex.EncodeToString(frame.Data)
			}

			fPort := ""
			if frame.FPort != nil {
				fPort = strconv.Itoa(int(*frame.FPort))
			}

			row := []string{
				frame.ReceivedAt.Format(time.RFC3339),
				strconv.FormatUint(uint64(frame.FCnt), 10),
				fPort,
				dataHex,
				fmt.Sprintf("%.1f", frame.GetRSSI()),
				fmt.Sprintf("%.1f", frame.GetSNR()),
				fmt.Sprintf("DR%d", frame.DR),
				strconv.FormatBool(frame.ADR),
				strconv.FormatBool(frame.Confirmed),
			}

			if err := writer.Write(row); err != nil {
				return
			}
		}

	case "json":
		fallthrough
	default:
		data := make([]map[string]interface{}, len(frames))
		for i, frame := range frames {
			data[i] = map[string]interface{}{
				"timestamp": frame.ReceivedAt,
				"fcnt":      frame.FCnt,
				"fport":     frame.FPort,
				"data":      hex.EncodeToString(frame.Data),
				"rssi":      frame.GetRSSI(),
				"snr":       frame.GetSNR(),
				"dr":        frame.DR,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"device_%s_data.json\"", devEUIStr))
		json.NewEncoder(w).Encode(data)
	}
}

// ptrToInt helper function
func ptrToInt(p *uint8) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

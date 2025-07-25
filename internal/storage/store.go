package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// Common errors
var (
	ErrNotFound     = errors.New("not found")
	ErrDuplicateKey = errors.New("duplicate key")
	ErrInvalidData  = errors.New("invalid data")
)

// Store defines the storage interface
type Store interface {
	// Transaction support
	BeginTx(ctx context.Context) (Store, error)
	Commit() error
	Rollback() error

	// User methods
	CreateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
	ListUsers(ctx context.Context, tenantID *uuid.UUID, limit, offset int) ([]*models.User, int64, error)

	// Tenant methods
	CreateTenant(ctx context.Context, tenant *models.Tenant) error
	GetTenant(ctx context.Context, id uuid.UUID) (*models.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *models.Tenant) error
	DeleteTenant(ctx context.Context, id uuid.UUID) error
	ListTenants(ctx context.Context, limit, offset int) ([]*models.Tenant, int64, error)

	// Application methods
	CreateApplication(ctx context.Context, app *models.Application) error
	GetApplication(ctx context.Context, id uuid.UUID) (*models.Application, error)
	UpdateApplication(ctx context.Context, app *models.Application) error
	DeleteApplication(ctx context.Context, id uuid.UUID) error
	ListApplications(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.Application, int64, error)

	// Device methods
	CreateDevice(ctx context.Context, device *models.Device) error
	GetDevice(ctx context.Context, devEUI lorawan.EUI64) (*models.Device, error)
	GetDeviceByDevAddr(ctx context.Context, devAddr lorawan.DevAddr) ([]*models.Device, error)
	UpdateDevice(ctx context.Context, device *models.Device) error
	DeleteDevice(ctx context.Context, devEUI lorawan.EUI64) error
	ListDevices(ctx context.Context, applicationID uuid.UUID, limit, offset int) ([]*models.Device, int64, error)

	// Device keys methods
	SetDeviceKeys(ctx context.Context, keys *models.DeviceKeys) error
	GetDeviceKeys(ctx context.Context, devEUI lorawan.EUI64) (*models.DeviceKeys, error)
	DeleteDeviceKeys(ctx context.Context, devEUI lorawan.EUI64) error

	// Device session methods
	GetDeviceSession(ctx context.Context, devEUI lorawan.EUI64) (*models.DeviceSession, error)
	SaveDeviceSession(ctx context.Context, session *models.DeviceSession) error
	DeleteDeviceSession(ctx context.Context, devEUI lorawan.EUI64) error
	GetDeviceSessionByDevAddr(ctx context.Context, devAddr lorawan.DevAddr) ([]*models.DeviceSession, error)

	// Gateway methods
	CreateGateway(ctx context.Context, gateway *models.Gateway) error
	GetGateway(ctx context.Context, gatewayID lorawan.EUI64) (*models.Gateway, error)
	UpdateGateway(ctx context.Context, gateway *models.Gateway) error
	DeleteGateway(ctx context.Context, gatewayID lorawan.EUI64) error
	ListGateways(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.Gateway, int64, error)

	// Device profile methods
	CreateDeviceProfile(ctx context.Context, profile *models.DeviceProfile) error
	GetDeviceProfile(ctx context.Context, id uuid.UUID) (*models.DeviceProfile, error)
	UpdateDeviceProfile(ctx context.Context, profile *models.DeviceProfile) error
	DeleteDeviceProfile(ctx context.Context, id uuid.UUID) error
	ListDeviceProfiles(ctx context.Context, tenantID *uuid.UUID, limit, offset int) ([]*models.DeviceProfile, int64, error)

	// Frame methods
	CreateUplinkFrame(ctx context.Context, frame *models.UplinkFrame) error
	ListUplinkFrames(ctx context.Context, devEUI lorawan.EUI64, limit, offset int) ([]*models.UplinkFrame, int64, error)

	CreateDownlinkFrame(ctx context.Context, frame *models.DownlinkFrame) error
	GetPendingDownlinks(ctx context.Context, devEUI lorawan.EUI64) ([]*models.DownlinkFrame, error)
	UpdateDownlinkFrame(ctx context.Context, frame *models.DownlinkFrame) error
	DeleteDownlinkFrame(ctx context.Context, id uuid.UUID) error // Add this line
	// Event log methods
	CreateEventLog(ctx context.Context, event *models.EventLog) error
	ListEventLogs(ctx context.Context, filters EventLogFilters, limit, offset int) ([]*models.EventLog, int64, error)
	// 上行帧相关
	SaveUplinkFrame(ctx context.Context, frame *models.UplinkFrame) error
	GetLastGatewayForDevice(ctx context.Context, devEUI lorawan.EUI64) (string, error)

	// Close the store
	Close() error
}

// EventLogFilters represents filters for event logs
type EventLogFilters struct {
	TenantID      *uuid.UUID
	ApplicationID *uuid.UUID
	DevEUI        *lorawan.EUI64
	GatewayID     *lorawan.EUI64
	Type          *models.EventType
	Level         *models.EventLevel
	StartTime     *time.Time
	EndTime       *time.Time
}

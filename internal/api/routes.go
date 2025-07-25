package api

import (
	"github.com/go-chi/chi/v5"
)

// setupAPIRoutes sets up API v1 routes
func (s *RESTServer) setupAPIRoutes(r chi.Router) {
	// Health check
	r.Get("/health", s.HandleHealth)
	r.Get("/", s.HandleRoot)

	// Auth routes (public)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", s.HandleLogin)
		r.Post("/refresh", s.HandleRefresh)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		// Users
		r.Route("/users", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListUsers)
			r.Post("/", s.HandleCreateUser)
			r.Get("/me", s.HandleGetCurrentUser)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.HandleGetUser)
				r.Put("/", s.HandleUpdateUser)
				r.Delete("/", s.HandleDeleteUser)
			})
		})

		// Tenants
		r.Route("/tenants", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListTenants)
			r.Post("/", s.HandleCreateTenant)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.HandleGetTenant)
				r.Put("/", s.HandleUpdateTenant)
				r.Delete("/", s.HandleDeleteTenant)
			})
		})

		// Applications
		r.Route("/applications", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListApplications)
			r.Post("/", s.HandleCreateApplication)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.HandleGetApplication)
				r.Put("/", s.HandleUpdateApplication)
				r.Delete("/", s.HandleDeleteApplication)
				// Integration endpoints - 新增
				r.Route("/integrations", func(r chi.Router) {
					r.Get("/", s.HandleGetIntegrations)
					r.Put("/http", s.HandleUpdateHTTPIntegration)
					r.Put("/mqtt", s.HandleUpdateMQTTIntegration)
					r.Post("/test", s.HandleTestIntegration)
				})
			})
		})

		// Devices
		r.Route("/devices", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListDevices)
			r.Post("/", s.HandleCreateDevice)
			r.Route("/{dev_eui}", func(r chi.Router) {
				r.Get("/", s.HandleGetDevice)
				r.Put("/", s.HandleUpdateDevice)
				r.Delete("/", s.HandleDeleteDevice)
				r.Post("/activate", s.HandleActivateDevice)
				// 添加这些路由
				r.Get("/keys", s.HandleGetDeviceKeys)
				r.Post("/keys", s.HandleSetDeviceKeys)
				// Data management
				r.Get("/data", s.HandleGetDeviceData)
				r.Get("/export", s.HandleExportDeviceData)

				// Downlink management
				r.Post("/downlink", s.HandleSendDownlink)
				r.Get("/downlink", s.HandleListDeviceDownlinks)
			})
		})

		// Downlinks
		r.Route("/downlinks", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Delete("/{id}", s.HandleCancelDownlink)
		})

		// Gateways
		r.Route("/gateways", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListGateways)
			r.Post("/", s.HandleCreateGateway)
			r.Route("/{gateway_id}", func(r chi.Router) {
				r.Get("/", s.HandleGetGateway)
				r.Put("/", s.HandleUpdateGateway)
				r.Delete("/", s.HandleDeleteGateway)
			})
		})

		// Device profiles
		r.Route("/device-profiles", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListDeviceProfiles)
			r.Post("/", s.HandleCreateDeviceProfile)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.HandleGetDeviceProfile)
				r.Put("/", s.HandleUpdateDeviceProfile)
				r.Delete("/", s.HandleDeleteDeviceProfile)
			})
		})

		// Events
		r.Route("/events", func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/", s.HandleListEvents)
		})
	})
}

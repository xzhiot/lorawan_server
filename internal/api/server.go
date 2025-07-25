package api

import (
    "context"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/rs/zerolog/log"

    "github.com/lorawan-server/lorawan-server-pro/internal/auth"
    "github.com/lorawan-server/lorawan-server-pro/internal/config"
    "github.com/lorawan-server/lorawan-server-pro/internal/storage"
    "github.com/lorawan-server/lorawan-server-pro/internal/validation"
)

// RESTServer represents the REST API server
type RESTServer struct {
    config    *config.Config
    store     storage.Store
    auth      *auth.JWTManager
    validator *validation.Validator
    router    chi.Router
    server    *http.Server
}

// NewRESTServer creates a new REST API server
func NewRESTServer(cfg *config.Config, store storage.Store) *RESTServer {
    s := &RESTServer{
        config:    cfg,
        store:     store,
        auth:      auth.NewJWTManager(&cfg.JWT),
        validator: validation.NewValidator(),
        router:    chi.NewRouter(),
    }
    
    s.setupRoutes()
    
    s.server = &http.Server{
        Handler:      s.router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    return s
}

// setupRoutes configures all routes
func (s *RESTServer) setupRoutes() {
    // Middleware
    s.router.Use(middleware.RequestID)
    s.router.Use(middleware.RealIP)
    s.router.Use(middleware.Logger)
    s.router.Use(middleware.Recoverer)
    s.router.Use(middleware.Timeout(60 * time.Second))
    
    // CORS
    s.router.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
        ExposedHeaders:   []string{"Link"},
        AllowCredentials: true,
        MaxAge:           300,
    }))
    
    // API routes
    s.router.Route("/api/v1", func(r chi.Router) {
        s.setupAPIRoutes(r)
    })
}

// ListenAndServe starts the server
func (s *RESTServer) ListenAndServe(addr string) error {
    s.server.Addr = addr
    
    // 挂载静态文件服务 (Web UI)
    webDir := s.config.Web.StaticDir
    if envWebDir := os.Getenv("WEB_DIR"); envWebDir != "" {
        webDir = envWebDir
    }
    
    // 检查 web 目录是否存在
    if _, err := os.Stat(webDir); os.IsNotExist(err) {
        log.Warn().Str("dir", webDir).Msg("Web directory not found, Web UI will not be available")
    } else {
        log.Info().Str("dir", webDir).Msg("Serving Web UI from directory")
        
        // 为所有非 API 路径提供静态文件
        s.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // API 路径由 chi 路由处理
            if strings.HasPrefix(r.URL.Path, "/api/") {
                s.router.ServeHTTP(w, r)
                return
            }
            
            // 其他路径提供静态文件
            fs := http.FileServer(http.Dir(webDir))
            
            // 如果是根路径或没有扩展名的路径，尝试提供 index.html
            if r.URL.Path == "/" || (!strings.Contains(r.URL.Path, ".") && r.URL.Path != "/") {
                http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
                return
            }
            
            fs.ServeHTTP(w, r)
        })
    }
    
    log.Info().Str("addr", addr).Msg("Starting REST API server")
    return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *RESTServer) Shutdown(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}

// authMiddleware is the authentication middleware
func (s *RESTServer) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get token from header
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            s.respondError(w, http.StatusUnauthorized, "missing authorization header")
            return
        }
        
        // Parse Bearer token
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            s.respondError(w, http.StatusUnauthorized, "invalid authorization header")
            return
        }
        
        // Validate token
        claims, err := s.auth.ValidateToken(parts[1])
        if err != nil {
            s.respondError(w, http.StatusUnauthorized, "invalid token")
            return
        }
        
        // Add claims to context
        ctx := context.WithValue(r.Context(), "claims", claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

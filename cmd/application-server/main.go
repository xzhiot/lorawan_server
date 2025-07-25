package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"

    "github.com/lorawan-server/lorawan-server-pro/internal/api"
    "github.com/lorawan-server/lorawan-server-pro/internal/config"
    "github.com/lorawan-server/lorawan-server-pro/internal/server"
    "github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

func main() {
    // Command line flags
    var configFile string
    flag.StringVar(&configFile, "config", "config/application-server.yml", "Configuration file path")
    flag.Parse()

    // Setup logging
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    zerolog.SetGlobalLevel(zerolog.InfoLevel)

    // Load configuration
    cfg, err := config.Load(configFile)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to load configuration")
    }

    // Set log level
    level, err := zerolog.ParseLevel(cfg.Log.Level)
    if err != nil {
        level = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(level)

    // Connect to database
    store, err := storage.NewPostgresStore(cfg.Database.DSN)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to connect to database")
    }
    defer store.Close()

    log.Info().Msg("Connected to database")

    // Create context
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start REST API server
    apiServer := api.NewRESTServer(cfg, store)

    // WaitGroup for services
    var wg sync.WaitGroup

    // Start API server
    wg.Add(1)
    go func() {
        defer wg.Done()
        addr := fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port)
        log.Info().Str("addr", addr).Msg("Starting REST API server")
        if err := apiServer.ListenAndServe(addr); err != nil {
            log.Fatal().Err(err).Msg("REST API server failed")
        }
    }()

    // Optional: Start NATS subscriber
    if cfg.NATS.URL != "" {
        log.Info().Str("url", cfg.NATS.URL).Msg("Connecting to NATS...")
        
        nc, err := nats.Connect(cfg.NATS.URL,
            nats.Name("lorawan-application-server"),
            nats.UserInfo(cfg.NATS.Username, cfg.NATS.Password),
            nats.ReconnectWait(cfg.NATS.ReconnectInterval),
            nats.MaxReconnects(cfg.NATS.MaxReconnects),
            nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
                log.Warn().Err(err).Msg("Disconnected from NATS")
            }),
            nats.ReconnectHandler(func(nc *nats.Conn) {
                log.Info().Msg("Reconnected to NATS")
            }),
            nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
                log.Error().
                    Err(err).
                    Str("subject", sub.Subject).
                    Msg("NATS error")
            }),
        )
        
        if err != nil {
            log.Warn().Err(err).Msg("Failed to connect to NATS, continuing without NATS support")
        } else {
            defer nc.Close()
            log.Info().Msg("Connected to NATS")

            // Start NATS subscriber
            subscriber := server.NewNATSSubscriber(nc, store)
            
            wg.Add(1)
            go func() {
                defer wg.Done()
                log.Info().Msg("Starting NATS subscriber")
                if err := subscriber.Start(ctx); err != nil {
                    log.Error().Err(err).Msg("NATS subscriber stopped")
                }
            }()
        }
    } else {
        log.Info().Msg("NATS not configured, running in standalone mode")
    }

    // Wait for signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    sig := <-sigChan
    log.Info().Str("signal", sig.String()).Msg("Received signal, shutting down")

    // Cancel context
    cancel()

    // Shutdown API server
    if err := apiServer.Shutdown(context.Background()); err != nil {
        log.Error().Err(err).Msg("Failed to shutdown API server gracefully")
    }

    // Wait for all services
    wg.Wait()

    log.Info().Msg("Application server stopped")
}

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/lorawan-server/lorawan-server-pro/internal/config"
	"github.com/lorawan-server/lorawan-server-pro/internal/gateway"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

func main() {
	// 命令行参数
	var configFile string
	flag.StringVar(&configFile, "config", "config/gateway-bridge.yml", "配置文件路径")
	flag.Parse()

	// 设置日志
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// 加载配置
	cfg, err := config.Load(configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("加载配置失败")
	}

	// 设置日志级别
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().Msg("LoRaWAN Gateway Bridge 启动中...")

	// 连接数据库
	store, err := storage.NewPostgresStore(cfg.Database.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("连接数据库失败")
	}
	defer store.Close()

	log.Info().Msg("已连接到数据库")

	// 连接 NATS
	nc, err := nats.Connect(cfg.NATS.URL,
		nats.Name("lorawan-gateway-bridge"),
		nats.UserInfo(cfg.NATS.Username, cfg.NATS.Password),
		nats.ReconnectWait(cfg.NATS.ReconnectInterval),
		nats.MaxReconnects(cfg.NATS.MaxReconnects),
	)

	if err != nil {
		log.Fatal().Err(err).Msg("连接 NATS 失败")
	}
	defer nc.Close()

	log.Info().Msg("已连接到 NATS")

	// 创建 UDP 包转发器 - 传递存储接口
	forwarder, err := gateway.NewUDPPacketForwarder(cfg.Gateway.UDPBind, nc, store)
	if err != nil {
		log.Fatal().Err(err).Msg("创建 UDP 转发器失败")
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务
	go func() {
		if err := forwarder.Start(ctx); err != nil {
			log.Error().Err(err).Msg("UDP 转发器停止")
		}
	}()

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Info().Str("signal", sig.String()).Msg("收到信号，正在关闭...")

	// 取消上下文
	cancel()

	log.Info().Msg("Gateway Bridge 已停止")
}

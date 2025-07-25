package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/lorawan-server/lorawan-server-pro/internal/config"
	"github.com/lorawan-server/lorawan-server-pro/internal/network"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

func main() {
	// 命令行参数
	var configPath = flag.String("config", "config/network-server.yml", "配置文件路径")
	var validateOnly = flag.Bool("validate", false, "仅验证配置文件")
	var showConfig = flag.Bool("show-config", false, "显示配置并退出")
	flag.Parse()

	// 设置日志
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Str("config_path", *configPath).Msg("加载配置失败")
	}

	// 设置日志级别
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		log.Warn().Str("level", cfg.Log.Level).Msg("无效的日志级别，使用info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// 如果只是显示配置，打印后退出
	if *showConfig {
		cfg.PrintConfigSummary()
		return
	}

	// 如果只是验证配置，打印摘要后退出
	if *validateOnly {
		cfg.PrintConfigSummary()
		fmt.Println("✅ 配置文件验证通过")

		// 验证CN470硬件兼容性
		if err := validateHardwareCompatibility(cfg); err != nil {
			log.Fatal().Err(err).Msg("硬件兼容性检查失败")
		}

		fmt.Println("✅ 硬件兼容性检查通过")
		return
	}

	log.Info().
		Str("config_path", *configPath).
		Str("cn470_mode", cfg.CN470.GetCN470Mode()).
		Msg("Network Server 启动")

	// 验证CN470硬件兼容性
	if err := validateHardwareCompatibility(cfg); err != nil {
		log.Fatal().Err(err).Msg("硬件兼容性检查失败")
	}

	// 连接数据库
	store, err := storage.NewPostgresStore(cfg.Database.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("连接数据库失败")
	}
	defer store.Close()

	// 连接NATS
	nc, err := nats.Connect(cfg.NATS.URL,
		nats.ReconnectWait(cfg.NATS.ReconnectInterval),
		nats.MaxReconnects(cfg.NATS.MaxReconnects))
	if err != nil {
		log.Fatal().Err(err).Msg("连接NATS失败")
	}
	defer nc.Close()

	// 创建处理器
	processor := network.NewProcessor(nc, store, cfg)

	// 启动处理器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动处理器协程
	go func() {
		if err := processor.Start(ctx); err != nil {
			log.Error().Err(err).Msg("处理器启动失败")
			cancel()
		}
	}()

	// 等待退出信号
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("收到退出信号，正在关闭...")
	case <-ctx.Done():
		log.Info().Msg("上下文取消，正在关闭...")
	}

	cancel()
	log.Info().Msg("Network Server 已关闭")
}

// validateHardwareCompatibility 验证硬件兼容性
func validateHardwareCompatibility(cfg *config.Config) error {
	if cfg.Network.Band != "CN470" {
		return nil // 只验证CN470
	}

	mode := cfg.CN470.GetCN470Mode()
	originalMode := mode

	switch mode {
	case "STANDARD_FDD":
		if !cfg.CN470.Hardware.SupportsTX500MHz {
			log.Warn().Msg("硬件不支持500MHz，自动切换到自定义FDD模式")
			cfg.CN470.Mode = "CUSTOM_FDD"
			mode = "CUSTOM_FDD"
		}

	case "CUSTOM_FDD":
		if !cfg.CN470.Hardware.SupportsTX470_490MHz {
			log.Warn().Msg("硬件不支持470-490MHz，自动切换到TDD模式")
			cfg.CN470.Mode = "TDD"
			mode = "TDD"
		}

	case "TDD":
		if !cfg.CN470.Hardware.SupportsTX470_490MHz {
			return fmt.Errorf("硬件不支持470-490MHz，无法运行任何CN470模式")
		}
	}

	// 验证频率范围
	if err := validateFrequencyConfiguration(cfg); err != nil {
		return fmt.Errorf("频率配置验证失败: %w", err)
	}

	// 如果模式发生了变化，重新打印配置摘要
	if originalMode != mode {
		log.Info().
			Str("original_mode", originalMode).
			Str("final_mode", mode).
			Msg("CN470模式已自动调整")

		// 重新设置默认值
		//cfg.validateAndSetCN470Defaults()
	}

	log.Info().
		Str("final_mode", cfg.CN470.GetCN470Mode()).
		Bool("supports_500mhz", cfg.CN470.Hardware.SupportsTX500MHz).
		Bool("supports_470_490mhz", cfg.CN470.Hardware.SupportsTX470_490MHz).
		Msg("硬件兼容性检查通过")

	return nil
}

// validateFrequencyConfiguration 验证频率配置
func validateFrequencyConfiguration(cfg *config.Config) error {
	if cfg.Network.Band != "CN470" {
		return nil
	}

	mode := cfg.CN470.GetCN470Mode()

	switch mode {
	case "STANDARD_FDD":
		// 验证500MHz支持
		if cfg.CN470.StandardFDD.DownlinkStartFreq < 500000000 ||
			cfg.CN470.StandardFDD.DownlinkEndFreq > 510000000 {
			return fmt.Errorf("标准FDD下行频率超出500-510MHz范围")
		}

	case "CUSTOM_FDD":
		// 验证470-490MHz范围
		if cfg.CN470.CustomFDD.UplinkEndFreq > 490000000 ||
			cfg.CN470.CustomFDD.DownlinkEndFreq > 490000000 {
			return fmt.Errorf("自定义FDD频率超出470-490MHz范围")
		}

		if cfg.CN470.CustomFDD.UplinkStartFreq < 470000000 ||
			cfg.CN470.CustomFDD.DownlinkStartFreq < 470000000 {
			return fmt.Errorf("自定义FDD频率低于470MHz下限")
		}

	case "TDD":
		// 验证TDD频率范围
		if cfg.CN470.TDD.StartFreq < 470000000 || cfg.CN470.TDD.EndFreq > 490000000 {
			return fmt.Errorf("TDD频率超出470-490MHz范围")
		}
	}

	return nil
}

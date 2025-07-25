package config

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/rs/zerolog/log" // 添加这行

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	API      APIConfig      `yaml:"api"`
	Web      WebConfig      `yaml:"web"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	NATS     NATSConfig     `yaml:"nats"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
	Network  NetworkConfig  `yaml:"network"`
	Gateway  GatewayConfig  `yaml:"gateway"`
	CN470    CN470Config    `yaml:"cn470"` // 新增CN470配置
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// APIConfig represents API configuration
type APIConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// WebConfig represents web UI configuration
type WebConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	StaticDir string `yaml:"static_dir"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// NATSConfig represents NATS configuration
type NATSConfig struct {
	URL               string        `yaml:"url"`
	ClusterID         string        `yaml:"cluster_id"`
	ClientID          string        `yaml:"client_id"`
	Username          string        `yaml:"username"`
	Password          string        `yaml:"password"`
	MaxReconnects     int           `yaml:"max_reconnects"`
	ReconnectInterval time.Duration `yaml:"reconnect_interval"`
}

// JWTConfig represents JWT configuration
type JWTConfig struct {
	Secret          string        `yaml:"secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// NetworkConfig represents network server configuration
type NetworkConfig struct {
	NetID               string        `yaml:"net_id"`
	DeduplicationWindow time.Duration `yaml:"deduplication_window"`
	DeviceSessionTTL    time.Duration `yaml:"device_session_ttl"`
	Band                string        `yaml:"band"`
	ADREnabled          bool          `yaml:"adr_enabled"`
}

// GatewayConfig represents gateway bridge configuration
type GatewayConfig struct {
	UDPBind       string        `yaml:"udp_bind"`
	StatsInterval time.Duration `yaml:"stats_interval"`
	PingInterval  time.Duration `yaml:"ping_interval"`
	PushTimeout   time.Duration `yaml:"push_timeout"`
}

// === 新增CN470相关配置结构 ===

// CN470Config CN470频段配置
type CN470Config struct {
	Mode        string              `yaml:"mode"` // STANDARD_FDD | CUSTOM_FDD | TDD
	Hardware    CN470HardwareConfig `yaml:"hardware"`
	StandardFDD CN470StandardFDD    `yaml:"standard_fdd"`
	CustomFDD   CN470CustomFDD      `yaml:"custom_fdd"`
	TDD         CN470TDD            `yaml:"tdd"`
	RXWindows   CN470RXWindows      `yaml:"rx_windows"`
	Channels    CN470Channels       `yaml:"channels"`
	ADR         CN470ADR            `yaml:"adr"`
	MAC         CN470MAC            `yaml:"mac"`
}

// CN470HardwareConfig 硬件能力配置
type CN470HardwareConfig struct {
	SupportsTX500MHz     bool `yaml:"supports_tx_500mhz"`
	SupportsTX470_490MHz bool `yaml:"supports_tx_470_490mhz"`
	TXPowerDBm           int  `yaml:"tx_power_dbm"`
	RXSensitivityDBm     int  `yaml:"rx_sensitivity_dbm"`
}

// CN470StandardFDD 标准FDD配置
type CN470StandardFDD struct {
	UplinkStartFreq   uint32 `yaml:"uplink_start_freq"`
	UplinkEndFreq     uint32 `yaml:"uplink_end_freq"`
	DownlinkStartFreq uint32 `yaml:"downlink_start_freq"`
	DownlinkEndFreq   uint32 `yaml:"downlink_end_freq"`
	FrequencyOffset   uint32 `yaml:"frequency_offset"`
	UplinkChannels    int    `yaml:"uplink_channels"`
	DownlinkChannels  int    `yaml:"downlink_channels"`
}

// CN470CustomFDD 自定义FDD配置
type CN470CustomFDD struct {
	UplinkStartFreq   uint32 `yaml:"uplink_start_freq"`
	UplinkEndFreq     uint32 `yaml:"uplink_end_freq"`
	DownlinkStartFreq uint32 `yaml:"downlink_start_freq"`
	DownlinkEndFreq   uint32 `yaml:"downlink_end_freq"`
	FrequencyOffset   uint32 `yaml:"frequency_offset"`
	UplinkChannels    int    `yaml:"uplink_channels"`
	DownlinkChannels  int    `yaml:"downlink_channels"`
}

// CN470TDD TDD配置
type CN470TDD struct {
	StartFreq      uint32 `yaml:"start_freq"`
	EndFreq        uint32 `yaml:"end_freq"`
	Channels       int    `yaml:"channels"`
	TXRXSwitchTime int    `yaml:"tx_rx_switch_time"` // 微秒
}

// CN470RXWindows RX窗口配置
type CN470RXWindows struct {
	RX1Delay         int    `yaml:"rx1_delay"`          // RX1延迟（秒）
	RX2Delay         int    `yaml:"rx2_delay"`          // RX2延迟（秒）
	JoinAcceptDelay1 int    `yaml:"join_accept_delay1"` // JOIN ACCEPT RX1延迟（秒）
	JoinAcceptDelay2 int    `yaml:"join_accept_delay2"` // JOIN ACCEPT RX2延迟（秒）
	RX2Frequency     uint32 `yaml:"rx2_frequency"`      // RX2频率
	RX2DataRate      int    `yaml:"rx2_data_rate"`      // RX2数据速率
	RX1DROffset      int    `yaml:"rx1_dr_offset"`      // RX1数据速率偏移
}

// CN470Channels 信道配置
type CN470Channels struct {
	DefaultChannels      int    `yaml:"default_channels"`
	MaxChannelsPerDevice int    `yaml:"max_channels_per_device"`
	ChannelSpacing       uint32 `yaml:"channel_spacing"`
	ChannelHopping       bool   `yaml:"channel_hopping"`
	EnabledSubBands      []int  `yaml:"enabled_sub_bands"`
}

// CN470ADR ADR配置
type CN470ADR struct {
	Enabled     bool `yaml:"enabled"`
	TargetSNR   int  `yaml:"target_snr"`
	MarginSNR   int  `yaml:"margin_snr"`
	HistorySize int  `yaml:"history_size"`
	MinDataRate int  `yaml:"min_data_rate"`
	MaxDataRate int  `yaml:"max_data_rate"`
	MinTXPower  int  `yaml:"min_tx_power"`
	MaxTXPower  int  `yaml:"max_tx_power"`
}

// CN470MAC MAC命令配置
type CN470MAC struct {
	PingSlotPeriodicity int  `yaml:"ping_slot_periodicity"`
	MaxDutyCycle        int  `yaml:"max_duty_cycle"`
	SupportsJoin        bool `yaml:"supports_join"`
	SupportsClassB      bool `yaml:"supports_class_b"`
	SupportsClassC      bool `yaml:"supports_class_c"`
}

// === CN470配置方法 ===

// GetCN470Mode 获取CN470工作模式
func (c *CN470Config) GetCN470Mode() string {
	if c.Mode == "" {
		return "CUSTOM_FDD" // 默认使用自定义FDD
	}
	return c.Mode
}

// IsCustomFDD 检查是否为自定义FDD模式
func (c *CN470Config) IsCustomFDD() bool {
	return c.GetCN470Mode() == "CUSTOM_FDD"
}

// IsStandardFDD 检查是否为标准FDD模式
func (c *CN470Config) IsStandardFDD() bool {
	return c.GetCN470Mode() == "STANDARD_FDD"
}

// IsTDD 检查是否为TDD模式
func (c *CN470Config) IsTDD() bool {
	return c.GetCN470Mode() == "TDD"
}

// GetDownlinkFrequency 根据配置模式计算下行频率
func (c *CN470Config) GetDownlinkFrequency(uplinkFreq uint32) uint32 {
	switch c.GetCN470Mode() {
	case "STANDARD_FDD":
		downlinkFreq := uplinkFreq + c.StandardFDD.FrequencyOffset
		if downlinkFreq >= c.StandardFDD.DownlinkStartFreq &&
			downlinkFreq <= c.StandardFDD.DownlinkEndFreq {
			return downlinkFreq
		}
		return c.RXWindows.RX2Frequency

	case "CUSTOM_FDD":
		downlinkFreq := uplinkFreq + c.CustomFDD.FrequencyOffset
		if downlinkFreq >= c.CustomFDD.DownlinkStartFreq &&
			downlinkFreq <= c.CustomFDD.DownlinkEndFreq {
			return downlinkFreq
		}
		return c.RXWindows.RX2Frequency

	case "TDD":
		if uplinkFreq >= c.TDD.StartFreq && uplinkFreq <= c.TDD.EndFreq {
			return uplinkFreq
		}
		return c.RXWindows.RX2Frequency

	default:
		return c.RXWindows.RX2Frequency
	}
}

// ValidateFrequency 验证频率是否在硬件支持范围内
func (c *CN470Config) ValidateFrequency(freq uint32) bool {
	if c.Hardware.SupportsTX500MHz {
		// 支持500MHz，可以使用标准FDD
		return (freq >= 470000000 && freq <= 490000000) ||
			(freq >= 500000000 && freq <= 510000000)
	}

	if c.Hardware.SupportsTX470_490MHz {
		// 只支持470-490MHz
		return freq >= 470000000 && freq <= 490000000
	}

	return false
}

// Load loads configuration from file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Apply environment overrides
	cfg.applyEnvOverrides()

	// 验证和设置CN470默认值
	if err := cfg.validateAndSetCN470Defaults(); err != nil {
		return nil, fmt.Errorf("CN470 config validation failed: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		c.Database.DSN = dsn
	}

	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		c.Redis.Addr = redisAddr
	}

	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		c.NATS.URL = natsURL
	}

	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		c.JWT.Secret = jwtSecret
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		c.Log.Level = logLevel
	}

	// CN470环境变量覆盖
	if cn470Mode := os.Getenv("CN470_MODE"); cn470Mode != "" {
		c.CN470.Mode = cn470Mode
	}
}

// validateAndSetCN470Defaults 验证CN470配置并设置默认值
func (c *Config) validateAndSetCN470Defaults() error {
	// 只在使用CN470频段时验证
	if c.Network.Band != "CN470" {
		return nil
	}

	// 设置CN470默认值
	if c.CN470.Mode == "" {
		c.CN470.Mode = "CUSTOM_FDD" // 默认使用自定义FDD
	}

	// 验证CN470模式
	switch c.CN470.Mode {
	case "STANDARD_FDD", "CUSTOM_FDD", "TDD":
		// 有效模式
	default:
		return fmt.Errorf("invalid CN470 mode: %s", c.CN470.Mode)
	}

	// 根据硬件能力自动调整模式
	if c.CN470.Mode == "STANDARD_FDD" && !c.CN470.Hardware.SupportsTX500MHz {
		fmt.Printf("Warning: Hardware doesn't support 500MHz, switching to CUSTOM_FDD\n")
		c.CN470.Mode = "CUSTOM_FDD"
	}

	// 设置默认硬件配置（针对SX1302）
	if c.CN470.Hardware.TXPowerDBm == 0 {
		c.CN470.Hardware.TXPowerDBm = 19 // SX1302默认19dBm
	}
	if !c.CN470.Hardware.SupportsTX500MHz && !c.CN470.Hardware.SupportsTX470_490MHz {
		// 如果没有明确配置，根据常见硬件设置默认值
		c.CN470.Hardware.SupportsTX470_490MHz = true // SX1302支持470-490MHz
		c.CN470.Hardware.SupportsTX500MHz = false    // SX1302不支持500MHz
	}

	// 设置默认频率配置
	if err := c.setDefaultFrequencyRanges(); err != nil {
		return err
	}

	// 设置默认RX窗口配置
	c.setDefaultRXWindows()

	// 设置默认信道配置
	c.setDefaultChannels()

	// 设置默认ADR配置
	c.setDefaultADR()
	// 验证信道数量和间隔的合理性
	if c.CN470.Mode == "CUSTOM_FDD" {
		bandwidth := c.CN470.CustomFDD.UplinkEndFreq - c.CN470.CustomFDD.UplinkStartFreq
		maxChannels := int(bandwidth / c.CN470.Channels.ChannelSpacing)

		if c.CN470.CustomFDD.UplinkChannels > maxChannels {
			return fmt.Errorf("too many channels (%d) for bandwidth %.1fMHz with spacing %.1fMHz (max: %d)",
				c.CN470.CustomFDD.UplinkChannels,
				float64(bandwidth)/1000000,
				float64(c.CN470.Channels.ChannelSpacing)/1000000,
				maxChannels)
		}
	}

	return nil
}

// setDefaultFrequencyRanges 设置默认频率范围
func (c *Config) setDefaultFrequencyRanges() error {
	switch c.CN470.Mode {
	case "STANDARD_FDD":
		if c.CN470.StandardFDD.UplinkStartFreq == 0 {
			c.CN470.StandardFDD.UplinkStartFreq = 470300000
			c.CN470.StandardFDD.UplinkEndFreq = 489300000
			c.CN470.StandardFDD.DownlinkStartFreq = 500300000
			c.CN470.StandardFDD.DownlinkEndFreq = 509700000
			c.CN470.StandardFDD.FrequencyOffset = 30000000
			c.CN470.StandardFDD.UplinkChannels = 96
			c.CN470.StandardFDD.DownlinkChannels = 48
		}

	case "CUSTOM_FDD":
		if c.CN470.CustomFDD.UplinkStartFreq == 0 {
			c.CN470.CustomFDD.UplinkStartFreq = 470300000
			c.CN470.CustomFDD.UplinkEndFreq = 479900000
			c.CN470.CustomFDD.DownlinkStartFreq = 480300000
			c.CN470.CustomFDD.DownlinkEndFreq = 489900000
			c.CN470.CustomFDD.FrequencyOffset = 10000000
			c.CN470.CustomFDD.UplinkChannels = 8
			c.CN470.CustomFDD.DownlinkChannels = 8
		}

		// 验证自定义FDD频率是否在硬件范围内
		if !c.CN470.Hardware.SupportsTX470_490MHz {
			return fmt.Errorf("hardware doesn't support 470-490MHz, cannot use CUSTOM_FDD mode")
		}

	case "TDD":
		if c.CN470.TDD.StartFreq == 0 {
			c.CN470.TDD.StartFreq = 470300000
			c.CN470.TDD.EndFreq = 489900000
			c.CN470.TDD.Channels = 96
			c.CN470.TDD.TXRXSwitchTime = 200 // 200微秒
		}
	}

	return nil
}

// setDefaultRXWindows 设置默认RX窗口配置
func (c *Config) setDefaultRXWindows() {
	if c.CN470.RXWindows.RX1Delay == 0 {
		c.CN470.RXWindows.RX1Delay = 1
	}
	if c.CN470.RXWindows.RX2Delay == 0 {
		c.CN470.RXWindows.RX2Delay = 2
	}

	// 添加 JOIN ACCEPT 延迟默认值
	if c.CN470.RXWindows.JoinAcceptDelay1 == 0 {
		c.CN470.RXWindows.JoinAcceptDelay1 = 5 // 默认5秒
	}
	if c.CN470.RXWindows.JoinAcceptDelay2 == 0 {
		c.CN470.RXWindows.JoinAcceptDelay2 = 6 // 默认6秒
	}

	if c.CN470.RXWindows.RX2Frequency == 0 {
		switch c.CN470.Mode {
		case "STANDARD_FDD":
			c.CN470.RXWindows.RX2Frequency = 505300000 // 505.3MHz
		case "CUSTOM_FDD":
			c.CN470.RXWindows.RX2Frequency = 480300000 // 480.3MHz
		case "TDD":
			c.CN470.RXWindows.RX2Frequency = 486300000 // 486.3MHz
		}
	}
}

// setDefaultChannels 设置默认信道配置
func (c *Config) setDefaultChannels() {
	// 根据模式设置不同的信道间隔
	if c.CN470.Channels.ChannelSpacing == 0 {
		switch c.CN470.Mode {
		case "CUSTOM_FDD":
			// 8个信道使用更大的间隔：9.6MHz / 8 = 1.2MHz
			c.CN470.Channels.ChannelSpacing = 1200000 // 1.2MHz
		default:
			c.CN470.Channels.ChannelSpacing = 200000 // 200kHz
		}
	}

	if c.CN470.Channels.DefaultChannels == 0 {
		c.CN470.Channels.DefaultChannels = 8 // 确保默认8个信道
	}

	if c.CN470.Channels.MaxChannelsPerDevice == 0 {
		c.CN470.Channels.MaxChannelsPerDevice = 8 // 从16改为8
	}

	if len(c.CN470.Channels.EnabledSubBands) == 0 {
		c.CN470.Channels.EnabledSubBands = []int{0} // 只启用第一个子频段
	}
}

// setDefaultADR 设置默认ADR配置
func (c *Config) setDefaultADR() {
	if c.CN470.ADR.TargetSNR == 0 {
		c.CN470.ADR.TargetSNR = 10
	}
	if c.CN470.ADR.MarginSNR == 0 {
		c.CN470.ADR.MarginSNR = 3
	}
	if c.CN470.ADR.HistorySize == 0 {
		c.CN470.ADR.HistorySize = 20
	}
	if c.CN470.ADR.MaxDataRate == 0 {
		c.CN470.ADR.MaxDataRate = 5
	}
	if c.CN470.ADR.MaxTXPower == 0 {
		c.CN470.ADR.MaxTXPower = 7
	}
	// 默认启用ADR
	c.CN470.ADR.Enabled = true
}

// PrintConfigSummary 打印配置摘要
func (c *Config) PrintConfigSummary() {
	fmt.Printf("=== LoRaWAN Server Configuration ===\n")
	fmt.Printf("Server: %s v%s\n", c.Server.Name, c.Server.Version)
	fmt.Printf("Network Band: %s\n", c.Network.Band)

	if c.Network.Band == "CN470" {
		fmt.Printf("CN470 Mode: %s\n", c.CN470.GetCN470Mode())

		switch c.CN470.GetCN470Mode() {
		case "STANDARD_FDD":
			fmt.Printf("  Uplink: %.1f-%.1f MHz (%d channels)\n",
				float64(c.CN470.StandardFDD.UplinkStartFreq)/1000000,
				float64(c.CN470.StandardFDD.UplinkEndFreq)/1000000,
				c.CN470.StandardFDD.UplinkChannels)
			fmt.Printf("  Downlink: %.1f-%.1f MHz (%d channels)\n",
				float64(c.CN470.StandardFDD.DownlinkStartFreq)/1000000,
				float64(c.CN470.StandardFDD.DownlinkEndFreq)/1000000,
				c.CN470.StandardFDD.DownlinkChannels)

		case "CUSTOM_FDD":
			fmt.Printf("  Uplink: %.1f-%.1f MHz (%d channels)\n",
				float64(c.CN470.CustomFDD.UplinkStartFreq)/1000000,
				float64(c.CN470.CustomFDD.UplinkEndFreq)/1000000,
				c.CN470.CustomFDD.UplinkChannels)
			fmt.Printf("  Downlink: %.1f-%.1f MHz (%d channels)\n",
				float64(c.CN470.CustomFDD.DownlinkStartFreq)/1000000,
				float64(c.CN470.CustomFDD.DownlinkEndFreq)/1000000,
				c.CN470.CustomFDD.DownlinkChannels)
			fmt.Printf("  Channel Spacing: %.1f MHz\n",
				float64(c.CN470.Channels.ChannelSpacing)/1000000)

			// 添加实际信道频率打印
			uplinkFreqs, downlinkFreqs := c.CN470.GetChannelFrequencies()
			fmt.Printf("  Actual Channels:\n")
			for i := 0; i < len(uplinkFreqs); i++ {
				fmt.Printf("    CH%d: UL=%.3fMHz, DL=%.3fMHz\n",
					i,
					float64(uplinkFreqs[i])/1000000,
					float64(downlinkFreqs[i])/1000000)
			}

		case "TDD":
			fmt.Printf("  TDD Range: %.1f-%.1f MHz (%d channels)\n",
				float64(c.CN470.TDD.StartFreq)/1000000,
				float64(c.CN470.TDD.EndFreq)/1000000,
				c.CN470.TDD.Channels)
		}

		fmt.Printf("RX2 Frequency: %.1f MHz\n",
			float64(c.CN470.RXWindows.RX2Frequency)/1000000)
		fmt.Printf("ADR Enabled: %v\n", c.CN470.ADR.Enabled)
		fmt.Printf("Hardware TX Range: 470-490MHz = %v, 500MHz = %v\n",
			c.CN470.Hardware.SupportsTX470_490MHz,
			c.CN470.Hardware.SupportsTX500MHz)
	}

	fmt.Printf("==========================================\n")
}

// GetChannelFrequencies 获取实际使用的信道频率
func (c *CN470Config) GetChannelFrequencies() ([]uint32, []uint32) {
	var uplinkFreqs []uint32
	var downlinkFreqs []uint32

	switch c.GetCN470Mode() {
	case "STANDARD_FDD":
		// 标准FDD: 470.3-489.3 MHz 上行，500.3-509.7 MHz 下行
		numChannels := c.StandardFDD.UplinkChannels
		if numChannels == 0 {
			numChannels = 96 // 默认96个信道
		}

		spacing := c.Channels.ChannelSpacing
		if spacing == 0 {
			spacing = 200000 // 默认200kHz
		}

		// 标准FDD使用96个上行信道，48个下行信道
		// 上行：470.3 + n*0.2 MHz (n=0-95)
		for i := 0; i < numChannels && i < 96; i++ {
			uplinkFreq := c.StandardFDD.UplinkStartFreq + uint32(i)*spacing
			if uplinkFreq <= c.StandardFDD.UplinkEndFreq {
				uplinkFreqs = append(uplinkFreqs, uplinkFreq)

				// 下行：只有偶数信道有对应的下行
				if i%2 == 0 && i/2 < 48 {
					downlinkFreq := c.StandardFDD.DownlinkStartFreq + uint32(i/2)*spacing*2
					if downlinkFreq <= c.StandardFDD.DownlinkEndFreq {
						downlinkFreqs = append(downlinkFreqs, downlinkFreq)
					}
				}
			}
		}

	case "CUSTOM_FDD":
		// 自定义FDD：使用配置的信道数和间隔
		numChannels := c.CustomFDD.UplinkChannels
		if numChannels == 0 {
			numChannels = 8 // 默认8个信道
		}

		spacing := c.Channels.ChannelSpacing
		if spacing == 0 {
			spacing = 1200000 // 默认1.2MHz
		}

		// 计算每个信道的频率
		for i := 0; i < numChannels; i++ {
			uplinkFreq := c.CustomFDD.UplinkStartFreq + uint32(i)*spacing
			downlinkFreq := c.CustomFDD.DownlinkStartFreq + uint32(i)*spacing

			// 检查是否在范围内
			if uplinkFreq <= c.CustomFDD.UplinkEndFreq &&
				downlinkFreq <= c.CustomFDD.DownlinkEndFreq {
				uplinkFreqs = append(uplinkFreqs, uplinkFreq)
				downlinkFreqs = append(downlinkFreqs, downlinkFreq)
			} else {
				// 添加警告日志
				fmt.Printf("Warning: Channel %d frequency out of range (UL: %d, DL: %d)\n",
					i, uplinkFreq, downlinkFreq)
			}
		}

	case "TDD":
		// TDD模式：上下行使用相同频率
		numChannels := c.TDD.Channels
		if numChannels == 0 {
			numChannels = 96 // 默认96个信道
		}

		spacing := c.Channels.ChannelSpacing
		if spacing == 0 {
			spacing = 200000 // 默认200kHz
		}

		// TDD模式下，上下行频率相同
		for i := 0; i < numChannels; i++ {
			freq := c.TDD.StartFreq + uint32(i)*spacing
			if freq <= c.TDD.EndFreq {
				uplinkFreqs = append(uplinkFreqs, freq)
				downlinkFreqs = append(downlinkFreqs, freq) // TDD模式下相同
			}
		}
	}

	return uplinkFreqs, downlinkFreqs
}

// GetChannelByFrequency 根据频率获取信道索引
func (c *CN470Config) GetChannelByFrequency(freq uint32, isUplink bool) int {
	uplinkFreqs, downlinkFreqs := c.GetChannelFrequencies()

	if isUplink {
		for i, f := range uplinkFreqs {
			if f == freq {
				return i
			}
		}
	} else {
		for i, f := range downlinkFreqs {
			if f == freq {
				return i
			}
		}
	}

	return -1 // 未找到
}

// GetEnabledChannels 获取启用的信道（根据子频段配置）
func (c *CN470Config) GetEnabledChannels() ([]uint32, []uint32) {
	allUplinkFreqs, allDownlinkFreqs := c.GetChannelFrequencies()

	if len(c.Channels.EnabledSubBands) == 0 {
		// 如果没有配置子频段，返回所有信道
		return allUplinkFreqs, allDownlinkFreqs
	}

	var enabledUplink []uint32
	var enabledDownlink []uint32

	// 每个子频段包含8个信道（对于自定义FDD可能不同）
	channelsPerSubBand := 8
	if c.GetCN470Mode() == "CUSTOM_FDD" && c.CustomFDD.UplinkChannels < 8 {
		channelsPerSubBand = c.CustomFDD.UplinkChannels
	}

	for _, subBand := range c.Channels.EnabledSubBands {
		startIdx := subBand * channelsPerSubBand
		endIdx := (subBand + 1) * channelsPerSubBand

		// 添加上行信道
		for i := startIdx; i < endIdx && i < len(allUplinkFreqs); i++ {
			enabledUplink = append(enabledUplink, allUplinkFreqs[i])
		}

		// 添加下行信道
		for i := startIdx; i < endIdx && i < len(allDownlinkFreqs); i++ {
			enabledDownlink = append(enabledDownlink, allDownlinkFreqs[i])
		}
	}

	return enabledUplink, enabledDownlink
}

// ValidateChannelFrequency 验证频率是否为有效的信道频率
func (c *CN470Config) ValidateChannelFrequency(freq uint32, isUplink bool) bool {
	uplinkFreqs, downlinkFreqs := c.GetChannelFrequencies()

	if isUplink {
		for _, f := range uplinkFreqs {
			if f == freq {
				return true
			}
		}
	} else {
		for _, f := range downlinkFreqs {
			if f == freq {
				return true
			}
		}
	}

	return false
}

// GetRandomChannel 随机选择一个启用的信道
func (c *CN470Config) GetRandomChannel() (uplinkFreq uint32, downlinkFreq uint32) {
	enabledUplink, enabledDownlink := c.GetEnabledChannels()

	if len(enabledUplink) == 0 || len(enabledDownlink) == 0 {
		// 如果没有启用的信道，使用默认的第一个信道
		allUplink, allDownlink := c.GetChannelFrequencies()
		if len(allUplink) > 0 && len(allDownlink) > 0 {
			return allUplink[0], allDownlink[0]
		}
		return 470300000, 480300000 // 绝对默认值
	}

	// 随机选择一个索引
	//idx := time.Now().UnixNano() % int64(len(enabledUplink))
	idx := rand.Intn(len(enabledUplink))
	// 对于CUSTOM_FDD，上下行信道是一一对应的
	if c.GetCN470Mode() == "CUSTOM_FDD" && idx < len(enabledDownlink) {
		return enabledUplink[idx], enabledDownlink[idx]
	}

	return enabledUplink[idx], enabledDownlink[0]
}

// 在 config.go 中添加新方法
func (c *CN470Config) AutoDetectMode() {
	// 环境变量优先
	if mode := os.Getenv("CN470_MODE"); mode != "" {
		c.Mode = mode
		return
	}

	// 基于硬件能力自动选择最优模式
	if c.Hardware.SupportsTX500MHz {
		// 实际支持500MHz，使用标准模式
		c.Mode = "STANDARD"
		log.Info().Msg("检测到500MHz支持，使用标准CN470模式")
	} else if c.Hardware.SupportsTX470_490MHz {
		// 仅支持470-490MHz
		c.Mode = "CUSTOM_FDD"
		log.Info().Msg("仅支持470-490MHz，使用自定义FDD模式")
	}
}

// 添加混合模式支持
func (c *CN470Config) GetDownlinkFrequencyAdaptive(uplinkFreq uint32) uint32 {
	// 首先尝试标准模式（+30MHz）
	if c.Hardware.SupportsTX500MHz {
		downlinkFreq := uplinkFreq + 30000000
		if downlinkFreq >= 500300000 && downlinkFreq <= 509700000 {
			return downlinkFreq
		}
	}

	// 降级到自定义模式（+10MHz）
	if c.Hardware.SupportsTX470_490MHz {
		downlinkFreq := uplinkFreq + 10000000
		if downlinkFreq >= 480300000 && downlinkFreq <= 489900000 {
			return downlinkFreq
		}
	}

	// 最后降级到同频
	return uplinkFreq
}

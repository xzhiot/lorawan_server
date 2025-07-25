package lorawan

import "fmt"

// === 新增CN470模式定义 ===

// CN470Mode CN470工作模式定义
type CN470Mode string

const (
	CN470StandardFDD CN470Mode = "STANDARD_FDD" // 标准FDD: 上行470MHz, 下行500MHz
	CN470CustomFDD   CN470Mode = "CUSTOM_FDD"   // 自定义FDD: 470-490MHz内分离
	CN470TDD         CN470Mode = "TDD"          // TDD: 470-490MHz时分复用
)

// RegionConfiguration represents region-specific configuration
type RegionConfiguration struct {
	Name                string
	DefaultChannels     []Channel
	DataRates           []DataRate
	MaxPayloadSizePerDR map[int]int
	RX1DROffsetTable    map[int]map[int]int
	DefaultRX2DR        int
	DefaultRX2Freq      uint32
	FrequencyPlan       string      // 添加频率计划字段
	ChannelPlan         ChannelPlan // 添加信道计划
}

// ChannelPlan 定义信道计划类型
type ChannelPlan struct {
	UplinkChannels   int // 上行信道数
	DownlinkChannels int // 下行信道数
	ChannelsPerPage  int // 每页信道数（CN470特有）
}

// Channel represents a LoRa channel
type Channel struct {
	Frequency uint32
	MinDR     int
	MaxDR     int
}

// DataRate represents a data rate configuration
type DataRate struct {
	SpreadFactor int
	Bandwidth    int
	BitRate      int
}

// GetRegionConfiguration returns configuration for a region
func GetRegionConfiguration(region string) *RegionConfiguration {
	switch region {
	case "EU868":
		return &EU868Configuration
	case "US915":
		return &US915Configuration
	case "CN470", "CN470_510":
		return &CN470Configuration
	default:
		return &EU868Configuration
	}
}

// EU868Configuration for EU 868MHz band
var EU868Configuration = RegionConfiguration{
	Name: "EU868",
	DefaultChannels: []Channel{
		{Frequency: 868100000, MinDR: 0, MaxDR: 5},
		{Frequency: 868300000, MinDR: 0, MaxDR: 5},
		{Frequency: 868500000, MinDR: 0, MaxDR: 5},
	},
	DataRates: []DataRate{
		{SpreadFactor: 12, Bandwidth: 125}, // DR0
		{SpreadFactor: 11, Bandwidth: 125}, // DR1
		{SpreadFactor: 10, Bandwidth: 125}, // DR2
		{SpreadFactor: 9, Bandwidth: 125},  // DR3
		{SpreadFactor: 8, Bandwidth: 125},  // DR4
		{SpreadFactor: 7, Bandwidth: 125},  // DR5
		{SpreadFactor: 7, Bandwidth: 250},  // DR6
	},
	MaxPayloadSizePerDR: map[int]int{
		0: 51,
		1: 51,
		2: 51,
		3: 115,
		4: 242,
		5: 242,
		6: 242,
	},
	RX1DROffsetTable: map[int]map[int]int{
		0: {0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
		1: {0: 1, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
		2: {0: 2, 1: 1, 2: 0, 3: 0, 4: 0, 5: 0},
		3: {0: 3, 1: 2, 2: 1, 3: 0, 4: 0, 5: 0},
		4: {0: 4, 1: 3, 2: 2, 3: 1, 4: 0, 5: 0},
		5: {0: 5, 1: 4, 2: 3, 3: 2, 4: 1, 5: 0},
	},
	DefaultRX2DR:   0,
	DefaultRX2Freq: 869525000,
}

// US915Configuration for US 915MHz band
var US915Configuration = RegionConfiguration{
	Name:            "US915",
	DefaultChannels: []Channel{
		// US915 has 72 channels (64 uplink + 8 downlink)
		// Simplified for brevity
	},
	DataRates: []DataRate{
		{SpreadFactor: 10, Bandwidth: 125}, // DR0
		{SpreadFactor: 9, Bandwidth: 125},  // DR1
		{SpreadFactor: 8, Bandwidth: 125},  // DR2
		{SpreadFactor: 7, Bandwidth: 125},  // DR3
		{SpreadFactor: 8, Bandwidth: 500},  // DR4
	},
	MaxPayloadSizePerDR: map[int]int{
		0: 11,
		1: 53,
		2: 125,
		3: 242,
		4: 242,
	},
	DefaultRX2DR:   8,
	DefaultRX2Freq: 923300000,
}

// CN470Configuration for China 470-490MHz band (Multi-mode support)
var CN470Configuration = RegionConfiguration{
	Name:          "CN470",
	FrequencyPlan: "CN470-FLEXIBLE", // 支持多种模式
	ChannelPlan: ChannelPlan{
		UplinkChannels:   96, // 标准96个上行信道
		DownlinkChannels: 48, // 标准48个下行信道(FDD) 或 96个(TDD)
		ChannelsPerPage:  16, // 16个信道为一组
	},
	DefaultChannels: generateCN470FlexibleChannels(),
	DataRates: []DataRate{
		{SpreadFactor: 12, Bandwidth: 125}, // DR0
		{SpreadFactor: 11, Bandwidth: 125}, // DR1
		{SpreadFactor: 10, Bandwidth: 125}, // DR2
		{SpreadFactor: 9, Bandwidth: 125},  // DR3
		{SpreadFactor: 8, Bandwidth: 125},  // DR4
		{SpreadFactor: 7, Bandwidth: 125},  // DR5
	},
	MaxPayloadSizePerDR: map[int]int{
		0: 51, 1: 51, 2: 51, 3: 115, 4: 222, 5: 222,
	},
	RX1DROffsetTable: map[int]map[int]int{
		0: {0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
		1: {0: 1, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
		2: {0: 2, 1: 1, 2: 0, 3: 0, 4: 0, 5: 0},
		3: {0: 3, 1: 2, 2: 1, 3: 0, 4: 0, 5: 0},
		4: {0: 4, 1: 3, 2: 2, 3: 1, 4: 0, 5: 0},
		5: {0: 5, 1: 4, 2: 3, 3: 2, 4: 1, 5: 0},
	},
	DefaultRX2DR:   0,
	DefaultRX2Freq: 480300000, // 默认使用自定义FDD的RX2频率
}

// generateCN470FlexibleChannels 生成支持多种模式的CN470信道
func generateCN470FlexibleChannels() []Channel {
	channels := make([]Channel, 0, 16)
	baseFreq := uint32(470300000) // 470.3 MHz

	// 生成前16个信道作为默认信道
	for i := 0; i < 16; i++ {
		freq := baseFreq + uint32(i*200000)

		// 只添加在470-490MHz范围内的信道
		if freq >= 470000000 && freq <= 490000000 {
			channels = append(channels, Channel{
				Frequency: freq,
				MinDR:     0,
				MaxDR:     5,
			})
		}
	}

	return channels
}

// generateCN470DefaultChannels 生成CN470默认信道（前8个上行信道）
func generateCN470DefaultChannels() []Channel {
	channels := make([]Channel, 8)
	baseFreq := uint32(470300000) // 470.3 MHz
	for i := 0; i < 8; i++ {
		channels[i] = Channel{
			Frequency: baseFreq + uint32(i*200000), // 200kHz spacing
			MinDR:     0,
			MaxDR:     5,
		}
	}
	return channels
}

// GetRX1DataRateOffset calculates RX1 data rate
func (r *RegionConfiguration) GetRX1DataRateOffset(uplinkDR, rx1DROffset uint8) (uint8, error) {
	if r.RX1DROffsetTable != nil {
		if drMap, ok := r.RX1DROffsetTable[int(uplinkDR)]; ok {
			if dr, ok := drMap[int(rx1DROffset)]; ok {
				return uint8(dr), nil
			}
		}
	}

	// Default behavior
	dr := int(uplinkDR) - int(rx1DROffset)
	if dr < 0 {
		dr = 0
	}
	return uint8(dr), nil
}

// GetCN470DownlinkFrequency 根据模式和上行频率计算下行频率
func (r *RegionConfiguration) GetCN470DownlinkFrequency(uplinkFreq uint32, mode CN470Mode) uint32 {
	if r.Name != "CN470" {
		return 0
	}

	switch mode {
	case CN470StandardFDD:
		// 标准FDD: 下行 = 上行 + 30MHz
		downlinkFreq := uplinkFreq + 30000000
		// 验证下行频率范围 (500.3-509.7 MHz)
		if downlinkFreq >= 500300000 && downlinkFreq <= 509700000 {
			return downlinkFreq
		}
		return r.DefaultRX2Freq

	case CN470CustomFDD:
		// 自定义FDD: 下行 = 上行 + 10MHz
		downlinkFreq := uplinkFreq + 10000000
		// 验证是否在470-490MHz范围内
		if downlinkFreq >= 470000000 && downlinkFreq <= 490000000 {
			return downlinkFreq
		}
		return r.DefaultRX2Freq

	case CN470TDD:
		// TDD: 下行使用相同频率
		if uplinkFreq >= 470000000 && uplinkFreq <= 490000000 {
			return uplinkFreq
		}
		return r.DefaultRX2Freq

	default:
		return r.DefaultRX2Freq
	}
}

// GetCN470ChannelPlan 获取CN470特定频段的信道计划
func GetCN470ChannelPlan(subBand int) []Channel {
	if subBand < 0 || subBand > 5 {
		subBand = 0 // 默认使用第一个子频段
	}

	channels := make([]Channel, 16) // 每个子频段16个信道
	baseFreq := uint32(470300000 + subBand*16*200000)

	for i := 0; i < 16; i++ {
		channels[i] = Channel{
			Frequency: baseFreq + uint32(i*200000),
			MinDR:     0,
			MaxDR:     5,
		}
	}

	return channels
}

// GetCN470ModeForHardware 根据硬件能力推荐CN470模式
func GetCN470ModeForHardware(supportsTX500MHz bool, supportsTX470_490MHz bool) CN470Mode {
	if supportsTX500MHz {
		return CN470StandardFDD // 支持标准FDD
	}

	if supportsTX470_490MHz {
		return CN470CustomFDD // 使用自定义FDD
	}

	return CN470TDD // 回退到TDD
}

// GetCN470ChannelPlanForMode 根据模式获取信道计划
func GetCN470ChannelPlanForMode(mode CN470Mode) ([]Channel, []Channel) {
	var uplinkChannels, downlinkChannels []Channel

	switch mode {
	case CN470StandardFDD:
		// 标准FDD: 96个上行信道，48个下行信道
		for ch := 0; ch < 96; ch++ {
			uplinkFreq := uint32(470300000 + ch*200000)
			uplinkChannels = append(uplinkChannels, Channel{
				Frequency: uplinkFreq,
				MinDR:     0, MaxDR: 5,
			})
		}
		for ch := 0; ch < 48; ch++ {
			downlinkFreq := uint32(500300000 + ch*200000)
			downlinkChannels = append(downlinkChannels, Channel{
				Frequency: downlinkFreq,
				MinDR:     0, MaxDR: 5,
			})
		}

	case CN470CustomFDD:
		// 自定义FDD: 48个上行信道(470.3-479.9MHz), 48个下行信道(480.3-489.9MHz)
		for ch := 0; ch < 48; ch++ {
			uplinkFreq := uint32(470300000 + ch*200000)
			downlinkFreq := uint32(480300000 + ch*200000)

			if uplinkFreq <= 490000000 && downlinkFreq <= 490000000 {
				uplinkChannels = append(uplinkChannels, Channel{
					Frequency: uplinkFreq,
					MinDR:     0, MaxDR: 5,
				})
				downlinkChannels = append(downlinkChannels, Channel{
					Frequency: downlinkFreq,
					MinDR:     0, MaxDR: 5,
				})
			}
		}

	case CN470TDD:
		// TDD: 相同信道用于上下行
		for ch := 0; ch < 96; ch++ {
			freq := uint32(470300000 + ch*200000)
			if freq <= 490000000 {
				channel := Channel{Frequency: freq, MinDR: 0, MaxDR: 5}
				uplinkChannels = append(uplinkChannels, channel)
				downlinkChannels = append(downlinkChannels, channel)
			}
		}
	}

	return uplinkChannels, downlinkChannels
}

// ValidateCN470Frequency 验证CN470频率是否有效
func ValidateCN470Frequency(freq uint32, mode CN470Mode) bool {
	switch mode {
	case CN470StandardFDD:
		// 标准FDD: 上行470-490MHz, 下行500-510MHz
		return (freq >= 470000000 && freq <= 490000000) ||
			(freq >= 500000000 && freq <= 510000000)

	case CN470CustomFDD, CN470TDD:
		// 自定义FDD和TDD: 仅470-490MHz
		return freq >= 470000000 && freq <= 490000000

	default:
		return false
	}
}

// GetCN470FrequencyOffset 获取CN470频率偏移
func GetCN470FrequencyOffset(mode CN470Mode) uint32 {
	switch mode {
	case CN470StandardFDD:
		return 30000000 // 30MHz
	case CN470CustomFDD:
		return 10000000 // 10MHz
	case CN470TDD:
		return 0 // 无偏移
	default:
		return 0
	}
}

// === 现有的GetCN470ChannelIndex方法保持兼容性 ===

// GetCN470ChannelIndex 获取CN470信道索引 (兼容现有代码)
func (r *RegionConfiguration) GetCN470ChannelIndex(frequency uint32) (int, error) {
	if r.Name != "CN470" {
		return -1, fmt.Errorf("not CN470 region")
	}

	// 支持470-490MHz范围 (适配多模式)
	if frequency >= 470300000 && frequency <= 490000000 {
		return int((frequency - 470300000) / 200000), nil
	}

	// 支持500MHz范围 (标准FDD下行)
	if frequency >= 500300000 && frequency <= 509700000 {
		return int((frequency - 500300000) / 200000), nil
	}

	return -1, fmt.Errorf("frequency %d Hz out of CN470 range", frequency)
}

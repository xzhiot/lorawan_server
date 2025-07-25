package network

import (
	"encoding/hex"

	"github.com/rs/zerolog/log"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// MACCommandHandler 处理 MAC 命令
type MACCommandHandler struct {
	store  storage.Store
	region *lorawan.RegionConfiguration
}

// NewMACCommandHandler 创建 MAC 命令处理器
func NewMACCommandHandler(store storage.Store, region string) *MACCommandHandler {
	return &MACCommandHandler{
		store:  store,
		region: lorawan.GetRegionConfiguration(region),
	}
}

// HandleUplink 处理上行 MAC 命令
func (h *MACCommandHandler) HandleUplink(session *models.DeviceSession, commands []lorawan.MACCommand) []lorawan.MACCommand {
	var responses []lorawan.MACCommand

	for _, cmd := range commands {
		switch cmd.CID {
		case lorawan.LinkCheckReq:
			responses = append(responses, h.handleLinkCheckReq(session))

		case lorawan.LinkADRAns:
			h.handleLinkADRAns(session, cmd.Payload)

		case lorawan.DevStatusAns:
			h.handleDevStatusAns(session, cmd.Payload)

		case lorawan.RXParamSetupAns:
			h.handleRXParamSetupAns(session, cmd.Payload)

		case lorawan.NewChannelAns:
			h.handleNewChannelAns(session, cmd.Payload)

		default:
			log.Warn().
				Uint8("cid", cmd.CID).
				Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
				Msg("未处理的 MAC 命令")
		}
	}

	// 检查是否需要发送 ADR 请求
	if session.ADR && h.shouldSendADRReq(session) {
		adrReq := h.createADRReq(session)
		if adrReq != nil {
			responses = append(responses, *adrReq)
		}
	}

	return responses
}

// handleLinkCheckReq 处理链路检查请求
func (h *MACCommandHandler) handleLinkCheckReq(session *models.DeviceSession) lorawan.MACCommand {
	// TODO: 从最近的上行数据中获取实际的 margin 和网关数
	margin := uint8(10) // SNR margin in dB
	gwCnt := uint8(1)   // 网关数量

	payload := []byte{margin, gwCnt}

	log.Debug().
		Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
		Uint8("margin", margin).
		Uint8("gwCnt", gwCnt).
		Msg("响应 LinkCheckReq")

	return lorawan.MACCommand{
		CID:     lorawan.LinkCheckAns,
		Payload: payload,
	}
}

// handleLinkADRAns 处理 ADR 响应
func (h *MACCommandHandler) handleLinkADRAns(session *models.DeviceSession, payload []byte) {
	if len(payload) != 1 {
		return
	}

	status := payload[0]
	powerACK := (status & 0x04) != 0
	dataRateACK := (status & 0x02) != 0
	channelMaskACK := (status & 0x01) != 0

	log.Debug().
		Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
		Bool("powerACK", powerACK).
		Bool("dataRateACK", dataRateACK).
		Bool("channelMaskACK", channelMaskACK).
		Msg("收到 LinkADRAns")

	// TODO: 根据响应更新 ADR 状态
}

// handleDevStatusAns 处理设备状态响应
func (h *MACCommandHandler) handleDevStatusAns(session *models.DeviceSession, payload []byte) {
	if len(payload) != 2 {
		return
	}

	battery := payload[0]
	margin := int8(payload[1]) // SNR margin

	log.Info().
		Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
		Uint8("battery", battery).
		Int8("margin", margin).
		Msg("收到设备状态")

	// TODO: 更新设备电池状态
}

// handleRXParamSetupAns 处理 RX 参数设置响应
func (h *MACCommandHandler) handleRXParamSetupAns(session *models.DeviceSession, payload []byte) {
	if len(payload) != 1 {
		return
	}

	status := payload[0]
	rx1DROffsetACK := (status & 0x04) != 0
	rx2DataRateACK := (status & 0x02) != 0
	channelACK := (status & 0x01) != 0

	log.Debug().
		Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
		Bool("rx1DROffsetACK", rx1DROffsetACK).
		Bool("rx2DataRateACK", rx2DataRateACK).
		Bool("channelACK", channelACK).
		Msg("收到 RXParamSetupAns")
}

// handleNewChannelAns 处理新信道响应
func (h *MACCommandHandler) handleNewChannelAns(session *models.DeviceSession, payload []byte) {
	if len(payload) != 1 {
		return
	}

	status := payload[0]
	dataRateACK := (status & 0x02) != 0
	channelFreqACK := (status & 0x01) != 0

	log.Debug().
		Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
		Bool("dataRateACK", dataRateACK).
		Bool("channelFreqACK", channelFreqACK).
		Msg("收到 NewChannelAns")
}

// shouldSendADRReq 判断是否需要发送 ADR 请求
func (h *MACCommandHandler) shouldSendADRReq(session *models.DeviceSession) bool {
	// TODO: 实现 ADR 算法
	// - 检查历史 SNR/RSSI
	// - 计算最优数据速率
	// - 决定是否需要调整
	return false
}

// createADRReq 创建 ADR 请求
func (h *MACCommandHandler) createADRReq(session *models.DeviceSession) *lorawan.MACCommand {
	dataRate := session.DR
	txPower := session.TXPower
	var chMask uint16
	redundancy := uint8(1) // NbTrans = 1

	// CN470特殊处理
	if h.region.Name == "CN470" {
		// CN470 每次只能启用8个连续信道
		// 这里简化处理，启用当前子频段的8个信道
		currentChannel := 0 // 从设备当前使用的信道推断
		subBand := currentChannel / 8

		// 根据子频段计算信道掩码控制值
		// CN470 使用 ChMaskCntl 来指示信道组
		chMaskCntl := uint8(subBand)

		// 设置对应子频段的8个信道
		chMask = 0x00FF // 启用8个信道

		payload := make([]byte, 4)
		payload[0] = (dataRate << 4) | (txPower & 0x0F)
		payload[1] = byte(chMask)
		payload[2] = byte(chMask >> 8)

		// CN470: ChMaskCntl 字段编码子频段
		// Bits 2:0 RFU, bits 6:3 ChMaskCntl
		payload[3] = (chMaskCntl << 3) | redundancy

		log.Debug().
			Int("subBand", subBand).
			Uint8("chMaskCntl", chMaskCntl).
			Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
			Msg("CN470 ADR 配置")

		return &lorawan.MACCommand{
			CID:     lorawan.LinkADRReq,
			Payload: payload,
		}
	} else {
		// 其他区域的默认处理
		chMask = 0x00FF // 启用前8个信道

		payload := make([]byte, 4)
		payload[0] = (dataRate << 4) | (txPower & 0x0F)
		payload[1] = byte(chMask)
		payload[2] = byte(chMask >> 8)
		payload[3] = redundancy

		return &lorawan.MACCommand{
			CID:     lorawan.LinkADRReq,
			Payload: payload,
		}
	}
}

// ADR 算法实现
type ADRAlgorithm struct {
	minDataRate int
	maxDataRate int
	minTxPower  int
	maxTxPower  int
	targetSNR   float64
	marginSNR   float64
	historySize int
}

// CalculateADR 计算 ADR 参数
func (a *ADRAlgorithm) CalculateADR(history []models.ADRHistory) (dataRate, txPower uint8, nbTrans uint8) {
	if len(history) < a.historySize {
		return 0, 0, 1 // 历史数据不足
	}

	// 计算平均 SNR
	var sumSNR float64
	var minGateways int = 100
	for _, h := range history {
		sumSNR += h.MaxSNR
		if h.GatewayCount < minGateways {
			minGateways = h.GatewayCount
		}
	}
	avgSNR := sumSNR / float64(len(history))

	// 计算 SNR 余量
	margin := avgSNR - a.targetSNR - a.marginSNR

	// 调整数据速率
	currentDR := int(history[len(history)-1].TXPower >> 4)
	newDR := currentDR

	if margin > 3 {
		// SNR 很好，可以提高数据速率
		newDR = currentDR + 1
		if newDR > a.maxDataRate {
			newDR = a.maxDataRate
		}
	} else if margin < -3 {
		// SNR 较差，降低数据速率
		newDR = currentDR - 1
		if newDR < a.minDataRate {
			newDR = a.minDataRate
		}
	}

	// 调整发射功率
	currentTxPower := int(history[len(history)-1].TXPower & 0x0F)
	newTxPower := currentTxPower

	if newDR == currentDR {
		if margin > 0 {
			// 降低发射功率以节省能量
			newTxPower = currentTxPower + 1 // 注意：值越大功率越小
			if newTxPower > a.maxTxPower {
				newTxPower = a.maxTxPower
			}
		} else if margin < 0 {
			// 增加发射功率
			newTxPower = currentTxPower - 1
			if newTxPower < a.minTxPower {
				newTxPower = a.minTxPower
			}
		}
	}

	// 根据网关数量调整重传次数
	if minGateways <= 1 {
		nbTrans = 3
	} else if minGateways <= 3 {
		nbTrans = 2
	} else {
		nbTrans = 1
	}

	return uint8(newDR), uint8(newTxPower), nbTrans
}

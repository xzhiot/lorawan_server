package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/google/uuid"
	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
)

// Semtech UDP 协议常量
const (
	ProtocolVersion = 2

	// 消息类型
	PushData = 0x00
	PushAck  = 0x01
	PullData = 0x02
	PullResp = 0x03
	PullAck  = 0x04
	TxAck    = 0x05
)

// UDPPacketForwarder 处理 Semtech UDP 协议
type UDPPacketForwarder struct {
	conn     *net.UDPConn
	nc       *nats.Conn
	store    storage.Store
	gateways map[string]*GatewayInfo
	mu       sync.RWMutex
	tokens   map[uint16]time.Time
}

// GatewayInfo 网关信息
type GatewayInfo struct {
	GatewayID      string
	PushAddr       *net.UDPAddr // PUSH_DATA 地址（上行）
	PullAddr       *net.UDPAddr // PULL_DATA 地址（下行）
	LastSeen       time.Time
	PullData       time.Time
	PullTokenBytes [2]byte
	ProtocolVer    uint8
}

// NewUDPPacketForwarder 创建 UDP 包转发器
func NewUDPPacketForwarder(bindAddr string, nc *nats.Conn, store storage.Store) (*UDPPacketForwarder, error) {
	addr, err := net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	return &UDPPacketForwarder{
		conn:     conn,
		nc:       nc,
		store:    store,
		gateways: make(map[string]*GatewayInfo),
		tokens:   make(map[uint16]time.Time),
	}, nil
}

// Start 启动 UDP 服务器
func (u *UDPPacketForwarder) Start(ctx context.Context) error {
	log.Info().Str("addr", u.conn.LocalAddr().String()).Msg("Gateway Bridge UDP 服务器启动")

	// 启动下行数据处理
	go u.handleDownlink(ctx)

	// 启动网关清理
	go u.cleanupGateways(ctx)

	// 处理上行 UDP 包
	buf := make([]byte, 65507)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, addr, err := u.conn.ReadFromUDP(buf)
			if err != nil {
				log.Error().Err(err).Msg("读取 UDP 包错误")
				continue
			}

			go u.handlePacket(buf[:n], addr)
		}
	}
}

// handlePacket 处理接收到的包
func (u *UDPPacketForwarder) handlePacket(data []byte, addr *net.UDPAddr) {
	if len(data) < 4 {
		return
	}

	version := data[0]
	token := binary.BigEndian.Uint16(data[1:3])
	identifier := data[3]

	// 检查协议版本
	if version != ProtocolVersion {
		log.Warn().
			Uint8("version", version).
			Str("addr", addr.String()).
			Msg("不支持的协议版本")
		return
	}

	switch identifier {
	case PushData:
		u.handlePushData(data, addr, token)
	case PullData:
		u.handlePullData(data, addr, token)
	case TxAck:
		u.handleTxAck(data, addr, token)
	default:
		log.Warn().
			Uint8("type", identifier).
			Str("addr", addr.String()).
			Msg("未知的包类型")
	}
}

// handlePushData 处理 PUSH_DATA
func (u *UDPPacketForwarder) handlePushData(data []byte, addr *net.UDPAddr, token uint16) {
	if len(data) < 12 {
		return
	}

	// 解析网关 MAC
	var gatewayMAC [8]byte
	copy(gatewayMAC[:], data[4:12])
	gatewayID := fmt.Sprintf("%016x", gatewayMAC)

	// 更新网关信息
	u.mu.Lock()
	gw, exists := u.gateways[gatewayID]
	if !exists {
		gw = &GatewayInfo{
			GatewayID:   gatewayID,
			ProtocolVer: ProtocolVersion,
		}
		u.gateways[gatewayID] = gw
	}
	gw.PushAddr = addr // 只更新 PUSH 地址
	gw.LastSeen = time.Now()
	u.mu.Unlock()

	// 发送 PUSH_ACK
	ack := make([]byte, 4)
	ack[0] = ProtocolVersion
	binary.BigEndian.PutUint16(ack[1:3], token)
	ack[3] = PushAck
	u.conn.WriteToUDP(ack, addr)

	// 更新数据库中的网关状态
	go u.updateGatewayInDB(gatewayID)

	// 解析 JSON 数据
	if len(data) > 12 {
		var payload map[string]interface{}
		if err := json.Unmarshal(data[12:], &payload); err != nil {
			log.Error().Err(err).Msg("解析 PUSH_DATA JSON 失败")
			return
		}

		// 处理接收到的数据包
		if rxpk, ok := payload["rxpk"].([]interface{}); ok {
			for _, pkt := range rxpk {
				u.handleRXPacket(gatewayID, pkt)
			}
		}

		// 处理状态信息
		if stat, ok := payload["stat"].(map[string]interface{}); ok {
			u.handleStat(gatewayID, stat)
		}
	}

	log.Debug().
		Str("gateway", gatewayID).
		Str("pushAddr", addr.String()).
		Msg("收到 PUSH_DATA")
}

// handlePullData 处理 PULL_DATA
func (u *UDPPacketForwarder) handlePullData(data []byte, addr *net.UDPAddr, token uint16) {
	if len(data) < 12 {
		return
	}

	// 解析网关 MAC
	var gatewayMAC [8]byte
	copy(gatewayMAC[:], data[4:12])
	gatewayID := fmt.Sprintf("%016x", gatewayMAC)

	// 更新网关信息
	u.mu.Lock()
	gw, exists := u.gateways[gatewayID]
	if !exists {
		gw = &GatewayInfo{
			GatewayID:   gatewayID,
			ProtocolVer: ProtocolVersion,
		}
		u.gateways[gatewayID] = gw
	}
	gw.PullAddr = addr // 只更新 PULL 地址
	gw.LastSeen = time.Now()
	gw.PullData = time.Now()
	gw.PullTokenBytes[0] = data[1]
	gw.PullTokenBytes[1] = data[2]
	u.mu.Unlock()

	// 发送 PULL_ACK
	ack := make([]byte, 4)
	ack[0] = ProtocolVersion
	binary.BigEndian.PutUint16(ack[1:3], token)
	ack[3] = PullAck
	u.conn.WriteToUDP(ack, addr)

	// 更新数据库中的网关状态
	go u.updateGatewayInDB(gatewayID)

	log.Debug().
		Str("gateway", gatewayID).
		Str("pullAddr", addr.String()).
		Hex("token", data[1:3]).
		Msg("收到 PULL_DATA，更新下行地址")
}

// handleRXPacket 处理接收包
func (u *UDPPacketForwarder) handleRXPacket(gatewayID string, pkt interface{}) {
	pktMap, ok := pkt.(map[string]interface{})
	if !ok {
		return
	}
	// 添加日志
	if tmst, ok := pktMap["tmst"]; ok {
		log.Debug().
			Interface("tmst", tmst).
			Interface("type", fmt.Sprintf("%T", tmst)).
			Msg("收到的上行时间戳")
	}
	// ✅ 确保 tmst 是正确的类型
	var tmstValue uint64
	switch v := pktMap["tmst"].(type) {
	case float64:
		tmstValue = uint64(v)
	case uint64:
		tmstValue = v
	case int64:
		tmstValue = uint64(v)
	case int:
		tmstValue = uint64(v)
	default:
		log.Error().
			Interface("tmst", v).
			Str("type", fmt.Sprintf("%T", v)).
			Msg("无法解析上行 tmst")
		return
	}

	// ✅ 创建 context，确保 tmst 是 float64
	context := map[string]interface{}{
		"gateway_id": gatewayID,
		"tmst":       float64(tmstValue), // 转换为 float64
	}
	contextBytes, _ := json.Marshal(context)
	contextB64 := base64.StdEncoding.EncodeToString(contextBytes)

	// 构建 NATS 消息
	msg := map[string]interface{}{
		"gatewayID": gatewayID,
		"rxpk":      pktMap,
		"context":   contextB64, // ✅ 添加 context
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("序列化 RX 包失败")
		return
	}

	// 发布到 NATS
	subject := fmt.Sprintf("gateway.%s.rx", gatewayID)
	if err := u.nc.Publish(subject, data); err != nil {
		log.Error().Err(err).Msg("发布到 NATS 失败")
		return
	}

	// 获取 LoRaWAN 数据
	if dataStr, ok := pktMap["data"].(string); ok {
		phyPayload, err := base64.StdEncoding.DecodeString(dataStr)
		if err == nil {
			log.Info().
				Str("gateway", gatewayID).
				Float64("freq", getFloat64(pktMap, "freq")).
				Float64("rssi", getFloat64(pktMap, "rssi")).
				Float64("snr", getFloat64(pktMap, "lsnr")).
				Int("size", len(phyPayload)).
				Msg("收到上行数据")
		}
	}
}

// handleStat 处理状态信息
func (u *UDPPacketForwarder) handleStat(gatewayID string, stat map[string]interface{}) {
	// 构建状态消息
	msg := map[string]interface{}{
		"gatewayID": gatewayID,
		"stat":      stat,
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// 发布到 NATS
	subject := fmt.Sprintf("gateway.%s.stat", gatewayID)
	u.nc.Publish(subject, data)

	log.Debug().
		Str("gateway", gatewayID).
		Uint64("rxnb", getUint64(stat, "rxnb")).
		Uint64("rxok", getUint64(stat, "rxok")).
		Msg("收到网关状态")
}

// handleTxAck 处理 TX_ACK
func (u *UDPPacketForwarder) handleTxAck(data []byte, addr *net.UDPAddr, token uint16) {
	if len(data) < 12 {
		return
	}

	// 解析网关 MAC
	var gatewayMAC [8]byte
	copy(gatewayMAC[:], data[4:12])
	gatewayID := fmt.Sprintf("%016x", gatewayMAC)

	var txAckData map[string]interface{}
	if len(data) > 12 {
		json.Unmarshal(data[12:], &txAckData)
	}

	// 发布到 NATS
	msg := map[string]interface{}{
		"gatewayID": gatewayID,
		"token":     token,
		"ack":       txAckData,
	}

	msgData, _ := json.Marshal(msg)
	subject := fmt.Sprintf("gateway.%s.txack", gatewayID)
	u.nc.Publish(subject, msgData)

	log.Debug().
		Str("gateway", gatewayID).
		Uint16("token", token).
		Interface("ack", txAckData).
		Msg("收到 TX_ACK")
}

// handleDownlink 处理下行数据
func (u *UDPPacketForwarder) handleDownlink(ctx context.Context) {
	log.Info().Msg("开始订阅下行消息")

	// 订阅下行消息
	sub, err := u.nc.Subscribe("gateway.*.tx", func(msg *nats.Msg) {
		log.Info().
			Str("subject", msg.Subject).
			Int("size", len(msg.Data)).
			Msg("收到 NATS 下行消息")

		var txMsg map[string]interface{}
		if err := json.Unmarshal(msg.Data, &txMsg); err != nil {
			log.Error().Err(err).Msg("解析下行消息失败")
			return
		}

		gatewayID, ok := txMsg["gatewayID"].(string)
		if !ok {
			log.Error().Msg("下行消息缺少 gatewayID")
			return
		}

		u.sendDownlink(gatewayID, txMsg)
	})

	if err != nil {
		log.Error().Err(err).Msg("订阅下行消息失败")
		return
	}

	<-ctx.Done()
	sub.Unsubscribe()
}

// sendDownlink 发送下行数据
func (u *UDPPacketForwarder) sendDownlink(gatewayID string, txMsg map[string]interface{}) {
	log.Info().
		Str("gateway", gatewayID).
		Interface("txMsg", txMsg).
		Msg("处理下行数据请求")

	u.mu.RLock()
	gw, exists := u.gateways[gatewayID]
	u.mu.RUnlock()

	if !exists {
		log.Warn().
			Str("gateway", gatewayID).
			Msg("网关不存在")
		return
	}

	if gw.PullAddr == nil {
		log.Warn().
			Str("gateway", gatewayID).
			Bool("hasPushAddr", gw.PushAddr != nil).
			Msg("网关没有 PULL 地址（未收到 PULL_DATA）")
		return
	}

	// 从消息中提取 txpk
	txpk, ok := txMsg["txpk"].(map[string]interface{})
	if !ok {
		log.Error().Msg("消息中没有 txpk 字段")
		return
	}

	// ✅ 新增：检查是否有 context 和 timing 信息
	contextStr, hasContext := txMsg["context"].(string)
	timing, hasTiming := txMsg["timing"].(map[string]interface{})

	// 构建 PULL_RESP
	resp := bytes.NewBuffer(nil)
	resp.WriteByte(ProtocolVersion)
	resp.WriteByte(gw.PullTokenBytes[0])
	resp.WriteByte(gw.PullTokenBytes[1])
	resp.WriteByte(PullResp)

	// ✅ 添加常量定义
	const MAX_TIMESTAMP = uint64(4294967295)      // uint32 最大值
	const OVERFLOW_THRESHOLD = uint64(4000000000) // 40亿，接近溢出的阈值
	const MIN_PREPARE_TIME = uint64(200000)       // 200ms 最小准备时间
	const MIN_SAFE_DELAY = uint64(50000)          // 50ms 最小安全延迟

	var jsonStr string

	// ✅ 修改：优先使用 context + timing 方式（ChirpStack 方式）
	if hasContext && hasTiming {
		// 解码 context
		contextBytes, err := base64.StdEncoding.DecodeString(contextStr)
		if err == nil {
			var context map[string]interface{}
			if err := json.Unmarshal(contextBytes, &context); err == nil {
				// 从 context 中获取原始时间戳
				if uplinkTmst, ok := context["tmst"].(float64); ok {
					// 获取延迟时间
					if delayStr, ok := timing["delay"].(string); ok {
						// 解析延迟（如 "1s" 或 "1000ms"）
						var delayUs uint64
						if strings.HasSuffix(delayStr, "ms") {
							ms, _ := strconv.ParseUint(strings.TrimSuffix(delayStr, "ms"), 10, 64)
							delayUs = ms * 1000
						} else if strings.HasSuffix(delayStr, "s") {
							s, _ := strconv.ParseUint(strings.TrimSuffix(delayStr, "s"), 10, 64)
							delayUs = s * 1000000
						}

						uplinkTmstUint := uint64(uplinkTmst)

						// ✅ 新增：检查时间戳年龄，避免使用过期的时间戳
						if uplinkTmstUint < 60000000 { // 60秒
							log.Warn().
								Uint64("uplinkTmst", uplinkTmstUint).
								Msg("上行时间戳太小，使用即时发送")
							jsonStr = u.createImmediateTxpk(txpk)
						} else if uplinkTmstUint > OVERFLOW_THRESHOLD {
							// 时间戳过大，使用即时发送
							log.Warn().
								Uint64("uplinkTmst", uplinkTmstUint).
								Str("gateway", gatewayID).
								Msg("时间戳接近溢出，改用即时发送")
							jsonStr = u.createImmediateTxpk(txpk)
						} else {
							// 计算下行时间戳
							downlinkTmst := uplinkTmstUint + delayUs

							// ✅ 处理时间戳溢出
							if downlinkTmst > MAX_TIMESTAMP {
								// 处理溢出：使用模运算
								downlinkTmst = downlinkTmst & MAX_TIMESTAMP

								// 确保有足够的时间
								if downlinkTmst < MIN_SAFE_DELAY {
									log.Warn().
										Uint64("wrappedTmst", downlinkTmst).
										Msg("溢出后时间不足，使用即时发送")
									jsonStr = u.createImmediateTxpk(txpk)
								} else {
									// 使用溢出后的时间戳
									jsonStr = u.createDelayedTxpk(txpk, downlinkTmst)
									log.Info().
										Uint64("originalTmst", uplinkTmstUint+delayUs).
										Uint64("wrappedTmst", downlinkTmst).
										Msg("时间戳溢出，使用模运算处理")
								}
							} else if downlinkTmst < uplinkTmstUint+MIN_PREPARE_TIME {
								// ✅ 确保有足够的准备时间
								log.Warn().
									Uint64("delay", delayUs).
									Msg("延迟太短，增加最小准备时间")
								downlinkTmst = uplinkTmstUint + MIN_PREPARE_TIME
								jsonStr = u.createDelayedTxpk(txpk, downlinkTmst)
							} else if downlinkTmst < uplinkTmstUint {
								// 下行时间戳小于上行时间戳（不应该发生）
								log.Error().
									Uint64("uplinkTmst", uplinkTmstUint).
									Uint64("downlinkTmst", downlinkTmst).
									Msg("计算错误：下行时间戳小于上行时间戳")
								jsonStr = u.createImmediateTxpk(txpk)
							} else {
								// 正常情况
								jsonStr = u.createDelayedTxpk(txpk, downlinkTmst)
								log.Info().
									Str("gateway", gatewayID).
									Str("mode", "context_delay").
									Uint64("uplinkTmst", uplinkTmstUint).
									Uint64("downlinkTmst", downlinkTmst).
									Str("delay", delayStr).
									Msg("使用 context + timing 模式")
							}
						}
					}
				}
			}
		}

		// 如果 context 解析失败，继续使用原有逻辑
		if jsonStr == "" {
			log.Warn().Msg("context 解析失败，回退到原有逻辑")
		}
	}

	// 如果没有使用 context 方式，使用原有逻辑
	if jsonStr == "" {
		// 检查是否是即时发送模式
		if imme, ok := txpk["imme"].(bool); ok && imme {
			// 即时发送模式
			jsonStr = u.createImmediateTxpk(txpk)
			log.Info().
				Str("gateway", gatewayID).
				Str("mode", "immediate").
				Msg("使用即时发送模式")
		} else {
			// 延时发送模式，需要 tmst
			var tmstValue uint64
			switch v := txpk["tmst"].(type) {
			case float64:
				tmstValue = uint64(v)
			case uint64:
				tmstValue = v
			case int64:
				tmstValue = uint64(v)
			case nil:
				log.Error().Msg("延时发送模式但 tmst 为 null")
				return
			default:
				log.Error().
					Interface("tmst", v).
					Str("type", fmt.Sprintf("%T", v)).
					Msg("无法解析 tmst 值")
				return
			}

			jsonStr = u.createDelayedTxpk(txpk, tmstValue)
			log.Info().
				Str("gateway", gatewayID).
				Uint64("tmst", tmstValue).
				Msg("使用延时发送模式")
		}
	}

	resp.Write([]byte(jsonStr))

	// 发送到网关的 PULL 地址
	n, err := u.conn.WriteToUDP(resp.Bytes(), gw.PullAddr)
	if err != nil {
		log.Error().
			Err(err).
			Str("gateway", gatewayID).
			Str("pullAddr", gw.PullAddr.String()).
			Msg("发送 PULL_RESP 失败")
		return
	}

	log.Info().
		Str("gateway", gatewayID).
		Int("bytes", n).
		Str("pullAddr", gw.PullAddr.String()).
		Hex("token", gw.PullTokenBytes[:]).
		Str("json", jsonStr).
		Msg("PULL_RESP 已发送")
}

// 辅助函数：创建即时发送的txpk
func (u *UDPPacketForwarder) createImmediateTxpk(txpk map[string]interface{}) string {
	return fmt.Sprintf(`{"txpk":{"imme":true,"rfch":%d,"powe":%d,"ant":%d,"brd":%d,"freq":%.1f,"modu":"%v","datr":"%v","codr":"%v","ipol":%v,"size":%d,"data":"%v"}}`,
		getInt(txpk, "rfch"),
		getInt(txpk, "powe"),
		getInt(txpk, "ant"),
		getInt(txpk, "brd"),
		txpk["freq"],
		txpk["modu"],
		txpk["datr"],
		txpk["codr"],
		txpk["ipol"],
		getInt(txpk, "size"),
		txpk["data"],
	)
}

// 辅助函数：创建延时发送的txpk
func (u *UDPPacketForwarder) createDelayedTxpk(txpk map[string]interface{}, tmst uint64) string {
	return fmt.Sprintf(`{"txpk":{"imme":false,"rfch":%d,"powe":%d,"ant":%d,"brd":%d,"tmst":%d,"freq":%.1f,"modu":"%v","datr":"%v","codr":"%v","ipol":%v,"size":%d,"data":"%v"}}`,
		getInt(txpk, "rfch"),
		getInt(txpk, "powe"),
		getInt(txpk, "ant"),
		getInt(txpk, "brd"),
		tmst,
		txpk["freq"],
		txpk["modu"],
		txpk["datr"],
		txpk["codr"],
		txpk["ipol"],
		getInt(txpk, "size"),
		txpk["data"],
	)
}

// 添加辅助函数
func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}

	// 添加调试日志
	log.Warn().
		Str("key", key).
		Interface("value", m[key]).
		Interface("type", fmt.Sprintf("%T", m[key])).
		Msg("getInt 无法转换，返回默认值 0")

	return 0
}

// cleanupGateways 清理离线网关
func (u *UDPPacketForwarder) cleanupGateways(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.mu.Lock()
			now := time.Now()
			for id, gw := range u.gateways {
				if now.Sub(gw.LastSeen) > 5*time.Minute {
					delete(u.gateways, id)
					log.Info().Str("gateway", id).Msg("网关离线，清理缓存")
				}
			}
			u.mu.Unlock()
		}
	}
}

// updateGatewayInDB 更新数据库中的网关状态
func (u *UDPPacketForwarder) updateGatewayInDB(gatewayID string) {
	if u.store == nil {
		log.Error().Msg("存储接口未初始化")
		return
	}

	ctx := context.Background()

	// 解析网关ID
	var gwID models.EUI64

	// 从十六进制字符串解析
	if len(gatewayID) != 16 {
		log.Error().Str("gateway", gatewayID).Msg("无效的网关ID长度")
		return
	}

	// 解析十六进制字符串到字节数组
	for i := 0; i < 8; i++ {
		var b byte
		_, err := fmt.Sscanf(gatewayID[i*2:i*2+2], "%02x", &b)
		if err != nil {
			log.Error().Err(err).Str("gateway", gatewayID).Msg("解析网关ID失败")
			return
		}
		gwID[i] = b
	}

	// 获取或创建网关
	gateway, err := u.store.GetGateway(ctx, lorawan.EUI64(gwID))
	if err != nil {
		if err == storage.ErrNotFound {
			// 网关不存在，创建新网关
			log.Info().Str("gateway", gatewayID).Msg("自动注册新网关")

			gateway = &models.Gateway{
				GatewayID:   gwID,
				Name:        fmt.Sprintf("Gateway %s", gatewayID[:8]),
				Description: "Auto-registered gateway",
				TenantModel: models.TenantModel{
					TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				},
			}

			// 设置初始时间
			now := time.Now()
			gateway.FirstSeenAt = &now
			gateway.LastSeenAt = &now

			if err := u.store.CreateGateway(ctx, gateway); err != nil {
				log.Error().Err(err).Str("gateway", gatewayID).Msg("创建网关失败")
				return
			}

			log.Info().Str("gateway", gatewayID).Msg("网关已自动注册")
		} else {
			log.Error().Err(err).Str("gateway", gatewayID).Msg("获取网关失败")
			return
		}
	} else {
		// 更新最后在线时间
		now := time.Now()
		gateway.LastSeenAt = &now

		// 如果是第一次上线，记录首次上线时间
		if gateway.FirstSeenAt == nil {
			gateway.FirstSeenAt = &now
		}

		// 更新网关
		if err := u.store.UpdateGateway(ctx, gateway); err != nil {
			log.Error().Err(err).Str("gateway", gatewayID).Msg("更新网关状态失败")
		} else {
			log.Debug().Str("gateway", gatewayID).Msg("网关状态已更新到数据库")
		}
	}
}

// 辅助函数
func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getUint64(m map[string]interface{}, key string) uint64 {
	if v, ok := m[key].(float64); ok {
		return uint64(v)
	}
	return 0
}

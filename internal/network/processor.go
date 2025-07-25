package network

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lorawan-server/lorawan-server-pro/internal/config"
	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
	"github.com/lorawan-server/lorawan-server-pro/pkg/crypto"
	"github.com/lorawan-server/lorawan-server-pro/pkg/lorawan"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

// 添加时间戳追踪器
type TimestampTracker struct {
	mu sync.RWMutex
	// 每个网关的时间戳历史
	gatewayTimestamps map[string]*GatewayTimestampInfo
}
type GatewayTimestampInfo struct {
	LastUplink     uint64    // 最后一次上行时间戳
	LastUplinkTime time.Time // 最后一次上行的系统时间
	TimestampDrift int64     // 时间戳漂移量
	ResetCount     int       // 检测到的重置次数
	IsReliable     bool      // 时间戳是否可靠
	WarmupCount    int       // 新增：预热计数
}

// 添加简单的缓存实现
type CacheItem struct {
	Value  interface{}
	Expiry time.Time
}

type SimpleCache struct {
	mu    sync.RWMutex
	items map[string]CacheItem
}

func NewSimpleCache() *SimpleCache {
	c := &SimpleCache{
		items: make(map[string]CacheItem),
	}
	// 启动清理协程
	go c.cleanup()
	return c
}

func (c *SimpleCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = CacheItem{
		Value:  value,
		Expiry: time.Now().Add(ttl),
	}
}

func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.Expiry) {
		return nil, false
	}

	return item.Value, true
}

func (c *SimpleCache) cleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.Expiry) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// 添加设备接收信息缓存结构
type DeviceRxInfo struct {
	GatewayID string
	RxInfo    map[string]interface{}
	Timestamp time.Time
}

// Processor 处理 LoRaWAN 数据包
type Processor struct {
	nc         *nats.Conn
	store      storage.Store
	region     *lorawan.RegionConfiguration
	macHandler *MACCommandHandler
	config     *config.Config

	// 添加设备上行缓存，用于下行时确定网关
	deviceRxCache map[lorawan.EUI64]*DeviceRxInfo
	rxCacheMutex  sync.RWMutex

	// 添加去重缓存
	joinCache        *SimpleCache
	timestampTracker *TimestampTracker
}

// 修改NewProcessor构造函数
func NewProcessor(nc *nats.Conn, store storage.Store, cfg *config.Config) *Processor {
	regionName := cfg.Network.Band
	if regionName == "" {
		regionName = "CN470"
	}

	return &Processor{
		nc:            nc,
		store:         store,
		region:        lorawan.GetRegionConfiguration(regionName),
		macHandler:    NewMACCommandHandler(store, regionName),
		config:        cfg,
		deviceRxCache: make(map[lorawan.EUI64]*DeviceRxInfo),
		joinCache:     NewSimpleCache(), // 使用简单缓存
		timestampTracker: &TimestampTracker{
			gatewayTimestamps: make(map[string]*GatewayTimestampInfo),
		},
	}
}

// UpdateAndCheck 更新并检查时间戳可靠性 - 优化版
func (t *TimestampTracker) UpdateAndCheck(gatewayID string, uplinkTmst uint64, isJoinAccept bool) (reliable bool, drift int64) {
	// JOIN ACCEPT 总是返回可靠，不需要预热
	if isJoinAccept {
		return true, 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// 获取或创建网关信息
	info, exists := t.gatewayTimestamps[gatewayID]
	if !exists {
		// 首次记录
		info = &GatewayTimestampInfo{
			LastUplink:     uplinkTmst,
			LastUplinkTime: now,
			IsReliable:     false,
			TimestampDrift: 0,
			ResetCount:     0,
			WarmupCount:    0,
		}
		t.gatewayTimestamps[gatewayID] = info

		log.Debug().
			Str("gateway", gatewayID).
			Uint64("initialTmst", uplinkTmst).
			Msg("网关首次记录")

		return false, 0
	}

	// 预热逻辑：普通数据需要至少3次稳定的时间戳
	if info.WarmupCount < 3 {
		info.WarmupCount++

		log.Debug().
			Str("gateway", gatewayID).
			Int("warmupCount", info.WarmupCount).
			Msg("网关时间戳预热中")

		// 更新时间戳信息
		info.LastUplink = uplinkTmst
		info.LastUplinkTime = now

		if info.WarmupCount < 3 {
			return false, 0
		}

		// 预热完成
		log.Info().
			Str("gateway", gatewayID).
			Msg("网关预热完成")

		info.IsReliable = true
		return true, 0
	}

	// 计算经过的时间
	timeDiff := now.Sub(info.LastUplinkTime)
	if timeDiff < time.Second {
		// 时间太短，不足以判断
		return info.IsReliable, info.TimestampDrift
	}

	// 计算时间戳差异
	timestampDiff := int64(uplinkTmst) - int64(info.LastUplink)
	expectedDiff := timeDiff.Microseconds()

	// 计算漂移
	drift = timestampDiff - expectedDiff

	// 阈值定义
	const DRIFT_THRESHOLD = int64(500000)    // 500ms漂移阈值
	const RESET_THRESHOLD = int64(-10000000) // 10秒回退表示重置

	// 判断可靠性
	if timestampDiff < RESET_THRESHOLD {
		// 时间戳大幅回退，可能是网关重启
		info.ResetCount++
		info.IsReliable = false
		info.WarmupCount = 0 // 重置预热计数

		log.Warn().
			Str("gateway", gatewayID).
			Uint64("current", uplinkTmst).
			Uint64("last", info.LastUplink).
			Int("resetCount", info.ResetCount).
			Msg("检测到网关时间戳重置")
	} else if math.Abs(float64(drift)) > float64(DRIFT_THRESHOLD) {
		// 漂移过大
		if info.IsReliable {
			log.Warn().
				Str("gateway", gatewayID).
				Int64("drift", drift).
				Msg("时间戳漂移超过阈值")
		}
		info.IsReliable = false
	} else {
		// 时间戳稳定
		if !info.IsReliable && info.ResetCount > 0 {
			log.Info().
				Str("gateway", gatewayID).
				Msg("时间戳恢复稳定")
		}
		info.IsReliable = true
		info.TimestampDrift = drift
	}

	// 更新记录
	info.LastUplink = uplinkTmst
	info.LastUplinkTime = now

	return info.IsReliable, drift
}

func (p *Processor) isTimestampTooOld(tmst uint64) bool {
	// 网关刚启动时的时间戳通常很小
	const STARTUP_THRESHOLD = uint64(60000000) // 60秒

	if tmst < STARTUP_THRESHOLD {
		log.Debug().
			Uint64("tmst", tmst).
			Msg("时间戳太小，可能网关刚启动")
		return true
	}

	// 新增：如果时间戳超过1小时，也认为异常
	const ONE_HOUR = uint64(3600000000) // 1小时的微秒数
	if tmst > ONE_HOUR {
		log.Debug().
			Uint64("tmst", tmst).
			Msg("时间戳超过1小时，可能异常")
		return true
	}

	// 检查时间戳是否在合理范围
	const MAX_REASONABLE = uint64(4294967295)
	if tmst > MAX_REASONABLE {
		return true
	}

	// 检查是否接近溢出
	const NEAR_OVERFLOW_THRESHOLD = MAX_REASONABLE - uint64(10000000)
	if tmst > NEAR_OVERFLOW_THRESHOLD {
		log.Debug().
			Uint64("tmst", tmst).
			Msg("时间戳接近溢出")
		return true
	}

	return false
}

// getTimestampAge 获取时间戳的年龄（仅用于调试）
func (p *Processor) getTimestampAge(tmst uint64) time.Duration {
	// 这是一个估算，因为我们无法获取网关的确切时间
	// 基于时间戳值进行粗略估计
	return time.Duration(tmst) * time.Microsecond
}

// Start 启动处理器 - 修改：添加下行订阅
func (p *Processor) Start(ctx context.Context) error {
	// 打印配置摘要
	p.config.PrintConfigSummary()

	// 验证CN470配置
	if err := p.validateCN470Configuration(); err != nil {
		return fmt.Errorf("CN470配置验证失败: %w", err)
	}

	// 订阅网关接收数据（上行）
	subRx, err := p.nc.Subscribe("gateway.*.rx", p.handleGatewayRX)
	if err != nil {
		return fmt.Errorf("订阅上行失败: %w", err)
	}

	// 添加：订阅设备下行请求
	subTx, err := p.nc.Subscribe("ns.device.*.tx", p.handleDeviceDownlinkRequest)
	if err != nil {
		return fmt.Errorf("订阅下行失败: %w", err)
	}
	// 启动时间戳清理
	go p.timestampTracker.StartCleanup(ctx)
	log.Info().
		Str("region", p.region.Name).
		Msg("Network Server 处理器启动，已订阅上行和下行消息")

	<-ctx.Done()
	subRx.Unsubscribe()
	subTx.Unsubscribe()
	return nil
}

// 添加新函数：处理设备下行请求
func (p *Processor) handleDeviceDownlinkRequest(msg *nats.Msg) {
	// 从主题中提取 DevEUI
	parts := strings.Split(msg.Subject, ".")
	if len(parts) != 4 {
		log.Error().Str("subject", msg.Subject).Msg("无效的下行主题格式")
		return
	}
	devEUIStr := parts[2]

	log.Info().
		Str("subject", msg.Subject).
		Str("devEUI", devEUIStr).
		Msg("收到设备下行请求")

	// 解析消息
	var downReq struct {
		DevEUI    string `json:"devEUI"`
		FPort     uint8  `json:"fPort"`
		Data      []byte `json:"data"`
		Confirmed bool   `json:"confirmed"`
		ID        string `json:"id"`
	}

	if err := json.Unmarshal(msg.Data, &downReq); err != nil {
		log.Error().Err(err).Msg("解析下行请求失败")
		return
	}

	// 解析 DevEUI
	devEUIBytes, err := hex.DecodeString(devEUIStr)
	if err != nil {
		log.Error().Err(err).Msg("解析 DevEUI 失败")
		return
	}

	var devEUI lorawan.EUI64
	copy(devEUI[:], devEUIBytes)

	ctx := context.Background()

	// 获取设备会话
	session, err := p.store.GetDeviceSession(ctx, devEUI)
	if err != nil {
		log.Error().Err(err).Str("devEUI", devEUIStr).Msg("获取设备会话失败")
		return
	}

	// 获取最近使用的网关信息
	gatewayID := p.getLastGatewayForDevice(devEUI)
	if gatewayID == "" {
		log.Error().Str("devEUI", devEUIStr).Msg("无法找到设备的网关")
		return
	}

	// 获取最近的上行信息（用于计算下行参数）
	lastRxInfo := p.getLastRxInfoForDevice(devEUI)
	if lastRxInfo == nil {
		log.Error().Str("devEUI", devEUIStr).Msg("无法找到设备的上行信息")
		return
	}

	// 构建下行帧
	var mtype lorawan.MType
	if downReq.Confirmed {
		mtype = lorawan.ConfirmedDataDown
	} else {
		mtype = lorawan.UnconfirmedDataDown
	}

	// 构建 MAC payload
	macPayload := lorawan.MACPayload{
		FHDR: lorawan.FHDR{
			DevAddr: lorawan.DevAddr(session.DevAddr),
			FCtrl: lorawan.FCtrl{
				ADR: false,
				ACK: false,
			},
			FCnt: uint16(session.NFCntDown & 0xFFFF),
		},
		FPort: &downReq.FPort,
	}

	// 加密数据（使用 DecryptFRMPayload，因为在 LoRaWAN 中加密和解密是相同操作）
	if downReq.FPort == 0 {
		// Port 0: 使用 NwkSEncKey（MAC 命令）
		key, _ := hex.DecodeString(session.NwkSEncKey)
		macPayload.FRMPayload, _ = crypto.DecryptFRMPayload(
			key,
			false,
			[4]byte(session.DevAddr),
			session.NFCntDown,
			downReq.Data,
		)
	} else {
		// 应用数据：使用 AppSKey
		key, _ := hex.DecodeString(session.AppSKey)
		macPayload.FRMPayload, _ = crypto.DecryptFRMPayload(
			key,
			false,
			[4]byte(session.DevAddr),
			session.NFCntDown,
			downReq.Data,
		)
	}

	// 序列化 MAC payload
	macBytes, _ := macPayload.Marshal(mtype, false)

	// 构建 PHY payload
	phyPayload := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: mtype,
			Major: lorawan.LoRaWAN1_0,
		},
		MACPayload: macBytes,
	}

	// 设置 MIC
	sNwkSIntKey, _ := hex.DecodeString(session.SNwkSIntKey)
	var key lorawan.AES128Key
	copy(key[:], sNwkSIntKey)
	phyPayload.SetDownlinkDataMIC(lorawan.LoRaWAN1_0, session.NFCntDown, key)

	// 更新帧计数器
	session.NFCntDown++
	p.store.SaveDeviceSession(ctx, session)

	log.Info().
		Str("devEUI", devEUIStr).
		Str("gatewayID", gatewayID).
		Uint32("fcnt", session.NFCntDown-1).
		Uint8("fPort", downReq.FPort).
		Int("dataLen", len(downReq.Data)).
		Msg("调度设备下行")

	// 计算下行延迟
	delay := time.Duration(p.config.CN470.RXWindows.RX1Delay) * time.Second

	// 发送到网关
	p.scheduleDownlink(gatewayID, lorawan.DevAddr(session.DevAddr), phyPayload, lastRxInfo, delay)
}

// handleGatewayRX 处理网关接收数据
func (p *Processor) handleGatewayRX(msg *nats.Msg) {
	var rxMsg struct {
		GatewayID string                 `json:"gatewayID"`
		RXPK      map[string]interface{} `json:"rxpk"`
		Context   string                 `json:"context"` // ✅ 添加 context
		Timestamp int64                  `json:"timestamp"`
	}

	if err := json.Unmarshal(msg.Data, &rxMsg); err != nil {
		log.Error().Err(err).Msg("解析 RX 消息失败")
		return
	}

	// 获取 PHY payload
	dataStr, ok := rxMsg.RXPK["data"].(string)
	if !ok {
		return
	}

	phyPayloadBytes, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		log.Error().Err(err).Msg("解码 PHY payload 失败")
		return
	}

	// 解析 PHY payload
	var phyPayload lorawan.PHYPayload
	if err := phyPayload.UnmarshalBinary(phyPayloadBytes); err != nil {
		log.Error().Err(err).Msg("解析 PHY payload 失败")
		return
	}
	rxInfo := rxMsg.RXPK
	if rxMsg.Context != "" {
		rxInfo["context"] = rxMsg.Context // ✅ 传递 context
	}
	// 根据消息类型处理
	switch phyPayload.MHDR.MType {
	case lorawan.JoinRequest:
		p.handleJoinRequest(&phyPayload, rxMsg.GatewayID, rxInfo)
	case lorawan.UnconfirmedDataUp, lorawan.ConfirmedDataUp:
		p.handleDataUp(&phyPayload, rxMsg.GatewayID, rxInfo)
	default:
		log.Warn().
			Uint8("mtype", uint8(phyPayload.MHDR.MType)).
			Msg("未处理的消息类型")
	}
}

func reverseEUI64(eui lorawan.EUI64) lorawan.EUI64 {
	var reversed lorawan.EUI64
	for i := 0; i < 8; i++ {
		reversed[i] = eui[7-i]
	}
	return reversed
}

// handleJoinRequest 处理入网请求
// handleJoinRequest 处理入网请求 - ChirpStack 风格实现
func (p *Processor) handleJoinRequest(phy *lorawan.PHYPayload, gatewayID string, rxInfo map[string]interface{}) {
	// 解析 Join Request
	var joinReq lorawan.JoinRequestPayload
	if err := joinReq.UnmarshalBinary(phy.MACPayload); err != nil {
		log.Error().Err(err).Msg("解析 Join Request 失败")
		return
	}

	// JOIN请求去重
	joinKey := fmt.Sprintf("join_%s_%s",
		joinReq.DevEUI.String(),
		hex.EncodeToString(joinReq.DevNonce[:]),
	)

	if _, found := p.joinCache.Get(joinKey); found {
		log.Debug().
			Str("devEUI", joinReq.DevEUI.String()).
			Msg("忽略重复的 JOIN REQUEST")
		return
	}

	// 标记已处理（10秒过期）
	p.joinCache.Set(joinKey, true, 10*time.Second)

	log.Info().
		Str("devEUI", joinReq.DevEUI.String()).
		Str("joinEUI", joinReq.JoinEUI.String()).
		Hex("devNonce", joinReq.DevNonce[:]).
		Msg("收到 JOIN REQUEST")

	ctx := context.Background()

	// 获取设备密钥
	keys, err := p.store.GetDeviceKeys(ctx, joinReq.DevEUI)
	if err != nil {
		// 尝试反序DevEUI
		reversedDevEUI := reverseEUI64(joinReq.DevEUI)
		keys, err = p.store.GetDeviceKeys(ctx, reversedDevEUI)
		if err != nil {
			log.Error().
				Err(err).
				Str("devEUI", joinReq.DevEUI.String()).
				Msg("获取设备密钥失败")
			return
		}
		joinReq.DevEUI = reversedDevEUI
	}

	// 验证 MIC
	appKeyBytes, err := hex.DecodeString(keys.AppKey)
	if err != nil {
		log.Error().Err(err).Msg("解析AppKey失败")
		return
	}
	// 添加这行日志来查看实际使用的AppKey
	log.Info().
		Str("devEUI", joinReq.DevEUI.String()).
		Str("usedAppKey", keys.AppKey). // ← 这里会显示您使用的AppKey
		Msg("正在使用的AppKey")
	var appKey lorawan.AES128Key
	copy(appKey[:], appKeyBytes)

	micOK, err := phy.ValidateUplinkJoinMIC(appKey)
	if err != nil || !micOK {
		log.Error().
			Str("devEUI", joinReq.DevEUI.String()).
			Bool("micOK", micOK).
			Msg("JOIN REQUEST MIC验证失败")
		return
	}

	log.Info().
		Str("devEUI", joinReq.DevEUI.String()).
		Msg("✅ JOIN REQUEST MIC验证成功")

	// 获取设备信息
	device, err := p.store.GetDevice(ctx, joinReq.DevEUI)
	if err != nil {
		log.Error().Err(err).Msg("获取设备信息失败")
		return
	}

	// 生成网络参数
	devAddr := p.generateDevAddr()
	joinNonce := p.generateJoinNonce()
	netID := [3]byte{0x01, 0xa6, 0xdb}

	// 生成会话密钥
	appSKey := p.deriveAppSKey(joinNonce, netID, joinReq.DevNonce, keys.AppKey)
	fNwkSIntKey := p.deriveFNwkSIntKey(joinNonce, netID, joinReq.DevNonce, keys.NwkKey)
	sNwkSIntKey := p.deriveSNwkSIntKey(joinNonce, netID, joinReq.DevNonce, keys.NwkKey)
	nwkSEncKey := p.deriveNwkSEncKey(joinNonce, netID, joinReq.DevNonce, keys.NwkKey)
	// ✅ 增强：清理设备相关的所有缓存
	// 清理设备接收缓存
	p.rxCacheMutex.Lock()
	delete(p.deviceRxCache, joinReq.DevEUI)
	// 也清理可能的反序DevEUI
	reversedDevEUI := reverseEUI64(joinReq.DevEUI)
	delete(p.deviceRxCache, reversedDevEUI)
	p.rxCacheMutex.Unlock()

	log.Debug().
		Str("devEUI", joinReq.DevEUI.String()).
		Msg("清理设备缓存")
	// ✅ 新增：清理旧会话
	// 每次新的 JOIN 成功时，删除旧的设备会话
	err = p.store.DeleteDeviceSession(ctx, joinReq.DevEUI)
	if err != nil && err != storage.ErrNotFound {
		log.Warn().
			Err(err).
			Str("devEUI", joinReq.DevEUI.String()).
			Msg("删除旧设备会话失败，但继续处理JOIN")
	} else if err == nil {
		log.Info().
			Str("devEUI", joinReq.DevEUI.String()).
			Msg("✅ 已清理旧设备会话")
	}

	// ✅ 新增：重置设备的帧计数器
	device.FCntUp = 0
	device.NFCntDown = 0
	device.AFCntDown = 0
	newDevAddr := models.DevAddr(devAddr)
	device.DevAddr = &newDevAddr // 更新新的DevAddr

	if err := p.store.UpdateDevice(ctx, device); err != nil {
		log.Error().
			Err(err).
			Str("devEUI", joinReq.DevEUI.String()).
			Msg("重置设备帧计数器失败")
	} else {
		log.Info().
			Str("devEUI", joinReq.DevEUI.String()).
			Str("newDevAddr", devAddr.String()).
			Msg("✅ 设备帧计数器已重置")
	}

	// 创建设备会话
	session := &models.DeviceSession{
		DevEUI:      models.EUI64(joinReq.DevEUI),
		DevAddr:     models.DevAddr(devAddr),
		JoinEUI:     models.EUI64(joinReq.JoinEUI),
		AppSKey:     hex.EncodeToString(appSKey[:]),
		FNwkSIntKey: hex.EncodeToString(fNwkSIntKey[:]),
		SNwkSIntKey: hex.EncodeToString(sNwkSIntKey[:]),
		NwkSEncKey:  hex.EncodeToString(nwkSEncKey[:]),
		FCntUp:      0, // ✅ 明确设置为0
		NFCntDown:   0, // ✅ 明确设置为0
		AFCntDown:   0, // ✅ 明确设置为0
		ConfFCnt:    0, // ✅ 明确设置为0
		RX1Delay:    5, // 标准5秒延迟
		RX2DR:       uint8(p.region.DefaultRX2DR),
		RX2Freq:     p.getRegionRX2Freq(),
	}

	if err := p.store.SaveDeviceSession(ctx, session); err != nil {
		log.Error().Err(err).Msg("保存设备会话失败")
		return
	}

	// 更新设备网关缓存
	p.updateDeviceRxCache(joinReq.DevEUI, gatewayID, rxInfo)

	// 构建 Join Accept
	joinAccept := lorawan.JoinAcceptPayload{
		JoinNonce: joinNonce,
		NetID:     netID,
		DevAddr:   devAddr,
		DLSettings: lorawan.DLSettings{
			RX1DROffset: 0,
			RX2DataRate: uint8(p.region.DefaultRX2DR),
		},
		RxDelay: 5, // 5 秒延迟
	}

	// CN470 添加 CFList
	if p.region.Name == "CN470" && p.shouldUseCFList() {
		cfList := p.generateCN470CFList()
		if len(cfList) == 16 {
			joinAccept.CFList = cfList
			log.Info().
				Hex("cfList", cfList).
				Msg("✅ 添加 CN470 CFList")
		} else {
			log.Error().
				Int("len", len(cfList)).
				Msg("❌ CFList 长度错误")
		}
	}

	// 在生成JOIN ACCEPT后，序列化前添加
	log.Info().
		Hex("joinNonce", joinNonce[:]).
		Hex("netID", netID[:]).
		Str("devAddr", devAddr.String()).
		Uint8("rx1DROffset", joinAccept.DLSettings.RX1DROffset).
		Uint8("rx2DataRate", joinAccept.DLSettings.RX2DataRate).
		Uint8("rxDelay", joinAccept.RxDelay).
		Bool("hasCFList", len(joinAccept.CFList) > 0).
		Msg("JOIN ACCEPT参数（加密前）")

	// 序列化并加密 Join Accept
	joinAcceptBytes, err := joinAccept.MarshalBinary()
	if err != nil {
		log.Error().Err(err).Msg("序列化JOIN ACCEPT失败")
		return
	}

	log.Info().
		Hex("marshaledBytes", joinAcceptBytes).
		Int("len", len(joinAcceptBytes)).
		Msg("JOIN ACCEPT序列化后（加密前）")

	// 构建 PHY payload
	acceptPHY := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: lorawan.JoinAccept,
			Major: lorawan.LoRaWAN1_0,
		},
		MACPayload: joinAcceptBytes,
	}

	// 使用修改后的方法，传入 JOIN REQUEST 的参数
	if err := acceptPHY.SetJoinAcceptMIC(appKey); err != nil {
		log.Error().Err(err).Msg("设置JOIN ACCEPT MIC失败")
		return
	}
	// 调试：记录加密前的状态
	log.Info().
		Hex("joinAcceptPlain", joinAcceptBytes).
		Hex("micPlain", acceptPHY.MIC[:]).
		Int("plainLen", len(joinAcceptBytes)).
		Msg("JOIN ACCEPT 加密前")
	// 加密 JOIN ACCEPT payload（包括MIC）
	if err := acceptPHY.EncryptJoinAcceptPayload(appKey); err != nil {
		log.Error().Err(err).Msg("加密JOIN ACCEPT失败")
		return
	}

	// 调试日志
	phyBytes, _ := acceptPHY.MarshalBinary()
	log.Info().
		Str("devEUI", joinReq.DevEUI.String()).
		Str("devAddr", devAddr.String()).
		Str("joinAcceptHex", hex.EncodeToString(phyBytes)).
		Int("size", len(phyBytes)).
		Str("encryptedBase64", base64.StdEncoding.EncodeToString(phyBytes)).
		Hex("encryptedHex", phyBytes).
		Msg("生成 JOIN ACCEPT")
	// 在生成JOIN ACCEPT后添加
	log.Info().
		Str("devEUI", joinReq.DevEUI.String()).
		Str("devAddr", devAddr.String()).
		Hex("joinNonce", joinNonce[:]).
		Hex("netID", netID[:]).
		Uint8("rxDelay", joinAccept.RxDelay).
		Uint8("rx1DROffset", joinAccept.DLSettings.RX1DROffset).
		Uint8("rx2DataRate", joinAccept.DLSettings.RX2DataRate).
		Bool("hasCFList", len(joinAccept.CFList) > 0).
		Msg("JOIN ACCEPT 参数详情")
	var joinAcceptDelay time.Duration
	if p.config.CN470.RXWindows.JoinAcceptDelay1 > 0 {
		joinAcceptDelay = time.Duration(p.config.CN470.RXWindows.JoinAcceptDelay1) * time.Second
	} else {
		// 默认使用标准的5秒延迟
		joinAcceptDelay = 5 * time.Second
	}
	// 发送 Join Accept - 使用标准5秒延迟
	p.scheduleDownlink(gatewayID, devAddr, acceptPHY, rxInfo, joinAcceptDelay)

	// 如果启用了 RX2 备份
	if p.shouldScheduleRX2() {
		go func() {
			// 等待一小段时间避免竞争
			time.Sleep(100 * time.Millisecond)

			// RX2 使用配置的延迟
			var rx2Delay time.Duration
			if p.config.CN470.RXWindows.JoinAcceptDelay2 > 0 {
				rx2Delay = time.Duration(p.config.CN470.RXWindows.JoinAcceptDelay2) * time.Second
			} else {
				// 默认6秒（5秒 + 1秒）
				rx2Delay = 6 * time.Second
			}

			// RX2 参数
			rx2Info := make(map[string]interface{})
			for k, v := range rxInfo {
				rx2Info[k] = v
			}
			rx2Info["freq"] = float64(p.getRegionRX2Freq()) / 1000000.0
			rx2Info["datr"] = p.getDRString(uint8(p.region.DefaultRX2DR))

			log.Debug().
				Float64("rx2Freq", rx2Info["freq"].(float64)).
				Str("rx2DR", rx2Info["datr"].(string)).
				Dur("rx2Delay", rx2Delay).
				Msg("调度 RX2 JOIN ACCEPT")

			p.scheduleDownlink(gatewayID, devAddr, acceptPHY, rx2Info, rx2Delay)
		}()
	}

	// 发布入网事件
	p.publishJoinEvent(device, devAddr)

	log.Info().
		Str("devEUI", joinReq.DevEUI.String()).
		Str("devAddr", devAddr.String()).
		Msg("✅ JOIN 处理完成")
}

// 辅助函数：是否应该使用 CFList
func (p *Processor) shouldUseCFList() bool {
	// 基于配置或设备特性判断
	// E78-470LN22S 可能不支持 CFList
	if os.Getenv("DISABLE_CFLIST") == "true" {
		return false
	}
	return true
}

// 辅助函数：是否应该调度 RX2
func (p *Processor) shouldScheduleRX2() bool {
	// 可以基于配置决定
	return os.Getenv("ENABLE_RX2_JOIN") == "true"
}

// handleDataUp 处理上行数据 - 修改：添加缓存更新和上行帧保存
func (p *Processor) handleDataUp(phy *lorawan.PHYPayload, gatewayID string, rxInfo map[string]interface{}) {
	// 解析 MAC payload
	var macPayload lorawan.MACPayload
	isUplink := phy.MHDR.MType == lorawan.UnconfirmedDataUp || phy.MHDR.MType == lorawan.ConfirmedDataUp
	if err := macPayload.Unmarshal(phy.MACPayload, phy.MHDR.MType, isUplink); err != nil {
		log.Error().Err(err).Msg("解析 MAC payload 失败")
		return
	}

	// ✅ 上行数据去重
	uplinkKey := fmt.Sprintf("up_%s_%d_%s",
		macPayload.FHDR.DevAddr.String(),
		macPayload.FHDR.FCnt,
		hex.EncodeToString(phy.MIC[:]),
	)
	if _, found := p.joinCache.Get(uplinkKey); found {
		log.Debug().
			Str("devAddr", macPayload.FHDR.DevAddr.String()).
			Uint16("fcnt", macPayload.FHDR.FCnt).
			Msg("忽略重复的上行数据")
		return
	}

	// 标记已处理（5秒过期）
	p.joinCache.Set(uplinkKey, true, 30*time.Second)

	ctx := context.Background()

	// 通过 DevAddr 查找设备会话
	sessions, err := p.store.GetDeviceSessionByDevAddr(ctx, macPayload.FHDR.DevAddr)
	if err != nil || len(sessions) == 0 {
		log.Warn().
			Str("devAddr", macPayload.FHDR.DevAddr.String()).
			Msg("未找到设备会话")
		return
	}

	// 验证 MIC 并找到正确的设备
	var validSession *models.DeviceSession
	for _, session := range sessions {
		fNwkSIntKey, _ := hex.DecodeString(session.FNwkSIntKey)
		_, _ = hex.DecodeString(session.SNwkSIntKey)

		var key lorawan.AES128Key
		copy(key[:], fNwkSIntKey)

		valid, err := phy.ValidateUplinkDataMIC(
			lorawan.LoRaWAN1_0,
			session.FCntUp,
			0, 0,
			key,
			lorawan.AES128Key{},
		)

		if err == nil && valid {
			validSession = session
			break
		}
	}

	if validSession == nil {
		log.Warn().Msg("MIC 验证失败")
		return
	}

	// 更新设备网关缓存
	p.updateDeviceRxCache(lorawan.EUI64(validSession.DevEUI), gatewayID, rxInfo)

	// 更新帧计数器
	fullFCnt := lorawan.GetFullFCnt(validSession.FCntUp, macPayload.FHDR.FCnt)

	// 特殊处理：如果设备发送 fcnt=0 且服务器期望 fcnt=1
	// 这可能是设备重启了
	if fullFCnt == 0 && validSession.FCntUp == 1 {
		log.Warn().
			Str("devEUI", hex.EncodeToString(validSession.DevEUI[:])).
			Uint32("received", fullFCnt).
			Uint32("expected", validSession.FCntUp).
			Msg("检测到设备可能重启，接受 fcnt=0")

		// 重置会话的帧计数器
		validSession.FCntUp = 0
		validSession.FCntDown = 0
		validSession.NFCntDown = 0
		validSession.AFCntDown = 0
	} else if fullFCnt < validSession.FCntUp {
		// 其他情况下，如果收到的帧计数器小于期望值
		log.Warn().
			Uint32("received", fullFCnt).
			Uint32("expected", validSession.FCntUp+1).
			Msg("帧计数器无效")
		return
	}

	// 正常情况：帧计数器必须大于上次的值
	if fullFCnt == validSession.FCntUp && fullFCnt != 0 {
		log.Warn().
			Uint32("received", fullFCnt).
			Msg("收到重复的帧计数器")
		return
	}

	validSession.FCntUp = fullFCnt

	// 解密 FRM payload
	var data []byte
	if macPayload.FPort != nil && len(macPayload.FRMPayload) > 0 {
		var key []byte
		if *macPayload.FPort == 0 {
			// MAC 命令使用 NwkSEncKey
			key, _ = hex.DecodeString(validSession.NwkSEncKey)
		} else {
			// 应用数据使用 AppSKey
			key, _ = hex.DecodeString(validSession.AppSKey)
		}

		data, err = crypto.DecryptFRMPayload(
			key,
			true,
			[4]byte(validSession.DevAddr),
			fullFCnt,
			macPayload.FRMPayload,
		)
		if err != nil {
			log.Error().Err(err).Msg("解密失败")
			return
		}
	}

	// 处理 MAC 命令
	var macCommands []lorawan.MACCommand
	if len(macPayload.FHDR.FOpts) > 0 {
		macCommands, _ = lorawan.ParseMACCommands(true, macPayload.FHDR.FOpts)
	}
	if macPayload.FPort != nil && *macPayload.FPort == 0 && len(data) > 0 {
		moreCmds, _ := lorawan.ParseMACCommands(true, data)
		macCommands = append(macCommands, moreCmds...)
	}

	// 处理 MAC 命令
	downlinkCmds := p.macHandler.HandleUplink(validSession, macCommands)

	// 更新设备会话
	p.store.SaveDeviceSession(ctx, validSession)

	// 获取设备信息
	device, err := p.store.GetDevice(ctx, lorawan.EUI64(validSession.DevEUI))
	if err != nil {
		log.Error().
			Err(err).
			Str("devEUI", hex.EncodeToString(validSession.DevEUI[:])).
			Msg("获取设备信息失败")
		return
	}

	// ✅ 新增：保存上行帧到数据库
	phyBytes, _ := phy.MarshalBinary()
	uplinkFrame := &models.UplinkFrame{
		DevEUI:        models.EUI64(validSession.DevEUI),    // 类型转换
		DevAddr:       models.DevAddr(validSession.DevAddr), // 类型转换
		ApplicationID: device.ApplicationID,
		PHYPayload:    phyBytes,
		FCnt:          uint32(fullFCnt), // 转换为 uint32
		FPort:         macPayload.FPort, // 直接使用，已经是 *uint8
		DR:            getInt(rxInfo, "dr"),
		ADR:           macPayload.FHDR.FCtrl.ADR,
		Data:          data,
		Confirmed:     phy.MHDR.MType == lorawan.ConfirmedDataUp,
		TXInfo:        nil,
		RXInfo: []map[string]interface{}{
			{
				"gatewayID": gatewayID,
				"rssi":      rxInfo["rssi"],
				"snr":       rxInfo["lsnr"],
				"frequency": rxInfo["freq"],
				"timestamp": rxInfo["tmst"],
				"channel":   rxInfo["chan"],
				"rfChain":   rxInfo["rfch"],
			},
		},
		ReceivedAt: time.Now(),
	}

	if err := p.store.SaveUplinkFrame(ctx, uplinkFrame); err != nil {
		log.Error().
			Err(err).
			Str("devEUI", hex.EncodeToString(validSession.DevEUI[:])).
			Msg("保存上行帧失败")
	}

	// 发布上行数据
	p.publishUplinkData(validSession, macPayload, data, rxInfo, device.ApplicationID)

	// ✅ 专门处理 ConfirmedDataUp - 关键修复
	if phy.MHDR.MType == lorawan.ConfirmedDataUp {
		log.Info().
			Str("devEUI", hex.EncodeToString(validSession.DevEUI[:])).
			Str("gatewayID", gatewayID).
			Uint32("fCnt", fullFCnt).
			Uint32("currentNFCntDown", validSession.NFCntDown).
			Msg("收到 ConfirmedDataUp，发送 ACK")

		// ✅ 关键修复：先创建ACK再更新计数器
		ackPHY := p.createACKResponse(validSession, downlinkCmds)

		// ✅ 然后更新下行计数器
		validSession.NFCntDown++

		// ✅ 立即保存设备会话
		if err := p.store.SaveDeviceSession(ctx, validSession); err != nil {
			log.Error().Err(err).Msg("保存设备会话失败")
			return
		}

		// ✅ 使用修复后的RX1延迟
		rx1Delay := 5 * time.Second // 标准1秒延迟
		if p.config.CN470.RXWindows.RX1Delay > 0 {
			rx1Delay = time.Duration(p.config.CN470.RXWindows.RX1Delay) * time.Second
		}

		lorawanDevAddr := lorawan.DevAddr(validSession.DevAddr)
		p.scheduleDownlink(gatewayID, lorawanDevAddr, ackPHY, rxInfo, rx1Delay)

		log.Info().
			Str("devEUI", hex.EncodeToString(validSession.DevEUI[:])).
			Str("gatewayID", gatewayID).
			Uint32("downlinkFCnt", validSession.NFCntDown-1). // ✅ 使用实际ACK的计数器
			Dur("rx1Delay", rx1Delay).
			Msg("✅ ACK 已调度")

		return // 已处理ACK，直接返回
	}

	// 处理其他需要下行的情况（非确认数据但有MAC命令）
	if len(downlinkCmds) > 0 || macPayload.FHDR.FCtrl.ADRACKReq {
		// CN470 特殊处理：检查是否需要调整信道
		if p.region.Name == "CN470" {
			p.handleCN470ChannelManagement(validSession, rxInfo)
		}

		p.handleDownlink(validSession, gatewayID, rxInfo, downlinkCmds, false)
	}
}

// getFPortValue 安全获取 FPort 值
func getFPortValue(fPort *uint8) int16 {
	if fPort == nil {
		return -1
	}
	return int16(*fPort)
}

// getInt 安全获取整数值
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

// 临时测试函数 - 完全硬编码ChirpStack风格的ACK
func (p *Processor) createTestACK(session *models.DeviceSession) lorawan.PHYPayload {
	// 直接构造一个包含LinkADRReq的ACK
	// LinkADRReq: CID(03) + DataRate_TXPower(53) + ChMask(ff00) + Redundancy(01)
	// DevStatusReq: CID(06)

	macPayload := lorawan.MACPayload{
		FHDR: lorawan.FHDR{
			DevAddr: lorawan.DevAddr(session.DevAddr),
			FCtrl: lorawan.FCtrl{
				ADR:       true,
				ACK:       true,
				ADRACKReq: false,
				ClassB:    false,
				FPending:  false,
			},
			FCnt:  uint16(session.NFCntDown & 0xFFFF),
			FOpts: []byte{0x03, 0x53, 0xff, 0x00, 0x01, 0x06}, // 硬编码的MAC命令
		},
	}

	// 序列化
	macBytes, _ := macPayload.Marshal(lorawan.UnconfirmedDataDown, false)

	phyPayload := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: lorawan.UnconfirmedDataDown,
			Major: lorawan.LoRaWAN1_0,
		},
		MACPayload: macBytes,
	}

	// 设置MIC
	sNwkSIntKey, _ := hex.DecodeString(session.SNwkSIntKey)
	var key lorawan.AES128Key
	copy(key[:], sNwkSIntKey)
	phyPayload.SetDownlinkDataMIC(lorawan.LoRaWAN1_0, session.NFCntDown, key)

	// 调试
	phyBytes, _ := phyPayload.MarshalBinary()
	log.Info().
		Hex("testACK", phyBytes).
		Int("size", len(phyBytes)).
		Msg("测试ACK")

	return phyPayload
}

// ✅ 正确的createACKResponse函数 - 匹配ChirpStack行为
func (p *Processor) createACKResponse(session *models.DeviceSession, macCommands []lorawan.MACCommand) lorawan.PHYPayload {
	// ❌ 注释掉自动添加MAC命令的代码 - 这是导致问题的根源！
	/*
		if len(macCommands) == 0 {
			// 从ChirpStack包中看到的LinkADRReq命令
			linkADRReq := lorawan.MACCommand{
				CID:     lorawan.LinkADRReq,             // CID = 0x03
				Payload: []byte{0x53, 0xff, 0x00, 0x01}, // 从ChirpStack抓包中的值
			}
			macCommands = append(macCommands, linkADRReq)

			log.Info().
				Int("macCmdCount", len(macCommands)).
				Msg("添加MAC命令以匹配ChirpStack ACK行为")
		}
	*/

	// 构建 MAC payload
	macPayload := lorawan.MACPayload{
		FHDR: lorawan.FHDR{
			DevAddr: lorawan.DevAddr(session.DevAddr),
			FCtrl: lorawan.FCtrl{
				ADR: false, // 关闭ADR
				ACK: true,  // ✅ ACK位
			},
			FCnt: uint16(session.NFCntDown & 0xFFFF),
		},
	}

	// ✅ 只有在真正需要时才编码MAC命令
	if len(macCommands) > 0 {
		macCmdBytes, err := lorawan.EncodeMACCommands(macCommands)
		if err != nil {
			log.Error().Err(err).Msg("编码MAC命令失败")
			macCmdBytes = []byte{}
		}

		// 验证FOpts长度（最大15字节）
		if len(macCmdBytes) > 15 {
			log.Warn().
				Int("len", len(macCmdBytes)).
				Msg("MAC命令超过15字节，需要使用FRMPayload")
			macCmdBytes = macCmdBytes[:15] // 截断到15字节
		}

		macPayload.FHDR.FOpts = macCmdBytes
	}

	// 序列化MAC payload
	macBytes, err := macPayload.Marshal(lorawan.UnconfirmedDataDown, false)
	if err != nil {
		log.Error().Err(err).Msg("序列化ACK失败")
		return lorawan.PHYPayload{}
	}

	// 构建PHY payload
	phyPayload := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: lorawan.UnconfirmedDataDown,
			Major: lorawan.LoRaWAN1_0,
		},
		MACPayload: macBytes,
	}

	// 设置MIC
	sNwkSIntKey, _ := hex.DecodeString(session.SNwkSIntKey)
	var key lorawan.AES128Key
	copy(key[:], sNwkSIntKey)
	phyPayload.SetDownlinkDataMIC(lorawan.LoRaWAN1_0, session.NFCntDown, key)

	// 调试日志
	phyBytes, _ := phyPayload.MarshalBinary()

	// 计算实际的FCtrl字节值（用于调试）
	fctrlByte := uint8(0)
	if macPayload.FHDR.FCtrl.ADR {
		fctrlByte |= 0x80
	}
	if macPayload.FHDR.FCtrl.ACK {
		fctrlByte |= 0x20
	}
	if len(macPayload.FHDR.FOpts) > 0 {
		fctrlByte |= uint8(len(macPayload.FHDR.FOpts) & 0x0F) // FOptsLen在低4位
	}

	log.Info().
		Hex("ackPayload", phyBytes).
		Int("totalSize", len(phyBytes)).
		Str("base64", base64.StdEncoding.EncodeToString(phyBytes)).
		Uint32("fcnt", session.NFCntDown).
		Int("fOptsLen", len(macPayload.FHDR.FOpts)).
		Hex("fOpts", macPayload.FHDR.FOpts).
		Hex("fCtrl", []byte{fctrlByte}).
		Bool("adr", macPayload.FHDR.FCtrl.ADR).
		Bool("ack", macPayload.FHDR.FCtrl.ACK).
		Msg("创建简单ACK（无自动MAC命令）")

	return phyPayload
}

// 在 handleDataUp 中更新设备上行缓存
func (p *Processor) updateDeviceRxCache(devEUI lorawan.EUI64, gatewayID string, rxInfo map[string]interface{}) {
	p.rxCacheMutex.Lock()
	defer p.rxCacheMutex.Unlock()

	p.deviceRxCache[devEUI] = &DeviceRxInfo{
		GatewayID: gatewayID,
		RxInfo:    rxInfo,
		Timestamp: time.Now(),
	}

	log.Debug().
		Str("devEUI", hex.EncodeToString(devEUI[:])).
		Str("gatewayID", gatewayID).
		Msg("更新设备网关缓存")
}

// getLastGatewayForDevice 获取设备最后使用的网关
func (p *Processor) getLastGatewayForDevice(devEUI lorawan.EUI64) string {
	// 首先尝试从内存缓存获取
	p.rxCacheMutex.RLock()
	if info, ok := p.deviceRxCache[devEUI]; ok {
		if time.Since(info.Timestamp) < 5*time.Minute {
			p.rxCacheMutex.RUnlock()
			log.Debug().
				Str("devEUI", hex.EncodeToString(devEUI[:])).
				Str("gatewayID", info.GatewayID).
				Msg("从缓存获取网关ID")
			return info.GatewayID
		}
	}

	// 尝试反序 DevEUI
	reversedDevEUI := reverseEUI64(devEUI)
	if info, ok := p.deviceRxCache[reversedDevEUI]; ok {
		if time.Since(info.Timestamp) < 5*time.Minute {
			p.rxCacheMutex.RUnlock()
			log.Debug().
				Str("devEUI", hex.EncodeToString(devEUI[:])).
				Str("reversedDevEUI", hex.EncodeToString(reversedDevEUI[:])).
				Str("gatewayID", info.GatewayID).
				Msg("使用反序DevEUI从缓存获取网关ID")
			return info.GatewayID
		}
	}
	p.rxCacheMutex.RUnlock()

	// 从数据库获取
	ctx := context.Background()
	gatewayID, err := p.store.GetLastGatewayForDevice(ctx, devEUI)
	if err != nil {
		log.Error().
			Err(err).
			Str("devEUI", hex.EncodeToString(devEUI[:])).
			Msg("从数据库获取网关ID失败")
		return ""
	}

	if gatewayID != "" {
		log.Debug().
			Str("devEUI", hex.EncodeToString(devEUI[:])).
			Str("gatewayID", gatewayID).
			Msg("从数据库获取到网关ID")

		// 更新缓存
		p.updateDeviceRxCache(devEUI, gatewayID, nil)
		return gatewayID
	}

	// 尝试反序 DevEUI 从数据库查询
	gatewayID, err = p.store.GetLastGatewayForDevice(ctx, reversedDevEUI)
	if err == nil && gatewayID != "" {
		log.Debug().
			Str("devEUI", hex.EncodeToString(devEUI[:])).
			Str("reversedDevEUI", hex.EncodeToString(reversedDevEUI[:])).
			Str("gatewayID", gatewayID).
			Msg("使用反序DevEUI从数据库获取到网关ID")
		return gatewayID
	}

	log.Warn().
		Str("devEUI", hex.EncodeToString(devEUI[:])).
		Msg("无法找到设备的网关ID")

	return ""
}

// 获取设备最近的接收信息
func (p *Processor) getLastRxInfoForDevice(devEUI lorawan.EUI64) map[string]interface{} {
	p.rxCacheMutex.RLock()
	defer p.rxCacheMutex.RUnlock()

	if info, ok := p.deviceRxCache[devEUI]; ok {
		// 检查是否过期（5分钟）
		if time.Since(info.Timestamp) < 5*time.Minute {
			return info.RxInfo
		}
	}

	// 返回默认值
	return map[string]interface{}{
		"tmst": uint64(time.Now().UnixNano() / 1000),
		"freq": 470.3, // CN470 默认频率
		"datr": "SF7BW125",
		"codr": "4/5",
	}
}

// handleDownlink 处理下行
func (p *Processor) handleDownlink(session *models.DeviceSession, gatewayID string, rxInfo map[string]interface{}, macCmds []lorawan.MACCommand, confirmed bool) {
	ctx := context.Background()

	// 检查待发送的应用数据
	frames, err := p.store.GetPendingDownlinks(ctx, lorawan.EUI64(session.DevEUI))
	if err != nil {
		log.Error().Err(err).Msg("获取待发送数据失败")
	}

	// 构建下行帧
	var fPort uint8
	var data []byte
	var mtype lorawan.MType

	if len(frames) > 0 && len(macCmds) == 0 {
		// 有应用数据
		frame := frames[0]
		fPort = uint8(frame.FPort)
		data = frame.Data
		if frame.Confirmed {
			mtype = lorawan.ConfirmedDataDown
		} else {
			mtype = lorawan.UnconfirmedDataDown
		}
	} else if confirmed || len(macCmds) > 0 {
		// 只有 ACK 或 MAC 命令
		if confirmed {
			mtype = lorawan.UnconfirmedDataDown
		} else {
			mtype = lorawan.UnconfirmedDataDown
		}
	} else {
		return
	}

	// 构建 MAC payload
	macPayload := lorawan.MACPayload{
		FHDR: lorawan.FHDR{
			DevAddr: lorawan.DevAddr(session.DevAddr),
			FCtrl: lorawan.FCtrl{
				ADR: session.ADR,
				ACK: confirmed,
			},
			FCnt: uint16(session.NFCntDown & 0xFFFF),
		},
	}

	// 添加 MAC 命令
	if len(macCmds) > 0 {
		if len(data) == 0 {
			// MAC 命令在 FRMPayload 中
			fPort = 0
			data, _ = lorawan.EncodeMACCommands(macCmds)
		} else {
			// MAC 命令在 FOpts 中
			macPayload.FHDR.FOpts, _ = lorawan.EncodeMACCommands(macCmds)
		}
	}

	if len(data) > 0 {
		macPayload.FPort = &fPort

		// 加密数据（使用 DecryptFRMPayload，因为在 LoRaWAN 中加密和解密是相同操作）
		var key []byte
		if fPort == 0 {
			key, _ = hex.DecodeString(session.NwkSEncKey)
		} else {
			key, _ = hex.DecodeString(session.AppSKey)
		}

		macPayload.FRMPayload, _ = crypto.DecryptFRMPayload(
			key,
			false,
			[4]byte(session.DevAddr),
			session.NFCntDown,
			data,
		)
	}

	// 序列化 MAC payload
	macBytes, _ := macPayload.Marshal(mtype, false)

	// 构建 PHY payload
	phyPayload := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			MType: mtype,
			Major: lorawan.LoRaWAN1_0,
		},
		MACPayload: macBytes,
	}

	// 设置 MIC
	sNwkSIntKey, _ := hex.DecodeString(session.SNwkSIntKey)
	var key lorawan.AES128Key
	copy(key[:], sNwkSIntKey)
	phyPayload.SetDownlinkDataMIC(lorawan.LoRaWAN1_0, session.NFCntDown, key)

	// 更新计数器
	session.NFCntDown++

	// CN470 特殊处理：确保使用正确的下行计数器
	if p.region.Name == "CN470" {
		session.ConfFCnt = session.NFCntDown
	}

	p.store.SaveDeviceSession(ctx, session)

	// 计算下行时间和频率
	delay := time.Duration(p.config.CN470.RXWindows.RX1Delay) * time.Second

	// 第一接收窗口 (RX1)
	p.scheduleDownlink(gatewayID, lorawan.DevAddr(session.DevAddr), phyPayload, rxInfo, delay)

	// 检查是否需要RX2窗口
	if p.shouldUseRX2() {
		// 准备 RX2 参数
		rx2Info := make(map[string]interface{})
		for k, v := range rxInfo {
			rx2Info[k] = v
		}

		// 使用配置中的RX2频率和数据速率
		rx2Info["freq"] = float64(p.config.CN470.RXWindows.RX2Frequency)
		rx2Info["datr"] = p.getDRString(uint8(p.config.CN470.RXWindows.RX2DataRate))

		// 使用配置中的RX2延迟
		rx2Delay := time.Duration(p.config.CN470.RXWindows.RX2Delay) * time.Second
		p.scheduleDownlink(gatewayID, lorawan.DevAddr(session.DevAddr), phyPayload, rx2Info, rx2Delay)
	}
}

func (p *Processor) scheduleDownlink(gatewayID string, devAddr lorawan.DevAddr, phy lorawan.PHYPayload, rxInfo map[string]interface{}, delay time.Duration) {
	phyBytes, _ := phy.MarshalBinary()

	// 获取上行频率并计算下行频率
	uplinkFreq := getFloat64(rxInfo, "freq")
	uplinkFreqUint32 := uint32(uplinkFreq * 1000000)

	// 根据配置计算下行频率
	var downlinkFreq float64
	var mode string

	if p.region.Name == "CN470" {
		mode = p.config.CN470.GetCN470Mode()
		var downlinkFreqUint32 uint32

		switch mode {
		case "STANDARD_FDD":
			downlinkFreqUint32 = uplinkFreqUint32 + 30000000
			if downlinkFreqUint32 < 500300000 || downlinkFreqUint32 > 509700000 {
				downlinkFreqUint32 = p.config.CN470.RXWindows.RX2Frequency
			}
		case "CUSTOM_FDD":
			downlinkFreqUint32 = uplinkFreqUint32 + 10000000
			if downlinkFreqUint32 < 480300000 || downlinkFreqUint32 > 489900000 {
				downlinkFreqUint32 = p.config.CN470.RXWindows.RX2Frequency
			}
		case "TDD":
			downlinkFreqUint32 = uplinkFreqUint32
		default:
			downlinkFreqUint32 = p.config.CN470.GetDownlinkFrequencyAdaptive(uplinkFreqUint32)
		}

		downlinkFreq = float64(downlinkFreqUint32) / 1000000.0
	} else {
		downlinkFreq = uplinkFreq
		mode = "OTHER"
	}

	// 获取数据速率
	dataRate := ""
	if dr, ok := rxInfo["datr"].(string); ok {
		dataRate = dr
	}

	// 获取编码率
	codeRate := "4/5"
	if codr, ok := rxInfo["codr"].(string); ok && codr != "" {
		codeRate = codr
	}

	// ✅ 检查是否有 context
	contextStr, hasContext := rxInfo["context"].(string)

	// ✅ 如果有 context，使用 context + timing 模式
	if hasContext && delay > 0 {
		// 构建消息，包含 context 和 timing
		msg := map[string]interface{}{
			"gatewayID": gatewayID,
			"txpk": map[string]interface{}{
				"imme": false, // 使用延时模式
				"rfch": 0,
				"powe": 19,
				"ant":  0,
				"brd":  0,
				"freq": downlinkFreq,
				"modu": "LORA",
				"datr": dataRate,
				"codr": codeRate,
				"ipol": true,
				"size": len(phyBytes),
				"data": base64.StdEncoding.EncodeToString(phyBytes),
			},
			"context": contextStr,
			"timing": map[string]interface{}{
				"delay": fmt.Sprintf("%dms", delay.Milliseconds()),
			},
		}

		data, _ := json.Marshal(msg)
		subject := fmt.Sprintf("gateway.%s.tx", gatewayID)

		if err := p.nc.Publish(subject, data); err != nil {
			log.Error().
				Err(err).
				Str("subject", subject).
				Msg("发布下行消息失败")
			return
		}

		log.Info().
			Str("devAddr", devAddr.String()).
			Str("gateway", gatewayID).
			Float64("freq", downlinkFreq).
			Str("dataRate", dataRate).
			Bool("hasContext", true).
			Dur("delay", delay).
			Str("region", p.region.Name).
			Msg("使用 context + timing 下行模式")

		return
	}

	// ✅ 如果没有 context 或者需要即时发送，使用原有逻辑
	// 对于普通数据，考虑使用即时发送避免时间戳问题
	useImmediate := false
	reason := ""

	// 获取上行时间戳
	uplinkTmst := getUint64(rxInfo, "tmst")

	// 如果时间戳太小或太大，使用即时发送
	if uplinkTmst < 60000000 || uplinkTmst > 4000000000 {
		useImmediate = true
		reason = "timestamp_out_of_range"
	}

	// 零延迟或JOIN ACCEPT之外的情况，考虑使用即时发送
	if !useImmediate && delay == 0 {
		useImmediate = true
		reason = "zero_delay"
	}

	// 构建下行包
	var txpk map[string]interface{}

	if useImmediate {
		txpk = map[string]interface{}{
			"imme": true,
			"rfch": 0,
			"powe": 19,
			"ant":  0,
			"brd":  0,
			"freq": downlinkFreq,
			"modu": "LORA",
			"datr": dataRate,
			"codr": codeRate,
			"ipol": true,
			"size": len(phyBytes),
			"data": base64.StdEncoding.EncodeToString(phyBytes),
		}
	} else {
		// 计算下行时间戳
		//const RX1_DELAY_ADJUSTMENT = 30000 // 30ms额外余量
		//delayMicroseconds := uint64(delay.Microseconds()) + RX1_DELAY_ADJUSTMENT
		delayMicroseconds := uint64(delay.Microseconds())
		downlinkTmst := uplinkTmst + delayMicroseconds

		txpk = map[string]interface{}{
			"imme": false,
			"rfch": 0,
			"powe": 19,
			"ant":  0,
			"brd":  0,
			"tmst": downlinkTmst,
			"freq": downlinkFreq,
			"modu": "LORA",
			"datr": dataRate,
			"codr": codeRate,
			"ipol": true,
			"size": len(phyBytes),
			"data": base64.StdEncoding.EncodeToString(phyBytes),
		}
	}

	msg := map[string]interface{}{
		"gatewayID": gatewayID,
		"txpk":      txpk,
	}

	data, _ := json.Marshal(msg)
	subject := fmt.Sprintf("gateway.%s.tx", gatewayID)

	if err := p.nc.Publish(subject, data); err != nil {
		log.Error().
			Err(err).
			Str("subject", subject).
			Msg("发布下行消息失败")
		return
	}

	// 记录日志
	logEvent := log.Info().
		Str("devAddr", devAddr.String()).
		Str("gateway", gatewayID).
		Float64("freq", downlinkFreq).
		Str("dataRate", dataRate).
		Bool("immediate", useImmediate).
		Str("region", p.region.Name)

	if useImmediate {
		logEvent.Str("reason", reason)
	} else {
		logEvent.
			Uint64("uplinkTmst", uplinkTmst).
			Uint64("downlinkTmst", uplinkTmst+uint64(delay.Microseconds()))
	}

	logEvent.Msg("普通下行数据调度")
}

// 增强的调试函数
func (p *Processor) debugDownlinkTiming(gatewayID string, uplinkTmst uint64, delay time.Duration,
	downlinkTmst uint64, immediate bool, reason string) {

	currentTime := uint64(time.Now().UnixNano() / 1000) // 微秒

	debugLog := log.Debug().
		Str("gateway", gatewayID).
		Uint64("systemTime", currentTime).
		Uint64("uplinkTmst", uplinkTmst).
		Dur("delay", delay).
		Bool("immediate", immediate).
		Str("reason", reason)

	if !immediate {
		debugLog = debugLog.
			Uint64("downlinkTmst", downlinkTmst).
			Int64("tmstAge", int64(currentTime-uplinkTmst)).
			Int64("tmstToDownlink", int64(downlinkTmst-uplinkTmst))

		// 检查时间戳是否在合理范围内
		if downlinkTmst > uplinkTmst {
			scheduledDelay := time.Duration(downlinkTmst-uplinkTmst) * time.Microsecond
			debugLog = debugLog.Dur("scheduledDelay", scheduledDelay)
		}
	}

	debugLog.Msg("下行时间调试信息")
}

// 辅助函数：获取使用即时发送的原因
func getImmediateReason(unreliable bool, overflow bool) string {
	if unreliable {
		return "timestamp_unreliable"
	}
	if overflow {
		return "timestamp_overflow"
	}
	return "forced"
}

// 添加定期清理过期的时间戳信息
func (t *TimestampTracker) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.cleanup()
		}
	}
}

func (t *TimestampTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for gwID, info := range t.gatewayTimestamps {
		// 清理超过10分钟没有更新的记录
		if now.Sub(info.LastUplinkTime) > 10*time.Minute {
			delete(t.gatewayTimestamps, gwID)
			log.Debug().
				Str("gateway", gwID).
				Msg("清理过期的时间戳记录")
		}
	}
}

// 辅助函数：计算下行频率
func (p *Processor) calculateDownlinkFrequency(uplinkFreq float64) float64 {
	if p.region.Name == "CN470" {
		uplinkFreqUint32 := uint32(uplinkFreq * 1000000)
		mode := p.config.CN470.GetCN470Mode()
		var downlinkFreqUint32 uint32

		switch mode {
		case "STANDARD_FDD":
			downlinkFreqUint32 = uplinkFreqUint32 + 30000000
			if downlinkFreqUint32 < 500300000 || downlinkFreqUint32 > 509700000 {
				downlinkFreqUint32 = p.config.CN470.RXWindows.RX2Frequency
			}
		case "CUSTOM_FDD":
			downlinkFreqUint32 = uplinkFreqUint32 + 10000000
			if downlinkFreqUint32 < 480300000 || downlinkFreqUint32 > 489900000 {
				downlinkFreqUint32 = p.config.CN470.RXWindows.RX2Frequency
			}
		case "TDD":
			downlinkFreqUint32 = uplinkFreqUint32
		default:
			downlinkFreqUint32 = p.config.CN470.GetDownlinkFrequencyAdaptive(uplinkFreqUint32)
		}

		return float64(downlinkFreqUint32) / 1000000.0
	}

	return uplinkFreq
}

// publishUplinkData 发布上行数据到应用服务器
func (p *Processor) publishUplinkData(session *models.DeviceSession, mac lorawan.MACPayload, data []byte, rxInfo map[string]interface{}, applicationID uuid.UUID) {
	// ✅ 关键修复：将 rxInfo 包装成数组
	rxInfoArray := []map[string]interface{}{rxInfo}
	msg := map[string]interface{}{
		"applicationID": applicationID.String(), // ✅ 添加这一行
		"devEUI":        hex.EncodeToString(session.DevEUI[:]),
		"devAddr":       hex.EncodeToString(session.DevAddr[:]),
		"fCnt":          session.FCntUp,
		"fPort":         mac.FPort,
		"data":          data,
		"rxInfo":        rxInfoArray,
		"adr":           mac.FHDR.FCtrl.ADR,
	}

	msgData, _ := json.Marshal(msg)
	subject := fmt.Sprintf("application.*.device.%s.rx", hex.EncodeToString(session.DevEUI[:]))

	p.nc.Publish(subject, msgData)
}

// publishJoinEvent 发布入网事件
func (p *Processor) publishJoinEvent(device *models.Device, devAddr lorawan.DevAddr) {
	msg := map[string]interface{}{
		"applicationID": device.ApplicationID.String(),
		"devEUI":        hex.EncodeToString(device.DevEUI[:]),
		"devAddr":       devAddr.String(),
	}

	msgData, _ := json.Marshal(msg)
	subject := fmt.Sprintf("application.%s.device.%s.join", device.ApplicationID.String(), hex.EncodeToString(device.DevEUI[:]))

	p.nc.Publish(subject, msgData)
}

// === 辅助函数 ===

func (p *Processor) generateDevAddr() lorawan.DevAddr {
	// 简化实现：随机生成
	var addr lorawan.DevAddr
	binary.BigEndian.PutUint32(addr[:], uint32(time.Now().UnixNano()))
	return addr
}

func (p *Processor) generateJoinNonce() [3]byte {
	var nonce [3]byte
	t := time.Now().UnixNano()
	nonce[0] = byte(t >> 16)
	nonce[1] = byte(t >> 8)
	nonce[2] = byte(t)
	return nonce
}

// 密钥推导函数（使用标准 LoRaWAN 实现）
func (p *Processor) deriveAppSKey(joinNonce [3]byte, netID [3]byte, devNonce [2]byte, appKey string) lorawan.AES128Key {
	keyBytes, _ := hex.DecodeString(appKey)
	_, appSKey, _ := lorawan.DeriveSessionKeys10(keyBytes, joinNonce, netID, devNonce)
	return appSKey
}

func (p *Processor) deriveFNwkSIntKey(joinNonce [3]byte, netID [3]byte, devNonce [2]byte, nwkKey string) lorawan.AES128Key {
	keyBytes, _ := hex.DecodeString(nwkKey)
	nwkSKey, _, _ := lorawan.DeriveSessionKeys10(keyBytes, joinNonce, netID, devNonce)
	return nwkSKey
}

func (p *Processor) deriveSNwkSIntKey(joinNonce [3]byte, netID [3]byte, devNonce [2]byte, nwkKey string) lorawan.AES128Key {
	// For LoRaWAN 1.0.x, SNwkSIntKey = FNwkSIntKey = NwkSKey
	return p.deriveFNwkSIntKey(joinNonce, netID, devNonce, nwkKey)
}

func (p *Processor) deriveNwkSEncKey(joinNonce [3]byte, netID [3]byte, devNonce [2]byte, nwkKey string) lorawan.AES128Key {
	// For LoRaWAN 1.0.x, NwkSEncKey = NwkSKey
	return p.deriveFNwkSIntKey(joinNonce, netID, devNonce, nwkKey)
}

// === CN470 特定函数 ===

// 修改getRegionRX2Freq函数
func (p *Processor) getRegionRX2Freq() uint32 {
	if p.region.Name == "CN470" {
		return p.config.CN470.RXWindows.RX2Frequency
	}
	return p.region.DefaultRX2Freq
}

// 修改getRegionTXPower函数
func (p *Processor) getRegionTXPower() int {
	if p.region.Name == "CN470" {
		return p.config.CN470.Hardware.TXPowerDBm
	}

	switch p.region.Name {
	case "EU868":
		return 14 // EU868 默认 14 dBm
	case "US915":
		return 20 // US915 默认 20 dBm
	default:
		return 14
	}
}

// 新增：CN470配置验证函数
func (p *Processor) validateCN470Configuration() error {
	if p.region.Name != "CN470" {
		return nil
	}

	mode := p.config.CN470.GetCN470Mode()
	// 添加详细的配置日志
	log.Info().
		Str("mode", mode).
		Bool("supports_500mhz", p.config.CN470.Hardware.SupportsTX500MHz).
		Bool("supports_470_490mhz", p.config.CN470.Hardware.SupportsTX470_490MHz).
		Uint32("rx2_freq", p.config.CN470.RXWindows.RX2Frequency).
		Msg("CN470配置详情")
	switch mode {
	case "STANDARD_FDD":
		if !p.config.CN470.Hardware.SupportsTX500MHz {
			return fmt.Errorf("硬件不支持500MHz，无法使用标准FDD模式")
		}

	case "CUSTOM_FDD":
		if !p.config.CN470.Hardware.SupportsTX470_490MHz {
			return fmt.Errorf("硬件不支持470-490MHz，无法使用自定义FDD模式")
		}

	case "TDD":
		if !p.config.CN470.Hardware.SupportsTX470_490MHz {
			return fmt.Errorf("硬件不支持470-490MHz，无法使用TDD模式")
		}
	}

	log.Info().
		Str("mode", mode).
		Bool("supports_500mhz", p.config.CN470.Hardware.SupportsTX500MHz).
		Bool("supports_470_490mhz", p.config.CN470.Hardware.SupportsTX470_490MHz).
		Msg("CN470配置验证通过")

	return nil
}

// 修改generateCN470CFList函数，使用配置
// generateCN470CFList 生成CN470的CFList
func (p *Processor) generateCN470CFList() []byte {
	cfList := make([]byte, 16)

	// 根据配置模式生成CFList
	mode := p.config.CN470.GetCN470Mode()

	log.Debug().
		Str("mode", mode).
		Msg("生成CN470 CFList")

	var frequencies []uint32

	switch mode {
	case "STANDARD_FDD":
		// 标准FDD: 使用标准CN470频率
		// 470.3, 470.5, 470.7, 470.9, 471.1 MHz
		baseFreq := uint32(470300000)
		for i := 1; i <= 5; i++ {
			freq := baseFreq + uint32(i)*200000
			frequencies = append(frequencies, freq)
		}
		log.Debug().
			Uints32("frequencies", frequencies).
			Msg("STANDARD_FDD CFList频率")

	case "CUSTOM_FDD":
		// 自定义FDD: 根据配置生成频率列表
		// 获取配置的起始频率和间隔
		startFreq := p.config.CN470.CustomFDD.UplinkStartFreq
		spacing := p.config.CN470.Channels.ChannelSpacing
		if spacing == 0 {
			spacing = 1200000 // 默认1.2MHz
		}

		// 生成频率列表（最多5个）
		// 注意：CH0 (470.3MHz) 通常是默认频率，设备已知
		// 所以CFList中包含 CH1-CH5
		for i := 1; i <= 5; i++ {
			freq := startFreq + uint32(i)*spacing
			// 验证频率是否在配置的范围内
			if freq <= p.config.CN470.CustomFDD.UplinkEndFreq {
				frequencies = append(frequencies, freq)
			}
		}

		log.Debug().
			Uint32("startFreq", startFreq).
			Uint32("spacing", spacing).
			Int("numFreqs", len(frequencies)).
			Msg("CUSTOM_FDD CFList频率计算")

	case "TDD":
		// TDD: 使用全范围频率
		startFreq := p.config.CN470.TDD.StartFreq
		spacing := p.config.CN470.Channels.ChannelSpacing
		if spacing == 0 {
			spacing = 200000 // TDD默认200kHz
		}

		// TDD模式下的频率列表
		for i := 1; i <= 5; i++ {
			freq := startFreq + uint32(i)*spacing
			if freq <= p.config.CN470.TDD.EndFreq {
				frequencies = append(frequencies, freq)
			}
		}

	default:
		// 未知模式，使用默认CN470频率
		frequencies = []uint32{
			470500000, // CH1
			470700000, // CH2
			470900000, // CH3
			471100000, // CH4
			471300000, // CH5
		}
	}

	// 将频率编码到CFList中
	// 频率以100Hz为单位，小端序，每个频率占3字节
	for i, freq := range frequencies {
		if i >= 5 {
			break // CFList最多支持5个频率
		}

		// 验证频率是否在硬件支持范围内
		if !p.config.CN470.ValidateFrequency(freq) {
			log.Warn().
				Uint32("freq", freq).
				Msg("频率不在硬件支持范围内，跳过")
			continue
		}

		// 转换为100Hz单位
		freqIn100Hz := freq / 100

		// 小端序编码（3字节）
		cfList[i*3] = byte(freqIn100Hz & 0xFF)
		cfList[i*3+1] = byte((freqIn100Hz >> 8) & 0xFF)
		cfList[i*3+2] = byte((freqIn100Hz >> 16) & 0xFF)

		log.Debug().
			Int("index", i).
			Uint32("freqHz", freq).
			Uint32("freq100Hz", freqIn100Hz).
			Hex("bytes", cfList[i*3:(i+1)*3]).
			Msg("CFList频率编码")
	}

	// CFListType = 0 表示频率列表
	cfList[15] = 0

	// 打印最终的CFList内容（用于调试）
	if log.Debug().Enabled() {
		var freqList []string
		for i := 0; i < 5; i++ {
			if cfList[i*3] != 0 || cfList[i*3+1] != 0 || cfList[i*3+2] != 0 {
				freq100Hz := uint32(cfList[i*3]) |
					(uint32(cfList[i*3+1]) << 8) |
					(uint32(cfList[i*3+2]) << 16)
				freqMHz := float64(freq100Hz) / 10000.0
				freqList = append(freqList, fmt.Sprintf("%.1fMHz", freqMHz))
			}
		}

		log.Debug().
			Strs("frequencies", freqList).
			Hex("cfList", cfList).
			Msg("生成的CN470 CFList")
	}

	return cfList
}

// handleCN470ChannelManagement 处理 CN470 信道管理
func (p *Processor) handleCN470ChannelManagement(session *models.DeviceSession, rxInfo map[string]interface{}) {
	// CN470 特定的信道管理逻辑
	uplinkFreq := getFloat64(rxInfo, "freq")

	// 计算当前使用的信道索引
	channelIndex := int((uplinkFreq - 470300000) / 200000)

	// 记录信道使用情况，用于后续的 ADR 和信道管理
	log.Debug().
		Str("devEUI", hex.EncodeToString(session.DevEUI[:])).
		Float64("freq", uplinkFreq).
		Int("channel", channelIndex).
		Msg("CN470 信道使用记录")

	// TODO: 实现更复杂的信道管理策略
	// - 跟踪设备使用的信道
	// - 根据网络负载调整信道分配
	// - 使用 NewChannelReq MAC 命令配置新信道
}

// shouldUseRX2 判断是否应该使用 RX2
// 修改shouldUseRX2函数
func (p *Processor) shouldUseRX2() bool {
	if p.region.Name == "CN470" {
		switch p.config.CN470.GetCN470Mode() {
		case "STANDARD_FDD", "CUSTOM_FDD":
			// FDD模式：通常只使用RX1，除非网络质量差
			return false
		case "TDD":
			// TDD模式：可能更需要RX2作为备用
			return true
		}
	}
	return false
}

// getDRString 将数据速率转换为字符串格式
func (p *Processor) getDRString(dr uint8) string {
	if p.region.Name == "CN470" {
		// CN470 数据速率格式
		switch dr {
		case 0:
			return "SF12BW125"
		case 1:
			return "SF11BW125"
		case 2:
			return "SF10BW125"
		case 3:
			return "SF9BW125"
		case 4:
			return "SF8BW125"
		case 5:
			return "SF7BW125"
		default:
			return "SF12BW125"
		}
	}
	// EU868 数据速率格式
	if p.region.Name == "EU868" {
		switch dr {
		case 0:
			return "SF12BW125"
		case 1:
			return "SF11BW125"
		case 2:
			return "SF10BW125"
		case 3:
			return "SF9BW125"
		case 4:
			return "SF8BW125"
		case 5:
			return "SF7BW125"
		case 6:
			return "SF7BW250"
		default:
			return "SF12BW125"
		}
	}
	// 默认返回
	return "SF12BW125"
}

// === 通用辅助函数 ===

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

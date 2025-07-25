package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

// ForwarderService 处理数据转发到外部系统
type ForwarderService struct {
	nc       *nats.Conn
	store    storage.Store
	
	// MQTT 客户端池
	mqttClients map[uuid.UUID]mqtt.Client
	clientsMu   sync.RWMutex
	
	// HTTP 客户端
	httpClient *http.Client
}

// NewForwarderService 创建转发服务
func NewForwarderService(nc *nats.Conn, store storage.Store) *ForwarderService {
	return &ForwarderService{
		nc:          nc,
		store:       store,
		mqttClients: make(map[uuid.UUID]mqtt.Client),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start 启动转发服务
func (s *ForwarderService) Start(ctx context.Context) error {
	// 订阅设备上行数据
	sub, err := s.nc.Subscribe("application.*.device.*.rx", s.handleUplinkData)
	if err != nil {
		return fmt.Errorf("subscribe to uplink data: %w", err)
	}

	// 订阅入网事件
	subJoin, err := s.nc.Subscribe("application.*.device.*.join", s.handleJoinEvent)
	if err != nil {
		return fmt.Errorf("subscribe to join events: %w", err)
	}

	// 初始化 MQTT 连接
	if err := s.initializeMQTTConnections(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to initialize MQTT connections")
	}

	log.Info().Msg("Integration forwarder service started")

	<-ctx.Done()
	
	sub.Unsubscribe()
	subJoin.Unsubscribe()
	s.closeAllMQTTConnections()
	
	return nil
}

// handleUplinkData 处理上行数据
func (s *ForwarderService) handleUplinkData(msg *nats.Msg) {
	// 解析主题获取应用ID
	parts := strings.Split(msg.Subject, ".")
	if len(parts) != 5 {
		return
	}

	appIDStr := parts[1]
	if appIDStr == "*" {
		// 从消息体中获取
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return
		}
		appIDStr, _ = data["applicationID"].(string)
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		log.Error().Err(err).Msg("Invalid application ID")
		return
	}

	// 获取应用配置
	ctx := context.Background()
	app, err := s.store.GetApplication(ctx, appID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get application")
		return
	}

	// 解析上行数据
	var uplinkData UplinkData
	if err := json.Unmarshal(msg.Data, &uplinkData); err != nil {
		log.Error().Err(err).Msg("Failed to parse uplink data")
		return
	}

	// 执行 payload 解码（如果配置了）
	if app.PayloadDecoder != "" && uplinkData.Data != nil {
		decoded := s.decodePayload(app.PayloadDecoder, uplinkData.Data)
		if decoded != nil {
			uplinkData.Object = decoded
		}
	}

	// 转发到 HTTP
	if s.isHTTPEnabled(app) {
		go s.forwardToHTTP(app, uplinkData)
	}

	// 转发到 MQTT
	if s.isMQTTEnabled(app) {
		go s.forwardToMQTT(app, uplinkData)
	}
}

// handleJoinEvent 处理入网事件
func (s *ForwarderService) handleJoinEvent(msg *nats.Msg) {
	// 类似处理逻辑，但使用 JoinEvent 结构
	parts := strings.Split(msg.Subject, ".")
	if len(parts) != 5 {
		return
	}

	appIDStr := parts[1]
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return
	}

	ctx := context.Background()
	app, err := s.store.GetApplication(ctx, appID)
	if err != nil {
		return
	}

	var joinEvent JoinEvent
	if err := json.Unmarshal(msg.Data, &joinEvent); err != nil {
		return
	}

	// 转发入网事件
	if s.isHTTPEnabled(app) {
		go s.forwardJoinToHTTP(app, joinEvent)
	}

	if s.isMQTTEnabled(app) {
		go s.forwardJoinToMQTT(app, joinEvent)
	}
}

// forwardToHTTP 转发数据到 HTTP
func (s *ForwarderService) forwardToHTTP(app *models.Application, data UplinkData) {
	config := s.getHTTPConfig(app)
	if config == nil || !config.Enabled {
		return
	}

	// 准备转发数据
	forwardData := map[string]interface{}{
		"applicationID":   app.ID.String(),
		"applicationName": app.Name,
		"deviceName":      data.DeviceName,
		"devEUI":          data.DevEUI,
		"devAddr":         data.DevAddr,
		"fCnt":            data.FCnt,
		"fPort":           data.FPort,
		"data":            data.Data,
		"object":          data.Object,
		"rxInfo":          data.RxInfo,
		"adr":             data.ADR,
		"timestamp":       time.Now(),
	}

	jsonData, err := json.Marshal(forwardData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal forward data")
		return
	}

	// 创建请求
	req, err := http.NewRequest("POST", config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create HTTP request")
		return
	}

	// 设置 headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("endpoint", config.Endpoint).
			Msg("Failed to forward data to HTTP")
		
		// TODO: 实现重试逻辑
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Error().
			Int("status", resp.StatusCode).
			Str("endpoint", config.Endpoint).
			Msg("HTTP forward failed")
		
		// TODO: 实现错误处理和重试
	} else {
		log.Debug().
			Str("devEUI", data.DevEUI).
			Str("endpoint", config.Endpoint).
			Msg("Data forwarded to HTTP successfully")
	}
}

// forwardToMQTT 转发数据到 MQTT
func (s *ForwarderService) forwardToMQTT(app *models.Application, data UplinkData) {
	config := s.getMQTTConfig(app)
	if config == nil || !config.Enabled {
		return
	}

	// 获取或创建 MQTT 客户端
	client := s.getMQTTClient(app.ID)
	if client == nil {
		client = s.createMQTTClient(app.ID, config)
		if client == nil {
			return
		}
	}

	// 准备主题
	topic := config.TopicPattern
	topic = strings.ReplaceAll(topic, "{app_id}", app.ID.String())
	topic = strings.ReplaceAll(topic, "{dev_eui}", data.DevEUI)
	topic = strings.ReplaceAll(topic, "{dev_addr}", data.DevAddr)

	// 准备数据
	forwardData := map[string]interface{}{
		"applicationID":   app.ID.String(),
		"applicationName": app.Name,
		"deviceName":      data.DeviceName,
		"devEUI":          data.DevEUI,
		"devAddr":         data.DevAddr,
		"fCnt":            data.FCnt,
		"fPort":           data.FPort,
		"data":            data.Data,
		"object":          data.Object,
		"rxInfo":          data.RxInfo,
		"adr":             data.ADR,
		"timestamp":       time.Now(),
	}

	jsonData, err := json.Marshal(forwardData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal MQTT data")
		return
	}

	// 发布消息
	token := client.Publish(topic, config.QoS, false, jsonData)
	if token.WaitTimeout(5 * time.Second) {
		if err := token.Error(); err != nil {
			log.Error().
				Err(err).
				Str("topic", topic).
				Msg("Failed to publish to MQTT")
		} else {
			log.Debug().
				Str("devEUI", data.DevEUI).
				Str("topic", topic).
				Msg("Data forwarded to MQTT successfully")
		}
	} else {
		log.Error().
			Str("topic", topic).
			Msg("MQTT publish timeout")
	}
}

// forwardJoinToHTTP 转发入网事件到 HTTP
func (s *ForwarderService) forwardJoinToHTTP(app *models.Application, event JoinEvent) {
	config := s.getHTTPConfig(app)
	if config == nil || !config.Enabled {
		return
	}

	// 准备数据
	forwardData := map[string]interface{}{
		"type":            "join",
		"applicationID":   app.ID.String(),
		"applicationName": app.Name,
		"devEUI":          event.DevEUI,
		"devAddr":         event.DevAddr,
		"timestamp":       time.Now(),
	}

	jsonData, err := json.Marshal(forwardData)
	if err != nil {
		return
	}

	// 创建请求
	req, err := http.NewRequest("POST", config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}

	// 设置 headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to forward join event to HTTP")
		return
	}
	defer resp.Body.Close()

	log.Debug().
		Str("devEUI", event.DevEUI).
		Int("status", resp.StatusCode).
		Msg("Join event forwarded to HTTP")
}

// forwardJoinToMQTT 转发入网事件到 MQTT
func (s *ForwarderService) forwardJoinToMQTT(app *models.Application, event JoinEvent) {
	config := s.getMQTTConfig(app)
	if config == nil || !config.Enabled {
		return
	}

	client := s.getMQTTClient(app.ID)
	if client == nil {
		return
	}

	// 准备主题
	topic := strings.ReplaceAll(config.TopicPattern, "/up", "/join")
	topic = strings.ReplaceAll(topic, "{app_id}", app.ID.String())
	topic = strings.ReplaceAll(topic, "{dev_eui}", event.DevEUI)

	// 准备数据
	forwardData := map[string]interface{}{
		"type":            "join",
		"applicationID":   app.ID.String(),
		"applicationName": app.Name,
		"devEUI":          event.DevEUI,
		"devAddr":         event.DevAddr,
		"timestamp":       time.Now(),
	}

	jsonData, _ := json.Marshal(forwardData)

	// 发布消息
	token := client.Publish(topic, config.QoS, false, jsonData)
	token.WaitTimeout(5 * time.Second)
}

// getMQTTClient 获取 MQTT 客户端
func (s *ForwarderService) getMQTTClient(appID uuid.UUID) mqtt.Client {
	s.clientsMu.RLock()
	client, exists := s.mqttClients[appID]
	s.clientsMu.RUnlock()
	
	if exists && client.IsConnected() {
		return client
	}
	
	return nil
}

// createMQTTClient 创建 MQTT 客户端
func (s *ForwarderService) createMQTTClient(appID uuid.UUID, config *MQTTConfig) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.BrokerURL)
	opts.SetClientID(fmt.Sprintf("lorawan-app-%s", appID))
	
	if config.Username != "" {
		opts.SetUsername(config.Username)
		opts.SetPassword(config.Password)
	}

	if config.TLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // 生产环境应该使用证书验证
		}
		opts.SetTLSConfig(tlsConfig)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetKeepAlive(30 * time.Second)

	// 连接处理
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Info().
			Str("appID", appID.String()).
			Msg("MQTT client connected")
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Error().
			Err(err).
			Str("appID", appID.String()).
			Msg("MQTT connection lost")
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	
	if token.WaitTimeout(10 * time.Second) && token.Error() == nil {
		s.clientsMu.Lock()
		s.mqttClients[appID] = client
		s.clientsMu.Unlock()
		return client
	}

	log.Error().
		Err(token.Error()).
		Str("appID", appID.String()).
		Msg("Failed to connect MQTT client")
	
	return nil
}

// initializeMQTTConnections 初始化所有 MQTT 连接
func (s *ForwarderService) initializeMQTTConnections(ctx context.Context) error {
	// 获取所有启用了 MQTT 的应用
	// TODO: 实现批量查询
	return nil
}

// closeAllMQTTConnections 关闭所有 MQTT 连接
func (s *ForwarderService) closeAllMQTTConnections() {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for appID, client := range s.mqttClients {
		if client.IsConnected() {
			client.Disconnect(250)
		}
		delete(s.mqttClients, appID)
		
		log.Info().
			Str("appID", appID.String()).
			Msg("MQTT client disconnected")
	}
}

// decodePayload 解码 payload
func (s *ForwarderService) decodePayload(decoder string, data []byte) map[string]interface{} {
	// TODO: 实现 JavaScript 解码器
	// 可以使用 goja 等 JavaScript 引擎
	return nil
}

// Helper functions

func (s *ForwarderService) isHTTPEnabled(app *models.Application) bool {
	if app.HTTPIntegration == nil {
		return false
	}
	config := s.getHTTPConfig(app)
	return config != nil && config.Enabled
}

func (s *ForwarderService) isMQTTEnabled(app *models.Application) bool {
	if app.MQTTIntegration == nil {
		return false
	}
	config := s.getMQTTConfig(app)
	return config != nil && config.Enabled
}

func (s *ForwarderService) getHTTPConfig(app *models.Application) *HTTPConfig {
	if app.HTTPIntegration == nil {
		return nil
	}
	
	var config HTTPConfig
	configMap := *app.HTTPIntegration
	configBytes, _ := json.Marshal(configMap)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil
	}
	
	return &config
}

func (s *ForwarderService) getMQTTConfig(app *models.Application) *MQTTConfig {
	if app.MQTTIntegration == nil {
		return nil
	}
	
	var config MQTTConfig
	configMap := *app.MQTTIntegration
	configBytes, _ := json.Marshal(configMap)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil
	}
	
	return &config
}

// Data structures

type UplinkData struct {
	ApplicationID string                   `json:"applicationID"`
	DevEUI        string                   `json:"devEUI"`
	DevAddr       string                   `json:"devAddr"`
	DeviceName    string                   `json:"deviceName,omitempty"`
	FCnt          uint32                   `json:"fCnt"`
	FPort         *uint8                   `json:"fPort"`
	Data          []byte                   `json:"data"`
	Object        map[string]interface{}   `json:"object,omitempty"`
	RxInfo        []map[string]interface{} `json:"rxInfo"`
	ADR           bool                     `json:"adr"`
}

type JoinEvent struct {
	ApplicationID string `json:"applicationID"`
	DevEUI        string `json:"devEUI"`
	DevAddr       string `json:"devAddr"`
}

type HTTPConfig struct {
	Enabled  bool              `json:"enabled"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
	Timeout  int               `json:"timeout"`
}

type MQTTConfig struct {
	Enabled      bool   `json:"enabled"`
	BrokerURL    string `json:"brokerUrl"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	TopicPattern string `json:"topicPattern"`
	QoS          byte   `json:"qos"`
	TLS          bool   `json:"tls"`
}

package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/lorawan-server/lorawan-server-pro/internal/models"
	"github.com/lorawan-server/lorawan-server-pro/internal/storage"
)

// Integration 相关的数据结构
type HTTPIntegration struct {
	Enabled  bool              `json:"enabled"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
	Timeout  int               `json:"timeout"` // 秒
}

type MQTTIntegration struct {
	Enabled      bool   `json:"enabled"`
	BrokerURL    string `json:"brokerUrl"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	TopicPattern string `json:"topicPattern"`
	QoS          byte   `json:"qos"`
	TLS          bool   `json:"tls"`
}

// HandleGetIntegrations 获取应用的集成配置
func (s *RESTServer) HandleGetIntegrations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	app, err := s.store.GetApplication(ctx, appID)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "application not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 解析集成配置
	var httpConfig HTTPIntegration
	var mqttConfig MQTTIntegration

	// HTTPIntegration 和 MQTTIntegration 是指针类型
	if app.HTTPIntegration != nil && len(*app.HTTPIntegration) > 0 {
		httpBytes, _ := json.Marshal(*app.HTTPIntegration)
		json.Unmarshal(httpBytes, &httpConfig)
	}

	if app.MQTTIntegration != nil && len(*app.MQTTIntegration) > 0 {
		mqttBytes, _ := json.Marshal(*app.MQTTIntegration)
		json.Unmarshal(mqttBytes, &mqttConfig)
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"http":           httpConfig,
		"mqtt":           mqttConfig,
		"payloadCodec":   app.PayloadCodec,
		"payloadDecoder": app.PayloadDecoder,
		"payloadEncoder": app.PayloadEncoder,
	})
}

// HandleUpdateHTTPIntegration 更新 HTTP 集成配置
func (s *RESTServer) HandleUpdateHTTPIntegration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	var req HTTPIntegration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 验证配置
	if req.Enabled && req.Endpoint == "" {
		s.respondError(w, http.StatusBadRequest, "endpoint is required when integration is enabled")
		return
	}

	// 默认超时30秒
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	app, err := s.store.GetApplication(ctx, appID)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "application not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 更新 HTTP 集成配置
	httpIntegration := models.Variables{
		"enabled":  req.Enabled,
		"endpoint": req.Endpoint,
		"headers":  req.Headers,
		"timeout":  req.Timeout,
	}

	app.HTTPIntegration = &httpIntegration

	if err := s.store.UpdateApplication(ctx, app); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "HTTP integration updated successfully",
	})
}

// HandleUpdateMQTTIntegration 更新 MQTT 集成配置
func (s *RESTServer) HandleUpdateMQTTIntegration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	var req MQTTIntegration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 验证配置
	if req.Enabled {
		if req.BrokerURL == "" {
			s.respondError(w, http.StatusBadRequest, "broker URL is required when integration is enabled")
			return
		}
		if req.TopicPattern == "" {
			req.TopicPattern = "application/{app_id}/device/{dev_eui}/up"
		}
	}

	app, err := s.store.GetApplication(ctx, appID)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "application not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 更新 MQTT 集成配置
	mqttIntegration := models.Variables{
		"enabled":      req.Enabled,
		"brokerUrl":    req.BrokerURL,
		"username":     req.Username,
		"password":     req.Password,
		"topicPattern": req.TopicPattern,
		"qos":          req.QoS,
		"tls":          req.TLS,
	}

	app.MQTTIntegration = &mqttIntegration

	if err := s.store.UpdateApplication(ctx, app); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 如果启用了 MQTT，可能需要重新连接
	if req.Enabled {
		// TODO: 通知 MQTT 管理器更新连接
		s.notifyMQTTManager(appID, req)
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "MQTT integration updated successfully",
	})
}

// HandleTestIntegration 测试集成连接
func (s *RESTServer) HandleTestIntegration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	var req struct {
		Type string `json:"type"` // "http" or "mqtt"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	app, err := s.store.GetApplication(ctx, appID)
	if err != nil {
		if err == storage.ErrNotFound {
			s.respondError(w, http.StatusNotFound, "application not found")
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch req.Type {
	case "http":
		if err := s.testHTTPIntegration(app); err != nil {
			s.respondError(w, http.StatusBadRequest, fmt.Sprintf("HTTP test failed: %v", err))
			return
		}
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"message": "HTTP integration test successful",
		})

	case "mqtt":
		if err := s.testMQTTIntegration(app); err != nil {
			s.respondError(w, http.StatusBadRequest, fmt.Sprintf("MQTT test failed: %v", err))
			return
		}
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"message": "MQTT integration test successful",
		})

	default:
		s.respondError(w, http.StatusBadRequest, "invalid integration type")
	}
}

// testHTTPIntegration 测试 HTTP 集成
func (s *RESTServer) testHTTPIntegration(app *models.Application) error {
	var config HTTPIntegration
	if app.HTTPIntegration == nil || len(*app.HTTPIntegration) == 0 {
		return fmt.Errorf("HTTP integration not configured")
	}

	// 直接序列化和反序列化
	configBytes, _ := json.Marshal(*app.HTTPIntegration)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return fmt.Errorf("invalid HTTP integration config: %v", err)
	}

	if !config.Enabled {
		return fmt.Errorf("HTTP integration is disabled")
	}

	// 创建测试数据
	testPayload := map[string]interface{}{
		"test":        true,
		"timestamp":   time.Now(),
		"application": app.Name,
		"message":     "Integration test",
	}

	jsonData, _ := json.Marshal(testPayload)

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	// 设置 headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// testMQTTIntegration 测试 MQTT 集成
func (s *RESTServer) testMQTTIntegration(app *models.Application) error {
	var config MQTTIntegration
	if app.MQTTIntegration == nil || len(*app.MQTTIntegration) == 0 {
		return fmt.Errorf("MQTT integration not configured")
	}

	// 直接序列化和反序列化
	configBytes, _ := json.Marshal(*app.MQTTIntegration)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return fmt.Errorf("invalid MQTT integration config: %v", err)
	}

	if !config.Enabled {
		return fmt.Errorf("MQTT integration is disabled")
	}

	// 创建 MQTT 客户端配置
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.BrokerURL)
	opts.SetClientID(fmt.Sprintf("lorawan-test-%s", app.ID))

	if config.Username != "" {
		opts.SetUsername(config.Username)
		opts.SetPassword(config.Password)
	}

	if config.TLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // 测试时可以跳过证书验证
		}
		opts.SetTLSConfig(tlsConfig)
	}

	// 连接超时
	opts.SetConnectTimeout(10 * time.Second)

	// 创建客户端并连接
	client := mqtt.NewClient(opts)
	token := client.Connect()

	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("connection timeout")
	}

	if err := token.Error(); err != nil {
		return err
	}

	// 断开连接
	client.Disconnect(250)

	return nil
}

// notifyMQTTManager 通知 MQTT 管理器更新连接
func (s *RESTServer) notifyMQTTManager(appID uuid.UUID, config MQTTIntegration) {
	// TODO: 实现 MQTT 管理器通知逻辑
	// 这里应该通过消息队列或其他方式通知 MQTT 管理器服务
	log.Info().
		Str("appID", appID.String()).
		Bool("enabled", config.Enabled).
		Msg("MQTT integration configuration updated")
}

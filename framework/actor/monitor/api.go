package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// ManagementAPI 管理API服务
type ManagementAPI struct {
	mu sync.RWMutex

	// 监控组件
	metricsCollector   *MetricsCollector
	healthCheckManager *HealthCheckManager

	// HTTP服务器
	server  *http.Server
	running bool

	// 配置
	config APIConfig
}

// APIConfig API配置
type APIConfig struct {
	// 监听地址
	Address string
	// 端口
	Port int
	// 启用HTTPS
	EnableHTTPS bool
	// TLS证书文件
	CertFile string
	// TLS密钥文件
	KeyFile string
	// 认证令牌
	AuthToken string
	// 启用CORS
	EnableCORS bool
	// 请求超时
	RequestTimeout time.Duration
	// 最大请求体大小
	MaxBodySize int64
}

// DefaultAPIConfig 默认API配置
func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Address:        "0.0.0.0",
		Port:           8080,
		EnableHTTPS:    false,
		CertFile:       "",
		KeyFile:        "",
		AuthToken:      "",
		EnableCORS:     true,
		RequestTimeout: 30 * time.Second,
		MaxBodySize:    10 * 1024 * 1024, // 10MB
	}
}

// APIResponse API响应
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ActorInfo actor信息
type ActorInfo struct {
	Address    string             `json:"address"`
	Type       string             `json:"type"`
	State      actor.State        `json:"state"`
	Health     actor.HealthStatus `json:"health"`
	Metrics    ActorMetrics       `json:"metrics,omitempty"`
	HealthInfo HealthCheckResult  `json:"health_info,omitempty"`
	StartedAt  time.Time          `json:"started_at,omitempty"`
	Uptime     time.Duration      `json:"uptime,omitempty"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Metrics      SystemMetrics      `json:"metrics"`
	HealthStatus SystemHealthStatus `json:"health_status"`
	Uptime       time.Duration      `json:"uptime"`
	StartTime    time.Time          `json:"start_time"`
}

// SystemHealthStatus 系统健康状态
type SystemHealthStatus struct {
	TotalActors     int `json:"total_actors"`
	HealthyActors   int `json:"healthy_actors"`
	DegradedActors  int `json:"degraded_actors"`
	UnhealthyActors int `json:"unhealthy_actors"`
	UnknownActors   int `json:"unknown_actors"`
}

// NewManagementAPI 创建新的管理API服务
func NewManagementAPI(metricsCollector *MetricsCollector, healthCheckManager *HealthCheckManager, config APIConfig) *ManagementAPI {
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.MaxBodySize == 0 {
		config.MaxBodySize = 10 * 1024 * 1024
	}

	return &ManagementAPI{
		metricsCollector:   metricsCollector,
		healthCheckManager: healthCheckManager,
		config:             config,
		running:            false,
	}
}

// Start 启动API服务
func (api *ManagementAPI) Start() error {
	api.mu.Lock()
	defer api.mu.Unlock()

	if api.running {
		return fmt.Errorf("API server already running")
	}

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%d", api.config.Address, api.config.Port)
	mux := http.NewServeMux()

	// 注册路由
	api.registerRoutes(mux)

	// 创建服务器
	api.server = &http.Server{
		Addr:         addr,
		Handler:      api.withMiddleware(mux),
		ReadTimeout:  api.config.RequestTimeout,
		WriteTimeout: api.config.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器
	go func() {
		var err error
		if api.config.EnableHTTPS && api.config.CertFile != "" && api.config.KeyFile != "" {
			err = api.server.ListenAndServeTLS(api.config.CertFile, api.config.KeyFile)
		} else {
			err = api.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("API server error: %v\n", err)
		}
	}()

	api.running = true
	fmt.Printf("Management API server started on %s\n", addr)

	return nil
}

// Stop 停止API服务
func (api *ManagementAPI) Stop() error {
	api.mu.Lock()
	defer api.mu.Unlock()

	if !api.running || api.server == nil {
		return fmt.Errorf("API server not running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := api.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("failed to shutdown API server: %v", err)
	}

	api.running = false
	fmt.Println("Management API server stopped")

	return nil
}

// IsRunning 返回是否正在运行
func (api *ManagementAPI) IsRunning() bool {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.running
}

// registerRoutes 注册路由
func (api *ManagementAPI) registerRoutes(mux *http.ServeMux) {
	// 健康检查
	mux.HandleFunc("GET /health", api.handleHealth)
	mux.HandleFunc("GET /ready", api.handleReady)

	// 系统信息
	mux.HandleFunc("GET /api/v1/system", api.handleSystemInfo)
	mux.HandleFunc("GET /api/v1/metrics", api.handleSystemMetrics)
	mux.HandleFunc("GET /api/v1/health", api.handleSystemHealth)

	// Actor管理
	mux.HandleFunc("GET /api/v1/actors", api.handleListActors)
	mux.HandleFunc("GET /api/v1/actors/{address}", api.handleGetActor)
	mux.HandleFunc("POST /api/v1/actors/{address}/restart", api.handleRestartActor)
	mux.HandleFunc("POST /api/v1/actors/{address}/stop", api.handleStopActor)
	mux.HandleFunc("POST /api/v1/actors/{address}/start", api.handleStartActor)
	mux.HandleFunc("GET /api/v1/actors/{address}/metrics", api.handleActorMetrics)
	mux.HandleFunc("GET /api/v1/actors/{address}/health", api.handleActorHealth)

	// 消息管理
	mux.HandleFunc("GET /api/v1/messages/types", api.handleMessageTypes)
	mux.HandleFunc("GET /api/v1/messages/senders", api.handleMessageSenders)
	mux.HandleFunc("GET /api/v1/messages/receivers", api.handleMessageReceivers)

	// 管理操作
	mux.HandleFunc("POST /api/v1/management/clear-metrics", api.handleClearMetrics)
	mux.HandleFunc("POST /api/v1/management/clear-health", api.handleClearHealth)
	mux.HandleFunc("POST /api/v1/management/check-all", api.handleCheckAll)
}

// withMiddleware 添加中间件
func (api *ManagementAPI) withMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS支持
		if api.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// 认证检查
		if api.config.AuthToken != "" {
			token := r.Header.Get("Authorization")
			if token != "Bearer "+api.config.AuthToken {
				api.writeError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
		}

		// 内容类型
		w.Header().Set("Content-Type", "application/json")

		// 请求体大小限制
		r.Body = http.MaxBytesReader(w, r.Body, api.config.MaxBodySize)

		handler.ServeHTTP(w, r)
	})
}

// writeResponse 写入JSON响应
func (api *ManagementAPI) writeResponse(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	response := APIResponse{
		Success:   status >= 200 && status < 300,
		Data:      data,
		Timestamp: time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeError 写入错误响应
func (api *ManagementAPI) writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	response := APIResponse{
		Success:   false,
		Message:   message,
		Error:     http.StatusText(status),
		Timestamp: time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleHealth 处理健康检查
func (api *ManagementAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	api.writeResponse(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleReady 处理就绪检查
func (api *ManagementAPI) handleReady(w http.ResponseWriter, r *http.Request) {
	if api.IsRunning() {
		api.writeResponse(w, http.StatusOK, map[string]string{
			"status": "ready",
		})
	} else {
		api.writeResponse(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
		})
	}
}

// handleSystemInfo 处理系统信息
func (api *ManagementAPI) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	// 获取系统指标
	systemMetrics := api.metricsCollector.GetSystemMetrics()

	// 获取健康状态
	healthResults := api.healthCheckManager.GetAllHealthResults()

	// 计算健康状态统计
	var healthStatus SystemHealthStatus
	for _, result := range healthResults {
		healthStatus.TotalActors++
		switch result.HealthStatus {
		case actor.HealthStatusHealthy:
			healthStatus.HealthyActors++
		case actor.HealthStatusDegraded:
			healthStatus.DegradedActors++
		case actor.HealthStatusUnhealthy:
			healthStatus.UnhealthyActors++
		case actor.HealthStatusUnknown:
			healthStatus.UnknownActors++
		}
	}

	systemInfo := SystemInfo{
		Metrics:      systemMetrics,
		HealthStatus: healthStatus,
		Uptime:       time.Since(systemMetrics.LastUpdateTime),
		StartTime:    systemMetrics.LastUpdateTime,
	}

	api.writeResponse(w, http.StatusOK, systemInfo)
}

// handleSystemMetrics 处理系统指标
func (api *ManagementAPI) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := api.metricsCollector.GetSystemMetrics()
	api.writeResponse(w, http.StatusOK, metrics)
}

// handleSystemHealth 处理系统健康状态
func (api *ManagementAPI) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	healthResults := api.healthCheckManager.GetAllHealthResults()

	// 按健康状态分组
	healthy := make([]HealthCheckResult, 0)
	degraded := make([]HealthCheckResult, 0)
	unhealthy := make([]HealthCheckResult, 0)
	unknown := make([]HealthCheckResult, 0)

	for _, result := range healthResults {
		switch result.HealthStatus {
		case actor.HealthStatusHealthy:
			healthy = append(healthy, result)
		case actor.HealthStatusDegraded:
			degraded = append(degraded, result)
		case actor.HealthStatusUnhealthy:
			unhealthy = append(unhealthy, result)
		case actor.HealthStatusUnknown:
			unknown = append(unknown, result)
		}
	}

	response := map[string]interface{}{
		"total":     len(healthResults),
		"healthy":   healthy,
		"degraded":  degraded,
		"unhealthy": unhealthy,
		"unknown":   unknown,
		"timestamp": time.Now(),
	}

	api.writeResponse(w, http.StatusOK, response)
}

// handleListActors 处理列出所有actor
func (api *ManagementAPI) handleListActors(w http.ResponseWriter, r *http.Request) {
	// 获取所有actor指标
	actorMetrics := api.metricsCollector.GetAllActorMetrics()

	// 获取所有健康检查结果
	healthResults := make(map[string]HealthCheckResult)
	for _, result := range api.healthCheckManager.GetAllHealthResults() {
		healthResults[result.ActorAddress] = result
	}

	// 构建actor信息列表
	actors := make([]ActorInfo, 0, len(actorMetrics))
	for _, metrics := range actorMetrics {
		healthInfo, hasHealth := healthResults[metrics.ActorAddress]

		actorInfo := ActorInfo{
			Address:   metrics.ActorAddress,
			Type:      metrics.ActorType,
			State:     metrics.State,
			Health:    metrics.HealthStatus,
			Metrics:   metrics,
			StartedAt: time.Now().Add(-metrics.Uptime),
			Uptime:    metrics.Uptime,
		}

		if hasHealth {
			actorInfo.HealthInfo = healthInfo
		}

		actors = append(actors, actorInfo)
	}

	api.writeResponse(w, http.StatusOK, actors)
}

// handleGetActor 处理获取单个actor信息
func (api *ManagementAPI) handleGetActor(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		api.writeError(w, http.StatusBadRequest, "Address is required")
		return
	}

	// 获取actor指标
	metrics, exists := api.metricsCollector.GetActorMetrics(address)
	if !exists {
		api.writeError(w, http.StatusNotFound, "Actor not found")
		return
	}

	// 获取健康检查结果
	healthInfo, hasHealth := api.healthCheckManager.GetHealthResult(address)

	actorInfo := ActorInfo{
		Address:   metrics.ActorAddress,
		Type:      metrics.ActorType,
		State:     metrics.State,
		Health:    metrics.HealthStatus,
		Metrics:   metrics,
		StartedAt: time.Now().Add(-metrics.Uptime),
		Uptime:    metrics.Uptime,
	}

	if hasHealth {
		actorInfo.HealthInfo = healthInfo
	}

	api.writeResponse(w, http.StatusOK, actorInfo)
}

// handleRestartActor 处理重启actor
func (api *ManagementAPI) handleRestartActor(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		api.writeError(w, http.StatusBadRequest, "Address is required")
		return
	}

	// 解析地址
	addr, err := message.ParseAddress(address)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid address: %v", err))
		return
	}

	// 获取actor管理器
	actorMgr := actor.GetDefaultActorManager()
	actor, err := actorMgr.Get(addr)
	if err != nil {
		api.writeError(w, http.StatusNotFound, "Actor not found")
		return
	}

	// 停止actor
	if err := actor.Stop(); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop actor: %v", err))
		return
	}

	// 启动actor
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := actor.Start(ctx); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start actor: %v", err))
		return
	}

	// 记录重启
	api.metricsCollector.RecordActorRestart(address)

	api.writeResponse(w, http.StatusOK, map[string]string{
		"message": "Actor restarted successfully",
		"address": address,
	})
}

// handleStopActor 处理停止actor
func (api *ManagementAPI) handleStopActor(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		api.writeError(w, http.StatusBadRequest, "Address is required")
		return
	}

	// 解析地址
	addr, err := message.ParseAddress(address)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid address: %v", err))
		return
	}

	// 获取actor管理器
	actorMgr := actor.GetDefaultActorManager()
	actor, err := actorMgr.Get(addr)
	if err != nil {
		api.writeError(w, http.StatusNotFound, "Actor not found")
		return
	}

	// 停止actor
	if err := actor.Stop(); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop actor: %v", err))
		return
	}

	api.writeResponse(w, http.StatusOK, map[string]string{
		"message": "Actor stopped successfully",
		"address": address,
	})
}

// handleStartActor 处理启动actor
func (api *ManagementAPI) handleStartActor(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		api.writeError(w, http.StatusBadRequest, "Address is required")
		return
	}

	// 解析地址
	addr, err := message.ParseAddress(address)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid address: %v", err))
		return
	}

	// 获取actor管理器
	actorMgr := actor.GetDefaultActorManager()
	actor, err := actorMgr.Get(addr)
	if err != nil {
		api.writeError(w, http.StatusNotFound, "Actor not found")
		return
	}

	// 启动actor
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := actor.Start(ctx); err != nil {
		api.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start actor: %v", err))
		return
	}

	api.writeResponse(w, http.StatusOK, map[string]string{
		"message": "Actor started successfully",
		"address": address,
	})
}

// handleActorMetrics 处理actor指标
func (api *ManagementAPI) handleActorMetrics(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		api.writeError(w, http.StatusBadRequest, "Address is required")
		return
	}

	metrics, exists := api.metricsCollector.GetActorMetrics(address)
	if !exists {
		api.writeError(w, http.StatusNotFound, "Actor metrics not found")
		return
	}

	api.writeResponse(w, http.StatusOK, metrics)
}

// handleActorHealth 处理actor健康状态
func (api *ManagementAPI) handleActorHealth(w http.ResponseWriter, r *http.Request) {
	address := r.PathValue("address")
	if address == "" {
		api.writeError(w, http.StatusBadRequest, "Address is required")
		return
	}

	healthInfo, exists := api.healthCheckManager.GetHealthResult(address)
	if !exists {
		api.writeError(w, http.StatusNotFound, "Actor health info not found")
		return
	}

	api.writeResponse(w, http.StatusOK, healthInfo)
}

// handleMessageTypes 处理消息类型统计
func (api *ManagementAPI) handleMessageTypes(w http.ResponseWriter, r *http.Request) {
	messageTypes := api.metricsCollector.GetAllMessageTypeMetrics()
	api.writeResponse(w, http.StatusOK, messageTypes)
}

// handleMessageSenders 处理发送者统计
func (api *ManagementAPI) handleMessageSenders(w http.ResponseWriter, r *http.Request) {
	// 这里需要扩展MetricsCollector以提供获取所有发送者指标的方法
	// 暂时返回空响应
	api.writeResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Not implemented yet",
	})
}

// handleMessageReceivers 处理接收者统计
func (api *ManagementAPI) handleMessageReceivers(w http.ResponseWriter, r *http.Request) {
	// 这里需要扩展MetricsCollector以提供获取所有接收者指标的方法
	// 暂时返回空响应
	api.writeResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Not implemented yet",
	})
}

// handleClearMetrics 处理清空指标
func (api *ManagementAPI) handleClearMetrics(w http.ResponseWriter, r *http.Request) {
	api.metricsCollector.Clear()
	api.writeResponse(w, http.StatusOK, map[string]string{
		"message": "Metrics cleared successfully",
	})
}

// handleClearHealth 处理清空健康检查结果
func (api *ManagementAPI) handleClearHealth(w http.ResponseWriter, r *http.Request) {
	// 这里需要扩展HealthCheckManager以提供清空结果的方法
	// 暂时返回空响应
	api.writeResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Not implemented yet",
	})
}

// handleCheckAll 处理检查所有actor
func (api *ManagementAPI) handleCheckAll(w http.ResponseWriter, r *http.Request) {
	// 获取所有actor地址
	actorMetrics := api.metricsCollector.GetAllActorMetrics()

	results := make([]map[string]interface{}, 0, len(actorMetrics))

	for _, metrics := range actorMetrics {
		// 解析地址
		addr, err := message.ParseAddress(metrics.ActorAddress)
		if err != nil {
			continue
		}

		// 获取actor
		actorMgr := actor.GetDefaultActorManager()
		actor, err := actorMgr.Get(addr)
		if err != nil {
			continue
		}

		// 执行健康检查
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := api.healthCheckManager.CheckActor(ctx, actor)
		cancel()

		results = append(results, map[string]interface{}{
			"address": metrics.ActorAddress,
			"success": err == nil,
			"result":  result,
			"error":   err,
		})
	}

	api.writeResponse(w, http.StatusOK, map[string]interface{}{
		"total":   len(results),
		"results": results,
	})
}

package service_discovery

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	// Check 执行健康检查
	Check(ctx context.Context, service *ServiceInfo) error
	// Type 返回检查器类型
	Type() string
}

// HTTPHealthChecker HTTP健康检查器
type HTTPHealthChecker struct {
	timeout time.Duration
	path    string
	logger  *zap.Logger
}

// NewHTTPHealthChecker 创建HTTP健康检查器
func NewHTTPHealthChecker(path string, timeout time.Duration) *HTTPHealthChecker {
	if path == "" {
		path = "/health"
	}
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	return &HTTPHealthChecker{
		path:    path,
		timeout: timeout,
		logger:  zap.L().Named("http-health-checker"),
	}
}

// Check 执行HTTP健康检查
func (hc *HTTPHealthChecker) Check(ctx context.Context, service *ServiceInfo) error {
	if service == nil {
		return fmt.Errorf("service is nil")
	}

	url := fmt.Sprintf("http://%s%s", service.Address, hc.path)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: hc.timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP health check returned status: %d", resp.StatusCode)
	}

	return nil
}

// Type 返回检查器类型
func (hc *HTTPHealthChecker) Type() string {
	return "http"
}

// TCPHealthChecker TCP健康检查器
type TCPHealthChecker struct {
	timeout time.Duration
	logger  *zap.Logger
}

// NewTCPHealthChecker 创建TCP健康检查器
func NewTCPHealthChecker(timeout time.Duration) *TCPHealthChecker {
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	return &TCPHealthChecker{
		timeout: timeout,
		logger:  zap.L().Named("tcp-health-checker"),
	}
}

// Check 执行TCP健康检查
func (hc *TCPHealthChecker) Check(ctx context.Context, service *ServiceInfo) error {
	if service == nil {
		return fmt.Errorf("service is nil")
	}

	// 简化实现：使用net.DialTimeout
	// 实际应该使用net.DialContext
	conn, err := (&net.Dialer{
		Timeout: hc.timeout,
	}).DialContext(ctx, "tcp", service.Address)
	if err != nil {
		return fmt.Errorf("TCP health check failed: %w", err)
	}
	conn.Close()

	return nil
}

// Type 返回检查器类型
func (hc *TCPHealthChecker) Type() string {
	return "tcp"
}

// HealthCheckManager 健康检查管理器
type HealthCheckManager struct {
	discovery Discovery
	checker   HealthChecker
	interval  time.Duration
	timeout   time.Duration
	logger    *zap.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	services  map[string]*ServiceInfo  // 正在监控的服务
	status    map[string]ServiceStatus // 服务状态
	callbacks []func(serviceID string, oldStatus, newStatus ServiceStatus)
}

// NewHealthCheckManager 创建健康检查管理器
func NewHealthCheckManager(discovery Discovery, checker HealthChecker, interval time.Duration) *HealthCheckManager {
	if interval == 0 {
		interval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthCheckManager{
		discovery: discovery,
		checker:   checker,
		interval:  interval,
		timeout:   5 * time.Second,
		logger:    zap.L().Named("health-check-manager"),
		ctx:       ctx,
		cancel:    cancel,
		services:  make(map[string]*ServiceInfo),
		status:    make(map[string]ServiceStatus),
	}
}

// Start 启动健康检查管理器
func (hcm *HealthCheckManager) Start() error {
	hcm.wg.Add(1)
	go hcm.run()

	hcm.logger.Info("health check manager started",
		zap.String("checker_type", hcm.checker.Type()),
		zap.Duration("interval", hcm.interval),
	)

	return nil
}

// Stop 停止健康检查管理器
func (hcm *HealthCheckManager) Stop() {
	hcm.cancel()
	hcm.wg.Wait()

	hcm.logger.Info("health check manager stopped")
}

// AddService 添加要监控的服务
func (hcm *HealthCheckManager) AddService(service *ServiceInfo) {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()

	hcm.services[service.ID] = service
	hcm.status[service.ID] = ServiceStatusUnknown

	hcm.logger.Debug("service added to health check",
		zap.String("service_id", service.ID),
		zap.String("service_name", service.Name),
		zap.String("address", service.Address),
	)
}

// RemoveService 移除监控的服务
func (hcm *HealthCheckManager) RemoveService(serviceID string) {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()

	delete(hcm.services, serviceID)
	delete(hcm.status, serviceID)

	hcm.logger.Debug("service removed from health check",
		zap.String("service_id", serviceID),
	)
}

// GetStatus 获取服务状态
func (hcm *HealthCheckManager) GetStatus(serviceID string) ServiceStatus {
	hcm.mu.RLock()
	defer hcm.mu.RUnlock()

	status, exists := hcm.status[serviceID]
	if !exists {
		return ServiceStatusUnknown
	}
	return status
}

// RegisterCallback 注册状态变化回调
func (hcm *HealthCheckManager) RegisterCallback(callback func(serviceID string, oldStatus, newStatus ServiceStatus)) {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()

	hcm.callbacks = append(hcm.callbacks, callback)
}

// run 运行健康检查循环
func (hcm *HealthCheckManager) run() {
	defer hcm.wg.Done()

	ticker := time.NewTicker(hcm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hcm.ctx.Done():
			return
		case <-ticker.C:
			hcm.checkAllServices()
		}
	}
}

// checkAllServices 检查所有服务
func (hcm *HealthCheckManager) checkAllServices() {
	hcm.mu.RLock()
	services := make([]*ServiceInfo, 0, len(hcm.services))
	for _, service := range hcm.services {
		services = append(services, service)
	}
	hcm.mu.RUnlock()

	var wg sync.WaitGroup
	for _, service := range services {
		wg.Add(1)
		go func(s *ServiceInfo) {
			defer wg.Done()
			hcm.checkService(s)
		}(service)
	}
	wg.Wait()
}

// checkService 检查单个服务
func (hcm *HealthCheckManager) checkService(service *ServiceInfo) {
	ctx, cancel := context.WithTimeout(hcm.ctx, hcm.timeout)
	defer cancel()

	oldStatus := hcm.GetStatus(service.ID)
	var newStatus ServiceStatus

	err := hcm.checker.Check(ctx, service)
	if err != nil {
		hcm.logger.Warn("health check failed",
			zap.String("service_id", service.ID),
			zap.String("service_name", service.Name),
			zap.String("address", service.Address),
			zap.Error(err),
		)
		newStatus = ServiceStatusUnhealthy
	} else {
		newStatus = ServiceStatusHealthy
	}

	// 更新状态
	hcm.mu.Lock()
	hcm.status[service.ID] = newStatus
	hcm.mu.Unlock()

	// 触发回调
	if oldStatus != newStatus {
		hcm.triggerCallbacks(service.ID, oldStatus, newStatus)
	}
}

// triggerCallbacks 触发状态变化回调
func (hcm *HealthCheckManager) triggerCallbacks(serviceID string, oldStatus, newStatus ServiceStatus) {
	hcm.mu.RLock()
	callbacks := make([]func(serviceID string, oldStatus, newStatus ServiceStatus), len(hcm.callbacks))
	copy(callbacks, hcm.callbacks)
	hcm.mu.RUnlock()

	for _, callback := range callbacks {
		callback(serviceID, oldStatus, newStatus)
	}

	hcm.logger.Info("service status changed",
		zap.String("service_id", serviceID),
		zap.String("old_status", string(oldStatus)),
		zap.String("new_status", string(newStatus)),
	)
}

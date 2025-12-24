package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// HealthCheckManager 健康检查管理器
type HealthCheckManager struct {
	mu sync.RWMutex

	// 健康检查配置
	config HealthCheckConfig

	// 健康检查结果
	results map[string]HealthCheckResult

	// 健康检查器
	checkers map[string]HealthChecker

	// 事件处理器
	eventHandlers []HealthEventHandler

	// 停止通道
	stopChan chan struct{}
	stopped  bool
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	// 检查间隔
	CheckInterval time.Duration
	// 超时时间
	Timeout time.Duration
	// 最大失败次数
	MaxFailures int
	// 恢复检查间隔
	RecoveryCheckInterval time.Duration
	// 启用自动恢复
	EnableAutoRecovery bool
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	// Actor地址
	ActorAddress string
	// Actor类型
	ActorType string
	// 健康状态
	HealthStatus actor.HealthStatus
	// 最后检查时间
	LastCheckTime time.Time
	// 检查耗时
	CheckDuration time.Duration
	// 错误信息
	Error string
	// 连续失败次数
	ConsecutiveFailures int
	// 总检查次数
	TotalChecks int
	// 失败检查次数
	FailedChecks int
	// 最后成功时间
	LastSuccessTime time.Time
	// 最后失败时间
	LastFailureTime time.Time
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	// Check 执行健康检查
	Check(ctx context.Context, actor actor.Actor) (actor.HealthStatus, error)
	// Name 返回检查器名称
	Name() string
	// Description 返回检查器描述
	Description() string
}

// HealthEventHandler 健康事件处理器
type HealthEventHandler interface {
	// OnHealthChanged 健康状态变更时调用
	OnHealthChanged(address string, oldStatus, newStatus actor.HealthStatus, result HealthCheckResult)
	// OnHealthCheckFailed 健康检查失败时调用
	OnHealthCheckFailed(address string, result HealthCheckResult)
	// OnHealthCheckRecovered 健康检查恢复时调用
	OnHealthCheckRecovered(address string, result HealthCheckResult)
}

// DefaultHealthChecker 默认健康检查器
type DefaultHealthChecker struct{}

// NewDefaultHealthChecker 创建新的默认健康检查器
func NewDefaultHealthChecker() *DefaultHealthChecker {
	return &DefaultHealthChecker{}
}

// Check 执行健康检查
func (c *DefaultHealthChecker) Check(ctx context.Context, a actor.Actor) (actor.HealthStatus, error) {
	if a == nil {
		return actor.HealthStatusUnknown, fmt.Errorf("actor is nil")
	}

	return a.HealthCheck(ctx)
}

// Name 返回检查器名称
func (c *DefaultHealthChecker) Name() string {
	return "default"
}

// Description 返回检查器描述
func (c *DefaultHealthChecker) Description() string {
	return "Default actor health checker"
}

// MailboxHealthChecker 邮箱健康检查器
type MailboxHealthChecker struct {
	// 邮箱满阈值（百分比）
	FullThreshold float64
}

// NewMailboxHealthChecker 创建新的邮箱健康检查器
func NewMailboxHealthChecker(fullThreshold float64) *MailboxHealthChecker {
	return &MailboxHealthChecker{
		FullThreshold: fullThreshold,
	}
}

// Check 执行健康检查
func (c *MailboxHealthChecker) Check(ctx context.Context, a actor.Actor) (actor.HealthStatus, error) {
	if a == nil {
		return actor.HealthStatusUnknown, fmt.Errorf("actor is nil")
	}

	// 首先执行默认健康检查
	status, err := a.HealthCheck(ctx)
	if err != nil {
		return status, err
	}

	// 检查邮箱状态
	mailbox := a.Mailbox()
	if mailbox != nil {
		capacity := mailbox.Capacity()
		size := mailbox.Size()

		if capacity > 0 {
			fillRatio := float64(size) / float64(capacity)
			if fillRatio >= c.FullThreshold {
				// 邮箱过满，返回降级状态
				return actor.HealthStatusDegraded, nil
			}
		}
	}

	return status, nil
}

// Name 返回检查器名称
func (c *MailboxHealthChecker) Name() string {
	return "mailbox"
}

// Description 返回检查器描述
func (c *MailboxHealthChecker) Description() string {
	return fmt.Sprintf("Mailbox health checker (full threshold: %.2f%%)", c.FullThreshold*100)
}

// NewHealthCheckManager 创建新的健康检查管理器
func NewHealthCheckManager(config HealthCheckConfig) *HealthCheckManager {
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.MaxFailures == 0 {
		config.MaxFailures = 3
	}
	if config.RecoveryCheckInterval == 0 {
		config.RecoveryCheckInterval = 10 * time.Second
	}

	return &HealthCheckManager{
		config:        config,
		results:       make(map[string]HealthCheckResult),
		checkers:      make(map[string]HealthChecker),
		eventHandlers: make([]HealthEventHandler, 0),
		stopChan:      make(chan struct{}),
		stopped:       false,
	}
}

// Start 启动健康检查管理器
func (m *HealthCheckManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return fmt.Errorf("health check manager already stopped")
	}

	// 启动健康检查循环
	go m.healthCheckLoop()

	return nil
}

// Stop 停止健康检查管理器
func (m *HealthCheckManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return
	}

	m.stopped = true
	close(m.stopChan)
}

// RegisterActor 注册actor进行健康检查
func (m *HealthCheckManager) RegisterActor(a actor.Actor) error {
	if a == nil {
		return fmt.Errorf("actor is nil")
	}

	address := a.Address()
	if address == nil {
		return fmt.Errorf("actor address is nil")
	}

	addrStr := address.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 初始化健康检查结果
	m.results[addrStr] = HealthCheckResult{
		ActorAddress:        addrStr,
		ActorType:           getActorType(a),
		HealthStatus:        actor.HealthStatusUnknown,
		LastCheckTime:       time.Time{},
		CheckDuration:       0,
		Error:               "",
		ConsecutiveFailures: 0,
		TotalChecks:         0,
		FailedChecks:        0,
		LastSuccessTime:     time.Time{},
		LastFailureTime:     time.Time{},
	}

	return nil
}

// UnregisterActor 取消注册actor
func (m *HealthCheckManager) UnregisterActor(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.results, address)
}

// RegisterHealthChecker 注册健康检查器
func (m *HealthCheckManager) RegisterHealthChecker(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkers[checker.Name()] = checker
}

// UnregisterHealthChecker 取消注册健康检查器
func (m *HealthCheckManager) UnregisterHealthChecker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checkers, name)
}

// CheckActor 立即检查指定actor的健康状态
func (m *HealthCheckManager) CheckActor(ctx context.Context, a actor.Actor) (HealthCheckResult, error) {
	if a == nil {
		return HealthCheckResult{}, fmt.Errorf("actor is nil")
	}

	address := a.Address()
	if address == nil {
		return HealthCheckResult{}, fmt.Errorf("actor address is nil")
	}

	addrStr := address.String()

	// 执行健康检查
	result, err := m.performHealthCheck(ctx, a)
	if err != nil {
		return result, err
	}

	// 更新结果
	m.mu.Lock()
	oldResult, exists := m.results[addrStr]
	m.results[addrStr] = result
	m.mu.Unlock()

	// 触发事件
	if exists && oldResult.HealthStatus != result.HealthStatus {
		m.notifyHealthChanged(addrStr, oldResult.HealthStatus, result.HealthStatus, result)
	}

	if result.Error != "" {
		m.notifyHealthCheckFailed(addrStr, result)
	} else if oldResult.Error != "" && result.Error == "" {
		m.notifyHealthCheckRecovered(addrStr, result)
	}

	return result, nil
}

// GetHealthResult 获取健康检查结果
func (m *HealthCheckManager) GetHealthResult(address string) (HealthCheckResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.results[address]
	return result, exists
}

// GetAllHealthResults 获取所有健康检查结果
func (m *HealthCheckManager) GetAllHealthResults() []HealthCheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]HealthCheckResult, 0, len(m.results))
	for _, result := range m.results {
		results = append(results, result)
	}

	return results
}

// GetUnhealthyActors 获取不健康的actor
func (m *HealthCheckManager) GetUnhealthyActors() []HealthCheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]HealthCheckResult, 0)
	for _, result := range m.results {
		if result.HealthStatus != actor.HealthStatusHealthy {
			results = append(results, result)
		}
	}

	return results
}

// GetDegradedActors 获取降级的actor
func (m *HealthCheckManager) GetDegradedActors() []HealthCheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]HealthCheckResult, 0)
	for _, result := range m.results {
		if result.HealthStatus == actor.HealthStatusDegraded {
			results = append(results, result)
		}
	}

	return results
}

// RegisterEventHandler 注册事件处理器
func (m *HealthCheckManager) RegisterEventHandler(handler HealthEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.eventHandlers = append(m.eventHandlers, handler)
}

// UnregisterEventHandler 取消注册事件处理器
func (m *HealthCheckManager) UnregisterEventHandler(handler HealthEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, h := range m.eventHandlers {
		if h == handler {
			m.eventHandlers = append(m.eventHandlers[:i], m.eventHandlers[i+1:]...)
			break
		}
	}
}

// healthCheckLoop 健康检查循环
func (m *HealthCheckManager) healthCheckLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.performAllHealthChecks()
		}
	}
}

// performAllHealthChecks 执行所有健康检查
func (m *HealthCheckManager) performAllHealthChecks() {
	m.mu.RLock()
	// 复制地址列表以避免在检查期间持有锁
	addresses := make([]string, 0, len(m.results))
	for address := range m.results {
		addresses = append(addresses, address)
	}
	m.mu.RUnlock()

	// 获取actor管理器
	actorMgr := actor.GetDefaultActorManager()

	for _, address := range addresses {
		// 获取actor
		addr, err := message.ParseAddress(address)
		if err != nil {
			continue
		}

		actor, err := actorMgr.Get(addr)
		if err != nil {
			// actor不存在，从健康检查中移除
			m.UnregisterActor(address)
			continue
		}

		// 执行健康检查
		ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
		result, err := m.performHealthCheck(ctx, actor)
		cancel()

		if err != nil {
			// 记录错误但继续检查其他actor
			continue
		}

		// 更新结果
		m.mu.Lock()
		oldResult, exists := m.results[address]
		m.results[address] = result
		m.mu.Unlock()

		// 触发事件
		if exists && oldResult.HealthStatus != result.HealthStatus {
			m.notifyHealthChanged(address, oldResult.HealthStatus, result.HealthStatus, result)
		}

		if result.Error != "" {
			m.notifyHealthCheckFailed(address, result)
		} else if oldResult.Error != "" && result.Error == "" {
			m.notifyHealthCheckRecovered(address, result)
		}
	}
}

// performHealthCheck 执行健康检查
func (m *HealthCheckManager) performHealthCheck(ctx context.Context, a actor.Actor) (HealthCheckResult, error) {
	address := a.Address()
	if address == nil {
		return HealthCheckResult{}, fmt.Errorf("actor address is nil")
	}

	addrStr := address.String()
	actorType := getActorType(a)

	startTime := time.Now()

	// 执行所有检查器
	var finalStatus actor.HealthStatus = actor.HealthStatusHealthy
	var finalError string

	// 首先执行默认检查
	defaultChecker := NewDefaultHealthChecker()
	status, err := defaultChecker.Check(ctx, a)
	if err != nil {
		finalStatus = actor.HealthStatusUnhealthy
		finalError = err.Error()
	} else {
		finalStatus = status
	}

	// 执行其他检查器
	m.mu.RLock()
	checkers := make([]HealthChecker, 0, len(m.checkers))
	for _, checker := range m.checkers {
		if checker.Name() != "default" {
			checkers = append(checkers, checker)
		}
	}
	m.mu.RUnlock()

	for _, checker := range checkers {
		status, err := checker.Check(ctx, a)
		if err != nil {
			// 如果任何检查器失败，使用最差的健康状态
			if status == actor.HealthStatusUnhealthy {
				finalStatus = actor.HealthStatusUnhealthy
			} else if status == actor.HealthStatusDegraded && finalStatus != actor.HealthStatusUnhealthy {
				finalStatus = actor.HealthStatusDegraded
			}
			if finalError == "" {
				finalError = err.Error()
			}
		}
	}

	checkDuration := time.Since(startTime)

	// 获取当前结果
	m.mu.RLock()
	currentResult, exists := m.results[addrStr]
	m.mu.RUnlock()

	if !exists {
		currentResult = HealthCheckResult{
			ActorAddress: addrStr,
			ActorType:    actorType,
		}
	}

	// 更新结果
	result := HealthCheckResult{
		ActorAddress:        addrStr,
		ActorType:           actorType,
		HealthStatus:        finalStatus,
		LastCheckTime:       time.Now(),
		CheckDuration:       checkDuration,
		Error:               finalError,
		ConsecutiveFailures: currentResult.ConsecutiveFailures,
		TotalChecks:         currentResult.TotalChecks + 1,
		FailedChecks:        currentResult.FailedChecks,
		LastSuccessTime:     currentResult.LastSuccessTime,
		LastFailureTime:     currentResult.LastFailureTime,
	}

	if finalError != "" {
		result.ConsecutiveFailures++
		result.FailedChecks++
		result.LastFailureTime = time.Now()
	} else {
		result.ConsecutiveFailures = 0
		result.LastSuccessTime = time.Now()
	}

	return result, nil
}

// notifyHealthChanged 通知健康状态变更
func (m *HealthCheckManager) notifyHealthChanged(address string, oldStatus, newStatus actor.HealthStatus, result HealthCheckResult) {
	m.mu.RLock()
	handlers := make([]HealthEventHandler, len(m.eventHandlers))
	copy(handlers, m.eventHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		handler.OnHealthChanged(address, oldStatus, newStatus, result)
	}
}

// notifyHealthCheckFailed 通知健康检查失败
func (m *HealthCheckManager) notifyHealthCheckFailed(address string, result HealthCheckResult) {
	m.mu.RLock()
	handlers := make([]HealthEventHandler, len(m.eventHandlers))
	copy(handlers, m.eventHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		handler.OnHealthCheckFailed(address, result)
	}
}

// notifyHealthCheckRecovered 通知健康检查恢复
func (m *HealthCheckManager) notifyHealthCheckRecovered(address string, result HealthCheckResult) {
	m.mu.RLock()
	handlers := make([]HealthEventHandler, len(m.eventHandlers))
	copy(handlers, m.eventHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		handler.OnHealthCheckRecovered(address, result)
	}
}

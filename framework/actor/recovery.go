package actor

import (
	"reflect"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	// HandleError 处理错误
	HandleError(ctx *ActorContext, envelope *message.Envelope, err error) error
	// ShouldRetry 判断是否应该重试
	ShouldRetry(ctx *ActorContext, envelope *message.Envelope, err error) bool
	// GetRetryDelay 获取重试延迟
	GetRetryDelay(ctx *ActorContext, envelope *message.Envelope, err error) time.Duration
}

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	mu sync.RWMutex

	// 错误处理器映射
	errorHandlers map[string]ErrorHandler

	// 默认错误处理器
	defaultErrorHandler ErrorHandler

	// 重试配置
	maxRetries    int
	maxRetryDelay time.Duration
	backoffFactor float64
}

// NewRecoveryManager 创建新的恢复管理器
func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		errorHandlers:       make(map[string]ErrorHandler),
		defaultErrorHandler: NewDefaultErrorHandler(),
		maxRetries:          3,
		maxRetryDelay:       30 * time.Second,
		backoffFactor:       2.0,
	}
}

// RegisterErrorHandler 注册错误处理器
func (m *RecoveryManager) RegisterErrorHandler(errorType string, handler ErrorHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorHandlers[errorType] = handler
}

// UnregisterErrorHandler 取消注册错误处理器
func (m *RecoveryManager) UnregisterErrorHandler(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.errorHandlers, errorType)
}

// GetErrorHandler 获取错误处理器
func (m *RecoveryManager) GetErrorHandler(errorType string) ErrorHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if handler, exists := m.errorHandlers[errorType]; exists {
		return handler
	}

	return m.defaultErrorHandler
}

// SetDefaultErrorHandler 设置默认错误处理器
func (m *RecoveryManager) SetDefaultErrorHandler(handler ErrorHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultErrorHandler = handler
}

// SetRetryConfig 设置重试配置
func (m *RecoveryManager) SetRetryConfig(maxRetries int, maxRetryDelay time.Duration, backoffFactor float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxRetries = maxRetries
	m.maxRetryDelay = maxRetryDelay
	m.backoffFactor = backoffFactor
}

// HandleError 处理错误
func (m *RecoveryManager) HandleError(ctx *ActorContext, envelope *message.Envelope, err error) error {
	errorType := getErrorType(err)
	handler := m.GetErrorHandler(errorType)

	return handler.HandleError(ctx, envelope, err)
}

// ShouldRetry 判断是否应该重试
func (m *RecoveryManager) ShouldRetry(ctx *ActorContext, envelope *message.Envelope, err error) bool {
	errorType := getErrorType(err)
	handler := m.GetErrorHandler(errorType)

	return handler.ShouldRetry(ctx, envelope, err)
}

// GetRetryDelay 获取重试延迟
func (m *RecoveryManager) GetRetryDelay(ctx *ActorContext, envelope *message.Envelope, err error) time.Duration {
	errorType := getErrorType(err)
	handler := m.GetErrorHandler(errorType)

	return handler.GetRetryDelay(ctx, envelope, err)
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	maxRetries  int
	retryDelays []time.Duration
}

// NewDefaultErrorHandler 创建新的默认错误处理器
func NewDefaultErrorHandler() *DefaultErrorHandler {
	return &DefaultErrorHandler{
		maxRetries: 3,
		retryDelays: []time.Duration{
			1 * time.Second,
			5 * time.Second,
			10 * time.Second,
		},
	}
}

// HandleError 处理错误
func (h *DefaultErrorHandler) HandleError(ctx *ActorContext, envelope *message.Envelope, err error) error {
	// 默认实现：记录错误并返回
	// 在实际应用中，这里应该使用日志系统
	// fmt.Printf("Error handling message: actor=%s, error=%v\n", ctx.Address().String(), err)

	// 更新统计信息
	if ctx != nil {
		ctx.UpdateStats(func(stats *ActorStats) {
			stats.MessagesFailed++
		})
	}

	return err
}

// ShouldRetry 判断是否应该重试
func (h *DefaultErrorHandler) ShouldRetry(ctx *ActorContext, envelope *message.Envelope, err error) bool {
	if envelope == nil {
		return false
	}

	// 检查是否已经超过最大重试次数
	delivery := envelope.Delivery()
	if delivery.Attempt >= h.maxRetries {
		return false
	}

	// 检查错误类型是否可重试
	return isRetryableError(err)
}

// GetRetryDelay 获取重试延迟
func (h *DefaultErrorHandler) GetRetryDelay(ctx *ActorContext, envelope *message.Envelope, err error) time.Duration {
	if envelope == nil {
		return 0
	}

	delivery := envelope.Delivery()
	attempt := delivery.Attempt

	// 使用指数退避算法
	if attempt-1 < len(h.retryDelays) {
		return h.retryDelays[attempt-1]
	}

	// 如果超过配置的延迟次数，使用最后一个延迟
	return h.retryDelays[len(h.retryDelays)-1]
}

// CircuitBreaker 断路器
type CircuitBreaker struct {
	mu sync.RWMutex

	// 断路器状态
	state CircuitBreakerState

	// 失败计数
	failureCount     int
	failureThreshold int

	// 时间窗口
	windowStart time.Time
	windowSize  time.Duration

	// 半开状态超时
	halfOpenTimeout time.Duration
	halfOpenTimer   *time.Timer

	// 成功计数（用于半开状态）
	successCount     int
	successThreshold int

	// 回调函数
	onStateChange func(oldState, newState CircuitBreakerState)
}

// CircuitBreakerState 断路器状态
type CircuitBreakerState int

const (
	// CircuitBreakerClosed 闭合状态（正常）
	CircuitBreakerClosed CircuitBreakerState = iota
	// CircuitBreakerOpen 断开状态（停止服务）
	CircuitBreakerOpen
	// CircuitBreakerHalfOpen 半开状态（尝试恢复）
	CircuitBreakerHalfOpen
)

// NewCircuitBreaker 创建新的断路器
func NewCircuitBreaker(failureThreshold int, windowSize time.Duration) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:            CircuitBreakerClosed,
		failureThreshold: failureThreshold,
		windowSize:       windowSize,
		halfOpenTimeout:  30 * time.Second,
		successThreshold: 3,
		windowStart:      time.Now(),
	}

	return cb
}

// Execute 执行操作（受断路器保护）
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// 检查断路器状态
	if !cb.AllowRequest() {
		return &CircuitBreakerError{"circuit breaker is open"}
	}

	// 执行操作
	err := fn()

	// 记录结果
	if err != nil {
		cb.OnFailure()
	} else {
		cb.OnSuccess()
	}

	return err
}

// AllowRequest 检查是否允许请求
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerHalfOpen:
		// 在半开状态下，允许部分请求通过
		return true
	case CircuitBreakerOpen:
		return false
	default:
		return false
	}
}

// OnFailure 记录失败
func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 重置时间窗口（如果已过期）
	if time.Since(cb.windowStart) > cb.windowSize {
		cb.failureCount = 0
		cb.windowStart = time.Now()
	}

	cb.failureCount++

	// 检查是否需要切换到断开状态
	if cb.state == CircuitBreakerClosed && cb.failureCount >= cb.failureThreshold {
		cb.transitionTo(CircuitBreakerOpen)
	}

	// 如果在半开状态下失败，切换回断开状态
	if cb.state == CircuitBreakerHalfOpen {
		cb.transitionTo(CircuitBreakerOpen)
	}
}

// OnSuccess 记录成功
func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 如果在半开状态下成功，增加成功计数
	if cb.state == CircuitBreakerHalfOpen {
		cb.successCount++

		// 如果达到成功阈值，切换回闭合状态
		if cb.successCount >= cb.successThreshold {
			cb.transitionTo(CircuitBreakerClosed)
		}
	}

	// 如果在闭合状态下，重置失败计数（可选）
	if cb.state == CircuitBreakerClosed {
		cb.failureCount = 0
		cb.windowStart = time.Now()
	}
}

// transitionTo 切换状态
func (cb *CircuitBreaker) transitionTo(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState

	// 重置计数器
	cb.failureCount = 0
	cb.successCount = 0
	cb.windowStart = time.Now()

	// 处理状态特定的逻辑
	switch newState {
	case CircuitBreakerOpen:
		// 启动定时器切换到半开状态
		if cb.halfOpenTimer != nil {
			cb.halfOpenTimer.Stop()
		}
		cb.halfOpenTimer = time.AfterFunc(cb.halfOpenTimeout, func() {
			cb.mu.Lock()
			cb.transitionTo(CircuitBreakerHalfOpen)
			cb.mu.Unlock()
		})

	case CircuitBreakerHalfOpen:
		// 半开状态不需要特殊处理

	case CircuitBreakerClosed:
		// 停止定时器
		if cb.halfOpenTimer != nil {
			cb.halfOpenTimer.Stop()
			cb.halfOpenTimer = nil
		}
	}

	// 触发状态变更回调
	if cb.onStateChange != nil {
		cb.onStateChange(oldState, newState)
	}
}

// SetOnStateChange 设置状态变更回调
func (cb *CircuitBreaker) SetOnStateChange(fn func(oldState, newState CircuitBreakerState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.onStateChange = fn
}

// State 返回当前状态
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.state
}

// Reset 重置断路器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.transitionTo(CircuitBreakerClosed)
}

// CircuitBreakerError 断路器错误
type CircuitBreakerError struct {
	message string
}

func (e *CircuitBreakerError) Error() string {
	return "circuit breaker error: " + e.message
}

// Supervisor 监督者
type Supervisor struct {
	mu sync.RWMutex

	// 子actor映射
	children map[string]Actor

	// 监督策略
	strategy SupervisionStrategy

	// 最大重启次数
	maxRestarts       int
	maxRestartsWindow time.Duration

	// 重启计数
	restartCounts map[string]RestartInfo

	// 监控器
	monitor Monitor
}

// RestartInfo 重启信息
type RestartInfo struct {
	Count       int
	LastRestart time.Time
	WindowStart time.Time
}

// NewSupervisor 创建新的监督者
func NewSupervisor(strategy SupervisionStrategy) *Supervisor {
	return &Supervisor{
		children:          make(map[string]Actor),
		strategy:          strategy,
		maxRestarts:       3,
		maxRestartsWindow: 1 * time.Minute,
		restartCounts:     make(map[string]RestartInfo),
		monitor:           GetDefaultMonitor(),
	}
}

// AddChild 添加子actor
func (s *Supervisor) AddChild(child Actor) error {
	if child == nil {
		return ErrInvalidActor
	}

	address := child.Address()
	if address == nil {
		return ErrInvalidAddress
	}

	addrStr := address.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.children[addrStr]; exists {
		return ErrActorAlreadyRegistered
	}

	s.children[addrStr] = child
	s.restartCounts[addrStr] = RestartInfo{
		Count:       0,
		LastRestart: time.Time{},
		WindowStart: time.Now(),
	}

	return nil
}

// RemoveChild 移除子actor
func (s *Supervisor) RemoveChild(address message.Address) error {
	if address == nil {
		return ErrInvalidAddress
	}

	addrStr := address.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.children[addrStr]; !exists {
		return ErrActorNotFound
	}

	delete(s.children, addrStr)
	delete(s.restartCounts, addrStr)

	return nil
}

// HandleFailure 处理子actor失败
func (s *Supervisor) HandleFailure(failedActor Actor, err error) error {
	address := failedActor.Address()
	if address == nil {
		return ErrInvalidAddress
	}

	addrStr := address.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取重启信息
	info, exists := s.restartCounts[addrStr]
	if !exists {
		info = RestartInfo{
			Count:       0,
			LastRestart: time.Time{},
			WindowStart: time.Now(),
		}
	}

	// 检查时间窗口
	if time.Since(info.WindowStart) > s.maxRestartsWindow {
		// 重置计数器和时间窗口
		info.Count = 0
		info.WindowStart = time.Now()
	}

	// 检查是否超过最大重启次数
	if info.Count >= s.maxRestarts {
		// 停止actor
		failedActor.Stop()
		delete(s.children, addrStr)
		delete(s.restartCounts, addrStr)

		return &ActorError{"actor exceeded maximum restart count"}
	}

	// 应用监督策略
	switch s.strategy {
	case SupervisionStrategyResume:
		// 继续处理下一条消息
		return nil

	case SupervisionStrategyRestart:
		// 重启actor
		info.Count++
		info.LastRestart = time.Now()
		s.restartCounts[addrStr] = info

		// 停止actor
		failedActor.Stop()

		// 重新启动actor（在实际应用中，这里应该重新创建actor）
		// 简化实现：直接返回错误
		return &ActorError{"actor restart required"}

	case SupervisionStrategyStop:
		// 停止actor
		failedActor.Stop()
		delete(s.children, addrStr)
		delete(s.restartCounts, addrStr)

		return &ActorError{"actor stopped due to failure"}

	case SupervisionStrategyEscalate:
		// 升级到父监督者处理
		return &ActorError{"failure escalated to parent supervisor"}

	default:
		return &ActorError{"unknown supervision strategy"}
	}
}

// SetStrategy 设置监督策略
func (s *Supervisor) SetStrategy(strategy SupervisionStrategy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.strategy = strategy
}

// SetMaxRestarts 设置最大重启次数
func (s *Supervisor) SetMaxRestarts(maxRestarts int, window time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.maxRestarts = maxRestarts
	s.maxRestartsWindow = window
}

// SetMonitor 设置监控器
func (s *Supervisor) SetMonitor(monitor Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.monitor = monitor
}

// Children 返回所有子actor
func (s *Supervisor) Children() []Actor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	children := make([]Actor, 0, len(s.children))
	for _, child := range s.children {
		children = append(children, child)
	}

	return children
}

// RestartInfo 返回重启信息
func (s *Supervisor) RestartInfo(address string) (RestartInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.restartCounts[address]
	return info, exists
}

// 辅助函数：获取错误类型
func getErrorType(err error) string {
	if err == nil {
		return "unknown"
	}

	// 尝试获取错误类型名
	t := reflect.TypeOf(err)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t.Name()
}

// 辅助函数：判断错误是否可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 根据错误类型判断是否可重试
	// 在实际应用中，这里应该有更复杂的逻辑
	errorType := getErrorType(err)

	// 临时性错误通常可重试
	switch errorType {
	case "TimeoutError", "TemporaryError", "NetworkError":
		return true
	default:
		return false
	}
}

// 全局默认恢复管理器
var (
	defaultRecoveryManager     *RecoveryManager
	defaultRecoveryManagerOnce sync.Once
)

// GetDefaultRecoveryManager 获取默认恢复管理器
func GetDefaultRecoveryManager() *RecoveryManager {
	defaultRecoveryManagerOnce.Do(func() {
		defaultRecoveryManager = NewRecoveryManager()
	})
	return defaultRecoveryManager
}

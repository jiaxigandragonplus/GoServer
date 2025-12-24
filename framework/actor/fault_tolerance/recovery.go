package fault_tolerance

import (
	"reflect"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	// HandleError 处理错误
	HandleError(ctx *actor.ActorContext, envelope *message.Envelope, err error) error
	// ShouldRetry 判断是否应该重试
	ShouldRetry(ctx *actor.ActorContext, envelope *message.Envelope, err error) bool
	// GetRetryDelay 获取重试延迟
	GetRetryDelay(ctx *actor.ActorContext, envelope *message.Envelope, err error) time.Duration
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
func (m *RecoveryManager) HandleError(ctx *actor.ActorContext, envelope *message.Envelope, err error) error {
	errorType := getErrorType(err)
	handler := m.GetErrorHandler(errorType)

	return handler.HandleError(ctx, envelope, err)
}

// ShouldRetry 判断是否应该重试
func (m *RecoveryManager) ShouldRetry(ctx *actor.ActorContext, envelope *message.Envelope, err error) bool {
	errorType := getErrorType(err)
	handler := m.GetErrorHandler(errorType)

	return handler.ShouldRetry(ctx, envelope, err)
}

// GetRetryDelay 获取重试延迟
func (m *RecoveryManager) GetRetryDelay(ctx *actor.ActorContext, envelope *message.Envelope, err error) time.Duration {
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
func (h *DefaultErrorHandler) HandleError(ctx *actor.ActorContext, envelope *message.Envelope, err error) error {
	// 默认实现：记录错误并返回
	// 在实际应用中，这里应该使用日志系统
	// fmt.Printf("Error handling message: actor=%s, error=%v\n", ctx.Address().String(), err)

	// 更新统计信息
	if ctx != nil {
		ctx.UpdateStats(func(stats *actor.ActorStats) {
			stats.MessagesFailed++
		})
	}

	return err
}

// ShouldRetry 判断是否应该重试
func (h *DefaultErrorHandler) ShouldRetry(ctx *actor.ActorContext, envelope *message.Envelope, err error) bool {
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
func (h *DefaultErrorHandler) GetRetryDelay(ctx *actor.ActorContext, envelope *message.Envelope, err error) time.Duration {
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

// LoggingErrorHandler 日志错误处理器
type LoggingErrorHandler struct {
	DefaultErrorHandler
}

// NewLoggingErrorHandler 创建新的日志错误处理器
func NewLoggingErrorHandler() *LoggingErrorHandler {
	return &LoggingErrorHandler{
		DefaultErrorHandler: *NewDefaultErrorHandler(),
	}
}

// HandleError 处理错误（记录日志）
func (h *LoggingErrorHandler) HandleError(ctx *actor.ActorContext, envelope *message.Envelope, err error) error {
	if ctx != nil {
		// 在实际应用中，这里应该使用日志系统
		// log.Printf("Error handling message: actor=%s, error=%v\n", ctx.Address().String(), err)
	}

	return h.DefaultErrorHandler.HandleError(ctx, envelope, err)
}

// CircuitBreakerErrorHandler 断路器错误处理器
type CircuitBreakerErrorHandler struct {
	DefaultErrorHandler
	circuitBreaker *CircuitBreaker
}

// NewCircuitBreakerErrorHandler 创建新的断路器错误处理器
func NewCircuitBreakerErrorHandler(circuitBreaker *CircuitBreaker) *CircuitBreakerErrorHandler {
	return &CircuitBreakerErrorHandler{
		DefaultErrorHandler: *NewDefaultErrorHandler(),
		circuitBreaker:      circuitBreaker,
	}
}

// HandleError 处理错误（使用断路器）
func (h *CircuitBreakerErrorHandler) HandleError(ctx *actor.ActorContext, envelope *message.Envelope, err error) error {
	// 如果断路器打开，直接返回错误
	if h.circuitBreaker != nil && !h.circuitBreaker.AllowRequest() {
		return &CircuitBreakerError{"circuit breaker is open"}
	}

	return h.DefaultErrorHandler.HandleError(ctx, envelope, err)
}

// RetryErrorHandler 重试错误处理器
type RetryErrorHandler struct {
	DefaultErrorHandler
	maxRetries int
}

// NewRetryErrorHandler 创建新的重试错误处理器
func NewRetryErrorHandler(maxRetries int) *RetryErrorHandler {
	return &RetryErrorHandler{
		DefaultErrorHandler: *NewDefaultErrorHandler(),
		maxRetries:          maxRetries,
	}
}

// ShouldRetry 判断是否应该重试
func (h *RetryErrorHandler) ShouldRetry(ctx *actor.ActorContext, envelope *message.Envelope, err error) bool {
	if envelope == nil {
		return false
	}

	delivery := envelope.Delivery()
	if delivery.Attempt >= h.maxRetries {
		return false
	}

	return isRetryableError(err)
}

// GetRetryDelay 获取重试延迟（使用指数退避）
func (h *RetryErrorHandler) GetRetryDelay(ctx *actor.ActorContext, envelope *message.Envelope, err error) time.Duration {
	if envelope == nil {
		return 0
	}

	delivery := envelope.Delivery()
	attempt := delivery.Attempt

	// 指数退避算法：2^attempt * baseDelay
	baseDelay := 100 * time.Millisecond
	delay := baseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
			break
		}
	}

	return delay
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
	case "CircuitBreakerError":
		return false // 断路器错误不可重试
	default:
		// 默认情况下，非致命错误可重试
		return true
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

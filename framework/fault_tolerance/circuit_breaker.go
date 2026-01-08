package fault_tolerance

import (
	"sync"
	"time"
)

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

// CircuitBreakerError 断路器错误
type CircuitBreakerError struct {
	message string
}

func (e *CircuitBreakerError) Error() string {
	return "circuit breaker error: " + e.message
}

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
		// 这里可以添加更复杂的逻辑，比如限制请求速率
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

// SetFailureThreshold 设置失败阈值
func (cb *CircuitBreaker) SetFailureThreshold(threshold int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureThreshold = threshold
}

// SetWindowSize 设置时间窗口大小
func (cb *CircuitBreaker) SetWindowSize(windowSize time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.windowSize = windowSize
}

// SetHalfOpenTimeout 设置半开状态超时时间
func (cb *CircuitBreaker) SetHalfOpenTimeout(timeout time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.halfOpenTimeout = timeout
}

// SetSuccessThreshold 设置成功阈值
func (cb *CircuitBreaker) SetSuccessThreshold(threshold int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successThreshold = threshold
}

// FailureCount 返回失败计数
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.failureCount
}

// SuccessCount 返回成功计数
func (cb *CircuitBreaker) SuccessCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.successCount
}

// TimeWindowBreaker 时间窗口断路器（基于时间窗口的失败计数）
type TimeWindowBreaker struct {
	CircuitBreaker
}

// NewTimeWindowBreaker 创建新的时间窗口断路器
func NewTimeWindowBreaker(failureThreshold int, windowSize time.Duration) *TimeWindowBreaker {
	return &TimeWindowBreaker{
		CircuitBreaker: *NewCircuitBreaker(failureThreshold, windowSize),
	}
}

// CountBasedBreaker 计数断路器（基于连续失败计数）
type CountBasedBreaker struct {
	CircuitBreaker
	consecutiveFailures    int
	maxConsecutiveFailures int
}

// NewCountBasedBreaker 创建新的计数断路器
func NewCountBasedBreaker(maxConsecutiveFailures int) *CountBasedBreaker {
	return &CountBasedBreaker{
		CircuitBreaker:         *NewCircuitBreaker(maxConsecutiveFailures, 0),
		maxConsecutiveFailures: maxConsecutiveFailures,
	}
}

// OnFailure 记录失败（计数断路器）
func (cb *CountBasedBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++

	// 检查是否需要切换到断开状态
	if cb.state == CircuitBreakerClosed && cb.consecutiveFailures >= cb.maxConsecutiveFailures {
		cb.transitionTo(CircuitBreakerOpen)
	}

	// 如果在半开状态下失败，切换回断开状态
	if cb.state == CircuitBreakerHalfOpen {
		cb.transitionTo(CircuitBreakerOpen)
	}
}

// OnSuccess 记录成功（计数断路器）
func (cb *CountBasedBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 重置连续失败计数
	cb.consecutiveFailures = 0

	// 调用父类方法
	cb.CircuitBreaker.OnSuccess()
}

// PercentageBreaker 百分比断路器（基于失败百分比）
type PercentageBreaker struct {
	CircuitBreaker
	totalRequests        int
	failedRequests       int
	failurePercentage    float64
	maxFailurePercentage float64
}

// NewPercentageBreaker 创建新的百分比断路器
func NewPercentageBreaker(maxFailurePercentage float64, windowSize time.Duration) *PercentageBreaker {
	return &PercentageBreaker{
		CircuitBreaker:       *NewCircuitBreaker(0, windowSize),
		maxFailurePercentage: maxFailurePercentage,
	}
}

// OnFailure 记录失败（百分比断路器）
func (cb *PercentageBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++
	cb.failedRequests++

	// 计算失败百分比
	if cb.totalRequests > 0 {
		cb.failurePercentage = float64(cb.failedRequests) / float64(cb.totalRequests) * 100
	}

	// 检查是否需要切换到断开状态
	if cb.state == CircuitBreakerClosed && cb.failurePercentage >= cb.maxFailurePercentage {
		cb.transitionTo(CircuitBreakerOpen)
	}

	// 如果在半开状态下失败，切换回断开状态
	if cb.state == CircuitBreakerHalfOpen {
		cb.transitionTo(CircuitBreakerOpen)
	}
}

// OnSuccess 记录成功（百分比断路器）
func (cb *PercentageBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	// 计算失败百分比
	if cb.totalRequests > 0 {
		cb.failurePercentage = float64(cb.failedRequests) / float64(cb.totalRequests) * 100
	}

	// 调用父类方法
	cb.CircuitBreaker.OnSuccess()
}

// ResetCounters 重置计数器（百分比断路器）
func (cb *PercentageBreaker) ResetCounters() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests = 0
	cb.failedRequests = 0
	cb.failurePercentage = 0
}

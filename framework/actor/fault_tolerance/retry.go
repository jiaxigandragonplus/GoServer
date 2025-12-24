package fault_tolerance

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// RetryPolicy 重试策略接口
type RetryPolicy interface {
	// ShouldRetry 判断是否应该重试
	ShouldRetry(ctx context.Context, attempt int, err error) bool
	// GetDelay 获取重试延迟
	GetDelay(ctx context.Context, attempt int, err error) time.Duration
	// GetMaxAttempts 获取最大重试次数
	GetMaxAttempts() int
}

// RetryExecutor 重试执行器
type RetryExecutor struct {
	policy RetryPolicy
}

// NewRetryExecutor 创建新的重试执行器
func NewRetryExecutor(policy RetryPolicy) *RetryExecutor {
	return &RetryExecutor{
		policy: policy,
	}
}

// Execute 执行操作（带重试）
func (e *RetryExecutor) Execute(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= e.policy.GetMaxAttempts(); attempt++ {
		// 执行操作
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否应该重试
		if !e.policy.ShouldRetry(ctx, attempt, err) {
			break
		}

		// 获取重试延迟
		delay := e.policy.GetDelay(ctx, attempt, err)

		// 等待延迟
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 继续重试
		}
	}

	return lastErr
}

// FixedRetryPolicy 固定重试策略
type FixedRetryPolicy struct {
	maxAttempts     int
	delay           time.Duration
	retryableErrors map[string]bool
}

// NewFixedRetryPolicy 创建新的固定重试策略
func NewFixedRetryPolicy(maxAttempts int, delay time.Duration) *FixedRetryPolicy {
	return &FixedRetryPolicy{
		maxAttempts:     maxAttempts,
		delay:           delay,
		retryableErrors: make(map[string]bool),
	}
}

// ShouldRetry 判断是否应该重试
func (p *FixedRetryPolicy) ShouldRetry(ctx context.Context, attempt int, err error) bool {
	if attempt >= p.maxAttempts {
		return false
	}

	// 检查错误类型是否可重试
	if len(p.retryableErrors) > 0 {
		errorType := getErrorType(err)
		return p.retryableErrors[errorType]
	}

	// 默认所有错误都可重试
	return true
}

// GetDelay 获取重试延迟
func (p *FixedRetryPolicy) GetDelay(ctx context.Context, attempt int, err error) time.Duration {
	return p.delay
}

// GetMaxAttempts 获取最大重试次数
func (p *FixedRetryPolicy) GetMaxAttempts() int {
	return p.maxAttempts
}

// AddRetryableError 添加可重试错误类型
func (p *FixedRetryPolicy) AddRetryableError(errorType string) {
	p.retryableErrors[errorType] = true
}

// RemoveRetryableError 移除可重试错误类型
func (p *FixedRetryPolicy) RemoveRetryableError(errorType string) {
	delete(p.retryableErrors, errorType)
}

// ExponentialBackoffRetryPolicy 指数退避重试策略
type ExponentialBackoffRetryPolicy struct {
	maxAttempts     int
	initialDelay    time.Duration
	maxDelay        time.Duration
	backoffFactor   float64
	jitterFactor    float64
	retryableErrors map[string]bool
}

// NewExponentialBackoffRetryPolicy 创建新的指数退避重试策略
func NewExponentialBackoffRetryPolicy(maxAttempts int, initialDelay, maxDelay time.Duration) *ExponentialBackoffRetryPolicy {
	return &ExponentialBackoffRetryPolicy{
		maxAttempts:     maxAttempts,
		initialDelay:    initialDelay,
		maxDelay:        maxDelay,
		backoffFactor:   2.0,
		jitterFactor:    0.1,
		retryableErrors: make(map[string]bool),
	}
}

// ShouldRetry 判断是否应该重试
func (p *ExponentialBackoffRetryPolicy) ShouldRetry(ctx context.Context, attempt int, err error) bool {
	if attempt >= p.maxAttempts {
		return false
	}

	// 检查错误类型是否可重试
	if len(p.retryableErrors) > 0 {
		errorType := getErrorType(err)
		return p.retryableErrors[errorType]
	}

	// 默认所有错误都可重试
	return true
}

// GetDelay 获取重试延迟
func (p *ExponentialBackoffRetryPolicy) GetDelay(ctx context.Context, attempt int, err error) time.Duration {
	// 计算指数退避延迟
	delay := p.initialDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * p.backoffFactor)
		if delay > p.maxDelay {
			delay = p.maxDelay
			break
		}
	}

	// 添加抖动
	if p.jitterFactor > 0 {
		jitter := float64(delay) * p.jitterFactor
		delay += time.Duration((rand.Float64()*2 - 1) * jitter)
		if delay < 0 {
			delay = 0
		}
	}

	return delay
}

// GetMaxAttempts 获取最大重试次数
func (p *ExponentialBackoffRetryPolicy) GetMaxAttempts() int {
	return p.maxAttempts
}

// SetBackoffFactor 设置退避因子
func (p *ExponentialBackoffRetryPolicy) SetBackoffFactor(factor float64) {
	p.backoffFactor = factor
}

// SetJitterFactor 设置抖动因子
func (p *ExponentialBackoffRetryPolicy) SetJitterFactor(factor float64) {
	p.jitterFactor = factor
}

// AddRetryableError 添加可重试错误类型
func (p *ExponentialBackoffRetryPolicy) AddRetryableError(errorType string) {
	p.retryableErrors[errorType] = true
}

// FibonacciRetryPolicy 斐波那契重试策略
type FibonacciRetryPolicy struct {
	maxAttempts     int
	initialDelay    time.Duration
	maxDelay        time.Duration
	retryableErrors map[string]bool
	fibCache        map[int]int
	fibCacheMu      sync.RWMutex
}

// NewFibonacciRetryPolicy 创建新的斐波那契重试策略
func NewFibonacciRetryPolicy(maxAttempts int, initialDelay, maxDelay time.Duration) *FibonacciRetryPolicy {
	return &FibonacciRetryPolicy{
		maxAttempts:     maxAttempts,
		initialDelay:    initialDelay,
		maxDelay:        maxDelay,
		retryableErrors: make(map[string]bool),
		fibCache:        make(map[int]int),
	}
}

// ShouldRetry 判断是否应该重试
func (p *FibonacciRetryPolicy) ShouldRetry(ctx context.Context, attempt int, err error) bool {
	if attempt >= p.maxAttempts {
		return false
	}

	// 检查错误类型是否可重试
	if len(p.retryableErrors) > 0 {
		errorType := getErrorType(err)
		return p.retryableErrors[errorType]
	}

	// 默认所有错误都可重试
	return true
}

// GetDelay 获取重试延迟
func (p *FibonacciRetryPolicy) GetDelay(ctx context.Context, attempt int, err error) time.Duration {
	// 计算斐波那契数
	fib := p.fibonacci(attempt)

	// 计算延迟
	delay := p.initialDelay * time.Duration(fib)
	if delay > p.maxDelay {
		delay = p.maxDelay
	}

	return delay
}

// GetMaxAttempts 获取最大重试次数
func (p *FibonacciRetryPolicy) GetMaxAttempts() int {
	return p.maxAttempts
}

// fibonacci 计算斐波那契数
func (p *FibonacciRetryPolicy) fibonacci(n int) int {
	if n <= 1 {
		return n
	}

	p.fibCacheMu.RLock()
	if val, exists := p.fibCache[n]; exists {
		p.fibCacheMu.RUnlock()
		return val
	}
	p.fibCacheMu.RUnlock()

	p.fibCacheMu.Lock()
	defer p.fibCacheMu.Unlock()

	// 再次检查（双重检查锁定）
	if val, exists := p.fibCache[n]; exists {
		return val
	}

	val := p.fibonacci(n-1) + p.fibonacci(n-2)
	p.fibCache[n] = val
	return val
}

// AddRetryableError 添加可重试错误类型
func (p *FibonacciRetryPolicy) AddRetryableError(errorType string) {
	p.retryableErrors[errorType] = true
}

// ActorRetryHandler actor重试处理器
type ActorRetryHandler struct {
	retryExecutor *RetryExecutor
}

// NewActorRetryHandler 创建新的actor重试处理器
func NewActorRetryHandler(policy RetryPolicy) *ActorRetryHandler {
	return &ActorRetryHandler{
		retryExecutor: NewRetryExecutor(policy),
	}
}

// HandleMessageWithRetry 处理消息（带重试）
func (h *ActorRetryHandler) HandleMessageWithRetry(ctx context.Context, actor actor.Actor, envelope *message.Envelope) error {
	return h.retryExecutor.Execute(ctx, func() error {
		return actor.HandleMessage(ctx, envelope)
	})
}

// MessageRetryDecorator 消息重试装饰器
type MessageRetryDecorator struct {
	actor        actor.Actor
	retryHandler *ActorRetryHandler
}

// NewMessageRetryDecorator 创建新的消息重试装饰器
func NewMessageRetryDecorator(actor actor.Actor, policy RetryPolicy) *MessageRetryDecorator {
	return &MessageRetryDecorator{
		actor:        actor,
		retryHandler: NewActorRetryHandler(policy),
	}
}

// HandleMessage 处理消息（带重试）
func (d *MessageRetryDecorator) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	return d.retryHandler.HandleMessageWithRetry(ctx, d.actor, envelope)
}

// Address 返回actor地址
func (d *MessageRetryDecorator) Address() message.Address {
	return d.actor.Address()
}

// Mailbox 返回actor邮箱
func (d *MessageRetryDecorator) Mailbox() mailbox.Mailbox {
	return d.actor.Mailbox()
}

// Start 启动actor
func (d *MessageRetryDecorator) Start(ctx context.Context) error {
	return d.actor.Start(ctx)
}

// Stop 停止actor
func (d *MessageRetryDecorator) Stop() error {
	return d.actor.Stop()
}

// IsRunning 返回是否正在运行
func (d *MessageRetryDecorator) IsRunning() bool {
	return d.actor.IsRunning()
}

// PreStart 在actor启动前调用
func (d *MessageRetryDecorator) PreStart(ctx context.Context) error {
	return d.actor.PreStart(ctx)
}

// PostStart 在actor启动后调用
func (d *MessageRetryDecorator) PostStart(ctx context.Context) error {
	return d.actor.PostStart(ctx)
}

// PreStop 在actor停止前调用
func (d *MessageRetryDecorator) PreStop(ctx context.Context) error {
	return d.actor.PreStop(ctx)
}

// PostStop 在actor停止后调用
func (d *MessageRetryDecorator) PostStop(ctx context.Context) error {
	return d.actor.PostStop(ctx)
}

// HealthCheck 健康检查
func (d *MessageRetryDecorator) HealthCheck(ctx context.Context) (actor.HealthStatus, error) {
	return d.actor.HealthCheck(ctx)
}

// GetState 获取actor状态
func (d *MessageRetryDecorator) GetState() actor.State {
	return d.actor.GetState()
}

// SetState 设置actor状态
func (d *MessageRetryDecorator) SetState(state actor.State) error {
	return d.actor.SetState(state)
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
	RetryableErrors []string
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:     3,
	InitialDelay:    100 * time.Millisecond,
	MaxDelay:        30 * time.Second,
	BackoffFactor:   2.0,
	JitterFactor:    0.1,
	RetryableErrors: []string{"TimeoutError", "NetworkError", "TemporaryError"},
}

// CreateRetryPolicyFromConfig 从配置创建重试策略
func CreateRetryPolicyFromConfig(config RetryConfig) RetryPolicy {
	policy := NewExponentialBackoffRetryPolicy(
		config.MaxAttempts,
		config.InitialDelay,
		config.MaxDelay,
	)
	policy.SetBackoffFactor(config.BackoffFactor)
	policy.SetJitterFactor(config.JitterFactor)

	for _, errorType := range config.RetryableErrors {
		policy.AddRetryableError(errorType)
	}

	return policy
}

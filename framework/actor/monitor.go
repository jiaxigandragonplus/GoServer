package actor

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// DefaultMonitor 默认监控器实现
type DefaultMonitor struct {
	mu sync.RWMutex

	// 统计信息
	stats MonitorStats

	// 失败记录
	failures    []ActorFailure
	maxFailures int

	// 事件处理器
	eventHandlers []MonitorEventHandler
}

// MonitorStats 监控器统计信息
type MonitorStats struct {
	// TotalActors 总actor数
	TotalActors int64
	// ActiveActors 活跃actor数
	ActiveActors int64
	// FailedActors 失败actor数
	FailedActors int64
	// TotalMessages 总消息数
	TotalMessages int64
	// FailedMessages 失败消息数
	FailedMessages int64
	// TotalRestarts 总重启次数
	TotalRestarts int64
	// LastFailureTime 最后失败时间
	LastFailureTime time.Time
}

// ActorFailure actor失败记录
type ActorFailure struct {
	ActorAddress string
	ActorType    string
	Error        string
	Timestamp    time.Time
	Strategy     SupervisionStrategy
}

// MonitorEventHandler 监控器事件处理器
type MonitorEventHandler interface {
	// OnActorFailure actor失败时调用
	OnActorFailure(failure ActorFailure)
	// OnActorStarted actor启动时调用
	OnActorStarted(address string, actorType string)
	// OnActorStopped actor停止时调用
	OnActorStopped(address string, actorType string)
	// OnMessageProcessed 消息处理完成时调用
	OnMessageProcessed(address string, success bool)
}

// NewDefaultMonitor 创建新的默认监控器
func NewDefaultMonitor() *DefaultMonitor {
	return &DefaultMonitor{
		stats:         MonitorStats{},
		failures:      make([]ActorFailure, 0),
		maxFailures:   1000,
		eventHandlers: make([]MonitorEventHandler, 0),
	}
}

// HandleFailure 处理actor失败
func (m *DefaultMonitor) HandleFailure(ctx *ActorContext, failedActor Actor, err error) SupervisionStrategy {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新统计信息
	m.stats.FailedActors++
	m.stats.LastFailureTime = time.Now()

	// 记录失败
	failure := ActorFailure{
		ActorAddress: failedActor.Address().String(),
		ActorType:    getActorType(failedActor),
		Error:        err.Error(),
		Timestamp:    time.Now(),
		Strategy:     ctx.SupervisionStrategy(),
	}

	m.failures = append(m.failures, failure)

	// 保持失败记录数量在限制内
	if len(m.failures) > m.maxFailures {
		m.failures = m.failures[1:]
	}

	// 触发事件处理器
	for _, handler := range m.eventHandlers {
		handler.OnActorFailure(failure)
	}

	return ctx.SupervisionStrategy()
}

// OnActorStarted actor启动时调用
func (m *DefaultMonitor) OnActorStarted(ctx *ActorContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalActors++
	m.stats.ActiveActors++

	// 触发事件处理器
	address := ctx.Address().String()
	actorType := getActorType(ctx.Actor())
	for _, handler := range m.eventHandlers {
		handler.OnActorStarted(address, actorType)
	}
}

// OnActorStopped actor停止时调用
func (m *DefaultMonitor) OnActorStopped(ctx *ActorContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.ActiveActors--

	// 触发事件处理器
	address := ctx.Address().String()
	actorType := getActorType(ctx.Actor())
	for _, handler := range m.eventHandlers {
		handler.OnActorStopped(address, actorType)
	}
}

// OnMessageProcessed 消息处理完成时调用
func (m *DefaultMonitor) OnMessageProcessed(ctx *ActorContext, envelope *message.Envelope, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalMessages++
	if err != nil {
		m.stats.FailedMessages++
	}

	// 触发事件处理器
	address := ctx.Address().String()
	success := err == nil
	for _, handler := range m.eventHandlers {
		handler.OnMessageProcessed(address, success)
	}
}

// Stats 返回统计信息
func (m *DefaultMonitor) Stats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// Failures 返回失败记录
func (m *DefaultMonitor) Failures() []ActorFailure {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	failures := make([]ActorFailure, len(m.failures))
	copy(failures, m.failures)

	return failures
}

// RecentFailures 返回最近的失败记录
func (m *DefaultMonitor) RecentFailures(count int) []ActorFailure {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if count > len(m.failures) {
		count = len(m.failures)
	}

	// 返回最近的count个失败记录
	start := len(m.failures) - count
	recent := make([]ActorFailure, count)
	copy(recent, m.failures[start:])

	return recent
}

// ClearFailures 清空失败记录
func (m *DefaultMonitor) ClearFailures() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failures = make([]ActorFailure, 0)
}

// SetMaxFailures 设置最大失败记录数
func (m *DefaultMonitor) SetMaxFailures(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxFailures = max

	// 如果当前记录数超过最大值，截断
	if len(m.failures) > max {
		m.failures = m.failures[len(m.failures)-max:]
	}
}

// RegisterEventHandler 注册事件处理器
func (m *DefaultMonitor) RegisterEventHandler(handler MonitorEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.eventHandlers = append(m.eventHandlers, handler)
}

// UnregisterEventHandler 取消注册事件处理器
func (m *DefaultMonitor) UnregisterEventHandler(handler MonitorEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, h := range m.eventHandlers {
		if h == handler {
			m.eventHandlers = append(m.eventHandlers[:i], m.eventHandlers[i+1:]...)
			break
		}
	}
}

// LoggingMonitor 日志监控器（将事件记录到日志）
type LoggingMonitor struct{}

// NewLoggingMonitor 创建新的日志监控器
func NewLoggingMonitor() *LoggingMonitor {
	return &LoggingMonitor{}
}

// HandleFailure 处理actor失败
func (m *LoggingMonitor) HandleFailure(ctx *ActorContext, failedActor Actor, err error) SupervisionStrategy {
	address := failedActor.Address().String()
	actorType := getActorType(failedActor)
	strategy := ctx.SupervisionStrategy()

	fmt.Printf("[MONITOR] Actor failure: address=%s, type=%s, error=%v, strategy=%v\n",
		address, actorType, err, strategy)

	return strategy
}

// OnActorStarted actor启动时调用
func (m *LoggingMonitor) OnActorStarted(ctx *ActorContext) {
	address := ctx.Address().String()
	actorType := getActorType(ctx.Actor())

	fmt.Printf("[MONITOR] Actor started: address=%s, type=%s\n", address, actorType)
}

// OnActorStopped actor停止时调用
func (m *LoggingMonitor) OnActorStopped(ctx *ActorContext) {
	address := ctx.Address().String()
	actorType := getActorType(ctx.Actor())

	fmt.Printf("[MONITOR] Actor stopped: address=%s, type=%s\n", address, actorType)
}

// OnMessageProcessed 消息处理完成时调用
func (m *LoggingMonitor) OnMessageProcessed(ctx *ActorContext, envelope *message.Envelope, err error) {
	address := ctx.Address().String()
	msgType := envelope.Message().Type()

	if err != nil {
		fmt.Printf("[MONITOR] Message processing failed: address=%s, type=%s, error=%v\n",
			address, msgType, err)
	}
}

// CompositeMonitor 组合监控器
type CompositeMonitor struct {
	monitors []Monitor
}

// NewCompositeMonitor 创建新的组合监控器
func NewCompositeMonitor(monitors ...Monitor) *CompositeMonitor {
	return &CompositeMonitor{
		monitors: monitors,
	}
}

// HandleFailure 处理actor失败
func (m *CompositeMonitor) HandleFailure(ctx *ActorContext, failedActor Actor, err error) SupervisionStrategy {
	var strategy SupervisionStrategy

	for _, monitor := range m.monitors {
		s := monitor.HandleFailure(ctx, failedActor, err)
		// 使用第一个监控器的策略
		if strategy == 0 {
			strategy = s
		}
	}

	return strategy
}

// OnActorStarted actor启动时调用
func (m *CompositeMonitor) OnActorStarted(ctx *ActorContext) {
	for _, monitor := range m.monitors {
		monitor.OnActorStarted(ctx)
	}
}

// OnActorStopped actor停止时调用
func (m *CompositeMonitor) OnActorStopped(ctx *ActorContext) {
	for _, monitor := range m.monitors {
		monitor.OnActorStopped(ctx)
	}
}

// OnMessageProcessed 消息处理完成时调用
func (m *CompositeMonitor) OnMessageProcessed(ctx *ActorContext, envelope *message.Envelope, err error) {
	for _, monitor := range m.monitors {
		monitor.OnMessageProcessed(ctx, envelope, err)
	}
}

// AddMonitor 添加监控器
func (m *CompositeMonitor) AddMonitor(monitor Monitor) {
	m.monitors = append(m.monitors, monitor)
}

// RemoveMonitor 移除监控器
func (m *CompositeMonitor) RemoveMonitor(monitor Monitor) {
	for i, mon := range m.monitors {
		if mon == monitor {
			m.monitors = append(m.monitors[:i], m.monitors[i+1:]...)
			break
		}
	}
}

// ActorStateManager actor状态管理器
type ActorStateManager struct {
	mu sync.RWMutex

	// actor状态映射
	states map[string]ActorStateInfo

	// 状态变更处理器
	stateChangeHandlers []StateChangeHandler
}

// ActorStateInfo actor状态信息
type ActorStateInfo struct {
	Address    string
	ActorType  string
	State      State
	Health     HealthStatus
	Stats      ActorStats
	StartedAt  time.Time
	StoppedAt  time.Time
	LastUpdate time.Time
}

// StateChangeHandler 状态变更处理器
type StateChangeHandler interface {
	// OnStateChanged 状态变更时调用
	OnStateChanged(oldState, newState ActorStateInfo)
	// OnHealthChanged 健康状态变更时调用
	OnHealthChanged(address string, oldHealth, newHealth HealthStatus)
}

// NewActorStateManager 创建新的actor状态管理器
func NewActorStateManager() *ActorStateManager {
	return &ActorStateManager{
		states:              make(map[string]ActorStateInfo),
		stateChangeHandlers: make([]StateChangeHandler, 0),
	}
}

// UpdateState 更新actor状态
func (m *ActorStateManager) UpdateState(actor Actor) error {
	if actor == nil {
		return ErrInvalidActor
	}

	address := actor.Address()
	if address == nil {
		return ErrInvalidAddress
	}

	addrStr := address.String()

	// 获取当前状态
	state := actor.GetState()
	health, _ := actor.HealthCheck(context.Background())

	// 获取统计信息（如果actor支持）
	var stats ActorStats
	if baseActor, ok := actor.(*BaseActor); ok {
		stats = baseActor.stats
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	oldState, exists := m.states[addrStr]

	newState := ActorStateInfo{
		Address:    addrStr,
		ActorType:  getActorType(actor),
		State:      state,
		Health:     health,
		Stats:      stats,
		LastUpdate: time.Now(),
	}

	// 如果是新actor，设置启动时间
	if !exists && state == StateRunning {
		newState.StartedAt = time.Now()
	}

	// 如果actor停止，设置停止时间
	if exists && oldState.State != StateStopped && state == StateStopped {
		newState.StoppedAt = time.Now()
		newState.StartedAt = oldState.StartedAt
	}

	// 保存新状态
	m.states[addrStr] = newState

	// 触发状态变更事件
	if exists {
		for _, handler := range m.stateChangeHandlers {
			handler.OnStateChanged(oldState, newState)

			// 检查健康状态是否变更
			if oldState.Health != newState.Health {
				handler.OnHealthChanged(addrStr, oldState.Health, newState.Health)
			}
		}
	}

	return nil
}

// GetState 获取actor状态
func (m *ActorStateManager) GetState(address string) (ActorStateInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[address]
	return state, exists
}

// GetAllStates 获取所有actor状态
func (m *ActorStateManager) GetAllStates() []ActorStateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]ActorStateInfo, 0, len(m.states))
	for _, state := range m.states {
		states = append(states, state)
	}

	return states
}

// GetStatesByHealth 根据健康状态获取actor状态
func (m *ActorStateManager) GetStatesByHealth(health HealthStatus) []ActorStateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]ActorStateInfo, 0)
	for _, state := range m.states {
		if state.Health == health {
			states = append(states, state)
		}
	}

	return states
}

// RemoveState 移除actor状态
func (m *ActorStateManager) RemoveState(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, address)
}

// RegisterStateChangeHandler 注册状态变更处理器
func (m *ActorStateManager) RegisterStateChangeHandler(handler StateChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stateChangeHandlers = append(m.stateChangeHandlers, handler)
}

// UnregisterStateChangeHandler 取消注册状态变更处理器
func (m *ActorStateManager) UnregisterStateChangeHandler(handler StateChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, h := range m.stateChangeHandlers {
		if h == handler {
			m.stateChangeHandlers = append(m.stateChangeHandlers[:i], m.stateChangeHandlers[i+1:]...)
			break
		}
	}
}

// 辅助函数：获取actor类型
func getActorType(actor Actor) string {
	if actor == nil {
		return "unknown"
	}

	// 尝试通过反射获取类型名
	t := reflect.TypeOf(actor)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t.Name()
}

// 全局默认监控器
var (
	defaultMonitor     Monitor
	defaultMonitorOnce sync.Once
)

// GetDefaultMonitor 获取默认监控器
func GetDefaultMonitor() Monitor {
	defaultMonitorOnce.Do(func() {
		defaultMonitor = NewDefaultMonitor()
	})
	return defaultMonitor
}

// 全局默认状态管理器
var (
	defaultStateManager     *ActorStateManager
	defaultStateManagerOnce sync.Once
)

// GetDefaultStateManager 获取默认状态管理器
func GetDefaultStateManager() *ActorStateManager {
	defaultStateManagerOnce.Do(func() {
		defaultStateManager = NewActorStateManager()
	})
	return defaultStateManager
}

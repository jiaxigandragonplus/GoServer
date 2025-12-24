package fault_tolerance

import (
	"context"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
	"github.com/GooLuck/GoServer/framework/actor/monitor"
)

// SupervisionStrategy 监督策略类型
type SupervisionStrategy int

const (
	// SupervisionStrategyResume 恢复策略：继续处理下一条消息
	SupervisionStrategyResume SupervisionStrategy = iota
	// SupervisionStrategyRestart 重启策略：重启失败的actor
	SupervisionStrategyRestart
	// SupervisionStrategyStop 停止策略：停止失败的actor
	SupervisionStrategyStop
	// SupervisionStrategyEscalate 升级策略：将失败升级到父监督者
	SupervisionStrategyEscalate
)

// Supervisor 监督者接口
type Supervisor interface {
	// HandleFailure 处理actor失败
	HandleFailure(failedActor actor.Actor, err error) error
	// AddChild 添加子actor
	AddChild(child actor.Actor) error
	// RemoveChild 移除子actor
	RemoveChild(address message.Address) error
	// Children 返回所有子actor
	Children() []actor.Actor
	// SetStrategy 设置监督策略
	SetStrategy(strategy SupervisionStrategy)
	// SetMaxRestarts 设置最大重启次数
	SetMaxRestarts(maxRestarts int, window time.Duration)
}

// FaultToleranceError 容错错误
type FaultToleranceError struct {
	message string
}

func (e *FaultToleranceError) Error() string {
	return "fault tolerance error: " + e.message
}

// DefaultSupervisor 默认监督者实现
type DefaultSupervisor struct {
	mu sync.RWMutex

	// 子actor映射
	children map[string]actor.Actor

	// 监督策略
	strategy SupervisionStrategy

	// 最大重启次数配置
	maxRestarts       int
	maxRestartsWindow time.Duration

	// 重启计数
	restartCounts map[string]RestartInfo

	// 监控器
	monitor actor.Monitor
}

// RestartInfo 重启信息
type RestartInfo struct {
	Count       int
	LastRestart time.Time
	WindowStart time.Time
}

// NewDefaultSupervisor 创建新的默认监督者
func NewDefaultSupervisor(strategy SupervisionStrategy) *DefaultSupervisor {
	return &DefaultSupervisor{
		children:          make(map[string]actor.Actor),
		strategy:          strategy,
		maxRestarts:       3,
		maxRestartsWindow: 1 * time.Minute,
		restartCounts:     make(map[string]RestartInfo),
		monitor:           monitor.GetDefaultMonitor(),
	}
}

// HandleFailure 处理子actor失败
func (s *DefaultSupervisor) HandleFailure(failedActor actor.Actor, err error) error {
	address := failedActor.Address()
	if address == nil {
		return actor.ErrInvalidAddress
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

		return &FaultToleranceError{"actor exceeded maximum restart count"}
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

		// 重新启动actor
		ctx := context.Background()
		if err := failedActor.Start(ctx); err != nil {
			return &FaultToleranceError{"failed to restart actor: " + err.Error()}
		}

		return nil

	case SupervisionStrategyStop:
		// 停止actor
		failedActor.Stop()
		delete(s.children, addrStr)
		delete(s.restartCounts, addrStr)

		return &FaultToleranceError{"actor stopped due to failure"}

	case SupervisionStrategyEscalate:
		// 升级到父监督者处理
		return &FaultToleranceError{"failure escalated to parent supervisor"}

	default:
		return &FaultToleranceError{"unknown supervision strategy"}
	}
}

// AddChild 添加子actor
func (s *DefaultSupervisor) AddChild(child actor.Actor) error {
	if child == nil {
		return actor.ErrInvalidActor
	}

	address := child.Address()
	if address == nil {
		return actor.ErrInvalidAddress
	}

	addrStr := address.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.children[addrStr]; exists {
		return actor.ErrActorAlreadyRegistered
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
func (s *DefaultSupervisor) RemoveChild(address message.Address) error {
	if address == nil {
		return actor.ErrInvalidAddress
	}

	addrStr := address.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.children[addrStr]; !exists {
		return actor.ErrActorNotFound
	}

	delete(s.children, addrStr)
	delete(s.restartCounts, addrStr)

	return nil
}

// Children 返回所有子actor
func (s *DefaultSupervisor) Children() []actor.Actor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	children := make([]actor.Actor, 0, len(s.children))
	for _, child := range s.children {
		children = append(children, child)
	}

	return children
}

// SetStrategy 设置监督策略
func (s *DefaultSupervisor) SetStrategy(strategy SupervisionStrategy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.strategy = strategy
}

// SetMaxRestarts 设置最大重启次数
func (s *DefaultSupervisor) SetMaxRestarts(maxRestarts int, window time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.maxRestarts = maxRestarts
	s.maxRestartsWindow = window
}

// SetMonitor 设置监控器
func (s *DefaultSupervisor) SetMonitor(monitor actor.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.monitor = monitor
}

// RestartInfo 返回重启信息
func (s *DefaultSupervisor) RestartInfo(address string) (RestartInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.restartCounts[address]
	return info, exists
}

// OneForOneSupervisor 一对一监督者（每个子actor独立监督）
type OneForOneSupervisor struct {
	DefaultSupervisor
}

// NewOneForOneSupervisor 创建新的一对一监督者
func NewOneForOneSupervisor(strategy SupervisionStrategy) *OneForOneSupervisor {
	return &OneForOneSupervisor{
		DefaultSupervisor: *NewDefaultSupervisor(strategy),
	}
}

// OneForAllSupervisor 一对所有监督者（一个子actor失败影响所有子actor）
type OneForAllSupervisor struct {
	DefaultSupervisor
}

// NewOneForAllSupervisor 创建新的一对所有监督者
func NewOneForAllSupervisor(strategy SupervisionStrategy) *OneForAllSupervisor {
	return &OneForAllSupervisor{
		DefaultSupervisor: *NewDefaultSupervisor(strategy),
	}
}

// HandleFailure 处理子actor失败（一对所有策略）
func (s *OneForAllSupervisor) HandleFailure(failedActor actor.Actor, err error) error {
	// 先调用父类处理失败
	if err := s.DefaultSupervisor.HandleFailure(failedActor, err); err != nil {
		return err
	}

	// 一对所有策略：如果一个actor失败，重启所有子actor
	if s.strategy == SupervisionStrategyRestart {
		s.mu.Lock()
		defer s.mu.Unlock()

		// 重启所有子actor
		for addrStr, child := range s.children {
			// 跳过已经失败的actor（已经在父类中处理）
			if child.Address().String() == failedActor.Address().String() {
				continue
			}

			// 停止并重启actor
			child.Stop()
			ctx := context.Background()
			if err := child.Start(ctx); err != nil {
				// 记录错误但继续处理其他actor
				continue
			}

			// 更新重启计数
			info := s.restartCounts[addrStr]
			info.Count++
			info.LastRestart = time.Now()
			s.restartCounts[addrStr] = info
		}
	}

	return nil
}

// RestartingSupervisor 重启监督者（专门用于重启策略）
type RestartingSupervisor struct {
	DefaultSupervisor
}

// NewRestartingSupervisor 创建新的重启监督者
func NewRestartingSupervisor() *RestartingSupervisor {
	return &RestartingSupervisor{
		DefaultSupervisor: *NewDefaultSupervisor(SupervisionStrategyRestart),
	}
}

// StoppingSupervisor 停止监督者（专门用于停止策略）
type StoppingSupervisor struct {
	DefaultSupervisor
}

// NewStoppingSupervisor 创建新的停止监督者
func NewStoppingSupervisor() *StoppingSupervisor {
	return &StoppingSupervisor{
		DefaultSupervisor: *NewDefaultSupervisor(SupervisionStrategyStop),
	}
}

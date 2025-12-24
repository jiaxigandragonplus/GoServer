package router

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// ActorRouter actor路由器接口
type ActorRouter interface {
	// Route 路由消息到目标actor地址
	Route(ctx context.Context, msg message.Message, candidates []message.Address) ([]message.Address, error)
	// AddCandidate 添加候选actor地址
	AddCandidate(address message.Address) error
	// RemoveCandidate 移除候选actor地址
	RemoveCandidate(address message.Address) error
	// GetCandidates 获取所有候选actor地址
	GetCandidates() []message.Address
	// UpdateCandidateStatus 更新候选actor状态
	UpdateCandidateStatus(address message.Address, status CandidateStatus) error
	// GetCandidateStatus 获取候选actor状态
	GetCandidateStatus(address message.Address) (CandidateStatus, error)
}

// CandidateStatus 候选actor状态
type CandidateStatus struct {
	// Address actor地址
	Address message.Address
	// Healthy 是否健康
	Healthy bool
	// Load 当前负载（0-100）
	Load int
	// LastHeartbeat 最后心跳时间
	LastHeartbeat time.Time
	// Metadata 元数据
	Metadata map[string]string
}

// RoutingStrategy 路由策略
type RoutingStrategy int

const (
	// StrategyRoundRobin 轮询策略
	StrategyRoundRobin RoutingStrategy = iota
	// StrategyRandom 随机策略
	StrategyRandom
	// StrategyHash 哈希策略（基于消息内容）
	StrategyHash
	// StrategyBroadcast 广播策略
	StrategyBroadcast
	// StrategyLeastLoaded 最小负载策略
	StrategyLeastLoaded
	// StrategySticky 粘性会话策略
	StrategySticky
	// StrategyLocationAware 位置感知策略
	StrategyLocationAware
	// StrategyCustom 自定义策略
	StrategyCustom
)

// GroupingStrategy 分组策略
type GroupingStrategy int

const (
	// GroupByMessageType 按消息类型分组
	GroupByMessageType GroupingStrategy = iota
	// GroupBySender 按发送者分组
	GroupBySender
	// GroupByKey 按自定义键分组
	GroupByKey
	// GroupByHash 按哈希值分组
	GroupByHash
	// GroupBySession 按会话分组（扩展）
	GroupBySession
	// GroupByTenant 按租户分组（扩展）
	GroupByTenant
	// GroupByPriority 按优先级分组（扩展）
	GroupByPriority
)

// BaseActorRouter 基础actor路由器实现
type BaseActorRouter struct {
	candidates     map[string]CandidateStatus
	candidatesLock sync.RWMutex
	strategy       RoutingStrategy
	grouping       GroupingStrategy
	rrIndex        int
	rrLock         sync.Mutex
	stickySessions map[string]message.Address // 粘性会话映射
	stickyLock     sync.RWMutex
	customRouter   CustomRouter // 自定义路由器
}

// CustomRouter 自定义路由器接口
type CustomRouter interface {
	Route(ctx context.Context, msg message.Message, candidates []CandidateStatus) ([]message.Address, error)
}

// NewBaseActorRouter 创建新的基础actor路由器
func NewBaseActorRouter(strategy RoutingStrategy, grouping GroupingStrategy) *BaseActorRouter {
	return &BaseActorRouter{
		candidates:     make(map[string]CandidateStatus),
		strategy:       strategy,
		grouping:       grouping,
		rrIndex:        0,
		stickySessions: make(map[string]message.Address),
	}
}

// NewBaseActorRouterWithCustom 创建带自定义路由器的actor路由器
func NewBaseActorRouterWithCustom(customRouter CustomRouter) *BaseActorRouter {
	return &BaseActorRouter{
		candidates:   make(map[string]CandidateStatus),
		strategy:     StrategyCustom,
		grouping:     GroupByMessageType,
		customRouter: customRouter,
	}
}

// Route 路由消息
func (r *BaseActorRouter) Route(ctx context.Context, msg message.Message, candidates []message.Address) ([]message.Address, error) {
	if len(candidates) == 0 {
		// 如果没有提供候选列表，使用内部管理的候选
		r.candidatesLock.RLock()
		internalCandidates := make([]message.Address, 0, len(r.candidates))
		for _, status := range r.candidates {
			if status.Healthy {
				internalCandidates = append(internalCandidates, status.Address)
			}
		}
		r.candidatesLock.RUnlock()
		candidates = internalCandidates
	}

	if len(candidates) == 0 {
		return nil, errors.New("no available candidates for routing")
	}

	// 应用分组策略
	groupedCandidates := r.applyGrouping(msg, candidates)

	// 应用路由策略
	return r.applyRoutingStrategy(ctx, msg, groupedCandidates)
}

// AddCandidate 添加候选actor
func (r *BaseActorRouter) AddCandidate(address message.Address) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	addrStr := address.String()

	r.candidatesLock.Lock()
	defer r.candidatesLock.Unlock()

	if _, exists := r.candidates[addrStr]; exists {
		return fmt.Errorf("candidate already exists: %s", addrStr)
	}

	r.candidates[addrStr] = CandidateStatus{
		Address:       address,
		Healthy:       true,
		Load:          0,
		LastHeartbeat: time.Now(),
		Metadata:      make(map[string]string),
	}

	return nil
}

// RemoveCandidate 移除候选actor
func (r *BaseActorRouter) RemoveCandidate(address message.Address) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	addrStr := address.String()

	r.candidatesLock.Lock()
	defer r.candidatesLock.Unlock()

	if _, exists := r.candidates[addrStr]; !exists {
		return fmt.Errorf("candidate not found: %s", addrStr)
	}

	delete(r.candidates, addrStr)

	// 清理粘性会话
	r.stickyLock.Lock()
	for key, addr := range r.stickySessions {
		if addr.String() == addrStr {
			delete(r.stickySessions, key)
		}
	}
	r.stickyLock.Unlock()

	return nil
}

// GetCandidates 获取所有候选actor
func (r *BaseActorRouter) GetCandidates() []message.Address {
	r.candidatesLock.RLock()
	defer r.candidatesLock.RUnlock()

	candidates := make([]message.Address, 0, len(r.candidates))
	for _, status := range r.candidates {
		candidates = append(candidates, status.Address)
	}

	return candidates
}

// UpdateCandidateStatus 更新候选actor状态
func (r *BaseActorRouter) UpdateCandidateStatus(address message.Address, status CandidateStatus) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	addrStr := address.String()

	r.candidatesLock.Lock()
	defer r.candidatesLock.Unlock()

	if _, exists := r.candidates[addrStr]; !exists {
		return fmt.Errorf("candidate not found: %s", addrStr)
	}

	// 保留地址
	status.Address = address
	r.candidates[addrStr] = status

	return nil
}

// GetCandidateStatus 获取候选actor状态
func (r *BaseActorRouter) GetCandidateStatus(address message.Address) (CandidateStatus, error) {
	if address == nil {
		return CandidateStatus{}, errors.New("address cannot be nil")
	}

	addrStr := address.String()

	r.candidatesLock.RLock()
	defer r.candidatesLock.RUnlock()

	status, exists := r.candidates[addrStr]
	if !exists {
		return CandidateStatus{}, fmt.Errorf("candidate not found: %s", addrStr)
	}

	return status, nil
}

// applyGrouping 应用分组策略
func (r *BaseActorRouter) applyGrouping(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) <= 1 {
		return candidates
	}

	switch r.grouping {
	case GroupByMessageType:
		return r.groupByMessageType(msg, candidates)
	case GroupBySender:
		return r.groupBySender(msg, candidates)
	case GroupByKey:
		return r.groupByKey(msg, candidates)
	case GroupByHash:
		return r.groupByHash(msg, candidates)
	default:
		return candidates
	}
}

// applyRoutingStrategy 应用路由策略
func (r *BaseActorRouter) applyRoutingStrategy(ctx context.Context, msg message.Message, candidates []message.Address) ([]message.Address, error) {
	if r.strategy == StrategyCustom && r.customRouter != nil {
		// 获取候选状态
		r.candidatesLock.RLock()
		candidateStatuses := make([]CandidateStatus, 0, len(candidates))
		for _, addr := range candidates {
			if status, exists := r.candidates[addr.String()]; exists {
				candidateStatuses = append(candidateStatuses, status)
			}
		}
		r.candidatesLock.RUnlock()

		return r.customRouter.Route(ctx, msg, candidateStatuses)
	}

	switch r.strategy {
	case StrategyRoundRobin:
		return r.applyRoundRobin(msg, candidates), nil
	case StrategyRandom:
		return r.applyRandom(msg, candidates), nil
	case StrategyHash:
		return r.applyHash(msg, candidates), nil
	case StrategyBroadcast:
		return candidates, nil
	case StrategyLeastLoaded:
		return r.applyLeastLoaded(msg, candidates), nil
	case StrategySticky:
		return r.applySticky(msg, candidates), nil
	case StrategyLocationAware:
		return r.applyLocationAware(msg, candidates), nil
	default:
		// 默认使用轮询
		return r.applyRoundRobin(msg, candidates), nil
	}
}

// applyRoundRobin 应用轮询策略
func (r *BaseActorRouter) applyRoundRobin(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	r.rrLock.Lock()
	index := r.rrIndex % len(candidates)
	target := candidates[index]
	r.rrIndex = (index + 1) % len(candidates)
	r.rrLock.Unlock()

	return []message.Address{target}
}

// applyRandom 应用随机策略
func (r *BaseActorRouter) applyRandom(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	index := rand.Intn(len(candidates))
	return []message.Address{candidates[index]}
}

// applyHash 应用哈希策略
func (r *BaseActorRouter) applyHash(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	// 使用消息ID进行哈希
	h := fnv.New32a()
	h.Write([]byte(msg.ID()))
	hash := h.Sum32()
	index := int(hash % uint32(len(candidates)))

	return []message.Address{candidates[index]}
}

// applyLeastLoaded 应用最小负载策略
func (r *BaseActorRouter) applyLeastLoaded(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	r.candidatesLock.RLock()
	defer r.candidatesLock.RUnlock()

	// 找到负载最小的候选
	var minLoad = 101 // 初始值大于最大负载
	var selected message.Address

	for _, addr := range candidates {
		if status, exists := r.candidates[addr.String()]; exists && status.Healthy {
			if status.Load < minLoad {
				minLoad = status.Load
				selected = addr
			}
		}
	}

	if selected == nil {
		// 如果没有找到健康的候选，返回第一个
		return []message.Address{candidates[0]}
	}

	return []message.Address{selected}
}

// applySticky 应用粘性会话策略
func (r *BaseActorRouter) applySticky(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	// 使用发送者ID作为粘性键
	sender := msg.Sender()
	if sender == nil {
		// 如果没有发送者，使用消息ID
		return r.applyHash(msg, candidates)
	}

	senderStr := sender.String()

	r.stickyLock.RLock()
	stickyAddr, exists := r.stickySessions[senderStr]
	r.stickyLock.RUnlock()

	if exists {
		// 检查粘性地址是否在候选列表中且健康
		for _, addr := range candidates {
			if addr.String() == stickyAddr.String() {
				// 检查健康状态
				r.candidatesLock.RLock()
				status, statusExists := r.candidates[addr.String()]
				r.candidatesLock.RUnlock()

				if statusExists && status.Healthy {
					return []message.Address{addr}
				}
				// 如果不健康，继续选择新的
				break
			}
		}
	}

	// 选择新的地址（使用轮询）
	newAddr := r.applyRoundRobin(msg, candidates)[0]

	// 更新粘性映射
	r.stickyLock.Lock()
	r.stickySessions[senderStr] = newAddr
	r.stickyLock.Unlock()

	return []message.Address{newAddr}
}

// applyLocationAware 应用位置感知策略（简化实现）
func (r *BaseActorRouter) applyLocationAware(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	// 简化实现：优先选择与发送者在同一区域的候选
	sender := msg.Sender()
	if sender == nil {
		return r.applyRoundRobin(msg, candidates)
	}

	// 这里应该实现更复杂的位置感知逻辑
	// 当前简化实现：使用哈希策略
	return r.applyHash(msg, candidates)
}

// groupByMessageType 按消息类型分组
func (r *BaseActorRouter) groupByMessageType(msg message.Message, candidates []message.Address) []message.Address {
	// 简化实现：所有候选都可以处理所有消息类型
	// 实际应用中可以根据actor能力进行过滤
	return candidates
}

// groupBySender 按发送者分组
func (r *BaseActorRouter) groupBySender(msg message.Message, candidates []message.Address) []message.Address {
	// 简化实现：所有候选都可以处理所有发送者
	return candidates
}

// groupByKey 按自定义键分组
func (r *BaseActorRouter) groupByKey(msg message.Message, candidates []message.Address) []message.Address {
	// 简化实现：需要消息提供分组键
	// 当前返回所有候选
	return candidates
}

// groupByHash 按哈希值分组
func (r *BaseActorRouter) groupByHash(msg message.Message, candidates []message.Address) []message.Address {
	if len(candidates) <= 1 {
		return candidates
	}

	// 使用消息ID进行哈希分组
	h := fnv.New32a()
	h.Write([]byte(msg.ID()))
	hash := h.Sum32()

	// 选择哈希值对应的候选
	index := int(hash % uint32(len(candidates)))
	return []message.Address{candidates[index]}
}

// RouterManager actor路由器管理器
type RouterManager struct {
	routers map[string]ActorRouter
	mu      sync.RWMutex
}

// NewRouterManager 创建新的路由器管理器
func NewRouterManager() *RouterManager {
	return &RouterManager{
		routers: make(map[string]ActorRouter),
	}
}

// RegisterRouter 注册路由器
func (rm *RouterManager) RegisterRouter(name string, router ActorRouter) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.routers[name]; exists {
		return fmt.Errorf("router already registered: %s", name)
	}

	rm.routers[name] = router
	return nil
}

// GetRouter 获取路由器
func (rm *RouterManager) GetRouter(name string) (ActorRouter, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	router, exists := rm.routers[name]
	if !exists {
		return nil, fmt.Errorf("router not found: %s", name)
	}

	return router, nil
}

// UnregisterRouter 注销路由器
func (rm *RouterManager) UnregisterRouter(name string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.routers[name]; !exists {
		return fmt.Errorf("router not found: %s", name)
	}

	delete(rm.routers, name)
	return nil
}

// 全局默认actor路由器
var (
	defaultActorRouter     ActorRouter
	defaultActorRouterOnce sync.Once
)

// GetDefaultActorRouter 获取默认actor路由器
func GetDefaultActorRouter() ActorRouter {
	defaultActorRouterOnce.Do(func() {
		defaultActorRouter = NewBaseActorRouter(StrategyRoundRobin, GroupByMessageType)
	})
	return defaultActorRouter
}

// SetDefaultActorRouter 设置默认actor路由器
func SetDefaultActorRouter(router ActorRouter) {
	defaultActorRouter = router
}

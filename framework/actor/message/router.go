package message

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

// Router 消息路由器接口
type Router interface {
	// Route 路由消息到目标地址
	Route(ctx context.Context, msg Message) ([]Address, error)
	// AddRoute 添加路由规则
	AddRoute(rule RouteRule) error
	// RemoveRoute 移除路由规则
	RemoveRoute(ruleID string) error
	// GetRoutes 获取所有路由规则
	GetRoutes() []RouteRule
	// ClearRoutes 清空所有路由规则
	ClearRoutes()
}

// RouteRule 路由规则
type RouteRule struct {
	// ID 规则ID
	ID string
	// Pattern 匹配模式
	Pattern string
	// Targets 目标地址列表
	Targets []Address
	// Strategy 路由策略
	Strategy RoutingStrategy
	// Priority 规则优先级
	Priority int
	// CreatedAt 创建时间
	CreatedAt time.Time
}

// RoutingStrategy 路由策略
type RoutingStrategy int

const (
	// StrategyBroadcast 广播策略：发送给所有目标
	StrategyBroadcast RoutingStrategy = iota
	// StrategyRoundRobin 轮询策略：轮流发送给目标
	StrategyRoundRobin
	// StrategyRandom 随机策略：随机选择一个目标
	StrategyRandom
	// StrategyHash 哈希策略：根据消息内容哈希选择目标
	StrategyHash
	// StrategyPriority 优先级策略：按优先级选择目标
	StrategyPriority
	// StrategyFirstAvailable 第一个可用策略：选择第一个可用目标
	StrategyFirstAvailable
)

// BaseRouter 基础路由器实现
type BaseRouter struct {
	rules     map[string]RouteRule
	rulesLock sync.RWMutex
	rrIndex   map[string]int // 轮询索引
	rrLock    sync.Mutex
}

// NewBaseRouter 创建新的基础路由器
func NewBaseRouter() *BaseRouter {
	return &BaseRouter{
		rules:   make(map[string]RouteRule),
		rrIndex: make(map[string]int),
	}
}

// Route 路由消息
func (r *BaseRouter) Route(ctx context.Context, msg Message) ([]Address, error) {
	r.rulesLock.RLock()
	defer r.rulesLock.RUnlock()

	// 收集所有匹配的规则
	var matchedRules []RouteRule
	for _, rule := range r.rules {
		if r.matchRule(rule, msg) {
			matchedRules = append(matchedRules, rule)
		}
	}

	// 按优先级排序（优先级高的在前）
	sortRulesByPriority(matchedRules)

	// 应用规则
	var targets []Address
	for _, rule := range matchedRules {
		ruleTargets, err := r.applyStrategy(rule, msg)
		if err != nil {
			return nil, err
		}
		targets = append(targets, ruleTargets...)
	}

	// 去重
	targets = deduplicateAddresses(targets)

	if len(targets) == 0 {
		return nil, fmt.Errorf("no route found for message type: %s", msg.Type())
	}

	return targets, nil
}

// AddRoute 添加路由规则
func (r *BaseRouter) AddRoute(rule RouteRule) error {
	if rule.ID == "" {
		return fmt.Errorf("route rule ID cannot be empty")
	}

	if len(rule.Targets) == 0 {
		return fmt.Errorf("route rule must have at least one target")
	}

	rule.CreatedAt = time.Now()

	r.rulesLock.Lock()
	r.rules[rule.ID] = rule
	r.rulesLock.Unlock()

	return nil
}

// RemoveRoute 移除路由规则
func (r *BaseRouter) RemoveRoute(ruleID string) error {
	r.rulesLock.Lock()
	defer r.rulesLock.Unlock()

	if _, exists := r.rules[ruleID]; !exists {
		return fmt.Errorf("route rule not found: %s", ruleID)
	}

	delete(r.rules, ruleID)
	return nil
}

// GetRoutes 获取所有路由规则
func (r *BaseRouter) GetRoutes() []RouteRule {
	r.rulesLock.RLock()
	defer r.rulesLock.RUnlock()

	rules := make([]RouteRule, 0, len(r.rules))
	for _, rule := range r.rules {
		rules = append(rules, rule)
	}

	return rules
}

// ClearRoutes 清空所有路由规则
func (r *BaseRouter) ClearRoutes() {
	r.rulesLock.Lock()
	r.rules = make(map[string]RouteRule)
	r.rulesLock.Unlock()
}

// matchRule 检查消息是否匹配规则
func (r *BaseRouter) matchRule(rule RouteRule, msg Message) bool {
	// 简单实现：按消息类型匹配
	// 实际应用中可以实现更复杂的模式匹配
	return rule.Pattern == msg.Type() || rule.Pattern == "*"
}

// applyStrategy 应用路由策略
func (r *BaseRouter) applyStrategy(rule RouteRule, msg Message) ([]Address, error) {
	switch rule.Strategy {
	case StrategyBroadcast:
		return rule.Targets, nil
	case StrategyRoundRobin:
		return r.applyRoundRobin(rule, msg), nil
	case StrategyRandom:
		return r.applyRandom(rule, msg), nil
	case StrategyHash:
		return r.applyHash(rule, msg), nil
	case StrategyPriority:
		return r.applyPriority(rule, msg), nil
	case StrategyFirstAvailable:
		return r.applyFirstAvailable(rule, msg), nil
	default:
		return rule.Targets, nil
	}
}

// applyRoundRobin 应用轮询策略
func (r *BaseRouter) applyRoundRobin(rule RouteRule, msg Message) []Address {
	r.rrLock.Lock()
	defer r.rrLock.Unlock()

	indexKey := rule.ID
	index := r.rrIndex[indexKey] % len(rule.Targets)
	target := rule.Targets[index]
	r.rrIndex[indexKey] = (index + 1) % len(rule.Targets)

	return []Address{target}
}

// applyRandom 应用随机策略
func (r *BaseRouter) applyRandom(rule RouteRule, msg Message) []Address {
	// 简化实现：使用时间戳作为随机源
	index := time.Now().UnixNano() % int64(len(rule.Targets))
	return []Address{rule.Targets[index]}
}

// applyHash 应用哈希策略
func (r *BaseRouter) applyHash(rule RouteRule, msg Message) []Address {
	// 使用消息ID进行哈希
	h := fnv.New32a()
	h.Write([]byte(msg.ID()))
	hash := h.Sum32()
	index := int(hash % uint32(len(rule.Targets)))
	return []Address{rule.Targets[index]}
}

// applyPriority 应用优先级策略
func (r *BaseRouter) applyPriority(rule RouteRule, msg Message) []Address {
	// 简化实现：返回第一个目标
	// 实际应用中可以根据目标的状态、负载等确定优先级
	if len(rule.Targets) > 0 {
		return []Address{rule.Targets[0]}
	}
	return []Address{}
}

// applyFirstAvailable 应用第一个可用策略
func (r *BaseRouter) applyFirstAvailable(rule RouteRule, msg Message) []Address {
	// 简化实现：返回第一个目标
	// 实际应用中应该检查目标是否可用
	if len(rule.Targets) > 0 {
		return []Address{rule.Targets[0]}
	}
	return []Address{}
}

// sortRulesByPriority 按优先级排序规则
func sortRulesByPriority(rules []RouteRule) {
	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority < rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

// deduplicateAddresses 地址去重
func deduplicateAddresses(addresses []Address) []Address {
	seen := make(map[string]bool)
	result := make([]Address, 0, len(addresses))

	for _, addr := range addresses {
		if addr == nil {
			continue
		}
		addrStr := addr.String()
		if !seen[addrStr] {
			seen[addrStr] = true
			result = append(result, addr)
		}
	}

	return result
}

// RouterManager 路由器管理器
type RouterManager struct {
	routers map[string]Router
	mu      sync.RWMutex
}

// NewRouterManager 创建新的路由器管理器
func NewRouterManager() *RouterManager {
	return &RouterManager{
		routers: make(map[string]Router),
	}
}

// RegisterRouter 注册路由器
func (rm *RouterManager) RegisterRouter(name string, router Router) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.routers[name]; exists {
		return fmt.Errorf("router already registered: %s", name)
	}

	rm.routers[name] = router
	return nil
}

// GetRouter 获取路由器
func (rm *RouterManager) GetRouter(name string) (Router, error) {
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

// DefaultRouter 默认路由器
var (
	defaultRouter     Router
	defaultRouterOnce sync.Once
)

// GetDefaultRouter 获取默认路由器
func GetDefaultRouter() Router {
	defaultRouterOnce.Do(func() {
		defaultRouter = NewBaseRouter()
	})
	return defaultRouter
}

// SetDefaultRouter 设置默认路由器
func SetDefaultRouter(router Router) {
	defaultRouter = router
}

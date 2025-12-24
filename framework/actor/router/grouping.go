package router

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// MessageGrouper 消息分组器接口
type MessageGrouper interface {
	// Group 将消息分组到特定的actor
	Group(ctx context.Context, msg message.Message) (string, error)
	// GetGroupActor 获取分组对应的actor地址
	GetGroupActor(ctx context.Context, groupKey string) (message.Address, error)
	// UpdateGroupMapping 更新分组映射
	UpdateGroupMapping(groupKey string, address message.Address) error
	// RemoveGroupMapping 移除分组映射
	RemoveGroupMapping(groupKey string) error
	// GetGroupStats 获取分组统计信息
	GetGroupStats() map[string]GroupStats
}

// GroupStats 分组统计信息
type GroupStats struct {
	// GroupKey 分组键
	GroupKey string
	// ActorAddress actor地址
	ActorAddress message.Address
	// MessageCount 消息数量
	MessageCount int64
	// LastMessageTime 最后消息时间
	LastMessageTime time.Time
	// CreatedTime 创建时间
	CreatedTime time.Time
	// AverageProcessingTime 平均处理时间
	AverageProcessingTime time.Duration
}

// 注意：GroupingStrategy 类型和所有常量已在 router.go 中定义
// 这里不再重复定义

// BaseMessageGrouper 基础消息分组器实现
type BaseMessageGrouper struct {
	strategy       GroupingStrategy
	groupMappings  map[string]message.Address // groupKey -> address
	groupStats     map[string]GroupStats
	groupLock      sync.RWMutex
	hashRing       *ConsistentHash
	sessionManager *SessionManager
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions    map[string]string // sessionID -> groupKey
	sessionLock sync.RWMutex
}

// NewBaseMessageGrouper 创建新的基础消息分组器
func NewBaseMessageGrouper(strategy GroupingStrategy) *BaseMessageGrouper {
	grouper := &BaseMessageGrouper{
		strategy:      strategy,
		groupMappings: make(map[string]message.Address),
		groupStats:    make(map[string]GroupStats),
	}

	if strategy == GroupByHash {
		grouper.hashRing = NewConsistentHash(100)
	}

	if strategy == GroupBySession {
		grouper.sessionManager = &SessionManager{
			sessions: make(map[string]string),
		}
	}

	return grouper
}

// Group 分组消息
func (g *BaseMessageGrouper) Group(ctx context.Context, msg message.Message) (string, error) {
	switch g.strategy {
	case GroupByMessageType:
		return g.groupByMessageType(msg), nil
	case GroupBySender:
		return g.groupBySender(msg), nil
	case GroupByKey:
		return g.groupByKey(msg), nil
	case GroupByHash:
		return g.groupByHash(msg), nil
	case GroupBySession:
		return g.groupBySession(msg), nil
	case GroupByTenant:
		return g.groupByTenant(msg), nil
	case GroupByPriority:
		return g.groupByPriority(msg), nil
	default:
		return g.groupByMessageType(msg), nil
	}
}

// GetGroupActor 获取分组对应的actor
func (g *BaseMessageGrouper) GetGroupActor(ctx context.Context, groupKey string) (message.Address, error) {
	g.groupLock.RLock()
	address, exists := g.groupMappings[groupKey]
	g.groupLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no actor mapped for group: %s", groupKey)
	}

	return address, nil
}

// UpdateGroupMapping 更新分组映射
func (g *BaseMessageGrouper) UpdateGroupMapping(groupKey string, address message.Address) error {
	if groupKey == "" {
		return errors.New("group key cannot be empty")
	}

	if address == nil {
		return errors.New("address cannot be nil")
	}

	g.groupLock.Lock()
	defer g.groupLock.Unlock()

	// 更新映射
	g.groupMappings[groupKey] = address

	// 更新统计信息
	stats, exists := g.groupStats[groupKey]
	if !exists {
		stats = GroupStats{
			GroupKey:        groupKey,
			ActorAddress:    address,
			CreatedTime:     time.Now(),
			MessageCount:    0,
			LastMessageTime: time.Now(),
		}
	} else {
		stats.ActorAddress = address
		stats.LastMessageTime = time.Now()
	}
	g.groupStats[groupKey] = stats

	// 如果使用哈希策略，更新哈希环
	if g.strategy == GroupByHash && g.hashRing != nil {
		g.hashRing.AddNode(address)
	}

	return nil
}

// RemoveGroupMapping 移除分组映射
func (g *BaseMessageGrouper) RemoveGroupMapping(groupKey string) error {
	g.groupLock.Lock()
	defer g.groupLock.Unlock()

	// 从映射中移除
	delete(g.groupMappings, groupKey)

	// 从统计中移除
	delete(g.groupStats, groupKey)

	// 如果使用哈希策略，需要从哈希环中移除对应的节点
	// 注意：这里简化处理，实际需要更复杂的逻辑
	if g.strategy == GroupByHash && g.hashRing != nil {
		// 需要知道哪个地址被移除了，这里简化处理
		// 实际应用中应该维护groupKey到address的映射
	}

	return nil
}

// GetGroupStats 获取分组统计信息
func (g *BaseMessageGrouper) GetGroupStats() map[string]GroupStats {
	g.groupLock.RLock()
	defer g.groupLock.RUnlock()

	// 返回副本
	stats := make(map[string]GroupStats)
	for key, stat := range g.groupStats {
		stats[key] = stat
	}

	return stats
}

// RecordMessageProcessed 记录消息处理完成
func (g *BaseMessageGrouper) RecordMessageProcessed(groupKey string, processingTime time.Duration) {
	g.groupLock.Lock()
	defer g.groupLock.Unlock()

	stats, exists := g.groupStats[groupKey]
	if exists {
		stats.MessageCount++
		stats.LastMessageTime = time.Now()

		// 更新平均处理时间（指数移动平均）
		if stats.AverageProcessingTime == 0 {
			stats.AverageProcessingTime = processingTime
		} else {
			alpha := 0.1 // 平滑因子
			stats.AverageProcessingTime = time.Duration(
				alpha*float64(processingTime) + (1-alpha)*float64(stats.AverageProcessingTime),
			)
		}

		g.groupStats[groupKey] = stats
	}
}

// groupByMessageType 按消息类型分组
func (g *BaseMessageGrouper) groupByMessageType(msg message.Message) string {
	return msg.Type()
}

// groupBySender 按发送者分组
func (g *BaseMessageGrouper) groupBySender(msg message.Message) string {
	sender := msg.Sender()
	if sender != nil {
		return sender.String()
	}
	// 如果没有发送者，使用消息ID
	return msg.ID()
}

// groupByKey 按自定义键分组
func (g *BaseMessageGrouper) groupByKey(msg message.Message) string {
	// 尝试从消息中获取分组键
	// 这里需要消息接口支持GetMetadata方法
	// 简化实现：使用消息类型
	return msg.Type()
}

// groupByHash 按哈希值分组
func (g *BaseMessageGrouper) groupByHash(msg message.Message) string {
	// 使用消息ID进行哈希
	h := fnv.New32a()
	h.Write([]byte(msg.ID()))
	hash := h.Sum32()
	return fmt.Sprintf("hash_%d", hash)
}

// groupBySession 按会话分组
func (g *BaseMessageGrouper) groupBySession(msg message.Message) string {
	if g.sessionManager == nil {
		g.sessionManager = &SessionManager{
			sessions: make(map[string]string),
		}
	}

	// 尝试从消息中获取会话ID
	// 简化实现：使用发送者作为会话ID
	sender := msg.Sender()
	if sender == nil {
		return g.groupByMessageType(msg)
	}

	sessionID := sender.String()

	g.sessionManager.sessionLock.RLock()
	groupKey, exists := g.sessionManager.sessions[sessionID]
	g.sessionManager.sessionLock.RUnlock()

	if !exists {
		// 创建新的分组键
		groupKey = fmt.Sprintf("session_%s_%d", sessionID, time.Now().UnixNano())

		g.sessionManager.sessionLock.Lock()
		g.sessionManager.sessions[sessionID] = groupKey
		g.sessionManager.sessionLock.Unlock()
	}

	return groupKey
}

// groupByTenant 按租户分组
func (g *BaseMessageGrouper) groupByTenant(msg message.Message) string {
	// 尝试从消息中获取租户ID
	// 简化实现：使用消息类型前缀
	return fmt.Sprintf("tenant_%s", msg.Type())
}

// groupByPriority 按优先级分组
func (g *BaseMessageGrouper) groupByPriority(msg message.Message) string {
	// 尝试从消息中获取优先级
	// 简化实现：使用消息类型
	return fmt.Sprintf("priority_%s", msg.Type())
}

// GetSessionGroup 获取会话对应的分组
func (g *BaseMessageGrouper) GetSessionGroup(sessionID string) (string, bool) {
	if g.sessionManager == nil {
		return "", false
	}

	g.sessionManager.sessionLock.RLock()
	groupKey, exists := g.sessionManager.sessions[sessionID]
	g.sessionManager.sessionLock.RUnlock()

	return groupKey, exists
}

// ClearSession 清除会话
func (g *BaseMessageGrouper) ClearSession(sessionID string) {
	if g.sessionManager == nil {
		return
	}

	g.sessionManager.sessionLock.Lock()
	delete(g.sessionManager.sessions, sessionID)
	g.sessionManager.sessionLock.Unlock()
}

// GroupRouter 分组路由器
type GroupRouter struct {
	grouper     MessageGrouper
	actorRouter ActorRouter
	groupCache  map[string][]message.Address // groupKey -> candidate addresses
	cacheLock   sync.RWMutex
	cacheTTL    time.Duration
}

// NewGroupRouter 创建新的分组路由器
func NewGroupRouter(grouper MessageGrouper, actorRouter ActorRouter) *GroupRouter {
	return &GroupRouter{
		grouper:     grouper,
		actorRouter: actorRouter,
		groupCache:  make(map[string][]message.Address),
		cacheTTL:    5 * time.Minute,
	}
}

// Route 路由消息（带分组）
func (gr *GroupRouter) Route(ctx context.Context, msg message.Message, candidates []message.Address) ([]message.Address, error) {
	// 1. 分组消息
	groupKey, err := gr.grouper.Group(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to group message: %w", err)
	}

	// 2. 检查缓存
	gr.cacheLock.RLock()
	cachedCandidates, cacheHit := gr.groupCache[groupKey]
	gr.cacheLock.RUnlock()

	if cacheHit {
		// 使用缓存的候选列表
		return gr.actorRouter.Route(ctx, msg, cachedCandidates)
	}

	// 3. 获取分组对应的actor
	groupActor, err := gr.grouper.GetGroupActor(ctx, groupKey)
	if err == nil {
		// 有明确的分组actor，直接返回
		return []message.Address{groupActor}, nil
	}

	// 4. 没有分组actor，使用普通路由
	// 可以选择一个候选作为该分组的actor
	if len(candidates) > 0 {
		// 使用哈希策略选择一个候选
		selected := gr.selectCandidateForGroup(groupKey, candidates)

		// 更新分组映射
		gr.grouper.UpdateGroupMapping(groupKey, selected)

		// 缓存候选列表
		gr.cacheLock.Lock()
		gr.groupCache[groupKey] = []message.Address{selected}
		gr.cacheLock.Unlock()

		return []message.Address{selected}, nil
	}

	// 5. 没有候选，返回错误
	return nil, errors.New("no candidates available for grouping")
}

// selectCandidateForGroup 为分组选择候选
func (gr *GroupRouter) selectCandidateForGroup(groupKey string, candidates []message.Address) message.Address {
	if len(candidates) == 0 {
		return nil
	}

	// 使用一致性哈希选择
	h := fnv.New32a()
	h.Write([]byte(groupKey))
	hash := h.Sum32()
	index := int(hash % uint32(len(candidates)))

	return candidates[index]
}

// ClearCache 清除缓存
func (gr *GroupRouter) ClearCache() {
	gr.cacheLock.Lock()
	gr.groupCache = make(map[string][]message.Address)
	gr.cacheLock.Unlock()
}

// ClearCacheForGroup 清除特定分组的缓存
func (gr *GroupRouter) ClearCacheForGroup(groupKey string) {
	gr.cacheLock.Lock()
	delete(gr.groupCache, groupKey)
	gr.cacheLock.Unlock()
}

// GetCacheStats 获取缓存统计
func (gr *GroupRouter) GetCacheStats() map[string]int {
	gr.cacheLock.RLock()
	defer gr.cacheLock.RUnlock()

	stats := make(map[string]int)
	stats["total_groups"] = len(gr.groupCache)

	totalCandidates := 0
	for _, candidates := range gr.groupCache {
		totalCandidates += len(candidates)
	}
	stats["total_cached_candidates"] = totalCandidates

	return stats
}

// GroupAwareRouter 分组感知路由器
type GroupAwareRouter struct {
	router      ActorRouter
	grouper     MessageGrouper
	groupRouter *GroupRouter
}

// NewGroupAwareRouter 创建新的分组感知路由器
func NewGroupAwareRouter(router ActorRouter, grouper MessageGrouper) *GroupAwareRouter {
	return &GroupAwareRouter{
		router:      router,
		grouper:     grouper,
		groupRouter: NewGroupRouter(grouper, router),
	}
}

// Route 路由消息
func (gar *GroupAwareRouter) Route(ctx context.Context, msg message.Message, candidates []message.Address) ([]message.Address, error) {
	// 检查消息是否需要分组
	if gar.requiresGrouping(msg) {
		return gar.groupRouter.Route(ctx, msg, candidates)
	}

	// 不需要分组，使用普通路由
	return gar.router.Route(ctx, msg, candidates)
}

// requiresGrouping 检查消息是否需要分组
func (gar *GroupAwareRouter) requiresGrouping(msg message.Message) bool {
	// 可以根据消息类型、内容等决定是否需要分组
	// 简化实现：所有消息都分组
	return true
}

// GetGrouper 获取分组器
func (gar *GroupAwareRouter) GetGrouper() MessageGrouper {
	return gar.grouper
}

// GetGroupRouter 获取分组路由器
func (gar *GroupAwareRouter) GetGroupRouter() *GroupRouter {
	return gar.groupRouter
}

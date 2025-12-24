package router

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// Select 从候选列表中选择一个或多个地址
	Select(ctx context.Context, msg message.Message, candidates []CandidateStatus) ([]message.Address, error)
	// UpdateMetrics 更新候选节点的指标
	UpdateMetrics(address message.Address, metrics LoadMetrics) error
	// GetMetrics 获取候选节点的指标
	GetMetrics(address message.Address) (LoadMetrics, error)
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context, address message.Address) (bool, error)
}

// LoadMetrics 负载指标
type LoadMetrics struct {
	// Address 节点地址
	Address message.Address
	// CPUUsage CPU使用率（0-100）
	CPUUsage float64
	// MemoryUsage 内存使用率（0-100）
	MemoryUsage float64
	// ActiveConnections 活跃连接数
	ActiveConnections int
	// MessageQueueSize 消息队列大小
	MessageQueueSize int
	// ProcessingLatency 处理延迟（毫秒）
	ProcessingLatency time.Duration
	// ErrorRate 错误率（0-1）
	ErrorRate float64
	// LastUpdate 最后更新时间
	LastUpdate time.Time
	// NodeType 节点类型（primary, secondary, etc.）
	NodeType string
	// Region 区域
	Region string
	// Zone 可用区
	Zone string
}

// LoadBalancingAlgorithm 负载均衡算法
type LoadBalancingAlgorithm int

const (
	// AlgorithmRoundRobin 轮询算法
	AlgorithmRoundRobin LoadBalancingAlgorithm = iota
	// AlgorithmLeastConnections 最少连接算法
	AlgorithmLeastConnections
	// AlgorithmLeastLoad 最小负载算法
	AlgorithmLeastLoad
	// AlgorithmWeightedRoundRobin 加权轮询算法
	AlgorithmWeightedRoundRobin
	// AlgorithmWeightedLeastConnections 加权最少连接算法
	AlgorithmWeightedLeastConnections
	// AlgorithmConsistentHash 一致性哈希算法
	AlgorithmConsistentHash
	// AlgorithmAdaptive 自适应算法
	AlgorithmAdaptive
)

// BaseLoadBalancer 基础负载均衡器实现
type BaseLoadBalancer struct {
	metrics         map[string]LoadMetrics
	metricsLock     sync.RWMutex
	algorithm       LoadBalancingAlgorithm
	rrIndex         int
	rrLock          sync.Mutex
	consistentHash  *ConsistentHash
	adaptiveWeights map[string]float64
	adaptiveLock    sync.RWMutex
}

// ConsistentHash 一致性哈希实现
type ConsistentHash struct {
	nodes      map[uint32]message.Address
	sortedKeys []uint32
	replicas   int
	hashLock   sync.RWMutex
}

// NewBaseLoadBalancer 创建新的基础负载均衡器
func NewBaseLoadBalancer(algorithm LoadBalancingAlgorithm) *BaseLoadBalancer {
	lb := &BaseLoadBalancer{
		metrics:   make(map[string]LoadMetrics),
		algorithm: algorithm,
		rrIndex:   0,
	}

	if algorithm == AlgorithmConsistentHash {
		lb.consistentHash = NewConsistentHash(100) // 100个虚拟节点
	}

	if algorithm == AlgorithmAdaptive {
		lb.adaptiveWeights = make(map[string]float64)
	}

	return lb
}

// Select 选择目标地址
func (lb *BaseLoadBalancer) Select(ctx context.Context, msg message.Message, candidates []CandidateStatus) ([]message.Address, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates available")
	}

	// 过滤健康的候选
	healthyCandidates := make([]CandidateStatus, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Healthy {
			healthyCandidates = append(healthyCandidates, candidate)
		}
	}

	if len(healthyCandidates) == 0 {
		return nil, errors.New("no healthy candidates available")
	}

	// 根据算法选择
	switch lb.algorithm {
	case AlgorithmRoundRobin:
		return lb.selectRoundRobin(healthyCandidates), nil
	case AlgorithmLeastConnections:
		return lb.selectLeastConnections(healthyCandidates), nil
	case AlgorithmLeastLoad:
		return lb.selectLeastLoad(healthyCandidates), nil
	case AlgorithmWeightedRoundRobin:
		return lb.selectWeightedRoundRobin(healthyCandidates), nil
	case AlgorithmWeightedLeastConnections:
		return lb.selectWeightedLeastConnections(healthyCandidates), nil
	case AlgorithmConsistentHash:
		return lb.selectConsistentHash(msg, healthyCandidates), nil
	case AlgorithmAdaptive:
		return lb.selectAdaptive(msg, healthyCandidates), nil
	default:
		return lb.selectRoundRobin(healthyCandidates), nil
	}
}

// UpdateMetrics 更新指标
func (lb *BaseLoadBalancer) UpdateMetrics(address message.Address, metrics LoadMetrics) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	addrStr := address.String()
	metrics.Address = address
	metrics.LastUpdate = time.Now()

	lb.metricsLock.Lock()
	lb.metrics[addrStr] = metrics
	lb.metricsLock.Unlock()

	// 更新一致性哈希
	if lb.algorithm == AlgorithmConsistentHash && lb.consistentHash != nil {
		lb.consistentHash.AddNode(address)
	}

	// 更新自适应权重
	if lb.algorithm == AlgorithmAdaptive {
		lb.updateAdaptiveWeight(addrStr, metrics)
	}

	return nil
}

// GetMetrics 获取指标
func (lb *BaseLoadBalancer) GetMetrics(address message.Address) (LoadMetrics, error) {
	if address == nil {
		return LoadMetrics{}, errors.New("address cannot be nil")
	}

	addrStr := address.String()

	lb.metricsLock.RLock()
	metrics, exists := lb.metrics[addrStr]
	lb.metricsLock.RUnlock()

	if !exists {
		return LoadMetrics{}, fmt.Errorf("metrics not found for address: %s", addrStr)
	}

	return metrics, nil
}

// HealthCheck 健康检查
func (lb *BaseLoadBalancer) HealthCheck(ctx context.Context, address message.Address) (bool, error) {
	if address == nil {
		return false, errors.New("address cannot be nil")
	}

	// 简化实现：检查指标是否过期
	lb.metricsLock.RLock()
	metrics, exists := lb.metrics[address.String()]
	lb.metricsLock.RUnlock()

	if !exists {
		return false, nil
	}

	// 如果指标超过30秒未更新，认为不健康
	if time.Since(metrics.LastUpdate) > 30*time.Second {
		return false, nil
	}

	// 检查错误率
	if metrics.ErrorRate > 0.5 { // 错误率超过50%认为不健康
		return false, nil
	}

	return true, nil
}

// selectRoundRobin 轮询选择
func (lb *BaseLoadBalancer) selectRoundRobin(candidates []CandidateStatus) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	lb.rrLock.Lock()
	index := lb.rrIndex % len(candidates)
	target := candidates[index].Address
	lb.rrIndex = (index + 1) % len(candidates)
	lb.rrLock.Unlock()

	return []message.Address{target}
}

// selectLeastConnections 最少连接选择
func (lb *BaseLoadBalancer) selectLeastConnections(candidates []CandidateStatus) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	lb.metricsLock.RLock()
	defer lb.metricsLock.RUnlock()

	var minConnections = int(^uint(0) >> 1) // 最大int值
	var selected message.Address

	for _, candidate := range candidates {
		metrics, exists := lb.metrics[candidate.Address.String()]
		if !exists {
			continue
		}

		if metrics.ActiveConnections < minConnections {
			minConnections = metrics.ActiveConnections
			selected = candidate.Address
		}
	}

	if selected == nil {
		// 如果没有找到指标，返回第一个候选
		return []message.Address{candidates[0].Address}
	}

	return []message.Address{selected}
}

// selectLeastLoad 最小负载选择
func (lb *BaseLoadBalancer) selectLeastLoad(candidates []CandidateStatus) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	lb.metricsLock.RLock()
	defer lb.metricsLock.RUnlock()

	var minLoad = 101.0 // 大于最大负载
	var selected message.Address

	for _, candidate := range candidates {
		metrics, exists := lb.metrics[candidate.Address.String()]
		if !exists {
			continue
		}

		// 计算综合负载（CPU和内存的加权平均）
		compositeLoad := (metrics.CPUUsage + metrics.MemoryUsage) / 2.0
		if compositeLoad < minLoad {
			minLoad = compositeLoad
			selected = candidate.Address
		}
	}

	if selected == nil {
		// 如果没有找到指标，返回第一个候选
		return []message.Address{candidates[0].Address}
	}

	return []message.Address{selected}
}

// selectWeightedRoundRobin 加权轮询选择
func (lb *BaseLoadBalancer) selectWeightedRoundRobin(candidates []CandidateStatus) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	// 计算总权重
	totalWeight := 0.0
	weights := make([]float64, len(candidates))
	addresses := make([]message.Address, len(candidates))

	lb.metricsLock.RLock()
	for i, candidate := range candidates {
		addresses[i] = candidate.Address
		metrics, exists := lb.metrics[candidate.Address.String()]
		if exists {
			// 权重与负载成反比
			weight := 100.0 - (metrics.CPUUsage+metrics.MemoryUsage)/2.0
			if weight < 1.0 {
				weight = 1.0
			}
			weights[i] = weight
		} else {
			weights[i] = 50.0 // 默认权重
		}
		totalWeight += weights[i]
	}
	lb.metricsLock.RUnlock()

	// 选择
	lb.rrLock.Lock()
	defer lb.rrLock.Unlock()

	// 简化实现：使用累积权重
	currentWeight := 0.0
	selection := lb.rrIndex % 100 // 使用0-99的范围
	lb.rrIndex = (lb.rrIndex + 1) % 100

	for i, weight := range weights {
		currentWeight += (weight / totalWeight) * 100.0
		if float64(selection) < currentWeight {
			return []message.Address{addresses[i]}
		}
	}

	// 默认返回第一个
	return []message.Address{addresses[0]}
}

// selectWeightedLeastConnections 加权最少连接选择
func (lb *BaseLoadBalancer) selectWeightedLeastConnections(candidates []CandidateStatus) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	lb.metricsLock.RLock()
	defer lb.metricsLock.RUnlock()

	var bestScore = -1.0
	var selected message.Address

	for _, candidate := range candidates {
		metrics, exists := lb.metrics[candidate.Address.String()]
		if !exists {
			continue
		}

		// 计算分数：权重 / (连接数 + 1)
		weight := 100.0 - (metrics.CPUUsage+metrics.MemoryUsage)/2.0
		if weight < 1.0 {
			weight = 1.0
		}
		score := weight / float64(metrics.ActiveConnections+1)

		if score > bestScore {
			bestScore = score
			selected = candidate.Address
		}
	}

	if selected == nil {
		// 如果没有找到指标，返回第一个候选
		return []message.Address{candidates[0].Address}
	}

	return []message.Address{selected}
}

// selectConsistentHash 一致性哈希选择
func (lb *BaseLoadBalancer) selectConsistentHash(msg message.Message, candidates []CandidateStatus) []message.Address {
	if lb.consistentHash == nil {
		// 回退到轮询
		return lb.selectRoundRobin(candidates)
	}

	// 使用消息ID作为键
	key := msg.ID()
	if key == "" {
		// 如果没有消息ID，使用发送者地址
		sender := msg.Sender()
		if sender != nil {
			key = sender.String()
		} else {
			// 使用时间戳
			key = fmt.Sprintf("%d", time.Now().UnixNano())
		}
	}

	node := lb.consistentHash.GetNode(key)
	if node == nil {
		// 如果没有找到节点，返回第一个候选
		return []message.Address{candidates[0].Address}
	}

	return []message.Address{node}
}

// selectAdaptive 自适应选择
func (lb *BaseLoadBalancer) selectAdaptive(msg message.Message, candidates []CandidateStatus) []message.Address {
	if len(candidates) == 0 {
		return []message.Address{}
	}

	lb.adaptiveLock.RLock()
	defer lb.adaptiveLock.RUnlock()

	// 如果没有自适应权重，使用加权轮询
	if len(lb.adaptiveWeights) == 0 {
		return lb.selectWeightedRoundRobin(candidates)
	}

	// 计算总权重
	totalWeight := 0.0
	weights := make([]float64, len(candidates))
	addresses := make([]message.Address, len(candidates))

	for i, candidate := range candidates {
		addresses[i] = candidate.Address
		addrStr := candidate.Address.String()
		weight, exists := lb.adaptiveWeights[addrStr]
		if exists {
			weights[i] = weight
		} else {
			weights[i] = 1.0 // 默认权重
		}
		totalWeight += weights[i]
	}

	// 使用加权随机选择
	randomValue := rand.Float64() * totalWeight
	cumulativeWeight := 0.0

	for i, weight := range weights {
		cumulativeWeight += weight
		if randomValue <= cumulativeWeight {
			return []message.Address{addresses[i]}
		}
	}

	// 默认返回第一个
	return []message.Address{addresses[0]}
}

// updateAdaptiveWeight 更新自适应权重
func (lb *BaseLoadBalancer) updateAdaptiveWeight(addrStr string, metrics LoadMetrics) {
	lb.adaptiveLock.Lock()
	defer lb.adaptiveLock.Unlock()

	// 基于指标计算权重
	// 1. 负载越低，权重越高
	cpuWeight := 100.0 - metrics.CPUUsage
	memoryWeight := 100.0 - metrics.MemoryUsage

	// 2. 连接数越少，权重越高
	connectionWeight := 100.0 / float64(metrics.ActiveConnections+1)

	// 3. 错误率越低，权重越高
	errorWeight := 100.0 * (1.0 - metrics.ErrorRate)

	// 4. 延迟越低，权重越高
	latencyWeight := 1000.0 / float64(metrics.ProcessingLatency.Milliseconds()+1)

	// 综合权重
	compositeWeight := (cpuWeight + memoryWeight + connectionWeight + errorWeight + latencyWeight) / 5.0

	// 平滑更新（指数移动平均）
	oldWeight, exists := lb.adaptiveWeights[addrStr]
	if exists {
		// EMA: new = alpha * new + (1-alpha) * old
		alpha := 0.3 // 平滑因子
		lb.adaptiveWeights[addrStr] = alpha*compositeWeight + (1-alpha)*oldWeight
	} else {
		lb.adaptiveWeights[addrStr] = compositeWeight
	}
}

// ClusterRouter 集群路由器
type ClusterRouter struct {
	loadBalancer LoadBalancer
	nodeManager  *ClusterNodeManager
	strategy     ClusterRoutingStrategy
}

// ClusterRoutingStrategy 集群路由策略
type ClusterRoutingStrategy int

const (
	// ClusterStrategyLocalFirst 本地优先策略
	ClusterStrategyLocalFirst ClusterRoutingStrategy = iota
	// ClusterStrategyCrossRegion 跨区域策略
	ClusterStrategyCrossRegion
	// ClusterStrategyFailover 故障转移策略
	ClusterStrategyFailover
	// ClusterStrategyReplication 复制策略
	ClusterStrategyReplication
)

// ClusterNodeManager 集群节点管理器
type ClusterNodeManager struct {
	nodes     map[string]ClusterNode
	nodesLock sync.RWMutex
	localNode string
}

// ClusterNode 集群节点
type ClusterNode struct {
	Address      message.Address
	Region       string
	Zone         string
	Healthy      bool
	LastSeen     time.Time
	Capabilities []string
}

// NewClusterRouter 创建新的集群路由器
func NewClusterRouter(strategy ClusterRoutingStrategy, loadBalancer LoadBalancer) *ClusterRouter {
	return &ClusterRouter{
		loadBalancer: loadBalancer,
		nodeManager:  NewClusterNodeManager(),
		strategy:     strategy,
	}
}

// NewClusterNodeManager 创建新的集群节点管理器
func NewClusterNodeManager() *ClusterNodeManager {
	return &ClusterNodeManager{
		nodes: make(map[string]ClusterNode),
	}
}

// Route 集群路由
func (cr *ClusterRouter) Route(ctx context.Context, msg message.Message, candidates []CandidateStatus) ([]message.Address, error) {
	// 根据集群策略过滤和排序候选
	filteredCandidates := cr.filterCandidatesByStrategy(candidates)

	if len(filteredCandidates) == 0 {
		return nil, errors.New("no suitable candidates found for cluster routing")
	}

	// 使用负载均衡器选择
	return cr.loadBalancer.Select(ctx, msg, filteredCandidates)
}

// filterCandidatesByStrategy 根据策略过滤候选
func (cr *ClusterRouter) filterCandidatesByStrategy(candidates []CandidateStatus) []CandidateStatus {
	switch cr.strategy {
	case ClusterStrategyLocalFirst:
		return cr.filterLocalFirst(candidates)
	case ClusterStrategyCrossRegion:
		return cr.filterCrossRegion(candidates)
	case ClusterStrategyFailover:
		return cr.filterFailover(candidates)
	case ClusterStrategyReplication:
		return cr.filterReplication(candidates)
	default:
		return candidates
	}
}

// filterLocalFirst 本地优先过滤
func (cr *ClusterRouter) filterLocalFirst(candidates []CandidateStatus) []CandidateStatus {
	localNode := cr.nodeManager.GetLocalNode()
	if localNode == "" {
		return candidates
	}

	// 优先选择本地节点
	localCandidates := make([]CandidateStatus, 0)
	remoteCandidates := make([]CandidateStatus, 0)

	for _, candidate := range candidates {
		nodeInfo := cr.nodeManager.GetNode(candidate.Address.String())
		if nodeInfo != nil && nodeInfo.Region == localNode {
			localCandidates = append(localCandidates, candidate)
		} else {
			remoteCandidates = append(remoteCandidates, candidate)
		}
	}

	// 如果本地有候选，只返回本地候选
	if len(localCandidates) > 0 {
		return localCandidates
	}

	return remoteCandidates
}

// filterCrossRegion 跨区域过滤
func (cr *ClusterRouter) filterCrossRegion(candidates []CandidateStatus) []CandidateStatus {
	// 按区域分组
	regionGroups := make(map[string][]CandidateStatus)
	for _, candidate := range candidates {
		nodeInfo := cr.nodeManager.GetNode(candidate.Address.String())
		if nodeInfo != nil {
			region := nodeInfo.Region
			regionGroups[region] = append(regionGroups[region], candidate)
		}
	}

	// 选择候选最多的区域
	var maxRegion string
	maxCount := 0
	for region, group := range regionGroups {
		if len(group) > maxCount {
			maxCount = len(group)
			maxRegion = region
		}
	}

	if maxRegion != "" {
		return regionGroups[maxRegion]
	}

	return candidates
}

// filterFailover 故障转移过滤
func (cr *ClusterRouter) filterFailover(candidates []CandidateStatus) []CandidateStatus {
	// 按健康状态排序：健康节点在前
	healthyCandidates := make([]CandidateStatus, 0)
	unhealthyCandidates := make([]CandidateStatus, 0)

	for _, candidate := range candidates {
		if candidate.Healthy {
			healthyCandidates = append(healthyCandidates, candidate)
		} else {
			unhealthyCandidates = append(unhealthyCandidates, candidate)
		}
	}

	// 只返回健康节点，如果没有健康节点才返回不健康节点
	if len(healthyCandidates) > 0 {
		return healthyCandidates
	}

	return unhealthyCandidates
}

// filterReplication 复制过滤
func (cr *ClusterRouter) filterReplication(candidates []CandidateStatus) []CandidateStatus {
	// 复制策略：返回所有候选（用于广播）
	return candidates
}

// AddNode 添加节点
func (cnm *ClusterNodeManager) AddNode(node ClusterNode) error {
	if node.Address == nil {
		return errors.New("node address cannot be nil")
	}

	addrStr := node.Address.String()
	node.LastSeen = time.Now()

	cnm.nodesLock.Lock()
	cnm.nodes[addrStr] = node
	cnm.nodesLock.Unlock()

	return nil
}

// RemoveNode 移除节点
func (cnm *ClusterNodeManager) RemoveNode(address message.Address) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	addrStr := address.String()

	cnm.nodesLock.Lock()
	delete(cnm.nodes, addrStr)
	cnm.nodesLock.Unlock()

	return nil
}

// GetNode 获取节点信息
func (cnm *ClusterNodeManager) GetNode(addrStr string) *ClusterNode {
	cnm.nodesLock.RLock()
	node, exists := cnm.nodes[addrStr]
	cnm.nodesLock.RUnlock()

	if !exists {
		return nil
	}

	return &node
}

// GetLocalNode 获取本地节点区域
func (cnm *ClusterNodeManager) GetLocalNode() string {
	// 简化实现：返回第一个节点的区域
	cnm.nodesLock.RLock()
	defer cnm.nodesLock.RUnlock()

	for _, node := range cnm.nodes {
		return node.Region
	}

	return ""
}

// SetLocalNode 设置本地节点区域
func (cnm *ClusterNodeManager) SetLocalNode(region string) {
	cnm.localNode = region
}

// GetAllNodes 获取所有节点
func (cnm *ClusterNodeManager) GetAllNodes() []ClusterNode {
	cnm.nodesLock.RLock()
	defer cnm.nodesLock.RUnlock()

	nodes := make([]ClusterNode, 0, len(cnm.nodes))
	for _, node := range cnm.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// NewConsistentHash 创建一致性哈希
func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		nodes:    make(map[uint32]message.Address),
		replicas: replicas,
	}
}

// AddNode 添加节点到一致性哈希
func (ch *ConsistentHash) AddNode(node message.Address) {
	if node == nil {
		return
	}

	ch.hashLock.Lock()
	defer ch.hashLock.Unlock()

	addrStr := node.String()
	for i := 0; i < ch.replicas; i++ {
		// 为每个虚拟节点生成哈希
		key := ch.hash(fmt.Sprintf("%s:%d", addrStr, i))
		ch.nodes[key] = node
	}

	// 重新排序键
	ch.updateSortedKeys()
}

// RemoveNode 从一致性哈希移除节点
func (ch *ConsistentHash) RemoveNode(node message.Address) {
	if node == nil {
		return
	}

	ch.hashLock.Lock()
	defer ch.hashLock.Unlock()

	addrStr := node.String()
	for i := 0; i < ch.replicas; i++ {
		key := ch.hash(fmt.Sprintf("%s:%d", addrStr, i))
		delete(ch.nodes, key)
	}

	// 重新排序键
	ch.updateSortedKeys()
}

// GetNode 获取节点
func (ch *ConsistentHash) GetNode(key string) message.Address {
	if len(ch.nodes) == 0 {
		return nil
	}

	ch.hashLock.RLock()
	defer ch.hashLock.RUnlock()

	hash := ch.hash(key)

	// 查找第一个大于等于该哈希值的节点
	idx := sort.Search(len(ch.sortedKeys), func(i int) bool {
		return ch.sortedKeys[i] >= hash
	})

	// 如果没找到，回到第一个节点（环形）
	if idx == len(ch.sortedKeys) {
		idx = 0
	}

	return ch.nodes[ch.sortedKeys[idx]]
}

// hash 计算哈希值
func (ch *ConsistentHash) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// updateSortedKeys 更新排序后的键
func (ch *ConsistentHash) updateSortedKeys() {
	ch.sortedKeys = make([]uint32, 0, len(ch.nodes))
	for key := range ch.nodes {
		ch.sortedKeys = append(ch.sortedKeys, key)
	}
	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

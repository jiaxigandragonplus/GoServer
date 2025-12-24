package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
	"github.com/GooLuck/GoServer/framework/logger"
)

// ClusterStatus 集群状态
type ClusterStatus int

const (
	// ClusterStatusInitializing 初始化中
	ClusterStatusInitializing ClusterStatus = iota
	// ClusterStatusJoining 加入中
	ClusterStatusJoining
	// ClusterStatusRunning 运行中
	ClusterStatusRunning
	// ClusterStatusLeaving 离开中
	ClusterStatusLeaving
	// ClusterStatusStopped 已停止
	ClusterStatusStopped
	// ClusterStatusFailed 失败
	ClusterStatusFailed
)

// NodeRole 节点角色
type NodeRole int

const (
	// NodeRoleFollower 跟随者
	NodeRoleFollower NodeRole = iota
	// NodeRoleCandidate 候选者
	NodeRoleCandidate
	// NodeRoleLeader 领导者
	NodeRoleLeader
	// NodeRoleObserver 观察者
	NodeRoleObserver
)

// NodeInfo 节点信息
type NodeInfo struct {
	// ID 节点ID
	ID string
	// Name 节点名称
	Name string
	// Address 节点地址
	Address string
	// Host 主机名
	Host string
	// Port 端口
	Port int
	// Role 节点角色
	Role NodeRole
	// Status 节点状态
	Status ClusterStatus
	// LastSeen 最后活跃时间
	LastSeen time.Time
	// Metadata 元数据
	Metadata map[string]string
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	// NodeID 节点ID
	NodeID string
	// NodeName 节点名称
	NodeName string
	// Host 主机地址
	Host string
	// Port 端口
	Port int
	// JoinAddresses 加入地址列表
	JoinAddresses []string
	// DiscoveryInterval 发现间隔
	DiscoveryInterval time.Duration
	// HeartbeatInterval 心跳间隔
	HeartbeatInterval time.Duration
	// ElectionTimeout 选举超时时间
	ElectionTimeout time.Duration
	// LeaderTimeout 领导者超时时间
	LeaderTimeout time.Duration
	// MaxRetries 最大重试次数
	MaxRetries int
	// RetryDelay 重试延迟
	RetryDelay time.Duration
	// EnableConsensus 是否启用一致性协议
	EnableConsensus bool
	// EnableGossip 是否启用Gossip传播
	EnableGossip bool
	// Metadata 元数据
	Metadata map[string]string
}

// DefaultConfig 默认配置
func DefaultConfig() *ClusterConfig {
	return &ClusterConfig{
		NodeID:            generateNodeID(),
		NodeName:          "node-" + generateNodeID()[:8],
		Host:              "localhost",
		Port:              8080,
		JoinAddresses:     []string{},
		DiscoveryInterval: 5 * time.Second,
		HeartbeatInterval: 1 * time.Second,
		ElectionTimeout:   3 * time.Second,
		LeaderTimeout:     10 * time.Second,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		EnableConsensus:   true,
		EnableGossip:      true,
		Metadata:          make(map[string]string),
	}
}

// Cluster 集群接口
type Cluster interface {
	// Start 启动集群
	Start(ctx context.Context) error
	// Stop 停止集群
	Stop() error
	// Join 加入集群
	Join(ctx context.Context, addresses []string) error
	// Leave 离开集群
	Leave() error
	// Status 获取集群状态
	Status() ClusterStatus
	// Nodes 获取所有节点
	Nodes() []*NodeInfo
	// GetNode 获取节点信息
	GetNode(nodeID string) (*NodeInfo, error)
	// Leader 获取领导者节点
	Leader() (*NodeInfo, error)
	// Send 发送消息到远程actor
	Send(ctx context.Context, receiver message.Address, msg message.Message) error
	// Broadcast 广播消息到所有节点
	Broadcast(ctx context.Context, msg message.Message) error
	// RegisterActor 注册actor到集群
	RegisterActor(actor actor.Actor) error
	// UnregisterActor 从集群注销actor
	UnregisterActor(address message.Address) error
	// ResolveAddress 解析地址到节点
	ResolveAddress(address message.Address) (*NodeInfo, error)
}

// ClusterManager 集群管理器
type ClusterManager struct {
	config    *ClusterConfig
	status    ClusterStatus
	nodes     map[string]*NodeInfo
	nodesLock sync.RWMutex
	self      *NodeInfo
	leaderID  string
	discovery Discovery
	transport Transport
	consensus Consensus
	actorMgr  *actor.ActorManager
	logger    logger.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// NewClusterManager 创建新的集群管理器
func NewClusterManager(config *ClusterConfig) (*ClusterManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 确保节点ID不为空
	if config.NodeID == "" {
		config.NodeID = generateNodeID()
	}

	// 创建自身节点信息
	self := &NodeInfo{
		ID:       config.NodeID,
		Name:     config.NodeName,
		Address:  fmt.Sprintf("%s:%d", config.Host, config.Port),
		Host:     config.Host,
		Port:     config.Port,
		Role:     NodeRoleFollower,
		Status:   ClusterStatusInitializing,
		LastSeen: time.Now(),
		Metadata: config.Metadata,
	}

	// 创建集群管理器
	cm := &ClusterManager{
		config:   config,
		status:   ClusterStatusInitializing,
		nodes:    make(map[string]*NodeInfo),
		self:     self,
		actorMgr: actor.GetDefaultActorManager(),
		logger:   logger.GetDefaultLogger(),
	}

	// 添加自身到节点列表
	cm.nodes[self.ID] = self

	// 初始化组件
	if err := cm.initComponents(); err != nil {
		return nil, err
	}

	return cm, nil
}

// initComponents 初始化组件
func (cm *ClusterManager) initComponents() error {
	// 初始化发现模块
	discovery, err := NewStaticDiscovery(cm.config.JoinAddresses)
	if err != nil {
		return fmt.Errorf("failed to initialize discovery: %w", err)
	}
	cm.discovery = discovery

	// 初始化传输层
	transport, err := NewTCPTransport(cm.config.Host, cm.config.Port)
	if err != nil {
		return fmt.Errorf("failed to initialize transport: %w", err)
	}
	cm.transport = transport

	// 初始化一致性模块
	if cm.config.EnableConsensus {
		consensus, err := NewRaftConsensus(cm.config, cm.transport)
		if err != nil {
			return fmt.Errorf("failed to initialize consensus: %w", err)
		}
		cm.consensus = consensus
	}

	return nil
}

// Start 启动集群
func (cm *ClusterManager) Start(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.status != ClusterStatusInitializing && cm.status != ClusterStatusStopped {
		return fmt.Errorf("cluster is already running or in transition")
	}

	cm.ctx, cm.cancel = context.WithCancel(ctx)
	cm.status = ClusterStatusJoining

	// 启动传输层
	if err := cm.transport.Start(cm.ctx); err != nil {
		cm.status = ClusterStatusFailed
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// 启动发现模块
	if err := cm.discovery.Start(cm.ctx); err != nil {
		cm.status = ClusterStatusFailed
		return fmt.Errorf("failed to start discovery: %w", err)
	}

	// 启动一致性模块
	if cm.consensus != nil {
		if err := cm.consensus.Start(cm.ctx); err != nil {
			cm.status = ClusterStatusFailed
			return fmt.Errorf("failed to start consensus: %w", err)
		}
	}

	// 启动心跳
	cm.wg.Add(1)
	go cm.heartbeatLoop()

	// 启动节点发现
	cm.wg.Add(1)
	go cm.discoveryLoop()

	// 启动消息处理
	cm.wg.Add(1)
	go cm.messageLoop()

	cm.status = ClusterStatusRunning
	cm.logger.Info("Cluster started", logger.String("node_id", cm.self.ID), logger.String("address", cm.self.Address))

	return nil
}

// Stop 停止集群
func (cm *ClusterManager) Stop() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.status != ClusterStatusRunning {
		return fmt.Errorf("cluster is not running")
	}

	cm.status = ClusterStatusLeaving

	// 取消上下文
	if cm.cancel != nil {
		cm.cancel()
	}

	// 等待所有goroutine结束
	cm.wg.Wait()

	// 停止组件
	if cm.consensus != nil {
		cm.consensus.Stop()
	}

	if cm.discovery != nil {
		cm.discovery.Stop()
	}

	if cm.transport != nil {
		cm.transport.Stop()
	}

	cm.status = ClusterStatusStopped
	cm.logger.Info("Cluster stopped", logger.String("node_id", cm.self.ID))

	return nil
}

// Join 加入集群
func (cm *ClusterManager) Join(ctx context.Context, addresses []string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.status != ClusterStatusRunning {
		return fmt.Errorf("cluster must be running to join")
	}

	// 更新发现模块的地址
	if len(addresses) > 0 {
		if staticDiscovery, ok := cm.discovery.(*StaticDiscovery); ok {
			staticDiscovery.UpdateAddresses(addresses)
		}
	}

	// 尝试加入集群
	cm.logger.Info("Joining cluster", logger.Any("addresses", addresses))
	return nil
}

// Leave 离开集群
func (cm *ClusterManager) Leave() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.status != ClusterStatusRunning {
		return fmt.Errorf("cluster is not running")
	}

	cm.status = ClusterStatusLeaving
	cm.logger.Info("Leaving cluster", logger.String("node_id", cm.self.ID))

	// 通知其他节点
	// 在实际实现中，这里应该发送离开消息

	return nil
}

// Status 获取集群状态
func (cm *ClusterManager) Status() ClusterStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.status
}

// Nodes 获取所有节点
func (cm *ClusterManager) Nodes() []*NodeInfo {
	cm.nodesLock.RLock()
	defer cm.nodesLock.RUnlock()

	nodes := make([]*NodeInfo, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// GetNode 获取节点信息
func (cm *ClusterManager) GetNode(nodeID string) (*NodeInfo, error) {
	cm.nodesLock.RLock()
	defer cm.nodesLock.RUnlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return node, nil
}

// Leader 获取领导者节点
func (cm *ClusterManager) Leader() (*NodeInfo, error) {
	cm.nodesLock.RLock()
	defer cm.nodesLock.RUnlock()

	if cm.leaderID == "" {
		return nil, fmt.Errorf("no leader elected")
	}

	leader, exists := cm.nodes[cm.leaderID]
	if !exists {
		return nil, fmt.Errorf("leader node not found: %s", cm.leaderID)
	}

	return leader, nil
}

// Send 发送消息到远程actor
func (cm *ClusterManager) Send(ctx context.Context, receiver message.Address, msg message.Message) error {
	if receiver == nil {
		return fmt.Errorf("receiver address cannot be nil")
	}

	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// 检查是否是本地地址
	if receiver.IsLocal() {
		// 本地消息，直接发送
		return cm.sendLocal(ctx, receiver, msg)
	}

	// 远程消息，通过传输层发送
	return cm.sendRemote(ctx, receiver, msg)
}

// Broadcast 广播消息到所有节点
func (cm *ClusterManager) Broadcast(ctx context.Context, msg message.Message) error {
	cm.nodesLock.RLock()
	nodes := make([]*NodeInfo, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		if node.ID != cm.self.ID {
			nodes = append(nodes, node)
		}
	}
	cm.nodesLock.RUnlock()

	var firstErr error
	for _, node := range nodes {
		// 创建远程地址
		addr, err := message.ParseAddress(fmt.Sprintf("tcp://%s/", node.Address))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// 发送消息
		if err := cm.Send(ctx, addr, msg); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// RegisterActor 注册actor到集群
func (cm *ClusterManager) RegisterActor(actor actor.Actor) error {
	if actor == nil {
		return fmt.Errorf("actor cannot be nil")
	}

	// 注册到本地actor管理器
	if err := cm.actorMgr.Register(actor); err != nil {
		return fmt.Errorf("failed to register actor: %w", err)
	}

	// 广播actor注册信息
	// 在实际实现中，这里应该通知其他节点

	return nil
}

// UnregisterActor 从集群注销actor
func (cm *ClusterManager) UnregisterActor(address message.Address) error {
	if address == nil {
		return fmt.Errorf("address cannot be nil")
	}

	// 从本地actor管理器移除
	if err := cm.actorMgr.Remove(address); err != nil {
		return fmt.Errorf("failed to unregister actor: %w", err)
	}

	// 广播actor注销信息
	// 在实际实现中，这里应该通知其他节点

	return nil
}

// ResolveAddress 解析地址到节点
func (cm *ClusterManager) ResolveAddress(address message.Address) (*NodeInfo, error) {
	if address == nil {
		return nil, fmt.Errorf("address cannot be nil")
	}

	// 简化实现：根据地址查找节点
	// 在实际实现中，应该有更复杂的地址解析逻辑
	addrStr := address.String()

	cm.nodesLock.RLock()
	defer cm.nodesLock.RUnlock()

	for _, node := range cm.nodes {
		// 检查地址是否匹配节点
		if addrStr == fmt.Sprintf("tcp://%s/", node.Address) {
			return node, nil
		}
	}

	return nil, fmt.Errorf("no node found for address: %s", addrStr)
}

// heartbeatLoop 心跳循环
func (cm *ClusterManager) heartbeatLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.sendHeartbeat()
		}
	}
}

// discoveryLoop 发现循环
func (cm *ClusterManager) discoveryLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.discoverNodes()
		}
	}
}

// messageLoop 消息处理循环
func (cm *ClusterManager) messageLoop() {
	defer cm.wg.Done()

	// 从传输层接收消息
	for {
		select {
		case <-cm.ctx.Done():
			return
		default:
			// 接收消息
			msg, err := cm.transport.Receive(cm.ctx)
			if err != nil {
				if cm.ctx.Err() != nil {
					return
				}
				cm.logger.Error("Failed to receive message", logger.ErrorField(err))
				continue
			}

			// 处理消息
			if err := cm.handleMessage(msg); err != nil {
				cm.logger.Error("Failed to handle message", logger.ErrorField(err))
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (cm *ClusterManager) sendHeartbeat() {
	// 更新自身最后活跃时间
	cm.nodesLock.Lock()
	cm.self.LastSeen = time.Now()
	cm.nodesLock.Unlock()

	// 如果是领导者，发送心跳给所有跟随者
	if cm.self.Role == NodeRoleLeader {
		cm.broadcastHeartbeat()
	}
}

// broadcastHeartbeat 广播心跳
func (cm *ClusterManager) broadcastHeartbeat() {
	// 在实际实现中，这里应该发送心跳消息给所有跟随者
	cm.logger.Debug("Broadcasting heartbeat", logger.String("node_id", cm.self.ID))
}

// discoverNodes 发现节点
func (cm *ClusterManager) discoverNodes() {
	// 从发现模块获取节点
	nodes, err := cm.discovery.Discover(cm.ctx)
	if err != nil {
		cm.logger.Error("Failed to discover nodes", logger.ErrorField(err))
		return
	}

	// 更新节点列表
	cm.updateNodes(nodes)
}

// updateNodes 更新节点列表
func (cm *ClusterManager) updateNodes(nodes []*NodeInfo) {
	cm.nodesLock.Lock()
	defer cm.nodesLock.Unlock()

	for _, node := range nodes {
		// 跳过自身
		if node.ID == cm.self.ID {
			continue
		}

		// 更新或添加节点
		if existing, exists := cm.nodes[node.ID]; exists {
			// 更新现有节点
			existing.LastSeen = time.Now()
			existing.Status = node.Status
			existing.Role = node.Role
			// 合并元数据
			for k, v := range node.Metadata {
				existing.Metadata[k] = v
			}
		} else {
			// 添加新节点
			cm.nodes[node.ID] = node
			cm.logger.Info("Discovered new node", logger.String("node_id", node.ID), logger.String("address", node.Address))
		}
	}

	// 清理超时节点
	now := time.Now()
	for id, node := range cm.nodes {
		if id == cm.self.ID {
			continue
		}

		// 如果节点超过2个心跳间隔未活跃，标记为失败
		if now.Sub(node.LastSeen) > 2*cm.config.HeartbeatInterval {
			node.Status = ClusterStatusFailed
			cm.logger.Warn("Node marked as failed", logger.String("node_id", node.ID), logger.Time("last_seen", node.LastSeen))
		}
	}
}

// sendLocal 发送本地消息
func (cm *ClusterManager) sendLocal(ctx context.Context, receiver message.Address, msg message.Message) error {
	// 获取actor
	actor, err := cm.actorMgr.Get(receiver)
	if err != nil {
		return fmt.Errorf("failed to get actor: %w", err)
	}

	// 创建信封
	envelope := message.NewEnvelope(msg)

	// 处理消息
	return actor.HandleMessage(ctx, envelope)
}

// sendRemote 发送远程消息
func (cm *ClusterManager) sendRemote(ctx context.Context, receiver message.Address, msg message.Message) error {
	// 解析目标节点
	node, err := cm.ResolveAddress(receiver)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	// 通过传输层发送消息
	return cm.transport.Send(ctx, node.Address, msg)
}

// handleMessage 处理接收到的消息
func (cm *ClusterManager) handleMessage(msg interface{}) error {
	// 在实际实现中，这里应该根据消息类型进行处理
	// 例如：心跳消息、选举消息、actor消息等
	cm.logger.Debug("Received message", logger.String("type", fmt.Sprintf("%T", msg)))
	return nil
}

// generateNodeID 生成节点ID
func generateNodeID() string {
	// 使用时间戳和随机数生成节点ID
	// 在实际应用中应该使用更可靠的ID生成器
	return fmt.Sprintf("node-%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		// 简化实现，实际应该使用crypto/rand
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

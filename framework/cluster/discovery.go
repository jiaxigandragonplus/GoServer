package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StaticDiscovery 静态发现实现
type StaticDiscovery struct {
	addresses []string
	nodes     map[string]*NodeInfo
	nodesLock sync.RWMutex
	self      *NodeInfo
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewStaticDiscovery 创建新的静态发现
func NewStaticDiscovery(addresses []string) (*StaticDiscovery, error) {
	ctx, cancel := context.WithCancel(context.Background())

	sd := &StaticDiscovery{
		addresses: addresses,
		nodes:     make(map[string]*NodeInfo),
		ctx:       ctx,
		cancel:    cancel,
	}

	return sd, nil
}

// Start 启动发现模块
func (sd *StaticDiscovery) Start(ctx context.Context) error {
	// 合并上下文
	sd.ctx, sd.cancel = context.WithCancel(ctx)

	// 启动发现循环
	sd.wg.Add(1)
	go sd.discoveryLoop()

	return nil
}

// Stop 停止发现模块
func (sd *StaticDiscovery) Stop() error {
	if sd.cancel != nil {
		sd.cancel()
	}

	sd.wg.Wait()
	return nil
}

// Discover 发现节点
func (sd *StaticDiscovery) Discover(ctx context.Context) ([]*NodeInfo, error) {
	sd.nodesLock.RLock()
	defer sd.nodesLock.RUnlock()

	nodes := make([]*NodeInfo, 0, len(sd.nodes))
	for _, node := range sd.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// Register 注册自身节点
func (sd *StaticDiscovery) Register(node *NodeInfo) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	sd.self = node

	sd.nodesLock.Lock()
	sd.nodes[node.ID] = node
	sd.nodesLock.Unlock()

	return nil
}

// Deregister 注销自身节点
func (sd *StaticDiscovery) Deregister(nodeID string) error {
	sd.nodesLock.Lock()
	delete(sd.nodes, nodeID)
	sd.nodesLock.Unlock()

	return nil
}

// UpdateAddresses 更新地址列表
func (sd *StaticDiscovery) UpdateAddresses(addresses []string) {
	sd.addresses = addresses
}

// discoveryLoop 发现循环
func (sd *StaticDiscovery) discoveryLoop() {
	defer sd.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sd.ctx.Done():
			return
		case <-ticker.C:
			sd.refreshNodes()
		}
	}
}

// refreshNodes 刷新节点
func (sd *StaticDiscovery) refreshNodes() {
	// 从静态地址创建节点
	for _, addr := range sd.addresses {
		// 解析地址
		node, err := sd.parseAddress(addr)
		if err != nil {
			continue
		}

		// 更新节点
		sd.updateNode(node)
	}
}

// parseAddress 解析地址
func (sd *StaticDiscovery) parseAddress(addr string) (*NodeInfo, error) {
	// 简化实现：假设地址格式为 host:port
	// 在实际实现中，应该有更复杂的地址解析逻辑
	return &NodeInfo{
		ID:       addr, // 使用地址作为ID
		Name:     "node-" + addr,
		Address:  addr,
		Host:     "localhost", // 简化
		Port:     8080,        // 简化
		Role:     NodeRoleFollower,
		Status:   ClusterStatusRunning,
		LastSeen: time.Now(),
		Metadata: make(map[string]string),
	}, nil
}

// updateNode 更新节点
func (sd *StaticDiscovery) updateNode(node *NodeInfo) {
	sd.nodesLock.Lock()
	defer sd.nodesLock.Unlock()

	if existing, exists := sd.nodes[node.ID]; exists {
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
		sd.nodes[node.ID] = node
	}
}

// ServiceDiscovery 服务发现实现（基于etcd/consul等）
type ServiceDiscovery struct {
	serviceName string
	endpoints   []string
	nodes       map[string]*NodeInfo
	nodesLock   sync.RWMutex
	self        *NodeInfo
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewServiceDiscovery 创建新的服务发现
func NewServiceDiscovery(serviceName string, endpoints []string) (*ServiceDiscovery, error) {
	ctx, cancel := context.WithCancel(context.Background())

	sd := &ServiceDiscovery{
		serviceName: serviceName,
		endpoints:   endpoints,
		nodes:       make(map[string]*NodeInfo),
		ctx:         ctx,
		cancel:      cancel,
	}

	return sd, nil
}

// Start 启动服务发现
func (sd *ServiceDiscovery) Start(ctx context.Context) error {
	sd.ctx, sd.cancel = context.WithCancel(ctx)

	// 启动发现循环
	sd.wg.Add(1)
	go sd.serviceDiscoveryLoop()

	return nil
}

// Stop 停止服务发现
func (sd *ServiceDiscovery) Stop() error {
	if sd.cancel != nil {
		sd.cancel()
	}

	sd.wg.Wait()
	return nil
}

// Discover 发现节点
func (sd *ServiceDiscovery) Discover(ctx context.Context) ([]*NodeInfo, error) {
	sd.nodesLock.RLock()
	defer sd.nodesLock.RUnlock()

	nodes := make([]*NodeInfo, 0, len(sd.nodes))
	for _, node := range sd.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// Register 注册自身节点
func (sd *ServiceDiscovery) Register(node *NodeInfo) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	sd.self = node

	// 在实际实现中，这里应该注册到服务发现系统（如etcd）
	sd.nodesLock.Lock()
	sd.nodes[node.ID] = node
	sd.nodesLock.Unlock()

	return nil
}

// Deregister 注销自身节点
func (sd *ServiceDiscovery) Deregister(nodeID string) error {
	// 在实际实现中，这里应该从服务发现系统注销
	sd.nodesLock.Lock()
	delete(sd.nodes, nodeID)
	sd.nodesLock.Unlock()

	return nil
}

// serviceDiscoveryLoop 服务发现循环
func (sd *ServiceDiscovery) serviceDiscoveryLoop() {
	defer sd.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sd.ctx.Done():
			return
		case <-ticker.C:
			sd.discoverServices()
		}
	}
}

// discoverServices 发现服务
func (sd *ServiceDiscovery) discoverServices() {
	// 在实际实现中，这里应该从服务发现系统获取服务实例
	// 这里使用简化实现
	sd.nodesLock.Lock()
	defer sd.nodesLock.Unlock()

	// 模拟发现新节点
	for i, endpoint := range sd.endpoints {
		nodeID := fmt.Sprintf("node-%s-%d", sd.serviceName, i)
		if _, exists := sd.nodes[nodeID]; !exists {
			sd.nodes[nodeID] = &NodeInfo{
				ID:       nodeID,
				Name:     fmt.Sprintf("%s-node-%d", sd.serviceName, i),
				Address:  endpoint,
				Host:     "localhost",
				Port:     8080 + i,
				Role:     NodeRoleFollower,
				Status:   ClusterStatusRunning,
				LastSeen: time.Now(),
				Metadata: make(map[string]string),
			}
		}
	}
}

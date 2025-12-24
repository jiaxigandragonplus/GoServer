package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
	"github.com/GooLuck/GoServer/framework/logger"
)

// TestClusterCreation 测试集群创建
func TestClusterCreation(t *testing.T) {
	config := DefaultConfig()
	config.NodeName = "test-node"
	config.Host = "localhost"
	config.Port = 9090

	clusterMgr, err := NewClusterManager(config)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	if clusterMgr == nil {
		t.Fatal("Cluster manager is nil")
	}

	if clusterMgr.Status() != ClusterStatusInitializing {
		t.Errorf("Expected status %v, got %v", ClusterStatusInitializing, clusterMgr.Status())
	}
}

// TestClusterStartStop 测试集群启动和停止
func TestClusterStartStop(t *testing.T) {
	config := DefaultConfig()
	config.NodeName = "test-start-stop"
	config.Host = "localhost"
	config.Port = 9091

	clusterMgr, err := NewClusterManager(config)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	ctx := context.Background()

	// 启动集群
	if err := clusterMgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// 检查状态
	if clusterMgr.Status() != ClusterStatusRunning {
		t.Errorf("Expected status %v after start, got %v", ClusterStatusRunning, clusterMgr.Status())
	}

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 停止集群
	if err := clusterMgr.Stop(); err != nil {
		t.Fatalf("Failed to stop cluster: %v", err)
	}

	// 检查状态
	if clusterMgr.Status() != ClusterStatusStopped {
		t.Errorf("Expected status %v after stop, got %v", ClusterStatusStopped, clusterMgr.Status())
	}
}

// TestClusterNodes 测试集群节点管理
func TestClusterNodes(t *testing.T) {
	config := DefaultConfig()
	config.NodeName = "test-nodes"
	config.Host = "localhost"
	config.Port = 9092

	clusterMgr, err := NewClusterManager(config)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	ctx := context.Background()

	// 启动集群
	if err := clusterMgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	defer clusterMgr.Stop()

	// 获取节点列表
	nodes := clusterMgr.Nodes()
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}

	// 检查自身节点
	selfNode := nodes[0]
	if selfNode.Name != config.NodeName {
		t.Errorf("Expected node name %s, got %s", config.NodeName, selfNode.Name)
	}

	if selfNode.Status != ClusterStatusRunning {
		t.Errorf("Expected node status %v, got %v", ClusterStatusRunning, selfNode.Status)
	}
}

// TestClusterActorRegistration 测试actor注册
func TestClusterActorRegistration(t *testing.T) {
	config := DefaultConfig()
	config.NodeName = "test-actor-reg"
	config.Host = "localhost"
	config.Port = 9093

	clusterMgr, err := NewClusterManager(config)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	ctx := context.Background()

	// 启动集群
	if err := clusterMgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	defer clusterMgr.Stop()

	// 创建actor
	addr, err := message.NewLocalActorAddress("/test/actor1")
	if err != nil {
		t.Fatalf("Failed to create address: %v", err)
	}

	actor, err := actor.NewBaseActor(addr, nil)
	if err != nil {
		t.Fatalf("Failed to create actor: %v", err)
	}

	// 注册actor
	if err := clusterMgr.RegisterActor(actor); err != nil {
		t.Fatalf("Failed to register actor: %v", err)
	}

	// 注销actor
	if err := clusterMgr.UnregisterActor(addr); err != nil {
		t.Fatalf("Failed to unregister actor: %v", err)
	}
}

// TestClusterConfig 测试集群配置
func TestClusterConfig(t *testing.T) {
	config := DefaultConfig()

	// 检查默认配置
	if config.NodeID == "" {
		t.Error("NodeID should not be empty")
	}

	if config.NodeName == "" {
		t.Error("NodeName should not be empty")
	}

	if config.Host == "" {
		t.Error("Host should not be empty")
	}

	if config.Port == 0 {
		t.Error("Port should not be 0")
	}

	if config.DiscoveryInterval == 0 {
		t.Error("DiscoveryInterval should not be 0")
	}

	if config.HeartbeatInterval == 0 {
		t.Error("HeartbeatInterval should not be 0")
	}
}

// TestDiscovery 测试发现模块
func TestDiscovery(t *testing.T) {
	addresses := []string{"localhost:8081", "localhost:8082"}
	discovery, err := NewStaticDiscovery(addresses)
	if err != nil {
		t.Fatalf("Failed to create discovery: %v", err)
	}

	ctx := context.Background()

	// 启动发现模块
	if err := discovery.Start(ctx); err != nil {
		t.Fatalf("Failed to start discovery: %v", err)
	}
	defer discovery.Stop()

	// 发现节点
	nodes, err := discovery.Discover(ctx)
	if err != nil {
		t.Fatalf("Failed to discover nodes: %v", err)
	}

	// 至少应该有2个节点
	if len(nodes) < 2 {
		t.Errorf("Expected at least 2 nodes, got %d", len(nodes))
	}
}

// TestTransport 测试传输层
func TestTransport(t *testing.T) {
	transport, err := NewSimpleTransport("localhost:9094")
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	ctx := context.Background()

	// 启动传输层
	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	defer transport.Stop()

	// 检查地址
	addr := transport.Address()
	if addr != "localhost:9094" {
		t.Errorf("Expected address localhost:9094, got %s", addr)
	}
}

// TestLoggerIntegration 测试日志集成
func TestLoggerIntegration(t *testing.T) {
	// 创建测试logger
	loggerConfig := &logger.Config{
		Level:  logger.InfoLevel,
		Format: "console",
	}

	loggerInstance, err := logger.NewLogger(loggerConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// 设置默认logger
	logger.SetDefaultLogger(loggerInstance)

	// 创建集群（会使用默认logger）
	config := DefaultConfig()
	config.NodeName = "test-logger"
	config.Host = "localhost"
	config.Port = 9095

	clusterMgr, err := NewClusterManager(config)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	if clusterMgr == nil {
		t.Fatal("Cluster manager is nil")
	}
}

// TestNodeInfo 测试节点信息
func TestNodeInfo(t *testing.T) {
	node := &NodeInfo{
		ID:       "test-id",
		Name:     "test-node",
		Address:  "localhost:8080",
		Host:     "localhost",
		Port:     8080,
		Role:     NodeRoleFollower,
		Status:   ClusterStatusRunning,
		LastSeen: time.Now(),
		Metadata: map[string]string{"key": "value"},
	}

	if node.ID != "test-id" {
		t.Errorf("Expected ID test-id, got %s", node.ID)
	}

	if node.Name != "test-node" {
		t.Errorf("Expected Name test-node, got %s", node.Name)
	}

	if node.Role != NodeRoleFollower {
		t.Errorf("Expected Role %v, got %v", NodeRoleFollower, node.Role)
	}

	if node.Status != ClusterStatusRunning {
		t.Errorf("Expected Status %v, got %v", ClusterStatusRunning, node.Status)
	}
}

// TestClusterStatus 测试集群状态转换
func TestClusterStatus(t *testing.T) {
	// 测试状态常量
	statuses := []ClusterStatus{
		ClusterStatusInitializing,
		ClusterStatusJoining,
		ClusterStatusRunning,
		ClusterStatusLeaving,
		ClusterStatusStopped,
		ClusterStatusFailed,
	}

	if len(statuses) != 6 {
		t.Errorf("Expected 6 status constants, got %d", len(statuses))
	}
}

// TestNodeRole 测试节点角色
func TestNodeRole(t *testing.T) {
	// 测试角色常量
	roles := []NodeRole{
		NodeRoleFollower,
		NodeRoleCandidate,
		NodeRoleLeader,
		NodeRoleObserver,
	}

	if len(roles) != 4 {
		t.Errorf("Expected 4 role constants, got %d", len(roles))
	}
}

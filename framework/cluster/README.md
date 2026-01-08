# Actor 集群支持

为actor框架提供分布式集群支持，包括节点发现、远程消息传递和一致性保证。

## 架构设计

### 核心组件

1. **集群管理器 (ClusterManager)**
   - 管理集群生命周期（启动、停止、加入、离开）
   - 维护节点成员信息
   - 协调集群操作

2. **节点发现 (Discovery)**
   - 静态发现 (StaticDiscovery)：基于配置的地址列表
   - 服务发现 (ServiceDiscovery)：基于etcd/consul等（预留接口）

3. **传输层 (Transport)**
   - TCP传输 (TCPTransport)：基于TCP的可靠消息传递
   - 简单传输 (SimpleTransport)：用于测试的模拟传输

4. **一致性协议 (Consensus)**
   - Raft共识 (RaftConsensus)：基于Raft算法的一致性保证
   - Gossip共识 (GossipConsensus)：基于Gossip协议的最终一致性

### 消息类型

- **心跳消息 (HeartbeatMessage)**：节点健康检查
- **加入消息 (JoinMessage)**：节点加入集群
- **离开消息 (LeaveMessage)**：节点离开集群
- **Actor消息 (ActorMessage)**：远程actor通信

## 使用示例

### 基本用法

```go
// 创建集群配置
config := cluster.DefaultConfig()
config.NodeName = "node-1"
config.Host = "localhost"
config.Port = 8080

// 创建集群管理器
clusterMgr, err := cluster.NewClusterManager(config)
if err != nil {
    log.Fatal(err)
}

// 启动集群
ctx := context.Background()
if err := clusterMgr.Start(ctx); err != nil {
    log.Fatal(err)
}

// 注册actor
actor, err := NewClusterActor("my-actor")
if err != nil {
    log.Fatal(err)
}
clusterMgr.RegisterActor(actor)

// 发送远程消息
remoteAddr, _ := message.ParseAddress("tcp://other-node:8080/actor/path")
msg := message.NewBaseMessage("greeting", "Hello")
clusterMgr.Send(ctx, remoteAddr, msg)

// 停止集群
clusterMgr.Stop()
```

### 多节点集群

```go
// 节点1
config1 := cluster.DefaultConfig()
config1.NodeName = "node-1"
config1.Port = 8081
cluster1, _ := cluster.NewClusterManager(config1)

// 节点2
config2 := cluster.DefaultConfig()
config2.NodeName = "node-2"
config2.Port = 8082
config2.JoinAddresses = []string{"localhost:8081"}
cluster2, _ := cluster.NewClusterManager(config2)

// 启动并加入
cluster1.Start(ctx)
cluster2.Start(ctx)
cluster2.Join(ctx, []string{"localhost:8081"})
```

## 配置选项

```go
type ClusterConfig struct {
    NodeID            string            // 节点ID
    NodeName          string            // 节点名称
    Host              string            // 主机地址
    Port              int               // 端口
    JoinAddresses     []string          // 加入地址列表
    DiscoveryInterval time.Duration     // 发现间隔
    HeartbeatInterval time.Duration     // 心跳间隔
    ElectionTimeout   time.Duration     // 选举超时
    LeaderTimeout     time.Duration     // 领导者超时
    MaxRetries        int               // 最大重试次数
    RetryDelay        time.Duration     // 重试延迟
    EnableConsensus   bool              // 是否启用一致性协议
    EnableGossip      bool              // 是否启用Gossip传播
    Metadata          map[string]string // 元数据
}
```

## 节点状态

```go
type ClusterStatus int

const (
    ClusterStatusInitializing // 初始化中
    ClusterStatusJoining      // 加入中
    ClusterStatusRunning      // 运行中
    ClusterStatusLeaving      // 离开中
    ClusterStatusStopped      // 已停止
    ClusterStatusFailed       // 失败
)
```

## 节点角色

```go
type NodeRole int

const (
    NodeRoleFollower  // 跟随者
    NodeRoleCandidate // 候选者
    NodeRoleLeader    // 领导者
    NodeRoleObserver  // 观察者
)
```

## 一致性状态

```go
type ConsensusStatus int

const (
    ConsensusStatusFollower  // 跟随者状态
    ConsensusStatusCandidate // 候选者状态
    ConsensusStatusLeader    // 领导者状态
    ConsensusStatusStopped   // 已停止
)
```

## 扩展接口

### 自定义发现模块

```go
type Discovery interface {
    Start(ctx context.Context) error
    Stop() error
    Discover(ctx context.Context) ([]*NodeInfo, error)
    Register(node *NodeInfo) error
    Deregister(nodeID string) error
}
```

### 自定义传输层

```go
type Transport interface {
    Start(ctx context.Context) error
    Stop() error
    Send(ctx context.Context, address string, msg message.Message) error
    Receive(ctx context.Context) (interface{}, error)
    Broadcast(ctx context.Context, msg message.Message) error
    Address() string
}
```

### 自定义一致性协议

```go
type Consensus interface {
    Start(ctx context.Context) error
    Stop() error
    Propose(ctx context.Context, value []byte) error
    Read(ctx context.Context) ([]byte, error)
    Status() ConsensusStatus
    Leader() string
    AddNode(nodeID string, address string) error
    RemoveNode(nodeID string) error
}
```

## 测试

运行测试：
```bash
go test ./framework/actor/cluster/... -v
```

运行示例：
```bash
go run ./framework/actor/cluster/example/example.go
```

## 实现说明

1. **简化实现**：当前实现为简化版本，适合学习和原型开发
2. **可扩展性**：所有核心组件都通过接口定义，支持自定义实现
3. **生产就绪**：需要添加持久化、安全、监控等特性才能用于生产环境

## 未来改进

1. **持久化存储**：节点状态和日志的持久化
2. **安全通信**：TLS加密和认证
3. **监控指标**：集群健康状态和性能指标
4. **动态配置**：运行时配置更新
5. **故障恢复**：自动故障检测和恢复
6. **负载均衡**：智能消息路由和负载均衡

## 依赖

- `framework/actor`：actor框架基础
- `framework/logger`：日志系统
- `framework/network`：网络通信（可选）

## 许可证

MIT
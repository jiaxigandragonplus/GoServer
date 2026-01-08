# Actor Router 系统

## 概述

Actor Router 系统是一个专门为 Actor 模型设计的路由和负载均衡框架。它提供了多种路由策略、负载均衡算法、集群路由和消息分组功能，与现有的 `message/router.go` 系统互补而非重复。

## 与现有系统的关系

### 现有 `message/router.go` 系统
- **定位**：消息级别的路由，基于消息类型和规则进行路由
- **功能**：静态路由规则、消息类型匹配、基本的路由策略
- **使用场景**：简单的消息路由，基于预定义规则的消息分发

### 新的 `actor/router` 系统
- **定位**：Actor 级别的路由，基于 Actor 状态、负载、位置等进行路由
- **功能**：动态路由、负载均衡、集群感知、消息分组、健康检查
- **使用场景**：复杂的分布式 Actor 系统，需要负载均衡、故障转移、会话粘性等高级功能

### 关键区别
1. **路由粒度**：message/router 关注消息，actor/router 关注 Actor
2. **动态性**：actor/router 支持基于实时状态的路由决策
3. **集群感知**：actor/router 支持跨节点、跨区域的路由
4. **负载均衡**：actor/router 内置多种负载均衡算法
5. **消息分组**：actor/router 支持将相关消息路由到同一 Actor

## 核心组件

### 1. ActorRouter 接口
```go
type ActorRouter interface {
    Route(ctx context.Context, msg message.Message, candidates []message.Address) ([]message.Address, error)
    AddCandidate(address message.Address) error
    RemoveCandidate(address message.Address) error
    GetCandidates() []message.Address
    UpdateCandidateStatus(address message.Address, status CandidateStatus) error
    GetCandidateStatus(address message.Address) (CandidateStatus, error)
}
```

### 2. 路由策略 (RoutingStrategy)
- `StrategyRoundRobin` - 轮询策略
- `StrategyRandom` - 随机策略
- `StrategyHash` - 哈希策略（基于消息内容）
- `StrategyBroadcast` - 广播策略
- `StrategyLeastLoaded` - 最小负载策略
- `StrategySticky` - 粘性会话策略
- `StrategyLocationAware` - 位置感知策略
- `StrategyCustom` - 自定义策略

### 3. 分组策略 (GroupingStrategy)
- `GroupByMessageType` - 按消息类型分组
- `GroupBySender` - 按发送者分组
- `GroupByKey` - 按自定义键分组
- `GroupByHash` - 按哈希值分组
- `GroupBySession` - 按会话分组
- `GroupByTenant` - 按租户分组
- `GroupByPriority` - 按优先级分组

### 4. 负载均衡器 (LoadBalancer)
```go
type LoadBalancer interface {
    Select(ctx context.Context, msg message.Message, candidates []CandidateStatus) ([]message.Address, error)
    UpdateMetrics(address message.Address, metrics LoadMetrics) error
    GetMetrics(address message.Address) (LoadMetrics, error)
    HealthCheck(ctx context.Context, address message.Address) (bool, error)
}
```

### 5. 负载均衡算法 (LoadBalancingAlgorithm)
- `AlgorithmRoundRobin` - 轮询算法
- `AlgorithmLeastConnections` - 最少连接算法
- `AlgorithmLeastLoad` - 最小负载算法
- `AlgorithmWeightedRoundRobin` - 加权轮询算法
- `AlgorithmWeightedLeastConnections` - 加权最少连接算法
- `AlgorithmConsistentHash` - 一致性哈希算法
- `AlgorithmAdaptive` - 自适应算法

### 6. 集群路由 (ClusterRouter)
- `ClusterStrategyLocalFirst` - 本地优先策略
- `ClusterStrategyCrossRegion` - 跨区域策略
- `ClusterStrategyFailover` - 故障转移策略
- `ClusterStrategyReplication` - 复制策略

### 7. 消息分组器 (MessageGrouper)
```go
type MessageGrouper interface {
    Group(ctx context.Context, msg message.Message) (string, error)
    GetGroupActor(ctx context.Context, groupKey string) (message.Address, error)
    UpdateGroupMapping(groupKey string, address message.Address) error
    RemoveGroupMapping(groupKey string) error
    GetGroupStats() map[string]GroupStats
}
```

## 使用示例

### 基础路由
```go
// 创建路由器
router := router.NewBaseActorRouter(router.StrategyRoundRobin, router.GroupByMessageType)

// 添加候选 Actor
router.AddCandidate(addr1)
router.AddCandidate(addr2)
router.AddCandidate(addr3)

// 路由消息
targets, err := router.Route(ctx, msg, candidates)
```

### 负载均衡
```go
// 创建负载均衡器
lb := router.NewBaseLoadBalancer(router.AlgorithmLeastLoad)

// 更新指标
metrics := router.LoadMetrics{
    Address:    addr,
    CPUUsage:   30.5,
    MemoryUsage: 45.2,
    LastUpdate: time.Now(),
}
lb.UpdateMetrics(addr, metrics)

// 选择目标
targets, err := lb.Select(ctx, msg, candidates)
```

### 消息分组
```go
// 创建分组器
grouper := router.NewBaseMessageGrouper(router.GroupBySender)

// 分组消息
groupKey, err := grouper.Group(ctx, msg)

// 获取分组对应的 Actor
actor, err := grouper.GetGroupActor(ctx, groupKey)
```

### 分组感知路由
```go
// 创建分组感知路由器
baseRouter := router.NewBaseActorRouter(router.StrategyRoundRobin, router.GroupByMessageType)
grouper := router.NewBaseMessageGrouper(router.GroupBySender)
groupAwareRouter := router.NewGroupAwareRouter(baseRouter, grouper)

// 路由消息（自动分组）
targets, err := groupAwareRouter.Route(ctx, msg, candidates)
```

## 与现有系统的集成

### 1. 独立使用
新的 actor/router 系统可以独立使用，不依赖现有的 message/router 系统。

### 2. 组合使用
两个系统可以组合使用，实现更复杂的路由逻辑：
- 使用 message/router 进行初步的消息分类和过滤
- 使用 actor/router 进行细粒度的 Actor 选择和负载均衡

### 3. 迁移路径
现有系统可以逐步迁移到新的 actor/router 系统：
1. 保持现有的 message/router 规则不变
2. 在新的服务中使用 actor/router 进行路由
3. 逐步将复杂的路由逻辑迁移到 actor/router
4. 最终可以完全替换或保留两个系统并行运行

## 设计优势

### 1. 解耦设计
- 路由逻辑与业务逻辑分离
- 支持多种路由策略的插件式替换
- 易于扩展新的路由算法

### 2. 性能优化
- 支持基于实时指标的路由决策
- 减少网络延迟（位置感知路由）
- 提高系统吞吐量（负载均衡）

### 3. 可靠性
- 支持故障转移和健康检查
- 会话粘性保证相关消息处理的一致性
- 集群路由提高系统可用性

### 4. 可观测性
- 丰富的统计信息和指标
- 分组统计和性能监控
- 易于集成到现有的监控系统

## 总结

新的 actor/router 系统是对现有 message/router 系统的补充和增强，专门为复杂的分布式 Actor 系统设计。它提供了更高级的路由功能，包括负载均衡、集群感知、消息分组等，能够更好地满足现代分布式系统的需求。

两个系统可以共存，根据具体需求选择使用或组合使用，为开发者提供了灵活的路由解决方案。
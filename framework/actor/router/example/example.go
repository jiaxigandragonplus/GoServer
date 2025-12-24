package example

import (
	"context"
	"fmt"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
	"github.com/GooLuck/GoServer/framework/actor/router"
)

// SimpleAddress 简单地址实现
type SimpleAddress struct {
	id string
}

func (a *SimpleAddress) String() string {
	return a.id
}

func (a *SimpleAddress) Protocol() string {
	return "simple"
}

func (a *SimpleAddress) Host() string {
	return "localhost"
}

func (a *SimpleAddress) Port() string {
	return "8080"
}

func (a *SimpleAddress) Path() string {
	return "/actors/" + a.id
}

func (a *SimpleAddress) Equal(other message.Address) bool {
	if other == nil {
		return false
	}
	return a.String() == other.String()
}

func (a *SimpleAddress) IsLocal() bool {
	return true
}

func (a *SimpleAddress) IsRemote() bool {
	return false
}

// RunSimpleExamples 运行简单示例
func RunSimpleExamples() {
	fmt.Println("=== Actor Router 简单示例 ===")

	// 示例1：基础路由策略
	exampleBasicRouting()

	// 示例2：负载均衡
	exampleLoadBalancing()

	// 示例3：消息分组
	exampleMessageGrouping()
}

// exampleBasicRouting 基础路由策略示例
func exampleBasicRouting() {
	fmt.Println("\n--- 示例1：基础路由策略 ---")

	ctx := context.Background()

	// 创建候选地址
	candidates := []message.Address{
		&SimpleAddress{id: "actor1"},
		&SimpleAddress{id: "actor2"},
		&SimpleAddress{id: "actor3"},
	}

	// 创建消息
	msg := message.NewBaseMessage("example.type", "test data")
	msg.SetSender(&SimpleAddress{id: "sender1"})

	// 1. 轮询策略
	roundRobinRouter := router.NewBaseActorRouter(router.StrategyRoundRobin, router.GroupByMessageType)
	for _, addr := range candidates {
		roundRobinRouter.AddCandidate(addr)
	}

	targets, err := roundRobinRouter.Route(ctx, msg, candidates)
	if err != nil {
		fmt.Printf("轮询路由错误: %v\n", err)
	} else {
		fmt.Printf("轮询策略选择: %v\n", targets[0].String())
	}

	// 2. 随机策略
	randomRouter := router.NewBaseActorRouter(router.StrategyRandom, router.GroupByMessageType)
	for _, addr := range candidates {
		randomRouter.AddCandidate(addr)
	}

	targets, err = randomRouter.Route(ctx, msg, candidates)
	if err != nil {
		fmt.Printf("随机路由错误: %v\n", err)
	} else {
		fmt.Printf("随机策略选择: %v\n", targets[0].String())
	}

	// 3. 广播策略
	broadcastRouter := router.NewBaseActorRouter(router.StrategyBroadcast, router.GroupByMessageType)
	for _, addr := range candidates {
		broadcastRouter.AddCandidate(addr)
	}

	targets, err = broadcastRouter.Route(ctx, msg, candidates)
	if err != nil {
		fmt.Printf("广播路由错误: %v\n", err)
	} else {
		fmt.Printf("广播策略选择 %d 个目标\n", len(targets))
	}
}

// exampleLoadBalancing 负载均衡示例
func exampleLoadBalancing() {
	fmt.Println("\n--- 示例2：负载均衡 ---")

	ctx := context.Background()

	// 创建负载均衡器
	loadBalancer := router.NewBaseLoadBalancer(router.AlgorithmLeastLoad)

	// 创建候选状态
	candidates := []router.CandidateStatus{
		{
			Address: &SimpleAddress{id: "node1"},
			Healthy: true,
			Load:    30,
		},
		{
			Address: &SimpleAddress{id: "node2"},
			Healthy: true,
			Load:    60,
		},
		{
			Address: &SimpleAddress{id: "node3"},
			Healthy: true,
			Load:    10,
		},
	}

	// 更新指标
	for _, candidate := range candidates {
		metrics := router.LoadMetrics{
			Address:           candidate.Address,
			CPUUsage:          float64(candidate.Load),
			MemoryUsage:       float64(candidate.Load),
			ActiveConnections: candidate.Load / 10,
			LastUpdate:        time.Now(),
		}
		loadBalancer.UpdateMetrics(candidate.Address, metrics)
	}

	// 创建消息
	msg := message.NewBaseMessage("load.balanced", "load balancing test")

	// 选择目标
	targets, err := loadBalancer.Select(ctx, msg, candidates)
	if err != nil {
		fmt.Printf("负载均衡选择错误: %v\n", err)
	} else {
		fmt.Printf("最小负载策略选择: %v\n", targets[0].String())
	}
}

// exampleMessageGrouping 消息分组示例
func exampleMessageGrouping() {
	fmt.Println("\n--- 示例3：消息分组 ---")

	ctx := context.Background()

	// 创建消息分组器
	grouper := router.NewBaseMessageGrouper(router.GroupBySender)

	// 创建消息
	msg1 := message.NewBaseMessage("user.action", "action1")
	msg1.SetSender(&SimpleAddress{id: "user123"})

	msg2 := message.NewBaseMessage("user.action", "action2")
	msg2.SetSender(&SimpleAddress{id: "user123"}) // 同一用户

	msg3 := message.NewBaseMessage("user.action", "action3")
	msg3.SetSender(&SimpleAddress{id: "user456"}) // 不同用户

	// 分组消息
	group1, err := grouper.Group(ctx, msg1)
	if err != nil {
		fmt.Printf("消息1分组错误: %v\n", err)
	} else {
		fmt.Printf("消息1分组键: %s\n", group1)
	}

	group2, err := grouper.Group(ctx, msg2)
	if err != nil {
		fmt.Printf("消息2分组错误: %v\n", err)
	} else {
		fmt.Printf("消息2分组键: %s (应与消息1相同: %v)\n", group2, group1 == group2)
	}

	group3, err := grouper.Group(ctx, msg3)
	if err != nil {
		fmt.Printf("消息3分组错误: %v\n", err)
	} else {
		fmt.Printf("消息3分组键: %s (应与消息1不同: %v)\n", group3, group1 != group3)
	}

	// 更新分组映射
	actor1 := &SimpleAddress{id: "actor_for_user123"}
	actor2 := &SimpleAddress{id: "actor_for_user456"}

	grouper.UpdateGroupMapping(group1, actor1)
	grouper.UpdateGroupMapping(group3, actor2)

	// 获取分组actor
	addr1, err := grouper.GetGroupActor(ctx, group1)
	if err != nil {
		fmt.Printf("获取分组1actor错误: %v\n", err)
	} else {
		fmt.Printf("分组 %s 的actor: %v\n", group1, addr1.String())
	}

	addr2, err := grouper.GetGroupActor(ctx, group3)
	if err != nil {
		fmt.Printf("获取分组3actor错误: %v\n", err)
	} else {
		fmt.Printf("分组 %s 的actor: %v\n", group3, addr2.String())
	}
}

// 主函数
func main() {
	RunSimpleExamples()
}

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/cluster"
	"github.com/GooLuck/GoServer/framework/actor/message"
	"github.com/GooLuck/GoServer/framework/logger"
)

// ClusterActor 集群actor示例
type ClusterActor struct {
	*actor.BaseActor
	name string
}

// NewClusterActor 创建新的集群actor
func NewClusterActor(name string) (*ClusterActor, error) {
	addr, err := message.NewLocalActorAddress(fmt.Sprintf("/cluster/%s", name))
	if err != nil {
		return nil, err
	}

	baseActor, err := actor.NewBaseActor(addr, nil)
	if err != nil {
		return nil, err
	}

	return &ClusterActor{
		BaseActor: baseActor,
		name:      name,
	}, nil
}

// HandleMessage 处理消息
func (ca *ClusterActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()
	log.Printf("Actor %s received message: type=%s, sender=%v",
		ca.name, msg.Type(), msg.Sender())

	// 回复消息
	if msg.Sender() != nil {
		replyMsg := message.NewBaseMessage("reply", fmt.Sprintf("Hello from %s", ca.name))
		return ca.Reply(ctx, msg, replyMsg)
	}

	return nil
}

// 示例1: 单节点集群
func exampleSingleNode() {
	fmt.Println("=== 示例1: 单节点集群 ===")

	// 创建集群配置
	config := cluster.DefaultConfig()
	config.NodeName = "node-1"
	config.Host = "localhost"
	config.Port = 8081

	// 创建集群管理器
	clusterMgr, err := cluster.NewClusterManager(config)
	if err != nil {
		log.Fatalf("Failed to create cluster manager: %v", err)
	}

	// 启动集群
	ctx := context.Background()
	if err := clusterMgr.Start(ctx); err != nil {
		log.Fatalf("Failed to start cluster: %v", err)
	}

	// 创建actor
	actor1, err := NewClusterActor("actor1")
	if err != nil {
		log.Fatalf("Failed to create actor: %v", err)
	}

	// 注册actor到集群
	if err := clusterMgr.RegisterActor(actor1); err != nil {
		log.Fatalf("Failed to register actor: %v", err)
	}

	// 启动actor
	if err := actor1.Start(ctx); err != nil {
		log.Fatalf("Failed to start actor: %v", err)
	}

	// 等待一段时间
	time.Sleep(2 * time.Second)

	// 停止actor
	actor1.Stop()

	// 停止集群
	clusterMgr.Stop()

	fmt.Println("单节点集群示例完成")
}

// 示例2: 多节点集群（模拟）
func exampleMultiNode() {
	fmt.Println("\n=== 示例2: 多节点集群（模拟）===")

	// 创建节点1
	config1 := cluster.DefaultConfig()
	config1.NodeName = "node-1"
	config1.Host = "localhost"
	config1.Port = 8081

	cluster1, err := cluster.NewClusterManager(config1)
	if err != nil {
		log.Fatalf("Failed to create cluster 1: %v", err)
	}

	// 创建节点2
	config2 := cluster.DefaultConfig()
	config2.NodeName = "node-2"
	config2.Host = "localhost"
	config2.Port = 8082
	config2.JoinAddresses = []string{"localhost:8081"}

	cluster2, err := cluster.NewClusterManager(config2)
	if err != nil {
		log.Fatalf("Failed to create cluster 2: %v", err)
	}

	ctx := context.Background()

	// 启动节点1
	if err := cluster1.Start(ctx); err != nil {
		log.Fatalf("Failed to start cluster 1: %v", err)
	}

	// 启动节点2
	if err := cluster2.Start(ctx); err != nil {
		log.Fatalf("Failed to start cluster 2: %v", err)
	}

	// 节点2加入集群
	if err := cluster2.Join(ctx, []string{"localhost:8081"}); err != nil {
		log.Printf("Failed to join cluster: %v", err)
	}

	// 创建actor
	actor1, err := NewClusterActor("actor-on-node1")
	if err != nil {
		log.Fatalf("Failed to create actor 1: %v", err)
	}

	actor2, err := NewClusterActor("actor-on-node2")
	if err != nil {
		log.Fatalf("Failed to create actor 2: %v", err)
	}

	// 注册actor
	cluster1.RegisterActor(actor1)
	cluster2.RegisterActor(actor2)

	// 启动actor
	actor1.Start(ctx)
	actor2.Start(ctx)

	// 显示集群状态
	fmt.Println("集群节点:")
	nodes1 := cluster1.Nodes()
	for _, node := range nodes1 {
		fmt.Printf("  - %s (%s) status=%v\n", node.Name, node.Address, node.Status)
	}

	// 等待一段时间
	time.Sleep(3 * time.Second)

	// 清理
	actor1.Stop()
	actor2.Stop()
	cluster1.Stop()
	cluster2.Stop()

	fmt.Println("多节点集群示例完成")
}

// 示例3: 远程消息传递
func exampleRemoteMessage() {
	fmt.Println("\n=== 示例3: 远程消息传递 ===")

	// 创建发送者actor
	senderActor, err := NewClusterActor("sender")
	if err != nil {
		log.Fatalf("Failed to create sender actor: %v", err)
	}

	// 创建接收者actor地址（模拟远程地址）
	receiverAddr, err := message.ParseAddress("tcp://localhost:8082/cluster/receiver")
	if err != nil {
		log.Fatalf("Failed to parse remote address: %v", err)
	}

	ctx := context.Background()

	// 启动发送者actor
	if err := senderActor.Start(ctx); err != nil {
		log.Fatalf("Failed to start sender actor: %v", err)
	}

	// 发送消息到远程actor
	msg := message.NewBaseMessage("greeting", "Hello from remote!")
	msg.SetReceiver(receiverAddr)

	if err := senderActor.Send(ctx, receiverAddr, msg); err != nil {
		log.Printf("Failed to send remote message (expected): %v", err)
	} else {
		fmt.Println("远程消息发送成功")
	}

	// 清理
	senderActor.Stop()

	fmt.Println("远程消息传递示例完成")
}

// 示例4: 集群广播
func exampleBroadcast() {
	fmt.Println("\n=== 示例4: 集群广播 ===")

	// 创建集群配置
	config := cluster.DefaultConfig()
	config.NodeName = "broadcast-node"
	config.Host = "localhost"
	config.Port = 8083

	// 创建集群管理器
	clusterMgr, err := cluster.NewClusterManager(config)
	if err != nil {
		log.Fatalf("Failed to create cluster manager: %v", err)
	}

	ctx := context.Background()

	// 启动集群
	if err := clusterMgr.Start(ctx); err != nil {
		log.Fatalf("Failed to start cluster: %v", err)
	}

	// 创建广播消息
	broadcastMsg := message.NewBaseMessage("broadcast", "This is a broadcast message")

	// 广播消息
	if err := clusterMgr.Broadcast(ctx, broadcastMsg); err != nil {
		log.Printf("Failed to broadcast message: %v", err)
	} else {
		fmt.Println("广播消息已发送")
	}

	// 清理
	clusterMgr.Stop()

	fmt.Println("集群广播示例完成")
}

// 示例5: 集群一致性
func exampleConsensus() {
	fmt.Println("\n=== 示例5: 集群一致性 ===")

	// 创建集群配置（启用一致性）
	config := cluster.DefaultConfig()
	config.NodeName = "consensus-node"
	config.Host = "localhost"
	config.Port = 8084
	config.EnableConsensus = true

	// 创建集群管理器
	clusterMgr, err := cluster.NewClusterManager(config)
	if err != nil {
		log.Fatalf("Failed to create cluster manager: %v", err)
	}

	ctx := context.Background()

	// 启动集群
	if err := clusterMgr.Start(ctx); err != nil {
		log.Fatalf("Failed to start cluster: %v", err)
	}

	// 获取集群状态
	status := clusterMgr.Status()
	fmt.Printf("集群状态: %v\n", status)

	// 获取节点列表
	nodes := clusterMgr.Nodes()
	fmt.Printf("集群节点数: %d\n", len(nodes))

	// 尝试获取领导者
	leader, err := clusterMgr.Leader()
	if err != nil {
		fmt.Printf("当前没有领导者: %v\n", err)
	} else {
		fmt.Printf("当前领导者: %s\n", leader.Name)
	}

	// 等待一段时间让选举完成
	time.Sleep(2 * time.Second)

	// 再次检查领导者
	leader, err = clusterMgr.Leader()
	if err != nil {
		fmt.Printf("仍然没有领导者: %v\n", err)
	} else {
		fmt.Printf("选举后的领导者: %s\n", leader.Name)
	}

	// 清理
	clusterMgr.Stop()

	fmt.Println("集群一致性示例完成")
}

func main() {
	// 设置日志级别
	loggerConfig := &logger.Config{
		Level:       logger.DebugLevel,
		Format:      "console",
		Development: true,
		Caller:      true,
	}

	loggerInstance, err := logger.NewLogger(loggerConfig)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	logger.SetDefaultLogger(loggerInstance)

	fmt.Println("开始运行集群示例...")

	// 运行示例
	exampleSingleNode()
	exampleMultiNode()
	exampleRemoteMessage()
	exampleBroadcast()
	exampleConsensus()

	fmt.Println("\n所有示例完成!")
}

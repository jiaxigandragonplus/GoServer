package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// CustomMessage 自定义消息类型
type CustomMessage struct {
	*message.BaseMessage
	CustomField string
}

// NewCustomMessage 创建自定义消息
func NewCustomMessage(data interface{}, customField string) *CustomMessage {
	baseMsg := message.NewBaseMessage("custom", data)
	return &CustomMessage{
		BaseMessage: baseMsg,
		CustomField: customField,
	}
}

func main() {
	fmt.Println("=== Actor消息系统示例 ===")

	// 示例1: 创建消息
	exampleCreateMessage()

	// 示例2: 地址管理
	exampleAddressManagement()

	// 示例3: 消息路由
	exampleMessageRouting()

	// 示例4: 消息序列化
	exampleMessageSerialization()

	// 示例5: 消息信封
	exampleMessageEnvelope()

	fmt.Println("\n=== 所有示例执行完成 ===")
}

// exampleCreateMessage 创建消息示例
func exampleCreateMessage() {
	fmt.Println("\n--- 示例1: 创建消息 ---")

	// 创建基础消息
	msg := message.NewBaseMessage("user.login", map[string]interface{}{
		"username": "john_doe",
		"password": "secret123",
	})

	// 设置消息属性
	msg.SetPriority(message.PriorityHigh)
	msg.SetTTL(30 * time.Second)
	msg.SetHeader("source", "web")
	msg.SetHeader("version", "1.0")

	fmt.Printf("消息类型: %s\n", msg.Type())
	fmt.Printf("消息ID: %s\n", msg.ID())
	fmt.Printf("消息优先级: %v\n", msg.Priority())
	fmt.Printf("消息TTL: %v\n", msg.TTL())
	fmt.Printf("消息数据: %v\n", msg.Data())
	fmt.Printf("消息头部: %v\n", msg.Headers())

	// 克隆消息
	clonedMsg := msg.Clone()
	fmt.Printf("克隆消息ID: %s\n", clonedMsg.ID())
}

// exampleAddressManagement 地址管理示例
func exampleAddressManagement() {
	fmt.Println("\n--- 示例2: 地址管理 ---")

	// 创建本地地址
	localAddr, err := message.NewLocalAddress("actor", "/user/john")
	if err != nil {
		log.Printf("创建本地地址失败: %v", err)
	} else {
		fmt.Printf("本地地址: %s\n", localAddr.String())
		fmt.Printf("是否是本地地址: %v\n", localAddr.IsLocal())
		fmt.Printf("是否是远程地址: %v\n", localAddr.IsRemote())
	}

	// 使用Must函数创建本地地址（如果失败会panic）
	localAddr2 := message.MustNewLocalAddress("actor", "/user/john2")
	fmt.Printf("Must本地地址: %s\n", localAddr2.String())

	// 创建远程地址
	remoteAddr, err := message.NewRemoteAddress("actor", "192.168.1.100", "8080", "/user/jane")
	if err != nil {
		log.Printf("创建远程地址失败: %v", err)
	} else {
		fmt.Printf("远程地址: %s\n", remoteAddr.String())
		fmt.Printf("是否是本地地址: %v\n", remoteAddr.IsLocal())
		fmt.Printf("是否是远程地址: %v\n", remoteAddr.IsRemote())
	}

	// 测试不支持的协议
	_, err = message.NewLocalAddress("unsupported", "/user/test")
	if err != nil {
		fmt.Printf("不支持的协议错误（预期）: %v\n", err)
	}

	// 解析地址
	parsedAddr, err := message.ParseAddress("actor://192.168.1.100:8080/user/jane")
	if err != nil {
		log.Printf("解析地址失败: %v", err)
	} else {
		fmt.Printf("解析的地址: %s\n", parsedAddr.String())
		fmt.Printf("协议: %s\n", parsedAddr.Protocol())
		fmt.Printf("主机: %s\n", parsedAddr.Host())
		fmt.Printf("端口: %s\n", parsedAddr.Port())
		fmt.Printf("路径: %s\n", parsedAddr.Path())
	}
}

// exampleMessageRouting 消息路由示例
func exampleMessageRouting() {
	fmt.Println("\n--- 示例3: 消息路由 ---")

	// 获取默认路由器
	router := message.GetDefaultRouter()

	// 创建目标地址
	target1 := message.MustNewLocalAddress("actor", "/user/alice")
	target2 := message.MustNewLocalAddress("actor", "/user/bob")
	target3 := message.MustNewLocalAddress("actor", "/user/charlie")

	// 添加路由规则
	rule := message.RouteRule{
		ID:       "user-messages",
		Pattern:  "user.*",
		Targets:  []message.Address{target1, target2, target3},
		Strategy: message.StrategyRoundRobin,
		Priority: 1,
	}

	if err := router.AddRoute(rule); err != nil {
		log.Printf("添加路由规则失败: %v", err)
		return
	}

	// 创建测试消息
	msg := message.NewBaseMessage("user.login", "test data")

	// 路由消息
	ctx := context.Background()
	targets, err := router.Route(ctx, msg)
	if err != nil {
		log.Printf("路由消息失败: %v", err)
		return
	}

	fmt.Printf("消息类型: %s\n", msg.Type())
	fmt.Printf("路由到的目标数量: %d\n", len(targets))
	for i, target := range targets {
		fmt.Printf("  目标%d: %s\n", i+1, target.String())
	}

	// 测试多次路由（轮询）
	fmt.Println("\n测试轮询路由:")
	for i := 0; i < 5; i++ {
		testMsg := message.NewBaseMessage("user.message", fmt.Sprintf("消息%d", i+1))
		targets, _ := router.Route(ctx, testMsg)
		if len(targets) > 0 {
			fmt.Printf("  消息%d -> %s\n", i+1, targets[0].String())
		}
	}

	// 获取所有路由规则
	rules := router.GetRoutes()
	fmt.Printf("\n当前路由规则数量: %d\n", len(rules))
}

// exampleMessageSerialization 消息序列化示例
func exampleMessageSerialization() {
	fmt.Println("\n--- 示例4: 消息序列化 ---")

	// 创建消息
	msg := message.NewBaseMessage("test.serialization", map[string]interface{}{
		"field1": "value1",
		"field2": 123,
		"field3": true,
	})

	// 设置地址
	sender := message.MustNewLocalAddress("actor", "/sender")
	receiver := message.MustNewLocalAddress("actor", "/receiver")
	msg.SetSender(sender)
	msg.SetReceiver(receiver)

	// 获取序列化注册表
	registry := message.GetDefaultSerializerRegistry()

	// JSON序列化
	jsonSerializer, err := registry.Get(message.FormatJSON)
	if err != nil {
		log.Printf("获取JSON序列化器失败: %v", err)
		return
	}

	jsonData, err := jsonSerializer.Serialize(msg)
	if err != nil {
		log.Printf("JSON序列化失败: %v", err)
		return
	}

	fmt.Printf("JSON序列化结果 (%d 字节):\n%s\n", len(jsonData), string(jsonData))

	// JSON反序列化
	deserializedMsg, err := jsonSerializer.Deserialize(jsonData)
	if err != nil {
		log.Printf("JSON反序列化失败: %v", err)
		return
	}

	fmt.Printf("反序列化消息类型: %s\n", deserializedMsg.Type())
	fmt.Printf("反序列化消息ID: %s\n", deserializedMsg.ID())
	fmt.Printf("反序列化消息数据: %v\n", deserializedMsg.Data())

	// 使用编解码器
	codec := message.NewMessageCodec(nil)
	encoded, err := codec.Encode(msg, message.FormatJSON)
	if err != nil {
		log.Printf("编码失败: %v", err)
		return
	}

	decoded, err := codec.Decode(encoded, message.FormatJSON)
	if err != nil {
		log.Printf("解码失败: %v", err)
		return
	}

	fmt.Printf("编解码器测试 - 原始消息ID: %s, 解码消息ID: %s\n", msg.ID(), decoded.ID())
}

// exampleMessageEnvelope 消息信封示例
func exampleMessageEnvelope() {
	fmt.Println("\n--- 示例5: 消息信封 ---")

	// 创建消息
	msg := message.NewBaseMessage("envelope.test", "信封测试数据")

	// 创建信封
	envelope := message.NewEnvelope(msg)

	// 设置投递信息
	delivery := envelope.Delivery()
	delivery.MaxAttempts = 5
	delivery.RetryDelay = 2 * time.Second
	envelope.SetDelivery(delivery)

	fmt.Printf("信封消息类型: %s\n", envelope.Message().Type())
	fmt.Printf("投递尝试次数: %d\n", envelope.Delivery().Attempt)
	fmt.Printf("最大尝试次数: %d\n", envelope.Delivery().MaxAttempts)
	fmt.Printf("是否应该重试: %v\n", envelope.ShouldRetry())

	// 模拟投递失败和重试
	fmt.Println("\n模拟投递过程:")
	for i := 0; i < 3; i++ {
		envelope.IncrementAttempt()
		fmt.Printf("  尝试 %d/%d - 是否应该重试: %v\n",
			envelope.Delivery().Attempt,
			envelope.Delivery().MaxAttempts,
			envelope.ShouldRetry())
	}

	// 标记为死信
	envelope.MarkAsDeadLetter(fmt.Errorf("投递失败"))
	fmt.Printf("是否标记为死信: %v\n", envelope.Delivery().DeadLetter)
	fmt.Printf("错误信息: %v\n", envelope.Delivery().Error)
}

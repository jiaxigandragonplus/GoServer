package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// EchoActor 回显actor，将收到的消息回显给发送者
type EchoActor struct {
	*actor.BaseActor
}

// NewEchoActor 创建新的回显actor
func NewEchoActor(address message.Address) (*EchoActor, error) {
	baseActor, err := actor.NewBaseActor(address, nil)
	if err != nil {
		return nil, err
	}

	return &EchoActor{
		BaseActor: baseActor,
	}, nil
}

// HandleMessage 处理消息 - 回显给发送者
func (a *EchoActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()

	fmt.Printf("EchoActor %s received message: type=%s, data=%v\n",
		a.Address().String(), msg.Type(), msg.Data())

	// 创建回显消息
	echoMsg := message.NewBaseMessage("echo.response", map[string]interface{}{
		"original":     msg.Data(),
		"timestamp":    time.Now().Unix(),
		"processed_by": a.Address().String(),
	})

	// 回复给发送者
	return a.Reply(ctx, msg, echoMsg)
}

// CounterActor 计数器actor，维护一个计数器
type CounterActor struct {
	*actor.BaseActor
	counter int
	mu      sync.RWMutex
}

// NewCounterActor 创建新的计数器actor
func NewCounterActor(address message.Address) (*CounterActor, error) {
	baseActor, err := actor.NewBaseActor(address, nil)
	if err != nil {
		return nil, err
	}

	return &CounterActor{
		BaseActor: baseActor,
		counter:   0,
	}, nil
}

// HandleMessage 处理消息 - 处理计数器操作
func (a *CounterActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()

	switch msg.Type() {
	case "counter.increment":
		a.mu.Lock()
		a.counter++
		current := a.counter
		a.mu.Unlock()

		// 发送回复
		replyMsg := message.NewBaseMessage("counter.response", map[string]interface{}{
			"action":  "increment",
			"counter": current,
			"success": true,
		})
		return a.Reply(ctx, msg, replyMsg)

	case "counter.decrement":
		a.mu.Lock()
		a.counter--
		current := a.counter
		a.mu.Unlock()

		replyMsg := message.NewBaseMessage("counter.response", map[string]interface{}{
			"action":  "decrement",
			"counter": current,
			"success": true,
		})
		return a.Reply(ctx, msg, replyMsg)

	case "counter.get":
		a.mu.RLock()
		current := a.counter
		a.mu.RUnlock()

		replyMsg := message.NewBaseMessage("counter.response", map[string]interface{}{
			"action":  "get",
			"counter": current,
			"success": true,
		})
		return a.Reply(ctx, msg, replyMsg)

	default:
		// 未知消息类型
		replyMsg := message.NewBaseMessage("error.response", map[string]interface{}{
			"error":   "unknown message type",
			"type":    msg.Type(),
			"success": false,
		})
		return a.Reply(ctx, msg, replyMsg)
	}
}

// GetCounter 获取当前计数器值
func (a *CounterActor) GetCounter() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.counter
}

func main() {
	fmt.Println("=== Actor系统完整示例 ===")

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 示例1: 基础actor使用
	exampleBasicActor(ctx)

	// 示例2: actor管理器使用
	exampleActorManager(ctx)

	// 示例3: actor间通信
	exampleActorCommunication(ctx)

	fmt.Println("\n=== 所有示例执行完成 ===")
}

func exampleBasicActor(ctx context.Context) {
	fmt.Println("\n--- 示例1: 基础actor使用 ---")

	// 创建回显actor地址
	echoAddr, err := message.NewLocalAddress("local", "/echo/service")
	if err != nil {
		fmt.Printf("创建地址失败: %v\n", err)
		return
	}

	// 创建回显actor
	echoActor, err := NewEchoActor(echoAddr)
	if err != nil {
		fmt.Printf("创建actor失败: %v\n", err)
		return
	}

	// 启动actor
	err = echoActor.Start(ctx)
	if err != nil {
		fmt.Printf("启动actor失败: %v\n", err)
		return
	}

	fmt.Printf("回显actor已启动: %s\n", echoActor.Address().String())

	// 创建测试消息
	testMsg := message.NewBaseMessage("test.echo", map[string]interface{}{
		"message": "Hello, Actor System!",
		"number":  42,
		"active":  true,
	})

	// 设置发送者
	senderAddr, _ := message.NewLocalAddress("local", "/test/client")
	testMsg.SetSender(senderAddr)
	testMsg.SetReceiver(echoAddr)

	// 发送消息到actor
	err = echoActor.Mailbox().Push(ctx, message.NewEnvelope(testMsg))
	if err != nil {
		fmt.Printf("发送消息失败: %v\n", err)
	} else {
		fmt.Println("消息已发送到回显actor")
	}

	// 等待一段时间让actor处理消息
	time.Sleep(100 * time.Millisecond)

	// 停止actor
	err = echoActor.Stop()
	if err != nil {
		fmt.Printf("停止actor失败: %v\n", err)
	} else {
		fmt.Println("回显actor已停止")
	}
}

func exampleActorManager(ctx context.Context) {
	fmt.Println("\n--- 示例2: actor管理器使用 ---")

	// 获取默认actor管理器
	manager := actor.GetDefaultActorManager()

	// 创建多个actor
	actorNames := []string{"alice", "bob", "charlie"}
	actors := make([]*CounterActor, len(actorNames))

	for i, name := range actorNames {
		addr, err := message.NewLocalAddress("local", "/counter/"+name)
		if err != nil {
			fmt.Printf("创建地址失败 %s: %v\n", name, err)
			continue
		}

		counterActor, err := NewCounterActor(addr)
		if err != nil {
			fmt.Printf("创建actor失败 %s: %v\n", name, err)
			continue
		}

		// 注册到管理器
		err = manager.Register(counterActor)
		if err != nil {
			fmt.Printf("注册actor失败 %s: %v\n", name, err)
			continue
		}

		actors[i] = counterActor
		fmt.Printf("创建并注册actor: %s\n", name)
	}

	// 启动所有actor
	err := manager.StartAll(ctx)
	if err != nil {
		fmt.Printf("启动actor失败: %v\n", err)
	} else {
		fmt.Println("所有actor已启动")
	}

	// 发送消息到bob的actor
	bobAddr, _ := message.NewLocalAddress("local", "/counter/bob")
	bobActor, err := manager.Get(bobAddr)
	if err != nil {
		fmt.Printf("获取bob actor失败: %v\n", err)
	} else {
		// 创建增量消息
		incMsg := message.NewBaseMessage("counter.increment", nil)
		senderAddr, _ := message.NewLocalAddress("actor", "/test/client")
		incMsg.SetSender(senderAddr)
		incMsg.SetReceiver(bobAddr)

		// 发送消息
		err = bobActor.Mailbox().Push(ctx, message.NewEnvelope(incMsg))
		if err != nil {
			fmt.Printf("发送消息失败: %v\n", err)
		} else {
			fmt.Println("已发送增量消息到bob actor")
		}
	}

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 停止所有actor
	err = manager.StopAll()
	if err != nil {
		fmt.Printf("停止actor失败: %v\n", err)
	} else {
		fmt.Println("所有actor已停止")
	}
}

func exampleActorCommunication(ctx context.Context) {
	fmt.Println("\n--- 示例3: actor间通信 ---")

	// 创建两个actor
	echoAddr, _ := message.NewLocalAddress("local", "/service/echo")
	echoActor, err := NewEchoActor(echoAddr)
	if err != nil {
		fmt.Printf("创建回显actor失败: %v\n", err)
		return
	}

	counterAddr, _ := message.NewLocalAddress("local", "/service/counter")
	counterActor, err := NewCounterActor(counterAddr)
	if err != nil {
		fmt.Printf("创建计数器actor失败: %v\n", err)
		return
	}

	// 启动actor
	echoActor.Start(ctx)
	counterActor.Start(ctx)

	fmt.Println("两个actor已启动，准备进行通信测试")

	// 测试1: 客户端发送消息到回显actor
	clientAddr, _ := message.NewLocalAddress("local", "/client/main")

	// 创建消息
	msgToEcho := message.NewBaseMessage("test.communication", map[string]interface{}{
		"from":    "client",
		"to":      "echo",
		"content": "测试actor间通信",
	})
	msgToEcho.SetSender(clientAddr)
	msgToEcho.SetReceiver(echoAddr)

	// 使用actor的Send方法发送消息
	err = echoActor.Send(ctx, echoAddr, msgToEcho)
	if err != nil {
		fmt.Printf("发送消息到回显actor失败: %v\n", err)
	} else {
		fmt.Println("已发送消息到回显actor")
	}

	// 测试2: 客户端发送消息到计数器actor
	msgToCounter := message.NewBaseMessage("counter.increment", map[string]interface{}{
		"amount": 5,
	})
	msgToCounter.SetSender(clientAddr)
	msgToCounter.SetReceiver(counterAddr)

	err = counterActor.Send(ctx, counterAddr, msgToCounter)
	if err != nil {
		fmt.Printf("发送消息到计数器actor失败: %v\n", err)
	} else {
		fmt.Println("已发送增量消息到计数器actor")
	}

	// 测试3: actor间直接通信（回显actor发送消息到计数器actor）
	echoToCounterMsg := message.NewBaseMessage("counter.get", nil)
	echoToCounterMsg.SetSender(echoAddr)
	echoToCounterMsg.SetReceiver(counterAddr)

	err = echoActor.Send(ctx, counterAddr, echoToCounterMsg)
	if err != nil {
		fmt.Printf("回显actor发送消息到计数器actor失败: %v\n", err)
	} else {
		fmt.Println("回显actor已发送查询消息到计数器actor")
	}

	// 等待处理
	time.Sleep(300 * time.Millisecond)

	// 停止actor
	echoActor.Stop()
	counterActor.Stop()

	fmt.Println("actor间通信测试完成")
}

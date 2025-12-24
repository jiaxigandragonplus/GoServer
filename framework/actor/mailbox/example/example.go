package main

import (
	"context"
	"fmt"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

func main() {
	fmt.Println("=== Actor邮箱系统示例 ===")

	// 示例1: 基础邮箱使用
	exampleBasicMailbox()

	// 示例2: 邮箱管理器使用
	exampleMailboxManager()

	// 示例3: 消息投递服务使用
	exampleDeliveryService()

	// 示例4: 死信队列使用
	exampleDeadLetterQueue()

	fmt.Println("\n=== 所有示例执行完成 ===")
}

func exampleBasicMailbox() {
	fmt.Println("\n--- 示例1: 基础邮箱使用 ---")

	// 创建有界邮箱配置
	config := mailbox.Config{
		Capacity:        5,
		DropPolicy:      mailbox.DropPolicyReject,
		PriorityEnabled: true,
		TTLEnabled:      true,
		DeadLetterQueue: &mailbox.DeadLetterQueueConfig{
			Enabled:       true,
			Capacity:      10,
			RetentionTime: 1 * time.Hour,
		},
	}

	// 创建邮箱
	mailbox := mailbox.NewBoundedMailbox(config)
	fmt.Printf("创建邮箱: 容量=%d, 当前大小=%d, 是否为空=%v\n",
		mailbox.Capacity(), mailbox.Size(), mailbox.IsEmpty())

	// 创建测试消息
	ctx := context.Background()
	msg := message.NewBaseMessage("user.login", map[string]interface{}{
		"username": "john_doe",
		"ip":       "192.168.1.100",
	})

	addr, _ := message.NewLocalAddress("local", "/user/service")
	msg.SetSender(addr)
	msg.SetReceiver(addr)

	envelope := message.NewEnvelope(msg)

	// 推送消息
	err := mailbox.Push(ctx, envelope)
	if err != nil {
		fmt.Printf("推送消息失败: %v\n", err)
		return
	}

	fmt.Printf("推送消息后: 大小=%d, 是否为空=%v\n", mailbox.Size(), mailbox.IsEmpty())

	// 弹出消息
	popped, err := mailbox.Pop(ctx)
	if err != nil {
		fmt.Printf("弹出消息失败: %v\n", err)
		return
	}

	fmt.Printf("弹出消息: 类型=%s, 数据=%v\n",
		popped.Message().Type(), popped.Message().Data())

	// 测试邮箱满的情况
	fmt.Println("\n测试邮箱满的情况:")
	for i := 0; i < 10; i++ {
		msg := message.NewBaseMessage(fmt.Sprintf("message.%d", i), i)
		msg.SetSender(addr)
		msg.SetReceiver(addr)
		envelope := message.NewEnvelope(msg)

		err := mailbox.Push(ctx, envelope)
		if err != nil {
			fmt.Printf("消息 %d: %v\n", i, err)
			break
		}
	}

	fmt.Printf("最终邮箱大小: %d\n", mailbox.Size())

	// 清空邮箱
	mailbox.Clear()
	fmt.Printf("清空后邮箱大小: %d\n", mailbox.Size())
}

func exampleMailboxManager() {
	fmt.Println("\n--- 示例2: 邮箱管理器使用 ---")

	// 创建邮箱管理器
	manager := mailbox.NewDefaultMailboxManager()

	// 创建多个actor地址
	addresses := []string{"/user/alice", "/user/bob", "/user/charlie"}

	for _, path := range addresses {
		addr, err := message.NewLocalAddress("local", path)
		if err != nil {
			fmt.Printf("创建地址失败 %s: %v\n", path, err)
			continue
		}

		// 为每个actor创建邮箱
		config := mailbox.Config{
			Capacity: 100,
		}

		_, err = manager.CreateMailbox(addr, config)
		if err != nil {
			fmt.Printf("创建邮箱失败 %s: %v\n", path, err)
			continue
		}

		fmt.Printf("为actor %s 创建邮箱\n", path)
	}

	// 获取统计信息
	stats := manager.Stats()
	fmt.Printf("邮箱管理器统计: 总邮箱数=%d, 活跃邮箱数=%d, 总消息数=%d\n",
		stats.TotalMailboxes, stats.ActiveMailboxes, stats.TotalMessages)

	// 列出所有邮箱
	mailboxes := manager.ListMailboxes()
	fmt.Printf("共有 %d 个邮箱\n", len(mailboxes))

	// 测试获取邮箱
	aliceAddr, _ := message.NewLocalAddress("local", "/user/alice")
	mailbox, err := manager.GetMailbox(aliceAddr)
	if err != nil {
		fmt.Printf("获取邮箱失败: %v\n", err)
	} else {
		fmt.Printf("成功获取alice的邮箱: 容量=%d\n", mailbox.Capacity())
	}

	// 移除邮箱
	err = manager.RemoveMailbox(aliceAddr)
	if err != nil {
		fmt.Printf("移除邮箱失败: %v\n", err)
	} else {
		fmt.Println("成功移除alice的邮箱")
	}

	// 关闭管理器
	err = manager.Close()
	if err != nil {
		fmt.Printf("关闭管理器失败: %v\n", err)
	} else {
		fmt.Println("成功关闭邮箱管理器")
	}
}

func exampleDeliveryService() {
	fmt.Println("\n--- 示例3: 消息投递服务使用 ---")

	// 创建邮箱管理器和路由器
	manager := mailbox.NewDefaultMailboxManager()
	router := message.GetDefaultRouter()

	// 创建投递服务
	service := mailbox.NewDeliveryService(manager, router)

	// 创建actor地址
	receiverAddr, _ := message.NewLocalAddress("actor", "/email/service")

	// 创建消息
	ctx := context.Background()
	msg := message.NewBaseMessage("email.send", map[string]interface{}{
		"to":      "user@example.com",
		"subject": "欢迎邮件",
		"body":    "欢迎使用我们的服务！",
	})

	senderAddr, _ := message.NewLocalAddress("local", "/api/gateway")
	msg.SetSender(senderAddr)
	msg.SetReceiver(receiverAddr)

	// 投递消息
	err := service.Deliver(ctx, msg)
	if err != nil {
		fmt.Printf("投递消息失败: %v\n", err)
		return
	}

	fmt.Println("消息投递成功")

	// 从邮箱获取消息
	mailbox, err := manager.GetMailbox(receiverAddr)
	if err != nil {
		fmt.Printf("获取邮箱失败: %v\n", err)
		return
	}

	fmt.Printf("接收者邮箱大小: %d\n", mailbox.Size())

	// 弹出消息
	envelope, err := mailbox.Pop(ctx)
	if err != nil {
		fmt.Printf("弹出消息失败: %v\n", err)
		return
	}

	receivedMsg := envelope.Message()
	fmt.Printf("收到消息: 类型=%s, 发送者=%s, 数据=%v\n",
		receivedMsg.Type(), receivedMsg.Sender().String(), receivedMsg.Data())
}

func exampleDeadLetterQueue() {
	fmt.Println("\n--- 示例4: 死信队列使用 ---")

	// 创建死信队列配置
	config := mailbox.DeadLetterQueueConfig{
		Enabled:       true,
		Capacity:      5,
		RetentionTime: 30 * time.Minute,
	}

	// 创建死信管理器
	dlm := mailbox.NewDeadLetterManager(config)

	// 注册日志处理器
	dlm.RegisterHandler(&mailbox.LoggingDeadLetterHandler{})

	// 创建存储处理器
	storageHandler := mailbox.NewStorageDeadLetterHandler()
	dlm.RegisterHandler(storageHandler)

	// 创建过期消息
	ctx := context.Background()
	msg := message.NewBaseMessage("expired.message", "此消息已过期")
	msg.SetTTL(1 * time.Millisecond) // 设置很短的TTL

	addr, _ := message.NewLocalAddress("local", "/test")
	msg.SetSender(addr)
	msg.SetReceiver(addr)

	envelope := message.NewEnvelope(msg)

	// 等待消息过期
	time.Sleep(2 * time.Millisecond)

	// 提交死信
	err := dlm.Submit(ctx, envelope, "消息过期")
	if err != nil {
		fmt.Printf("提交死信失败: %v\n", err)
		return
	}

	fmt.Println("死信提交成功")

	// 从死信队列获取
	queue := dlm.GetQueue()
	if queue != nil {
		fmt.Printf("死信队列大小: %d\n", queue.Size())

		// 尝试弹出死信
		deadLetter, err := queue.TryPop()
		if err != nil {
			fmt.Printf("弹出死信失败: %v\n", err)
		} else {
			fmt.Printf("死信信息: 原因=%s, 消息类型=%s, 时间=%v\n",
				deadLetter.Reason,
				deadLetter.Envelope.Message().Type(),
				deadLetter.Timestamp)
		}
	}

	// 检查存储的死信
	stored := storageHandler.GetStored()
	fmt.Printf("存储的死信数量: %d\n", len(stored))

	// 测试邮箱满时的死信处理
	fmt.Println("\n测试邮箱满时的死信处理:")
	mailboxConfig := mailbox.Config{
		Capacity:   2,
		DropPolicy: mailbox.DropPolicyDropOldest,
		TTLEnabled: true,
		DeadLetterQueue: &mailbox.DeadLetterQueueConfig{
			Enabled:       true,
			Capacity:      10,
			RetentionTime: 1 * time.Hour,
		},
	}

	mb := mailbox.NewBoundedMailbox(mailboxConfig)
	ctx = context.Background()

	// 填满邮箱
	for i := 0; i < 3; i++ {
		msg := message.NewBaseMessage(fmt.Sprintf("msg.%d", i), i)
		msg.SetSender(addr)
		msg.SetReceiver(addr)
		envelope := message.NewEnvelope(msg)

		err := mb.Push(ctx, envelope)
		if err != nil {
			fmt.Printf("消息 %d: %v\n", i, err)
		} else {
			fmt.Printf("消息 %d: 推送成功\n", i)
		}
	}

	fmt.Printf("最终邮箱大小: %d\n", mb.Size())
}

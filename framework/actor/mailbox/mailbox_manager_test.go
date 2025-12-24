package mailbox

import (
	"context"
	"testing"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

func TestDefaultMailboxManager(t *testing.T) {
	manager := NewDefaultMailboxManager()

	if manager == nil {
		t.Fatal("manager should not be nil")
	}

	// 测试初始状态
	stats := manager.Stats()
	if stats.TotalMailboxes != 0 {
		t.Errorf("initial TotalMailboxes should be 0, got %d", stats.TotalMailboxes)
	}

	if stats.ActiveMailboxes != 0 {
		t.Errorf("initial ActiveMailboxes should be 0, got %d", stats.ActiveMailboxes)
	}
}

func TestMailboxManagerCreateAndGet(t *testing.T) {
	manager := NewDefaultMailboxManager()

	// 创建地址
	addr, err := message.NewLocalAddress("local", "/test/actor")
	if err != nil {
		t.Fatalf("failed to create address: %v", err)
	}

	// 创建邮箱配置
	config := Config{
		Capacity: 100,
	}

	// 创建邮箱
	mailbox, err := manager.CreateMailbox(addr, config)
	if err != nil {
		t.Fatalf("failed to create mailbox: %v", err)
	}

	if mailbox == nil {
		t.Fatal("mailbox should not be nil")
	}

	// 获取邮箱
	retrieved, err := manager.GetMailbox(addr)
	if err != nil {
		t.Fatalf("failed to get mailbox: %v", err)
	}

	if retrieved != mailbox {
		t.Error("retrieved mailbox should be the same instance")
	}

	// 检查统计
	stats := manager.Stats()
	if stats.TotalMailboxes != 1 {
		t.Errorf("TotalMailboxes should be 1, got %d", stats.TotalMailboxes)
	}

	if stats.ActiveMailboxes != 1 {
		t.Errorf("ActiveMailboxes should be 1, got %d", stats.ActiveMailboxes)
	}
}

func TestMailboxManagerDuplicateCreate(t *testing.T) {
	manager := NewDefaultMailboxManager()

	addr, err := message.NewLocalAddress("local", "/test/actor")
	if err != nil {
		t.Fatalf("failed to create address: %v", err)
	}

	config := Config{
		Capacity: 100,
	}

	// 第一次创建应该成功
	_, err = manager.CreateMailbox(addr, config)
	if err != nil {
		t.Fatalf("first create should succeed: %v", err)
	}

	// 第二次创建应该失败
	_, err = manager.CreateMailbox(addr, config)
	if err == nil {
		t.Error("second create should fail with duplicate error")
	}
}

func TestMailboxManagerGetOrCreate(t *testing.T) {
	manager := NewDefaultMailboxManager()

	addr, err := message.NewLocalAddress("local", "/test/actor")
	if err != nil {
		t.Fatalf("failed to create address: %v", err)
	}

	config := Config{
		Capacity: 100,
	}

	// 获取或创建（应该创建）
	mailbox1, err := manager.GetOrCreateMailbox(addr, config)
	if err != nil {
		t.Fatalf("failed to get or create mailbox: %v", err)
	}

	if mailbox1 == nil {
		t.Fatal("mailbox should not be nil")
	}

	// 再次获取或创建（应该获取已有的）
	mailbox2, err := manager.GetOrCreateMailbox(addr, config)
	if err != nil {
		t.Fatalf("failed to get existing mailbox: %v", err)
	}

	if mailbox1 != mailbox2 {
		t.Error("GetOrCreate should return same instance for same address")
	}

	// 检查统计
	stats := manager.Stats()
	if stats.TotalMailboxes != 1 {
		t.Errorf("TotalMailboxes should be 1, got %d", stats.TotalMailboxes)
	}
}

func TestMailboxManagerRemove(t *testing.T) {
	manager := NewDefaultMailboxManager()

	addr, err := message.NewLocalAddress("local", "/test/actor")
	if err != nil {
		t.Fatalf("failed to create address: %v", err)
	}

	config := Config{
		Capacity: 100,
	}

	// 创建邮箱
	_, err = manager.CreateMailbox(addr, config)
	if err != nil {
		t.Fatalf("failed to create mailbox: %v", err)
	}

	// 检查是否存在
	if !manager.HasMailbox(addr) {
		t.Error("HasMailbox should return true for existing mailbox")
	}

	// 移除邮箱
	err = manager.RemoveMailbox(addr)
	if err != nil {
		t.Fatalf("failed to remove mailbox: %v", err)
	}

	// 检查是否已移除
	if manager.HasMailbox(addr) {
		t.Error("HasMailbox should return false after removal")
	}

	// 再次移除应该失败
	err = manager.RemoveMailbox(addr)
	if err == nil {
		t.Error("removing non-existent mailbox should fail")
	}

	// 检查统计
	stats := manager.Stats()
	if stats.ActiveMailboxes != 0 {
		t.Errorf("ActiveMailboxes should be 0 after removal, got %d", stats.ActiveMailboxes)
	}
}

func TestMailboxManagerListMailboxes(t *testing.T) {
	manager := NewDefaultMailboxManager()

	// 创建多个邮箱
	addresses := make([]message.Address, 3)
	for i := 0; i < 3; i++ {
		addr, err := message.NewLocalAddress("local", "/test/actor"+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to create address %d: %v", i, err)
		}
		addresses[i] = addr

		config := Config{
			Capacity: 100,
		}

		_, err = manager.CreateMailbox(addr, config)
		if err != nil {
			t.Fatalf("failed to create mailbox %d: %v", i, err)
		}
	}

	// 列出邮箱
	list := manager.ListMailboxes()
	if len(list) != 3 {
		t.Errorf("expected 3 mailboxes in list, got %d", len(list))
	}

	// 检查统计
	stats := manager.Stats()
	if stats.TotalMailboxes != 3 {
		t.Errorf("TotalMailboxes should be 3, got %d", stats.TotalMailboxes)
	}

	if stats.ActiveMailboxes != 3 {
		t.Errorf("ActiveMailboxes should be 3, got %d", stats.ActiveMailboxes)
	}
}

func TestMailboxManagerClose(t *testing.T) {
	manager := NewDefaultMailboxManager()

	// 创建几个邮箱
	for i := 0; i < 3; i++ {
		addr, err := message.NewLocalAddress("local", "/test/actor"+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to create address %d: %v", i, err)
		}

		config := Config{
			Capacity: 100,
		}

		_, err = manager.CreateMailbox(addr, config)
		if err != nil {
			t.Fatalf("failed to create mailbox %d: %v", i, err)
		}
	}

	// 关闭管理器
	err := manager.Close()
	if err != nil {
		t.Fatalf("failed to close manager: %v", err)
	}

	// 关闭后创建邮箱应该失败
	addr, _ := message.NewLocalAddress("local", "/new/actor")
	config := Config{Capacity: 100}
	_, err = manager.CreateMailbox(addr, config)
	if err == nil {
		t.Error("creating mailbox after close should fail")
	}

	// 关闭后获取邮箱应该失败
	existingAddr, _ := message.NewLocalAddress("local", "/test/actorA")
	_, err = manager.GetMailbox(existingAddr)
	if err == nil {
		t.Error("getting mailbox after close should fail")
	}

	// 检查统计
	stats := manager.Stats()
	if stats.ActiveMailboxes != 0 {
		t.Errorf("ActiveMailboxes should be 0 after close, got %d", stats.ActiveMailboxes)
	}
}

func TestDeliveryService(t *testing.T) {
	manager := NewDefaultMailboxManager()
	router := message.GetDefaultRouter()

	service := NewDeliveryService(manager, router)

	if service == nil {
		t.Fatal("service should not be nil")
	}

	// 创建消息
	ctx := context.Background()
	msg := message.NewBaseMessage("test.delivery", "test data")

	// 设置发送者和接收者
	sender, _ := message.NewLocalAddress("local", "/sender")
	receiver, _ := message.NewLocalAddress("local", "/receiver")
	msg.SetSender(sender)
	msg.SetReceiver(receiver)

	// 投递消息
	err := service.Deliver(ctx, msg)
	if err != nil {
		t.Fatalf("deliver failed: %v", err)
	}

	// 获取邮箱并检查消息
	mailbox, err := manager.GetMailbox(receiver)
	if err != nil {
		t.Fatalf("failed to get mailbox: %v", err)
	}

	if mailbox.Size() != 1 {
		t.Errorf("mailbox should have 1 message, got %d", mailbox.Size())
	}

	// 弹出消息
	envelope, err := mailbox.Pop(ctx)
	if err != nil {
		t.Fatalf("failed to pop message: %v", err)
	}

	if envelope.Message().Type() != "test.delivery" {
		t.Errorf("expected message type 'test.delivery', got %s", envelope.Message().Type())
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Capacity != 1000 {
		t.Errorf("default capacity should be 1000, got %d", config.Capacity)
	}

	if config.DropPolicy != DropPolicyReject {
		t.Errorf("default drop policy should be DropPolicyReject, got %v", config.DropPolicy)
	}

	if !config.PriorityEnabled {
		t.Error("default priority should be enabled")
	}

	if !config.TTLEnabled {
		t.Error("default TTL should be enabled")
	}

	if config.DeadLetterQueue == nil {
		t.Fatal("default dead letter queue config should not be nil")
	}

	if !config.DeadLetterQueue.Enabled {
		t.Error("default dead letter queue should be enabled")
	}

	if config.DeadLetterQueue.Capacity != 100 {
		t.Errorf("default dead letter queue capacity should be 100, got %d", config.DeadLetterQueue.Capacity)
	}
}

func TestGetDefaultManager(t *testing.T) {
	manager1 := GetDefaultManager()
	manager2 := GetDefaultManager()

	if manager1 != manager2 {
		t.Error("GetDefaultManager should return same instance")
	}

	if manager1 == nil {
		t.Fatal("default manager should not be nil")
	}
}

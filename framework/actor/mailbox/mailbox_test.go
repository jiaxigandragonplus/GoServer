package mailbox

import (
	"context"
	"testing"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

func TestNewBoundedMailbox(t *testing.T) {
	config := Config{
		Capacity: 10,
	}

	mailbox := NewBoundedMailbox(config)

	if mailbox == nil {
		t.Fatal("mailbox should not be nil")
	}

	if mailbox.Capacity() != 10 {
		t.Errorf("expected capacity 10, got %d", mailbox.Capacity())
	}

	if !mailbox.IsEmpty() {
		t.Error("new mailbox should be empty")
	}

	if mailbox.IsFull() {
		t.Error("new mailbox should not be full")
	}
}

func TestBoundedMailboxPushPop(t *testing.T) {
	config := Config{
		Capacity: 3,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	// 创建测试消息
	msg := message.NewBaseMessage("test.type", "test data")
	addr, _ := message.NewLocalAddress("local", "/test")
	msg.SetSender(addr)
	msg.SetReceiver(addr)
	envelope := message.NewEnvelope(msg)

	// 推送消息
	err := mailbox.Push(ctx, envelope)
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}

	if mailbox.Size() != 1 {
		t.Errorf("expected size 1, got %d", mailbox.Size())
	}

	if mailbox.IsEmpty() {
		t.Error("mailbox should not be empty after push")
	}

	// 弹出消息
	popped, err := mailbox.Pop(ctx)
	if err != nil {
		t.Fatalf("pop failed: %v", err)
	}

	if popped == nil {
		t.Fatal("popped envelope should not be nil")
	}

	if popped.Message().Type() != "test.type" {
		t.Errorf("expected message type 'test.type', got %s", popped.Message().Type())
	}

	if mailbox.Size() != 0 {
		t.Errorf("expected size 0 after pop, got %d", mailbox.Size())
	}

	if !mailbox.IsEmpty() {
		t.Error("mailbox should be empty after pop")
	}
}

func TestBoundedMailboxFull(t *testing.T) {
	config := Config{
		Capacity:   2,
		DropPolicy: DropPolicyReject,
		TTLEnabled: false,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	// 创建测试消息
	msg1 := message.NewBaseMessage("type1", "data1")
	addr, _ := message.NewLocalAddress("local", "/test")
	msg1.SetSender(addr)
	msg1.SetReceiver(addr)
	envelope1 := message.NewEnvelope(msg1)

	msg2 := message.NewBaseMessage("type2", "data2")
	msg2.SetSender(addr)
	msg2.SetReceiver(addr)
	envelope2 := message.NewEnvelope(msg2)

	msg3 := message.NewBaseMessage("type3", "data3")
	msg3.SetSender(addr)
	msg3.SetReceiver(addr)
	envelope3 := message.NewEnvelope(msg3)

	// 推送两个消息，应该成功
	err := mailbox.Push(ctx, envelope1)
	if err != nil {
		t.Fatalf("first push failed: %v", err)
	}

	err = mailbox.Push(ctx, envelope2)
	if err != nil {
		t.Fatalf("second push failed: %v", err)
	}

	if !mailbox.IsFull() {
		t.Error("mailbox should be full after pushing capacity messages")
	}

	// 第三个消息应该失败（拒绝策略）
	err = mailbox.Push(ctx, envelope3)
	if err != ErrMailboxFull {
		t.Errorf("expected ErrMailboxFull, got %v", err)
	}
}

func TestBoundedMailboxDropOldest(t *testing.T) {
	config := Config{
		Capacity:   2,
		DropPolicy: DropPolicyDropOldest,
		TTLEnabled: false,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	// 创建测试消息
	msg1 := message.NewBaseMessage("type1", "data1")
	addr, _ := message.NewLocalAddress("local", "/test")
	msg1.SetSender(addr)
	msg1.SetReceiver(addr)
	envelope1 := message.NewEnvelope(msg1)

	msg2 := message.NewBaseMessage("type2", "data2")
	msg2.SetSender(addr)
	msg2.SetReceiver(addr)
	envelope2 := message.NewEnvelope(msg2)

	msg3 := message.NewBaseMessage("type3", "data3")
	msg3.SetSender(addr)
	msg3.SetReceiver(addr)
	envelope3 := message.NewEnvelope(msg3)

	// 推送两个消息
	mailbox.Push(ctx, envelope1)
	mailbox.Push(ctx, envelope2)

	// 第三个消息应该丢弃最旧的消息（type1）
	err := mailbox.Push(ctx, envelope3)
	if err != nil {
		t.Fatalf("push with drop oldest should succeed, got error: %v", err)
	}

	if mailbox.Size() != 2 {
		t.Errorf("expected size 2 after dropping oldest, got %d", mailbox.Size())
	}

	// 弹出的应该是第二个和第三个消息
	popped1, _ := mailbox.Pop(ctx)
	if popped1.Message().Type() != "type2" {
		t.Errorf("expected first popped to be type2, got %s", popped1.Message().Type())
	}

	popped2, _ := mailbox.Pop(ctx)
	if popped2.Message().Type() != "type3" {
		t.Errorf("expected second popped to be type3, got %s", popped2.Message().Type())
	}
}

func TestBoundedMailboxTTL(t *testing.T) {
	config := Config{
		Capacity:   10,
		TTLEnabled: true,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	// 创建过期消息
	msg := message.NewBaseMessage("expired.type", "expired data")
	addr, _ := message.NewLocalAddress("local", "/test")
	msg.SetSender(addr)
	msg.SetReceiver(addr)
	msg.SetTTL(1 * time.Millisecond) // 非常短的TTL
	envelope := message.NewEnvelope(msg)

	// 等待消息过期
	time.Sleep(2 * time.Millisecond)

	// 推送应该失败，消息已过期
	err := mailbox.Push(ctx, envelope)
	if err != ErrMessageExpired {
		t.Errorf("expected ErrMessageExpired, got %v", err)
	}
}

func TestUnboundedMailbox(t *testing.T) {
	config := Config{
		Capacity: 0, // 0表示无界
	}

	mailbox := NewUnboundedMailbox(config)
	ctx := context.Background()

	if mailbox.Capacity() <= 0 {
		t.Error("unbounded mailbox should have positive capacity")
	}

	// 推送多个消息
	for i := 0; i < 100; i++ {
		msg := message.NewBaseMessage("test.type", i)
		addr, _ := message.NewLocalAddress("local", "/test")
		msg.SetSender(addr)
		msg.SetReceiver(addr)
		envelope := message.NewEnvelope(msg)

		err := mailbox.Push(ctx, envelope)
		if err != nil {
			t.Fatalf("push failed at iteration %d: %v", i, err)
		}
	}

	if mailbox.Size() != 100 {
		t.Errorf("expected size 100, got %d", mailbox.Size())
	}
}

func TestMailboxClose(t *testing.T) {
	config := Config{
		Capacity: 10,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	// 推送一个消息
	msg := message.NewBaseMessage("test.type", "test data")
	addr, _ := message.NewLocalAddress("local", "/test")
	msg.SetSender(addr)
	msg.SetReceiver(addr)
	envelope := message.NewEnvelope(msg)

	err := mailbox.Push(ctx, envelope)
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}

	// 关闭邮箱
	err = mailbox.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// 关闭后推送应该失败
	err = mailbox.Push(ctx, envelope)
	if err != ErrMailboxClosed {
		t.Errorf("expected ErrMailboxClosed after close, got %v", err)
	}

	// 关闭后弹出应该失败
	_, err = mailbox.Pop(ctx)
	if err != ErrMailboxClosed {
		t.Errorf("expected ErrMailboxClosed for pop after close, got %v", err)
	}
}

func TestTryPop(t *testing.T) {
	config := Config{
		Capacity: 10,
	}

	mailbox := NewBoundedMailbox(config)

	// 空邮箱时TryPop应该返回错误
	_, err := mailbox.TryPop()
	if err != ErrMailboxEmpty {
		t.Errorf("expected ErrMailboxEmpty, got %v", err)
	}

	// 推送消息
	ctx := context.Background()
	msg := message.NewBaseMessage("test.type", "test data")
	addr, _ := message.NewLocalAddress("local", "/test")
	msg.SetSender(addr)
	msg.SetReceiver(addr)
	envelope := message.NewEnvelope(msg)

	err = mailbox.Push(ctx, envelope)
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}

	// TryPop应该成功
	popped, err := mailbox.TryPop()
	if err != nil {
		t.Fatalf("TryPop failed: %v", err)
	}

	if popped == nil {
		t.Fatal("popped envelope should not be nil")
	}

	if popped.Message().Type() != "test.type" {
		t.Errorf("expected message type 'test.type', got %s", popped.Message().Type())
	}
}

func TestMailboxStats(t *testing.T) {
	config := Config{
		Capacity: 5,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	stats := mailbox.Stats()
	if stats.TotalPushed != 0 {
		t.Errorf("initial TotalPushed should be 0, got %d", stats.TotalPushed)
	}

	// 推送3个消息
	for i := 0; i < 3; i++ {
		msg := message.NewBaseMessage("test.type", i)
		addr, _ := message.NewLocalAddress("local", "/test")
		msg.SetSender(addr)
		msg.SetReceiver(addr)
		envelope := message.NewEnvelope(msg)

		err := mailbox.Push(ctx, envelope)
		if err != nil {
			t.Fatalf("push failed at iteration %d: %v", i, err)
		}
	}

	stats = mailbox.Stats()
	if stats.TotalPushed != 3 {
		t.Errorf("TotalPushed should be 3, got %d", stats.TotalPushed)
	}

	if stats.CurrentSize != 3 {
		t.Errorf("CurrentSize should be 3, got %d", stats.CurrentSize)
	}

	// 弹出1个消息
	_, err := mailbox.Pop(ctx)
	if err != nil {
		t.Fatalf("pop failed: %v", err)
	}

	stats = mailbox.Stats()
	if stats.TotalPopped != 1 {
		t.Errorf("TotalPopped should be 1, got %d", stats.TotalPopped)
	}

	if stats.CurrentSize != 2 {
		t.Errorf("CurrentSize should be 2, got %d", stats.CurrentSize)
	}
}

func TestMailboxClear(t *testing.T) {
	config := Config{
		Capacity: 10,
	}

	mailbox := NewBoundedMailbox(config)
	ctx := context.Background()

	// 推送几个消息
	for i := 0; i < 5; i++ {
		msg := message.NewBaseMessage("test.type", i)
		addr, _ := message.NewLocalAddress("local", "/test")
		msg.SetSender(addr)
		msg.SetReceiver(addr)
		envelope := message.NewEnvelope(msg)

		err := mailbox.Push(ctx, envelope)
		if err != nil {
			t.Fatalf("push failed at iteration %d: %v", i, err)
		}
	}

	if mailbox.Size() != 5 {
		t.Errorf("expected size 5 before clear, got %d", mailbox.Size())
	}

	// 清空邮箱
	mailbox.Clear()

	if mailbox.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", mailbox.Size())
	}

	if !mailbox.IsEmpty() {
		t.Error("mailbox should be empty after clear")
	}
}

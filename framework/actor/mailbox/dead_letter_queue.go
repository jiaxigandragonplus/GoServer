package mailbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// DeadLetter 死信
type DeadLetter struct {
	// Envelope 原始消息信封
	Envelope *message.Envelope
	// Reason 成为死信的原因
	Reason string
	// Timestamp 成为死信的时间
	Timestamp time.Time
	// Metadata 额外元数据
	Metadata map[string]interface{}
}

// DeadLetterQueue 死信队列接口
type DeadLetterQueue interface {
	// Push 推送死信到队列
	Push(ctx context.Context, deadLetter DeadLetter) error
	// Pop 从队列弹出死信
	Pop(ctx context.Context) (*DeadLetter, error)
	// TryPop 尝试从队列弹出死信（非阻塞）
	TryPop() (*DeadLetter, error)
	// Size 返回队列当前大小
	Size() int
	// Capacity 返回队列容量
	Capacity() int
	// IsEmpty 队列是否为空
	IsEmpty() bool
	// IsFull 队列是否已满
	IsFull() bool
	// Clear 清空队列
	Clear()
	// Close 关闭队列
	Close() error
	// Stats 返回队列统计信息
	Stats() DeadLetterStats
	// GetExpired 获取过期的死信
	GetExpired(retentionTime time.Duration) []*DeadLetter
}

// DeadLetterStats 死信队列统计信息
type DeadLetterStats struct {
	// TotalPushed 总推送死信数
	TotalPushed int64
	// TotalPopped 总弹出死信数
	TotalPopped int64
	// TotalExpired 总过期死信数
	TotalExpired int64
	// CurrentSize 当前大小
	CurrentSize int
	// MaxSize 历史最大大小
	MaxSize int
	// LastPushTime 最后推送时间
	LastPushTime time.Time
	// LastPopTime 最后弹出时间
	LastPopTime time.Time
}

// DefaultDeadLetterQueue 默认死信队列实现
type DefaultDeadLetterQueue struct {
	config   DeadLetterQueueConfig
	queue    []*DeadLetter
	stats    DeadLetterStats
	mu       sync.RWMutex
	closed   bool
	pushCond *sync.Cond
	popCond  *sync.Cond
}

// NewDefaultDeadLetterQueue 创建新的默认死信队列
func NewDefaultDeadLetterQueue(config DeadLetterQueueConfig) *DefaultDeadLetterQueue {
	dlq := &DefaultDeadLetterQueue{
		config: config,
		queue:  make([]*DeadLetter, 0, config.Capacity),
		stats:  DeadLetterStats{},
	}
	dlq.pushCond = sync.NewCond(&dlq.mu)
	dlq.popCond = sync.NewCond(&dlq.mu)
	return dlq
}

// Push 推送死信
func (q *DefaultDeadLetterQueue) Push(ctx context.Context, deadLetter DeadLetter) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return errors.New("dead letter queue is closed")
	}

	// 检查容量
	if q.config.Capacity > 0 && len(q.queue) >= q.config.Capacity {
		return ErrDeadLetterFull
	}

	// 设置时间戳
	if deadLetter.Timestamp.IsZero() {
		deadLetter.Timestamp = time.Now()
	}

	// 添加到队列
	q.queue = append(q.queue, &deadLetter)

	// 更新统计
	q.stats.TotalPushed++
	q.stats.CurrentSize = len(q.queue)
	if q.stats.CurrentSize > q.stats.MaxSize {
		q.stats.MaxSize = q.stats.CurrentSize
	}
	q.stats.LastPushTime = time.Now()

	// 通知等待的消费者
	q.popCond.Signal()

	return nil
}

// Pop 弹出死信
func (q *DefaultDeadLetterQueue) Pop(ctx context.Context) (*DeadLetter, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, errors.New("dead letter queue is closed")
	}

	// 等待队列非空
	for len(q.queue) == 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			q.popCond.Wait()
			if q.closed {
				return nil, errors.New("dead letter queue is closed")
			}
		}
	}

	// 弹出第一个元素
	deadLetter := q.queue[0]
	q.queue = q.queue[1:]

	// 更新统计
	q.stats.TotalPopped++
	q.stats.CurrentSize = len(q.queue)
	q.stats.LastPopTime = time.Now()

	// 通知等待的生产者
	q.pushCond.Signal()

	return deadLetter, nil
}

// TryPop 尝试弹出死信
func (q *DefaultDeadLetterQueue) TryPop() (*DeadLetter, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, errors.New("dead letter queue is closed")
	}

	if len(q.queue) == 0 {
		return nil, errors.New("dead letter queue is empty")
	}

	// 弹出第一个元素
	deadLetter := q.queue[0]
	q.queue = q.queue[1:]

	// 更新统计
	q.stats.TotalPopped++
	q.stats.CurrentSize = len(q.queue)
	q.stats.LastPopTime = time.Now()

	// 通知等待的生产者
	q.pushCond.Signal()

	return deadLetter, nil
}

// Size 返回大小
func (q *DefaultDeadLetterQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.queue)
}

// Capacity 返回容量
func (q *DefaultDeadLetterQueue) Capacity() int {
	return q.config.Capacity
}

// IsEmpty 是否为空
func (q *DefaultDeadLetterQueue) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.queue) == 0
}

// IsFull 是否已满
func (q *DefaultDeadLetterQueue) IsFull() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.config.Capacity > 0 && len(q.queue) >= q.config.Capacity
}

// Clear 清空队列
func (q *DefaultDeadLetterQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 更新过期统计
	q.stats.TotalExpired += int64(len(q.queue))

	q.queue = make([]*DeadLetter, 0, q.config.Capacity)
	q.stats.CurrentSize = 0

	// 通知等待的生产者
	q.pushCond.Broadcast()
}

// Close 关闭队列
func (q *DefaultDeadLetterQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil
	}

	q.closed = true

	// 唤醒所有等待的goroutine
	q.pushCond.Broadcast()
	q.popCond.Broadcast()

	return nil
}

// Stats 返回统计信息
func (q *DefaultDeadLetterQueue) Stats() DeadLetterStats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.stats
}

// GetExpired 获取过期的死信
func (q *DefaultDeadLetterQueue) GetExpired(retentionTime time.Duration) []*DeadLetter {
	q.mu.Lock()
	defer q.mu.Unlock()

	if retentionTime <= 0 {
		return nil
	}

	cutoffTime := time.Now().Add(-retentionTime)
	var expired []*DeadLetter
	var remaining []*DeadLetter

	for _, dl := range q.queue {
		if dl.Timestamp.Before(cutoffTime) {
			expired = append(expired, dl)
			q.stats.TotalExpired++
		} else {
			remaining = append(remaining, dl)
		}
	}

	q.queue = remaining
	q.stats.CurrentSize = len(q.queue)

	return expired
}

// DeadLetterHandler 死信处理器
type DeadLetterHandler interface {
	// Handle 处理死信
	Handle(ctx context.Context, deadLetter DeadLetter) error
}

// LoggingDeadLetterHandler 日志记录死信处理器
type LoggingDeadLetterHandler struct{}

// Handle 记录死信到日志
func (h *LoggingDeadLetterHandler) Handle(ctx context.Context, deadLetter DeadLetter) error {
	msg := deadLetter.Envelope.Message()
	fmt.Printf("[DEAD LETTER] Reason: %s, Message Type: %s, Sender: %v, Receiver: %v, Timestamp: %v\n",
		deadLetter.Reason,
		msg.Type(),
		msg.Sender(),
		msg.Receiver(),
		deadLetter.Timestamp)
	return nil
}

// StorageDeadLetterHandler 存储死信处理器
type StorageDeadLetterHandler struct {
	storage []DeadLetter
	mu      sync.RWMutex
}

// NewStorageDeadLetterHandler 创建新的存储死信处理器
func NewStorageDeadLetterHandler() *StorageDeadLetterHandler {
	return &StorageDeadLetterHandler{
		storage: make([]DeadLetter, 0),
	}
}

// Handle 存储死信
func (h *StorageDeadLetterHandler) Handle(ctx context.Context, deadLetter DeadLetter) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.storage = append(h.storage, deadLetter)
	return nil
}

// GetStored 获取存储的死信
func (h *StorageDeadLetterHandler) GetStored() []DeadLetter {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.storage
}

// ClearStorage 清空存储
func (h *StorageDeadLetterHandler) ClearStorage() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.storage = make([]DeadLetter, 0)
}

// DeadLetterManager 死信管理器
type DeadLetterManager struct {
	queue    DeadLetterQueue
	handlers []DeadLetterHandler
	mu       sync.RWMutex
}

// NewDeadLetterManager 创建新的死信管理器
func NewDeadLetterManager(config DeadLetterQueueConfig) *DeadLetterManager {
	return &DeadLetterManager{
		queue:    NewDefaultDeadLetterQueue(config),
		handlers: make([]DeadLetterHandler, 0),
	}
}

// RegisterHandler 注册死信处理器
func (m *DeadLetterManager) RegisterHandler(handler DeadLetterHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// Submit 提交死信
func (m *DeadLetterManager) Submit(ctx context.Context, envelope *message.Envelope, reason string) error {
	deadLetter := DeadLetter{
		Envelope:  envelope,
		Reason:    reason,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// 添加到队列
	if err := m.queue.Push(ctx, deadLetter); err != nil {
		return err
	}

	// 异步处理死信
	go m.processDeadLetter(deadLetter)

	return nil
}

// processDeadLetter 处理死信
func (m *DeadLetterManager) processDeadLetter(deadLetter DeadLetter) {
	ctx := context.Background()

	m.mu.RLock()
	handlers := make([]DeadLetterHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	// 调用所有处理器
	for _, handler := range handlers {
		if err := handler.Handle(ctx, deadLetter); err != nil {
			fmt.Printf("failed to handle dead letter: %v\n", err)
		}
	}
}

// GetQueue 获取死信队列
func (m *DeadLetterManager) GetQueue() DeadLetterQueue {
	return m.queue
}

// StartCleanupWorker 启动清理工作器
func (m *DeadLetterManager) StartCleanupWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.cleanupExpired()
			}
		}
	}()
}

// cleanupExpired 清理过期的死信
func (m *DeadLetterManager) cleanupExpired() {
	if m.queue == nil {
		return
	}

	// 获取过期的死信
	expired := m.queue.GetExpired(m.queue.(*DefaultDeadLetterQueue).config.RetentionTime)
	if len(expired) > 0 {
		fmt.Printf("cleaned up %d expired dead letters\n", len(expired))
	}
}

package mailbox

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// Mailbox 邮箱接口
type Mailbox interface {
	// Push 推送消息到邮箱
	Push(ctx context.Context, envelope *message.Envelope) error
	// Pop 从邮箱弹出消息（阻塞或非阻塞）
	Pop(ctx context.Context) (*message.Envelope, error)
	// TryPop 尝试从邮箱弹出消息（非阻塞）
	TryPop() (*message.Envelope, error)
	// Size 返回邮箱当前大小
	Size() int
	// Capacity 返回邮箱容量
	Capacity() int
	// IsEmpty 邮箱是否为空
	IsEmpty() bool
	// IsFull 邮箱是否已满
	IsFull() bool
	// Clear 清空邮箱
	Clear()
	// Close 关闭邮箱
	Close() error
	// Stats 返回邮箱统计信息
	Stats() Stats
}

// Stats 邮箱统计信息
type Stats struct {
	// TotalPushed 总推送消息数
	TotalPushed int64
	// TotalPopped 总弹出消息数
	TotalPopped int64
	// TotalDropped 总丢弃消息数
	TotalDropped int64
	// TotalDeadLetter 总死信消息数
	TotalDeadLetter int64
	// CurrentSize 当前大小
	CurrentSize int
	// MaxSize 历史最大大小
	MaxSize int
	// LastPushTime 最后推送时间
	LastPushTime time.Time
	// LastPopTime 最后弹出时间
	LastPopTime time.Time
}

// Config 邮箱配置
type Config struct {
	// Capacity 邮箱容量，0表示无界邮箱
	Capacity int
	// DropPolicy 满邮箱时的丢弃策略
	DropPolicy DropPolicy
	// PriorityEnabled 是否启用优先级
	PriorityEnabled bool
	// TTLEnabled 是否启用TTL检查
	TTLEnabled bool
	// DeadLetterQueue 死信队列配置
	DeadLetterQueue *DeadLetterQueueConfig
}

// DropPolicy 丢弃策略
type DropPolicy int

const (
	// DropPolicyReject 拒绝新消息（返回错误）
	DropPolicyReject DropPolicy = iota
	// DropPolicyDropOldest 丢弃最旧的消息
	DropPolicyDropOldest
	// DropPolicyDropNewest 丢弃最新的消息
	DropPolicyDropNewest
	// DropPolicyDropLowestPriority 丢弃最低优先级的消息
	DropPolicyDropLowestPriority
)

// DeadLetterQueueConfig 死信队列配置
type DeadLetterQueueConfig struct {
	// Enabled 是否启用死信队列
	Enabled bool
	// Capacity 死信队列容量
	Capacity int
	// RetentionTime 消息保留时间
	RetentionTime time.Duration
}

// 错误定义
var (
	ErrMailboxFull     = errors.New("mailbox is full")
	ErrMailboxClosed   = errors.New("mailbox is closed")
	ErrMailboxEmpty    = errors.New("mailbox is empty")
	ErrMessageExpired  = errors.New("message expired")
	ErrInvalidPriority = errors.New("invalid message priority")
	ErrDeadLetterFull  = errors.New("dead letter queue is full")
)

// New 创建新的邮箱
func New(config Config) Mailbox {
	if config.Capacity > 0 {
		return NewBoundedMailbox(config)
	}
	return NewUnboundedMailbox(config)
}

// NewBoundedMailbox 创建有界邮箱
func NewBoundedMailbox(config Config) Mailbox {
	mb := &boundedMailbox{
		config: config,
		queue:  make(chan *message.Envelope, config.Capacity),
		stats:  &Stats{},
		closed: make(chan struct{}),
	}

	// 初始化死信队列
	if config.DeadLetterQueue != nil && config.DeadLetterQueue.Enabled {
		mb.deadLetterMgr = NewDeadLetterManager(*config.DeadLetterQueue)
		mb.deadLetterQueue = mb.deadLetterMgr.GetQueue()
		// 启动清理工作器
		mb.deadLetterMgr.StartCleanupWorker(1 * time.Hour)
		// 注册默认日志处理器
		mb.deadLetterMgr.RegisterHandler(&LoggingDeadLetterHandler{})
	}

	return mb
}

// NewUnboundedMailbox 创建无界邮箱
func NewUnboundedMailbox(config Config) Mailbox {
	mb := &unboundedMailbox{
		config: config,
		queue:  make(chan *message.Envelope, 1000), // 使用缓冲通道但容量较大
		stats:  &Stats{},
		closed: make(chan struct{}),
	}

	// 初始化死信队列
	if config.DeadLetterQueue != nil && config.DeadLetterQueue.Enabled {
		mb.deadLetterMgr = NewDeadLetterManager(*config.DeadLetterQueue)
		mb.deadLetterQueue = mb.deadLetterMgr.GetQueue()
		// 启动清理工作器
		mb.deadLetterMgr.StartCleanupWorker(1 * time.Hour)
		// 注册默认日志处理器
		mb.deadLetterMgr.RegisterHandler(&LoggingDeadLetterHandler{})
	}

	return mb
}

// boundedMailbox 有界邮箱实现
type boundedMailbox struct {
	config          Config
	queue           chan *message.Envelope
	deadLetterQueue DeadLetterQueue
	deadLetterMgr   *DeadLetterManager
	stats           *Stats
	mu              sync.RWMutex
	closed          chan struct{}
}

func (m *boundedMailbox) Push(ctx context.Context, envelope *message.Envelope) error {
	select {
	case <-m.closed:
		return ErrMailboxClosed
	default:
	}

	// 检查消息是否过期
	if m.config.TTLEnabled && envelope.Message().TTL() > 0 {
		if time.Since(envelope.Message().Timestamp()) > envelope.Message().TTL() {
			m.stats.TotalDropped++
			// 发送到死信队列
			if m.deadLetterMgr != nil {
				m.deadLetterMgr.Submit(ctx, envelope, "message expired")
			}
			return ErrMessageExpired
		}
	}

	select {
	case m.queue <- envelope:
		m.updateStatsAfterPush()
		return nil
	default:
		// 邮箱已满，应用丢弃策略
		return m.handleFullMailbox(ctx, envelope)
	}
}

func (m *boundedMailbox) Pop(ctx context.Context) (*message.Envelope, error) {
	select {
	case <-m.closed:
		return nil, ErrMailboxClosed
	case envelope := <-m.queue:
		m.updateStatsAfterPop()
		return envelope, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *boundedMailbox) TryPop() (*message.Envelope, error) {
	select {
	case <-m.closed:
		return nil, ErrMailboxClosed
	case envelope := <-m.queue:
		m.updateStatsAfterPop()
		return envelope, nil
	default:
		return nil, ErrMailboxEmpty
	}
}

func (m *boundedMailbox) Size() int {
	return len(m.queue)
}

func (m *boundedMailbox) Capacity() int {
	return m.config.Capacity
}

func (m *boundedMailbox) IsEmpty() bool {
	return len(m.queue) == 0
}

func (m *boundedMailbox) IsFull() bool {
	return len(m.queue) == m.config.Capacity
}

func (m *boundedMailbox) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for {
		select {
		case <-m.queue:
			// 丢弃消息
		default:
			return
		}
	}
}

func (m *boundedMailbox) Close() error {
	select {
	case <-m.closed:
		return nil // 已经关闭
	default:
		close(m.closed)
		return nil
	}
}

func (m *boundedMailbox) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.CurrentSize = len(m.queue)
	return stats
}

func (m *boundedMailbox) updateStatsAfterPush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalPushed++
	m.stats.LastPushTime = time.Now()
	currentSize := len(m.queue)
	if currentSize > m.stats.MaxSize {
		m.stats.MaxSize = currentSize
	}
}

func (m *boundedMailbox) updateStatsAfterPop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalPopped++
	m.stats.LastPopTime = time.Now()
}

func (m *boundedMailbox) handleFullMailbox(ctx context.Context, envelope *message.Envelope) error {
	switch m.config.DropPolicy {
	case DropPolicyReject:
		return ErrMailboxFull
	case DropPolicyDropOldest:
		// 丢弃队列中最旧的消息
		select {
		case dropped := <-m.queue:
			m.stats.TotalDropped++
			// 将丢弃的消息发送到死信队列
			if m.deadLetterMgr != nil {
				m.deadLetterMgr.Submit(ctx, dropped, "dropped due to mailbox full (drop oldest)")
			}
			// 然后尝试重新推送新消息
			select {
			case m.queue <- envelope:
				m.updateStatsAfterPush()
				return nil
			default:
				// 仍然满，继续丢弃
				return m.handleFullMailbox(ctx, envelope)
			}
		default:
			return ErrMailboxFull
		}
	case DropPolicyDropNewest:
		// 直接丢弃新消息
		m.stats.TotalDropped++
		// 发送到死信队列
		if m.deadLetterMgr != nil {
			m.deadLetterMgr.Submit(ctx, envelope, "dropped due to mailbox full (drop newest)")
		}
		return nil
	case DropPolicyDropLowestPriority:
		// 需要实现优先级检查
		// 简化实现：丢弃新消息
		m.stats.TotalDropped++
		// 发送到死信队列
		if m.deadLetterMgr != nil {
			m.deadLetterMgr.Submit(ctx, envelope, "dropped due to mailbox full (drop lowest priority)")
		}
		return nil
	default:
		return ErrMailboxFull
	}
}

// unboundedMailbox 无界邮箱实现
type unboundedMailbox struct {
	config          Config
	queue           chan *message.Envelope
	deadLetterQueue DeadLetterQueue
	deadLetterMgr   *DeadLetterManager
	stats           *Stats
	mu              sync.RWMutex
	closed          chan struct{}
}

func (m *unboundedMailbox) Push(ctx context.Context, envelope *message.Envelope) error {
	select {
	case <-m.closed:
		return ErrMailboxClosed
	default:
	}

	// 检查消息是否过期
	if m.config.TTLEnabled && envelope.Message().TTL() > 0 {
		if time.Since(envelope.Message().Timestamp()) > envelope.Message().TTL() {
			m.stats.TotalDropped++
			// 发送到死信队列
			if m.deadLetterMgr != nil {
				m.deadLetterMgr.Submit(ctx, envelope, "message expired")
			}
			return ErrMessageExpired
		}
	}

	select {
	case m.queue <- envelope:
		m.updateStatsAfterPush()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *unboundedMailbox) Pop(ctx context.Context) (*message.Envelope, error) {
	select {
	case <-m.closed:
		return nil, ErrMailboxClosed
	case envelope := <-m.queue:
		m.updateStatsAfterPop()
		return envelope, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *unboundedMailbox) TryPop() (*message.Envelope, error) {
	select {
	case <-m.closed:
		return nil, ErrMailboxClosed
	case envelope := <-m.queue:
		m.updateStatsAfterPop()
		return envelope, nil
	default:
		return nil, ErrMailboxEmpty
	}
}

func (m *unboundedMailbox) Size() int {
	return len(m.queue)
}

func (m *unboundedMailbox) Capacity() int {
	return cap(m.queue)
}

func (m *unboundedMailbox) IsEmpty() bool {
	return len(m.queue) == 0
}

func (m *unboundedMailbox) IsFull() bool {
	return len(m.queue) == cap(m.queue)
}

func (m *unboundedMailbox) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for {
		select {
		case <-m.queue:
			// 丢弃消息
		default:
			return
		}
	}
}

func (m *unboundedMailbox) Close() error {
	select {
	case <-m.closed:
		return nil // 已经关闭
	default:
		close(m.closed)
		// 关闭死信管理器
		if m.deadLetterMgr != nil && m.deadLetterQueue != nil {
			m.deadLetterQueue.Close()
		}
		return nil
	}
}

func (m *unboundedMailbox) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.CurrentSize = len(m.queue)
	return stats
}

func (m *unboundedMailbox) updateStatsAfterPush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalPushed++
	m.stats.LastPushTime = time.Now()
	currentSize := len(m.queue)
	if currentSize > m.stats.MaxSize {
		m.stats.MaxSize = currentSize
	}
}

func (m *unboundedMailbox) updateStatsAfterPop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalPopped++
	m.stats.LastPopTime = time.Now()
}

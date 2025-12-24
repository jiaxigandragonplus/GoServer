package message

import (
	"context"
	"time"
)

// Message 消息接口
type Message interface {
	// Type 返回消息类型
	Type() string
	// Data 返回消息数据
	Data() interface{}
	// SetData 设置消息数据
	SetData(data interface{})
	// Sender 返回发送者地址
	Sender() Address
	// SetSender 设置发送者地址
	SetSender(sender Address)
	// Receiver 返回接收者地址
	Receiver() Address
	// SetReceiver 设置接收者地址
	SetReceiver(receiver Address)
	// Timestamp 返回消息时间戳
	Timestamp() time.Time
	// SetTimestamp 设置消息时间戳
	SetTimestamp(timestamp time.Time)
	// ID 返回消息ID
	ID() string
	// SetID 设置消息ID
	SetID(id string)
	// Priority 返回消息优先级
	Priority() Priority
	// SetPriority 设置消息优先级
	SetPriority(priority Priority)
	// TTL 返回消息生存时间
	TTL() time.Duration
	// SetTTL 设置消息生存时间
	SetTTL(ttl time.Duration)
	// Headers 返回消息头部
	Headers() map[string]string
	// SetHeader 设置消息头部
	SetHeader(key, value string)
	// GetHeader 获取消息头部
	GetHeader(key string) string
	// Clone 克隆消息
	Clone() Message
}

// Address 地址接口，表示Actor地址
type Address interface {
	// String 返回地址字符串表示
	String() string
	// Protocol 返回地址协议
	Protocol() string
	// Host 返回主机地址
	Host() string
	// Port 返回端口
	Port() string
	// Path 返回路径
	Path() string
	// Equal 比较地址是否相等
	Equal(other Address) bool
	// IsLocal 是否是本地地址
	IsLocal() bool
	// IsRemote 是否是远程地址
	IsRemote() bool
}

// Priority 消息优先级
type Priority int

const (
	// PriorityLow 低优先级
	PriorityLow Priority = iota
	// PriorityNormal 普通优先级
	PriorityNormal
	// PriorityHigh 高优先级
	PriorityHigh
	// PriorityCritical 关键优先级
	PriorityCritical
)

// Envelope 消息信封，包含消息和元数据
type Envelope struct {
	message    Message
	delivery   Delivery
	context    context.Context
	cancelFunc context.CancelFunc
}

// Delivery 消息投递信息
type Delivery struct {
	// Attempt 投递尝试次数
	Attempt int
	// MaxAttempts 最大尝试次数
	MaxAttempts int
	// RetryDelay 重试延迟
	RetryDelay time.Duration
	// DeadLetter 是否发送到死信队列
	DeadLetter bool
	// Error 投递错误
	Error error
}

// NewEnvelope 创建新的消息信封
func NewEnvelope(msg Message) *Envelope {
	ctx, cancel := context.WithCancel(context.Background())
	return &Envelope{
		message:    msg,
		delivery:   Delivery{Attempt: 1, MaxAttempts: 3},
		context:    ctx,
		cancelFunc: cancel,
	}
}

// Message 返回信封中的消息
func (e *Envelope) Message() Message {
	return e.message
}

// Delivery 返回投递信息
func (e *Envelope) Delivery() Delivery {
	return e.delivery
}

// SetDelivery 设置投递信息
func (e *Envelope) SetDelivery(delivery Delivery) {
	e.delivery = delivery
}

// Context 返回上下文
func (e *Envelope) Context() context.Context {
	return e.context
}

// Cancel 取消消息处理
func (e *Envelope) Cancel() {
	e.cancelFunc()
}

// IncrementAttempt 增加尝试次数
func (e *Envelope) IncrementAttempt() {
	e.delivery.Attempt++
}

// ShouldRetry 判断是否应该重试
func (e *Envelope) ShouldRetry() bool {
	return e.delivery.Attempt < e.delivery.MaxAttempts
}

// MarkAsDeadLetter 标记为死信
func (e *Envelope) MarkAsDeadLetter(err error) {
	e.delivery.DeadLetter = true
	e.delivery.Error = err
}

// BaseMessage 基础消息实现
type BaseMessage struct {
	msgType   string
	data      interface{}
	sender    Address
	receiver  Address
	timestamp time.Time
	id        string
	priority  Priority
	ttl       time.Duration
	headers   map[string]string
}

// NewBaseMessage 创建新的基础消息
func NewBaseMessage(msgType string, data interface{}) *BaseMessage {
	return &BaseMessage{
		msgType:   msgType,
		data:      data,
		timestamp: time.Now(),
		id:        generateID(),
		priority:  PriorityNormal,
		ttl:       0, // 0表示永不过期
		headers:   make(map[string]string),
	}
}

func (m *BaseMessage) Type() string {
	return m.msgType
}

func (m *BaseMessage) Data() interface{} {
	return m.data
}

func (m *BaseMessage) SetData(data interface{}) {
	m.data = data
}

func (m *BaseMessage) Sender() Address {
	return m.sender
}

func (m *BaseMessage) SetSender(sender Address) {
	m.sender = sender
}

func (m *BaseMessage) Receiver() Address {
	return m.receiver
}

func (m *BaseMessage) SetReceiver(receiver Address) {
	m.receiver = receiver
}

func (m *BaseMessage) Timestamp() time.Time {
	return m.timestamp
}

func (m *BaseMessage) SetTimestamp(timestamp time.Time) {
	m.timestamp = timestamp
}

func (m *BaseMessage) ID() string {
	return m.id
}

func (m *BaseMessage) SetID(id string) {
	m.id = id
}

func (m *BaseMessage) Priority() Priority {
	return m.priority
}

func (m *BaseMessage) SetPriority(priority Priority) {
	m.priority = priority
}

func (m *BaseMessage) TTL() time.Duration {
	return m.ttl
}

func (m *BaseMessage) SetTTL(ttl time.Duration) {
	m.ttl = ttl
}

func (m *BaseMessage) Headers() map[string]string {
	return m.headers
}

func (m *BaseMessage) SetHeader(key, value string) {
	if m.headers == nil {
		m.headers = make(map[string]string)
	}
	m.headers[key] = value
}

func (m *BaseMessage) GetHeader(key string) string {
	if m.headers == nil {
		return ""
	}
	return m.headers[key]
}

func (m *BaseMessage) Clone() Message {
	clone := &BaseMessage{
		msgType:   m.msgType,
		data:      m.data,
		sender:    m.sender,
		receiver:  m.receiver,
		timestamp: m.timestamp,
		id:        m.id,
		priority:  m.priority,
		ttl:       m.ttl,
		headers:   make(map[string]string),
	}

	// 深度复制headers
	for k, v := range m.headers {
		clone.headers[k] = v
	}

	return clone
}

// 生成消息ID的辅助函数
func generateID() string {
	// 使用时间戳和随机数生成ID
	// 这里使用简化实现，实际应用中应该使用更可靠的ID生成器
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		// 简化实现，实际应该使用crypto/rand
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

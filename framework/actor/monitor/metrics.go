package monitor

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// MetricsCollector 性能指标收集器
type MetricsCollector struct {
	mu sync.RWMutex

	// 系统级指标
	systemMetrics SystemMetrics

	// Actor级指标
	actorMetrics map[string]ActorMetrics

	// 消息级指标
	messageMetrics MessageMetrics

	// 时间窗口指标
	windowMetrics map[string]WindowMetrics

	// 事件处理器
	eventHandlers []MetricsEventHandler
}

// SystemMetrics 系统级指标
type SystemMetrics struct {
	// TotalActors 总actor数量
	TotalActors int64
	// ActiveActors 活跃actor数量
	ActiveActors int64
	// FailedActors 失败actor数量
	FailedActors int64
	// TotalMessages 总消息数量
	TotalMessages int64
	// FailedMessages 失败消息数量
	FailedMessages int64
	// TotalRestarts 总重启次数
	TotalRestarts int64
	// AvgMessageLatency 平均消息延迟（纳秒）
	AvgMessageLatency int64
	// MaxMessageLatency 最大消息延迟（纳秒）
	MaxMessageLatency int64
	// MinMessageLatency 最小消息延迟（纳秒）
	MinMessageLatency int64
	// MessageThroughput 消息吞吐量（消息/秒）
	MessageThroughput float64
	// LastUpdateTime 最后更新时间
	LastUpdateTime time.Time
}

// ActorMetrics Actor级指标
type ActorMetrics struct {
	// ActorAddress actor地址
	ActorAddress string
	// ActorType actor类型
	ActorType string
	// MessagesProcessed 已处理消息数
	MessagesProcessed int64
	// MessagesFailed 处理失败消息数
	MessagesFailed int64
	// ProcessingTime 总处理时间（纳秒）
	ProcessingTime int64
	// AvgProcessingTime 平均处理时间（纳秒）
	AvgProcessingTime int64
	// MaxProcessingTime 最大处理时间（纳秒）
	MaxProcessingTime int64
	// MinProcessingTime 最小处理时间（纳秒）
	MinProcessingTime int64
	// LastMessageTime 最后处理消息时间
	LastMessageTime time.Time
	// Uptime 运行时间
	Uptime time.Duration
	// RestartCount 重启次数
	RestartCount int
	// HealthStatus 健康状态
	HealthStatus actor.HealthStatus
	// State actor状态
	State actor.State
}

// MessageMetrics 消息级指标
type MessageMetrics struct {
	// ByType 按消息类型统计
	ByType map[string]MessageTypeMetrics
	// BySender 按发送者统计
	BySender map[string]SenderMetrics
	// ByReceiver 按接收者统计
	ByReceiver map[string]ReceiverMetrics
}

// MessageTypeMetrics 消息类型指标
type MessageTypeMetrics struct {
	// Type 消息类型
	Type string
	// Count 消息数量
	Count int64
	// FailedCount 失败数量
	FailedCount int64
	// AvgLatency 平均延迟（纳秒）
	AvgLatency int64
	// MaxLatency 最大延迟（纳秒）
	MaxLatency int64
	// MinLatency 最小延迟（纳秒）
	MinLatency int64
}

// SenderMetrics 发送者指标
type SenderMetrics struct {
	// SenderAddress 发送者地址
	SenderAddress string
	// MessagesSent 发送消息数
	MessagesSent int64
	// MessagesFailed 发送失败数
	MessagesFailed int64
	// LastSentTime 最后发送时间
	LastSentTime time.Time
}

// ReceiverMetrics 接收者指标
type ReceiverMetrics struct {
	// ReceiverAddress 接收者地址
	ReceiverAddress string
	// MessagesReceived 接收消息数
	MessagesReceived int64
	// MessagesFailed 接收失败数
	MessagesFailed int64
	// LastReceivedTime 最后接收时间
	LastReceivedTime time.Time
}

// WindowMetrics 时间窗口指标
type WindowMetrics struct {
	// WindowName 窗口名称
	WindowName string
	// WindowSize 窗口大小
	WindowSize time.Duration
	// StartTime 窗口开始时间
	StartTime time.Time
	// EndTime 窗口结束时间
	EndTime time.Time
	// Metrics 窗口内的指标
	Metrics SystemMetrics
}

// MetricsEventHandler 指标事件处理器
type MetricsEventHandler interface {
	// OnMetricsUpdated 指标更新时调用
	OnMetricsUpdated(collector *MetricsCollector)
	// OnActorMetricsUpdated actor指标更新时调用
	OnActorMetricsUpdated(address string, metrics ActorMetrics)
	// OnMessageMetricsUpdated 消息指标更新时调用
	OnMessageMetricsUpdated(msgType string, metrics MessageTypeMetrics)
}

// NewMetricsCollector 创建新的性能指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		systemMetrics: SystemMetrics{},
		actorMetrics:  make(map[string]ActorMetrics),
		messageMetrics: MessageMetrics{
			ByType:     make(map[string]MessageTypeMetrics),
			BySender:   make(map[string]SenderMetrics),
			ByReceiver: make(map[string]ReceiverMetrics),
		},
		windowMetrics: make(map[string]WindowMetrics),
		eventHandlers: make([]MetricsEventHandler, 0),
	}
}

// RecordActorStart 记录actor启动
func (c *MetricsCollector) RecordActorStart(address, actorType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemMetrics.TotalActors++
	c.systemMetrics.ActiveActors++
	c.systemMetrics.LastUpdateTime = time.Now()

	// 初始化actor指标
	c.actorMetrics[address] = ActorMetrics{
		ActorAddress:      address,
		ActorType:         actorType,
		MessagesProcessed: 0,
		MessagesFailed:    0,
		ProcessingTime:    0,
		AvgProcessingTime: 0,
		MaxProcessingTime: 0,
		MinProcessingTime: 0,
		LastMessageTime:   time.Time{},
		Uptime:            0,
		RestartCount:      0,
		HealthStatus:      actor.HealthStatusHealthy,
		State:             actor.StateRunning,
	}

	c.notifyMetricsUpdated()
}

// RecordActorStop 记录actor停止
func (c *MetricsCollector) RecordActorStop(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemMetrics.ActiveActors--
	c.systemMetrics.LastUpdateTime = time.Now()

	if metrics, exists := c.actorMetrics[address]; exists {
		metrics.State = actor.StateStopped
		c.actorMetrics[address] = metrics
	}

	c.notifyMetricsUpdated()
}

// RecordActorFailure 记录actor失败
func (c *MetricsCollector) RecordActorFailure(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemMetrics.FailedActors++
	c.systemMetrics.LastUpdateTime = time.Now()

	if metrics, exists := c.actorMetrics[address]; exists {
		metrics.HealthStatus = actor.HealthStatusUnhealthy
		c.actorMetrics[address] = metrics
	}

	c.notifyMetricsUpdated()
}

// RecordActorRestart 记录actor重启
func (c *MetricsCollector) RecordActorRestart(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemMetrics.TotalRestarts++
	c.systemMetrics.LastUpdateTime = time.Now()

	if metrics, exists := c.actorMetrics[address]; exists {
		metrics.RestartCount++
		metrics.HealthStatus = actor.HealthStatusHealthy
		metrics.State = actor.StateRunning
		c.actorMetrics[address] = metrics
	}

	c.notifyMetricsUpdated()
}

// RecordMessageProcessed 记录消息处理完成
func (c *MetricsCollector) RecordMessageProcessed(
	address string,
	msgType string,
	sender string,
	receiver string,
	processingTime time.Duration,
	success bool,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 更新系统指标
	c.systemMetrics.TotalMessages++
	if !success {
		c.systemMetrics.FailedMessages++
	}

	// 更新消息延迟统计
	latency := processingTime.Nanoseconds()
	if c.systemMetrics.TotalMessages == 1 {
		c.systemMetrics.AvgMessageLatency = latency
		c.systemMetrics.MaxMessageLatency = latency
		c.systemMetrics.MinMessageLatency = latency
	} else {
		// 更新平均延迟
		totalLatency := c.systemMetrics.AvgMessageLatency*(c.systemMetrics.TotalMessages-1) + latency
		c.systemMetrics.AvgMessageLatency = totalLatency / c.systemMetrics.TotalMessages

		// 更新最大/最小延迟
		if latency > c.systemMetrics.MaxMessageLatency {
			c.systemMetrics.MaxMessageLatency = latency
		}
		if latency < c.systemMetrics.MinMessageLatency {
			c.systemMetrics.MinMessageLatency = latency
		}
	}

	c.systemMetrics.LastUpdateTime = time.Now()

	// 更新actor指标
	if metrics, exists := c.actorMetrics[address]; exists {
		metrics.MessagesProcessed++
		if !success {
			metrics.MessagesFailed++
		}

		// 更新处理时间统计
		metrics.ProcessingTime += latency
		if metrics.MessagesProcessed == 1 {
			metrics.AvgProcessingTime = latency
			metrics.MaxProcessingTime = latency
			metrics.MinProcessingTime = latency
		} else {
			// 更新平均处理时间
			totalTime := metrics.AvgProcessingTime*(metrics.MessagesProcessed-1) + latency
			metrics.AvgProcessingTime = totalTime / metrics.MessagesProcessed

			// 更新最大/最小处理时间
			if latency > metrics.MaxProcessingTime {
				metrics.MaxProcessingTime = latency
			}
			if latency < metrics.MinProcessingTime {
				metrics.MinProcessingTime = latency
			}
		}

		metrics.LastMessageTime = time.Now()
		c.actorMetrics[address] = metrics

		// 通知actor指标更新
		c.notifyActorMetricsUpdated(address, metrics)
	}

	// 更新消息类型指标
	msgTypeMetrics := c.messageMetrics.ByType[msgType]
	msgTypeMetrics.Type = msgType
	msgTypeMetrics.Count++
	if !success {
		msgTypeMetrics.FailedCount++
	}

	// 更新消息延迟统计
	if msgTypeMetrics.Count == 1 {
		msgTypeMetrics.AvgLatency = latency
		msgTypeMetrics.MaxLatency = latency
		msgTypeMetrics.MinLatency = latency
	} else {
		// 更新平均延迟
		totalLatency := msgTypeMetrics.AvgLatency*(msgTypeMetrics.Count-1) + latency
		msgTypeMetrics.AvgLatency = totalLatency / msgTypeMetrics.Count

		// 更新最大/最小延迟
		if latency > msgTypeMetrics.MaxLatency {
			msgTypeMetrics.MaxLatency = latency
		}
		if latency < msgTypeMetrics.MinLatency {
			msgTypeMetrics.MinLatency = latency
		}
	}
	c.messageMetrics.ByType[msgType] = msgTypeMetrics

	// 更新发送者指标
	if sender != "" {
		senderMetrics := c.messageMetrics.BySender[sender]
		senderMetrics.SenderAddress = sender
		senderMetrics.MessagesSent++
		if !success {
			senderMetrics.MessagesFailed++
		}
		senderMetrics.LastSentTime = time.Now()
		c.messageMetrics.BySender[sender] = senderMetrics
	}

	// 更新接收者指标
	if receiver != "" {
		receiverMetrics := c.messageMetrics.ByReceiver[receiver]
		receiverMetrics.ReceiverAddress = receiver
		receiverMetrics.MessagesReceived++
		if !success {
			receiverMetrics.MessagesFailed++
		}
		receiverMetrics.LastReceivedTime = time.Now()
		c.messageMetrics.ByReceiver[receiver] = receiverMetrics
	}

	c.notifyMetricsUpdated()
}

// RecordMessageSent 记录消息发送
func (c *MetricsCollector) RecordMessageSent(sender, receiver, msgType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 更新发送者指标
	if sender != "" {
		senderMetrics := c.messageMetrics.BySender[sender]
		senderMetrics.SenderAddress = sender
		senderMetrics.MessagesSent++
		senderMetrics.LastSentTime = time.Now()
		c.messageMetrics.BySender[sender] = senderMetrics
	}

	// 更新接收者指标
	if receiver != "" {
		receiverMetrics := c.messageMetrics.ByReceiver[receiver]
		receiverMetrics.ReceiverAddress = receiver
		receiverMetrics.MessagesReceived++
		receiverMetrics.LastReceivedTime = time.Now()
		c.messageMetrics.ByReceiver[receiver] = receiverMetrics
	}

	c.notifyMetricsUpdated()
}

// UpdateActorHealth 更新actor健康状态
func (c *MetricsCollector) UpdateActorHealth(address string, health actor.HealthStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if metrics, exists := c.actorMetrics[address]; exists {
		oldHealth := metrics.HealthStatus
		metrics.HealthStatus = health
		c.actorMetrics[address] = metrics

		// 如果健康状态变差，更新系统失败actor计数
		if oldHealth == actor.HealthStatusHealthy && health != actor.HealthStatusHealthy {
			c.systemMetrics.FailedActors++
		} else if oldHealth != actor.HealthStatusHealthy && health == actor.HealthStatusHealthy {
			if c.systemMetrics.FailedActors > 0 {
				c.systemMetrics.FailedActors--
			}
		}

		c.notifyActorMetricsUpdated(address, metrics)
	}
}

// UpdateActorState 更新actor状态
func (c *MetricsCollector) UpdateActorState(address string, state actor.State) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if metrics, exists := c.actorMetrics[address]; exists {
		metrics.State = state
		c.actorMetrics[address] = metrics

		c.notifyActorMetricsUpdated(address, metrics)
	}
}

// GetSystemMetrics 获取系统指标
func (c *MetricsCollector) GetSystemMetrics() SystemMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.systemMetrics
}

// GetActorMetrics 获取actor指标
func (c *MetricsCollector) GetActorMetrics(address string) (ActorMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.actorMetrics[address]
	return metrics, exists
}

// GetAllActorMetrics 获取所有actor指标
func (c *MetricsCollector) GetAllActorMetrics() []ActorMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]ActorMetrics, 0, len(c.actorMetrics))
	for _, m := range c.actorMetrics {
		metrics = append(metrics, m)
	}

	return metrics
}

// GetMessageTypeMetrics 获取消息类型指标
func (c *MetricsCollector) GetMessageTypeMetrics(msgType string) (MessageTypeMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.messageMetrics.ByType[msgType]
	return metrics, exists
}

// GetAllMessageTypeMetrics 获取所有消息类型指标
func (c *MetricsCollector) GetAllMessageTypeMetrics() []MessageTypeMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]MessageTypeMetrics, 0, len(c.messageMetrics.ByType))
	for _, m := range c.messageMetrics.ByType {
		metrics = append(metrics, m)
	}

	return metrics
}

// GetSenderMetrics 获取发送者指标
func (c *MetricsCollector) GetSenderMetrics(sender string) (SenderMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.messageMetrics.BySender[sender]
	return metrics, exists
}

// GetReceiverMetrics 获取接收者指标
func (c *MetricsCollector) GetReceiverMetrics(receiver string) (ReceiverMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.messageMetrics.ByReceiver[receiver]
	return metrics, exists
}

// CreateWindow 创建时间窗口
func (c *MetricsCollector) CreateWindow(name string, windowSize time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.windowMetrics[name] = WindowMetrics{
		WindowName: name,
		WindowSize: windowSize,
		StartTime:  now,
		EndTime:    now.Add(windowSize),
		Metrics:    c.systemMetrics,
	}
}

// UpdateWindow 更新时间窗口
func (c *MetricsCollector) UpdateWindow(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	window, exists := c.windowMetrics[name]
	if !exists {
		return fmt.Errorf("window %s not found", name)
	}

	now := time.Now()
	if now.After(window.EndTime) {
		// 窗口已过期，重置
		window.StartTime = now
		window.EndTime = now.Add(window.WindowSize)
		window.Metrics = SystemMetrics{}
	}

	// 更新窗口指标
	window.Metrics = c.systemMetrics
	c.windowMetrics[name] = window

	return nil
}

// GetWindowMetrics 获取时间窗口指标
func (c *MetricsCollector) GetWindowMetrics(name string) (WindowMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.windowMetrics[name]
	return metrics, exists
}

// RegisterEventHandler 注册事件处理器
func (c *MetricsCollector) RegisterEventHandler(handler MetricsEventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventHandlers = append(c.eventHandlers, handler)
}

// UnregisterEventHandler 取消注册事件处理器
func (c *MetricsCollector) UnregisterEventHandler(handler MetricsEventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, h := range c.eventHandlers {
		if h == handler {
			c.eventHandlers = append(c.eventHandlers[:i], c.eventHandlers[i+1:]...)
			break
		}
	}
}

// Clear 清空所有指标
func (c *MetricsCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemMetrics = SystemMetrics{}
	c.actorMetrics = make(map[string]ActorMetrics)
	c.messageMetrics = MessageMetrics{
		ByType:     make(map[string]MessageTypeMetrics),
		BySender:   make(map[string]SenderMetrics),
		ByReceiver: make(map[string]ReceiverMetrics),
	}
	c.windowMetrics = make(map[string]WindowMetrics)

	c.notifyMetricsUpdated()
}

// notifyMetricsUpdated 通知指标更新
func (c *MetricsCollector) notifyMetricsUpdated() {
	for _, handler := range c.eventHandlers {
		handler.OnMetricsUpdated(c)
	}
}

// notifyActorMetricsUpdated 通知actor指标更新
func (c *MetricsCollector) notifyActorMetricsUpdated(address string, metrics ActorMetrics) {
	for _, handler := range c.eventHandlers {
		handler.OnActorMetricsUpdated(address, metrics)
	}
}

// notifyMessageMetricsUpdated 通知消息指标更新
func (c *MetricsCollector) notifyMessageMetricsUpdated(msgType string, metrics MessageTypeMetrics) {
	for _, handler := range c.eventHandlers {
		handler.OnMessageMetricsUpdated(msgType, metrics)
	}
}

// MetricsMonitor 实现actor.Monitor接口的指标监控器
type MetricsMonitor struct {
	collector *MetricsCollector
}

// NewMetricsMonitor 创建新的指标监控器
func NewMetricsMonitor(collector *MetricsCollector) *MetricsMonitor {
	return &MetricsMonitor{
		collector: collector,
	}
}

// HandleFailure 处理actor失败
func (m *MetricsMonitor) HandleFailure(ctx *actor.ActorContext, failedActor actor.Actor, err error) actor.SupervisionStrategy {
	address := failedActor.Address().String()
	m.collector.RecordActorFailure(address)
	return ctx.SupervisionStrategy()
}

// OnActorStarted actor启动时调用
func (m *MetricsMonitor) OnActorStarted(ctx *actor.ActorContext) {
	address := ctx.Address().String()
	actorType := getActorType(ctx.Actor())
	m.collector.RecordActorStart(address, actorType)
}

// OnActorStopped actor停止时调用
func (m *MetricsMonitor) OnActorStopped(ctx *actor.ActorContext) {
	address := ctx.Address().String()
	m.collector.RecordActorStop(address)
}

// OnMessageProcessed 消息处理完成时调用
func (m *MetricsMonitor) OnMessageProcessed(ctx *actor.ActorContext, envelope *message.Envelope, err error) {
	address := ctx.Address().String()
	msgType := envelope.Message().Type()
	sender := ""
	if envelope.Message().Sender() != nil {
		sender = envelope.Message().Sender().String()
	}
	receiver := ""
	if envelope.Message().Receiver() != nil {
		receiver = envelope.Message().Receiver().String()
	}

	// 这里我们无法获取处理时间，所以使用0
	processingTime := time.Duration(0)
	success := err == nil

	m.collector.RecordMessageProcessed(address, msgType, sender, receiver, processingTime, success)
}

// 辅助函数：获取actor类型
func getActorType(actor actor.Actor) string {
	if actor == nil {
		return "unknown"
	}

	// 使用反射获取类型名
	t := reflect.TypeOf(actor)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t.Name()
}

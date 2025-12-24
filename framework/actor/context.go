package actor

import (
	"context"
	"sync"

	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// Monitor 监控器接口，用于监控actor的行为和失败
type Monitor interface {
	// HandleFailure 处理actor失败
	HandleFailure(ctx *ActorContext, failedActor Actor, err error) SupervisionStrategy
	// OnActorStarted actor启动时调用
	OnActorStarted(ctx *ActorContext)
	// OnActorStopped actor停止时调用
	OnActorStopped(ctx *ActorContext)
	// OnMessageProcessed 消息处理完成时调用
	OnMessageProcessed(ctx *ActorContext, envelope *message.Envelope, err error)
}

// ActorContext actor上下文，提供actor的运行时环境
type ActorContext struct {
	// actor自身
	actor Actor

	// 父actor（如果有）
	parent Actor

	// 子actor映射
	children   map[string]Actor
	childrenMu sync.RWMutex

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 监控器
	monitor Monitor

	// 监督策略
	supervisionStrategy SupervisionStrategy

	// 统计信息
	stats ActorStats

	// 自定义数据
	data   map[string]interface{}
	dataMu sync.RWMutex
}

// NewActorContext 创建新的actor上下文
func NewActorContext(actor Actor, parent Actor) *ActorContext {
	ctx, cancel := context.WithCancel(context.Background())

	return &ActorContext{
		actor:               actor,
		parent:              parent,
		children:            make(map[string]Actor),
		ctx:                 ctx,
		cancel:              cancel,
		supervisionStrategy: SupervisionStrategyRestart,
		stats:               ActorStats{},
		data:                make(map[string]interface{}),
	}
}

// Actor 返回上下文关联的actor
func (c *ActorContext) Actor() Actor {
	return c.actor
}

// Parent 返回父actor
func (c *ActorContext) Parent() Actor {
	return c.parent
}

// Address 返回actor地址
func (c *ActorContext) Address() message.Address {
	return c.actor.Address()
}

// Mailbox 返回actor邮箱
func (c *ActorContext) Mailbox() mailbox.Mailbox {
	return c.actor.Mailbox()
}

// Context 返回上下文
func (c *ActorContext) Context() context.Context {
	return c.ctx
}

// Cancel 取消上下文
func (c *ActorContext) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

// AddChild 添加子actor
func (c *ActorContext) AddChild(child Actor) error {
	if child == nil {
		return ErrInvalidActor
	}

	addr := child.Address()
	if addr == nil {
		return ErrInvalidAddress
	}

	addrStr := addr.String()

	c.childrenMu.Lock()
	defer c.childrenMu.Unlock()

	if _, exists := c.children[addrStr]; exists {
		return ErrActorAlreadyRegistered
	}

	c.children[addrStr] = child
	return nil
}

// RemoveChild 移除子actor
func (c *ActorContext) RemoveChild(address message.Address) error {
	if address == nil {
		return ErrInvalidAddress
	}

	addrStr := address.String()

	c.childrenMu.Lock()
	defer c.childrenMu.Unlock()

	if _, exists := c.children[addrStr]; !exists {
		return ErrActorNotFound
	}

	delete(c.children, addrStr)
	return nil
}

// GetChild 获取子actor
func (c *ActorContext) GetChild(address message.Address) (Actor, error) {
	if address == nil {
		return nil, ErrInvalidAddress
	}

	addrStr := address.String()

	c.childrenMu.RLock()
	defer c.childrenMu.RUnlock()

	child, exists := c.children[addrStr]
	if !exists {
		return nil, ErrActorNotFound
	}

	return child, nil
}

// Children 返回所有子actor
func (c *ActorContext) Children() []Actor {
	c.childrenMu.RLock()
	defer c.childrenMu.RUnlock()

	children := make([]Actor, 0, len(c.children))
	for _, child := range c.children {
		children = append(children, child)
	}

	return children
}

// SetMonitor 设置监控器
func (c *ActorContext) SetMonitor(monitor Monitor) {
	c.monitor = monitor
}

// Monitor 返回监控器
func (c *ActorContext) Monitor() Monitor {
	return c.monitor
}

// SetSupervisionStrategy 设置监督策略
func (c *ActorContext) SetSupervisionStrategy(strategy SupervisionStrategy) {
	c.supervisionStrategy = strategy
}

// SupervisionStrategy 返回监督策略
func (c *ActorContext) SupervisionStrategy() SupervisionStrategy {
	return c.supervisionStrategy
}

// Stats 返回统计信息
func (c *ActorContext) Stats() ActorStats {
	return c.stats
}

// UpdateStats 更新统计信息
func (c *ActorContext) UpdateStats(updater func(*ActorStats)) {
	updater(&c.stats)
}

// SetData 设置自定义数据
func (c *ActorContext) SetData(key string, value interface{}) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	c.data[key] = value
}

// GetData 获取自定义数据
func (c *ActorContext) GetData(key string) (interface{}, bool) {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()

	value, exists := c.data[key]
	return value, exists
}

// DeleteData 删除自定义数据
func (c *ActorContext) DeleteData(key string) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	delete(c.data, key)
}

// Send 发送消息到另一个actor
func (c *ActorContext) Send(ctx context.Context, receiver message.Address, msg message.Message) error {
	return c.actor.(interface {
		Send(ctx context.Context, receiver message.Address, msg message.Message) error
	}).Send(ctx, receiver, msg)
}

// Reply 回复消息
func (c *ActorContext) Reply(ctx context.Context, originalMsg message.Message, replyMsg message.Message) error {
	return c.actor.(interface {
		Reply(ctx context.Context, originalMsg message.Message, replyMsg message.Message) error
	}).Reply(ctx, originalMsg, replyMsg)
}

// StartChild 启动子actor
func (c *ActorContext) StartChild(ctx context.Context, child Actor) error {
	if err := c.AddChild(child); err != nil {
		return err
	}

	return child.Start(ctx)
}

// StopChild 停止子actor
func (c *ActorContext) StopChild(address message.Address) error {
	child, err := c.GetChild(address)
	if err != nil {
		return err
	}

	if err := child.Stop(); err != nil {
		return err
	}

	return c.RemoveChild(address)
}

// StopAllChildren 停止所有子actor
func (c *ActorContext) StopAllChildren() error {
	children := c.Children()

	var firstErr error
	for _, child := range children {
		if err := child.Stop(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// HandleFailure 处理失败
func (c *ActorContext) HandleFailure(child Actor, err error) SupervisionStrategy {
	if c.monitor != nil {
		return c.monitor.HandleFailure(c, child, err)
	}

	return c.supervisionStrategy
}

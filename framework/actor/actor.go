package actor

import (
	"context"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// HealthStatus 健康状态
type HealthStatus int

const (
	// HealthStatusHealthy 健康
	HealthStatusHealthy HealthStatus = iota
	// HealthStatusDegraded 降级
	HealthStatusDegraded
	// HealthStatusUnhealthy 不健康
	HealthStatusUnhealthy
	// HealthStatusUnknown 未知
	HealthStatusUnknown
)

// State actor状态
type State int

const (
	// StateCreated 已创建
	StateCreated State = iota
	// StateStarting 启动中
	StateStarting
	// StateRunning 运行中
	StateRunning
	// StateStopping 停止中
	StateStopping
	// StateStopped 已停止
	StateStopped
	// StateFailed 失败
	StateFailed
)

// ActorStats actor统计信息
type ActorStats struct {
	// MessagesProcessed 已处理消息数
	MessagesProcessed int64
	// MessagesFailed 处理失败消息数
	MessagesFailed int64
	// LastMessageTime 最后处理消息时间
	LastMessageTime time.Time
	// Uptime 运行时间
	Uptime time.Duration
	// RestartCount 重启次数
	RestartCount int
}

// SupervisionStrategy 监督策略
type SupervisionStrategy int

const (
	// SupervisionStrategyResume 恢复（继续处理下一条消息）
	SupervisionStrategyResume SupervisionStrategy = iota
	// SupervisionStrategyRestart 重启actor
	SupervisionStrategyRestart
	// SupervisionStrategyStop 停止actor
	SupervisionStrategyStop
	// SupervisionStrategyEscalate 升级（让父actor处理）
	SupervisionStrategyEscalate
)

// Actor 接口定义了一个actor的基本行为
type Actor interface {
	// Address 返回actor的地址
	Address() message.Address
	// Mailbox 返回actor的邮箱
	Mailbox() mailbox.Mailbox
	// Start 启动actor的消息处理循环
	Start(ctx context.Context) error
	// Stop 停止actor
	Stop() error
	// HandleMessage 处理单个消息（由消息处理循环调用）
	HandleMessage(ctx context.Context, envelope *message.Envelope) error
	// IsRunning 返回actor是否正在运行
	IsRunning() bool

	// 生命周期钩子
	// PreStart 在actor启动前调用
	PreStart(ctx context.Context) error
	// PostStart 在actor启动后调用
	PostStart(ctx context.Context) error
	// PreStop 在actor停止前调用
	PreStop(ctx context.Context) error
	// PostStop 在actor停止后调用
	PostStop(ctx context.Context) error

	// 状态管理
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) (HealthStatus, error)
	// GetState 获取actor状态
	GetState() State
	// SetState 设置actor状态
	SetState(state State) error
}

// BaseActor 基础actor实现
type BaseActor struct {
	address    message.Address
	mailbox    mailbox.Mailbox
	mailboxMgr mailbox.MailboxManager
	running    bool
	state      State
	mu         sync.RWMutex
	wg         sync.WaitGroup
	cancel     context.CancelFunc
	createdAt  time.Time
	startedAt  time.Time
	stoppedAt  time.Time
	stats      ActorStats
	context    *ActorContext
}

// NewBaseActor 创建新的基础actor
func NewBaseActor(address message.Address, mailboxMgr mailbox.MailboxManager) (*BaseActor, error) {
	return NewBaseActorWithParent(address, mailboxMgr, nil)
}

// NewBaseActorWithParent 创建新的基础actor（指定父actor）
func NewBaseActorWithParent(address message.Address, mailboxMgr mailbox.MailboxManager, parent Actor) (*BaseActor, error) {
	if address == nil {
		return nil, ErrInvalidAddress
	}

	if mailboxMgr == nil {
		mailboxMgr = mailbox.GetDefaultManager()
	}

	// 获取或创建邮箱
	mb, err := mailboxMgr.GetOrCreateMailbox(address, mailbox.DefaultConfig())
	if err != nil {
		return nil, err
	}

	now := time.Now()
	actor := &BaseActor{
		address:    address,
		mailbox:    mb,
		mailboxMgr: mailboxMgr,
		running:    false,
		state:      StateCreated,
		createdAt:  now,
		stats:      ActorStats{},
	}

	// 创建actor上下文
	actor.context = NewActorContext(actor, parent)

	return actor, nil
}

// Address 返回actor地址
func (a *BaseActor) Address() message.Address {
	return a.address
}

// Mailbox 返回actor邮箱
func (a *BaseActor) Mailbox() mailbox.Mailbox {
	return a.mailbox
}

// Context 返回actor上下文
func (a *BaseActor) Context() *ActorContext {
	return a.context
}

// Start 启动actor
func (a *BaseActor) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return ErrAlreadyRunning
	}

	// 调用PreStart钩子
	if err := a.PreStart(ctx); err != nil {
		return err
	}

	// 创建带取消的上下文
	actorCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running = true

	// 启动消息处理循环
	a.wg.Add(1)
	go a.messageLoop(actorCtx)

	// 调用PostStart钩子
	if err := a.PostStart(ctx); err != nil {
		// 如果PostStart失败，停止actor
		a.cancel()
		a.wg.Wait()
		a.running = false
		return err
	}

	return nil
}

// Stop 停止actor
func (a *BaseActor) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return ErrNotRunning
	}

	// 调用PreStop钩子
	if err := a.PreStop(context.Background()); err != nil {
		return err
	}

	// 取消上下文
	if a.cancel != nil {
		a.cancel()
	}

	// 等待消息循环结束
	a.wg.Wait()
	a.running = false

	// 调用PostStop钩子
	if err := a.PostStop(context.Background()); err != nil {
		return err
	}

	return nil
}

// IsRunning 返回是否正在运行
func (a *BaseActor) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// PreStart 在actor启动前调用
func (a *BaseActor) PreStart(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = StateStarting
	return nil
}

// PostStart 在actor启动后调用
func (a *BaseActor) PostStart(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = StateRunning
	a.startedAt = time.Now()
	return nil
}

// PreStop 在actor停止前调用
func (a *BaseActor) PreStop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = StateStopping
	return nil
}

// PostStop 在actor停止后调用
func (a *BaseActor) PostStop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = StateStopped
	a.stoppedAt = time.Now()
	return nil
}

// HealthCheck 健康检查
func (a *BaseActor) HealthCheck(ctx context.Context) (HealthStatus, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return HealthStatusUnhealthy, nil
	}

	// 检查邮箱状态
	if a.mailbox != nil {
		if a.mailbox.IsFull() {
			return HealthStatusDegraded, nil
		}
	}

	return HealthStatusHealthy, nil
}

// GetState 获取actor状态
func (a *BaseActor) GetState() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState 设置actor状态
func (a *BaseActor) SetState(state State) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 验证状态转换
	if !a.isValidStateTransition(a.state, state) {
		return &ActorError{"invalid state transition"}
	}

	a.state = state
	return nil
}

// isValidStateTransition 检查状态转换是否有效
func (a *BaseActor) isValidStateTransition(from, to State) bool {
	// 允许的状态转换
	transitions := map[State][]State{
		StateCreated:  {StateStarting, StateFailed},
		StateStarting: {StateRunning, StateFailed},
		StateRunning:  {StateStopping, StateFailed},
		StateStopping: {StateStopped, StateFailed},
		StateStopped:  {StateStarting, StateFailed},
		StateFailed:   {StateStarting},
	}

	allowed, exists := transitions[from]
	if !exists {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}

	return false
}

// messageLoop 消息处理循环
func (a *BaseActor) messageLoop(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 从邮箱获取消息
			envelope, err := a.mailbox.Pop(ctx)
			if err != nil {
				// 如果是上下文取消，退出循环
				if ctx.Err() != nil {
					return
				}
				// 其他错误，继续循环
				continue
			}

			// 处理消息
			if err := a.HandleMessage(ctx, envelope); err != nil {
				// 记录错误但继续处理
				// 实际应用中应该使用日志系统
			}
		}
	}
}

// HandleMessage 处理消息（基础实现，子类应该重写）
func (a *BaseActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	// 基础实现只是记录消息
	// 子类应该重写这个方法来实现具体的业务逻辑
	msg := envelope.Message()
	// 在实际应用中，这里应该使用日志系统
	// fmt.Printf("Actor %s received message: type=%s, sender=%v\n",
	// 	a.address.String(), msg.Type(), msg.Sender())

	// 更新统计信息
	a.mu.Lock()
	a.stats.MessagesProcessed++
	a.stats.LastMessageTime = time.Now()
	a.mu.Unlock()

	_ = msg // 避免未使用变量警告
	return nil
}

// Send 发送消息到另一个actor
func (a *BaseActor) Send(ctx context.Context, receiver message.Address, msg message.Message) error {
	if receiver == nil {
		return ErrInvalidAddress
	}

	if msg == nil {
		return ErrInvalidMessage
	}

	// 设置发送者和接收者
	msg.SetSender(a.address)
	msg.SetReceiver(receiver)

	// 使用默认投递服务
	deliveryService := mailbox.NewDeliveryService(a.mailboxMgr, message.GetDefaultRouter())
	return deliveryService.Deliver(ctx, msg)
}

// Reply 回复消息
func (a *BaseActor) Reply(ctx context.Context, originalMsg message.Message, replyMsg message.Message) error {
	if originalMsg == nil {
		return ErrInvalidMessage
	}

	sender := originalMsg.Sender()
	if sender == nil {
		return ErrNoSender
	}

	// 设置回复消息的发送者和接收者
	replyMsg.SetSender(a.address)
	replyMsg.SetReceiver(sender)

	// 使用默认投递服务
	deliveryService := mailbox.NewDeliveryService(a.mailboxMgr, message.GetDefaultRouter())
	return deliveryService.Deliver(ctx, replyMsg)
}

// ActorManager actor管理器
type ActorManager struct {
	actors map[string]Actor
	mu     sync.RWMutex
}

// NewActorManager 创建新的actor管理器
func NewActorManager() *ActorManager {
	return &ActorManager{
		actors: make(map[string]Actor),
	}
}

// Register 注册actor
func (m *ActorManager) Register(actor Actor) error {
	if actor == nil {
		return ErrInvalidActor
	}

	addr := actor.Address()
	if addr == nil {
		return ErrInvalidAddress
	}

	addrStr := addr.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.actors[addrStr]; exists {
		return ErrActorAlreadyRegistered
	}

	m.actors[addrStr] = actor
	return nil
}

// Get 获取actor
func (m *ActorManager) Get(address message.Address) (Actor, error) {
	if address == nil {
		return nil, ErrInvalidAddress
	}

	addrStr := address.String()

	m.mu.RLock()
	defer m.mu.RUnlock()

	actor, exists := m.actors[addrStr]
	if !exists {
		return nil, ErrActorNotFound
	}

	return actor, nil
}

// Remove 移除actor
func (m *ActorManager) Remove(address message.Address) error {
	if address == nil {
		return ErrInvalidAddress
	}

	addrStr := address.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.actors[addrStr]; !exists {
		return ErrActorNotFound
	}

	delete(m.actors, addrStr)
	return nil
}

// StartAll 启动所有actor
func (m *ActorManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	actors := make([]Actor, 0, len(m.actors))
	for _, actor := range m.actors {
		actors = append(actors, actor)
	}
	m.mu.RUnlock()

	var firstErr error
	for _, actor := range actors {
		if err := actor.Start(ctx); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			// 继续启动其他actor
		}
	}

	return firstErr
}

// StopAll 停止所有actor
func (m *ActorManager) StopAll() error {
	m.mu.RLock()
	actors := make([]Actor, 0, len(m.actors))
	for _, actor := range m.actors {
		actors = append(actors, actor)
	}
	m.mu.RUnlock()

	var firstErr error
	for _, actor := range actors {
		if err := actor.Stop(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			// 继续停止其他actor
		}
	}

	return firstErr
}

// 错误定义
var (
	ErrInvalidAddress         = &ActorError{"invalid address"}
	ErrInvalidMessage         = &ActorError{"invalid message"}
	ErrInvalidActor           = &ActorError{"invalid actor"}
	ErrAlreadyRunning         = &ActorError{"actor already running"}
	ErrNotRunning             = &ActorError{"actor not running"}
	ErrNoSender               = &ActorError{"message has no sender"}
	ErrActorAlreadyRegistered = &ActorError{"actor already registered"}
	ErrActorNotFound          = &ActorError{"actor not found"}
)

// ActorError actor错误
type ActorError struct {
	message string
}

func (e *ActorError) Error() string {
	return "actor error: " + e.message
}

// 全局默认actor管理器
var (
	defaultActorManager     *ActorManager
	defaultActorManagerOnce sync.Once
)

// GetDefaultActorManager 获取默认actor管理器
func GetDefaultActorManager() *ActorManager {
	defaultActorManagerOnce.Do(func() {
		defaultActorManager = NewActorManager()
	})
	return defaultActorManager
}

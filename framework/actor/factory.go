package actor

import (
	"context"
	"reflect"
	"sync"

	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// ActorFactory actor工厂接口
type ActorFactory interface {
	// Create 创建actor实例
	Create(address message.Address) (Actor, error)
	// CreateWithParent 创建actor实例（指定父actor）
	CreateWithParent(address message.Address, parent Actor) (Actor, error)
	// ActorType 返回actor类型
	ActorType() string
}

// BaseActorFactory 基础actor工厂
type BaseActorFactory struct {
	actorType string
	creator   func(address message.Address, parent Actor) (Actor, error)
}

// NewBaseActorFactory 创建新的基础actor工厂
func NewBaseActorFactory(actorType string, creator func(address message.Address, parent Actor) (Actor, error)) *BaseActorFactory {
	return &BaseActorFactory{
		actorType: actorType,
		creator:   creator,
	}
}

// Create 创建actor实例
func (f *BaseActorFactory) Create(address message.Address) (Actor, error) {
	return f.creator(address, nil)
}

// CreateWithParent 创建actor实例（指定父actor）
func (f *BaseActorFactory) CreateWithParent(address message.Address, parent Actor) (Actor, error) {
	return f.creator(address, parent)
}

// ActorType 返回actor类型
func (f *BaseActorFactory) ActorType() string {
	return f.actorType
}

// ActorRegistry actor注册表
type ActorRegistry struct {
	factories map[string]ActorFactory
	mu        sync.RWMutex
}

// NewActorRegistry 创建新的actor注册表
func NewActorRegistry() *ActorRegistry {
	return &ActorRegistry{
		factories: make(map[string]ActorFactory),
	}
}

// Register 注册actor工厂
func (r *ActorRegistry) Register(factory ActorFactory) error {
	if factory == nil {
		return ErrInvalidActor
	}

	actorType := factory.ActorType()
	if actorType == "" {
		return &ActorError{"actor type cannot be empty"}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[actorType]; exists {
		return &ActorError{"actor factory already registered: " + actorType}
	}

	r.factories[actorType] = factory
	return nil
}

// Unregister 取消注册actor工厂
func (r *ActorRegistry) Unregister(actorType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[actorType]; !exists {
		return &ActorError{"actor factory not found: " + actorType}
	}

	delete(r.factories, actorType)
	return nil
}

// Create 创建actor
func (r *ActorRegistry) Create(actorType string, address message.Address) (Actor, error) {
	r.mu.RLock()
	factory, exists := r.factories[actorType]
	r.mu.RUnlock()

	if !exists {
		return nil, &ActorError{"actor factory not found: " + actorType}
	}

	return factory.Create(address)
}

// CreateWithParent 创建actor（指定父actor）
func (r *ActorRegistry) CreateWithParent(actorType string, address message.Address, parent Actor) (Actor, error) {
	r.mu.RLock()
	factory, exists := r.factories[actorType]
	r.mu.RUnlock()

	if !exists {
		return nil, &ActorError{"actor factory not found: " + actorType}
	}

	return factory.CreateWithParent(address, parent)
}

// HasFactory 检查是否有指定类型的工厂
func (r *ActorRegistry) HasFactory(actorType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[actorType]
	return exists
}

// FactoryTypes 返回所有已注册的工厂类型
func (r *ActorRegistry) FactoryTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.factories))
	for actorType := range r.factories {
		types = append(types, actorType)
	}

	return types
}

// ActorBuilder actor构建器
type ActorBuilder struct {
	actorType   string
	address     message.Address
	parent      Actor
	mailboxMgr  mailbox.MailboxManager
	mailboxConf mailbox.Config
	contextData map[string]interface{}
	supervision SupervisionStrategy
	monitor     Monitor
}

// NewActorBuilder 创建新的actor构建器
func NewActorBuilder(actorType string) *ActorBuilder {
	return &ActorBuilder{
		actorType:   actorType,
		mailboxConf: mailbox.DefaultConfig(),
		contextData: make(map[string]interface{}),
		supervision: SupervisionStrategyRestart,
	}
}

// WithAddress 设置地址
func (b *ActorBuilder) WithAddress(address message.Address) *ActorBuilder {
	b.address = address
	return b
}

// WithParent 设置父actor
func (b *ActorBuilder) WithParent(parent Actor) *ActorBuilder {
	b.parent = parent
	return b
}

// WithMailboxManager 设置邮箱管理器
func (b *ActorBuilder) WithMailboxManager(mgr mailbox.MailboxManager) *ActorBuilder {
	b.mailboxMgr = mgr
	return b
}

// WithMailboxConfig 设置邮箱配置
func (b *ActorBuilder) WithMailboxConfig(config mailbox.Config) *ActorBuilder {
	b.mailboxConf = config
	return b
}

// WithContextData 设置上下文数据
func (b *ActorBuilder) WithContextData(key string, value interface{}) *ActorBuilder {
	b.contextData[key] = value
	return b
}

// WithSupervisionStrategy 设置监督策略
func (b *ActorBuilder) WithSupervisionStrategy(strategy SupervisionStrategy) *ActorBuilder {
	b.supervision = strategy
	return b
}

// WithMonitor 设置监控器
func (b *ActorBuilder) WithMonitor(monitor Monitor) *ActorBuilder {
	b.monitor = monitor
	return b
}

// Build 构建actor（使用默认注册表）
func (b *ActorBuilder) Build() (Actor, error) {
	return b.BuildWithRegistry(GetDefaultActorRegistry())
}

// BuildWithRegistry 构建actor（使用指定注册表）
func (b *ActorBuilder) BuildWithRegistry(registry *ActorRegistry) (Actor, error) {
	if b.address == nil {
		return nil, ErrInvalidAddress
	}

	actor, err := registry.CreateWithParent(b.actorType, b.address, b.parent)
	if err != nil {
		return nil, err
	}

	// 如果是BaseActor，可以配置更多选项
	if baseActor, ok := actor.(*BaseActor); ok {
		// 配置邮箱管理器
		if b.mailboxMgr != nil {
			// 重新创建邮箱
			mb, err := b.mailboxMgr.GetOrCreateMailbox(b.address, b.mailboxConf)
			if err != nil {
				return nil, err
			}
			baseActor.mailbox = mb
			baseActor.mailboxMgr = b.mailboxMgr
		}

		// 配置上下文
		if baseActor.context != nil {
			baseActor.context.SetSupervisionStrategy(b.supervision)
			baseActor.context.SetMonitor(b.monitor)

			for key, value := range b.contextData {
				baseActor.context.SetData(key, value)
			}
		}
	}

	return actor, nil
}

// ActorSystem actor系统
type ActorSystem struct {
	registry   *ActorRegistry
	manager    *ActorManager
	mailboxMgr mailbox.MailboxManager
	mu         sync.RWMutex
}

// NewActorSystem 创建新的actor系统
func NewActorSystem() *ActorSystem {
	return &ActorSystem{
		registry:   NewActorRegistry(),
		manager:    NewActorManager(),
		mailboxMgr: mailbox.GetDefaultManager(),
	}
}

// Registry 返回actor注册表
func (s *ActorSystem) Registry() *ActorRegistry {
	return s.registry
}

// Manager 返回actor管理器
func (s *ActorSystem) Manager() *ActorManager {
	return s.manager
}

// MailboxManager 返回邮箱管理器
func (s *ActorSystem) MailboxManager() mailbox.MailboxManager {
	return s.mailboxMgr
}

// SetMailboxManager 设置邮箱管理器
func (s *ActorSystem) SetMailboxManager(mgr mailbox.MailboxManager) {
	s.mailboxMgr = mgr
}

// CreateActor 创建actor
func (s *ActorSystem) CreateActor(actorType string, address message.Address) (Actor, error) {
	actor, err := s.registry.Create(actorType, address)
	if err != nil {
		return nil, err
	}

	// 注册到管理器
	if err := s.manager.Register(actor); err != nil {
		return nil, err
	}

	return actor, nil
}

// CreateActorWithParent 创建actor（指定父actor）
func (s *ActorSystem) CreateActorWithParent(actorType string, address message.Address, parent Actor) (Actor, error) {
	actor, err := s.registry.CreateWithParent(actorType, address, parent)
	if err != nil {
		return nil, err
	}

	// 注册到管理器
	if err := s.manager.Register(actor); err != nil {
		return nil, err
	}

	// 如果是子actor，添加到父actor的上下文中
	if parent != nil {
		if parentCtx, ok := parent.(interface{ Context() *ActorContext }); ok {
			if err := parentCtx.Context().AddChild(actor); err != nil {
				// 如果添加失败，从管理器中移除
				s.manager.Remove(address)
				return nil, err
			}
		}
	}

	return actor, nil
}

// StartActor 创建并启动actor
func (s *ActorSystem) StartActor(ctx context.Context, actorType string, address message.Address) (Actor, error) {
	actor, err := s.CreateActor(actorType, address)
	if err != nil {
		return nil, err
	}

	if err := actor.Start(ctx); err != nil {
		// 如果启动失败，从管理器中移除
		s.manager.Remove(address)
		return nil, err
	}

	return actor, nil
}

// StopActor 停止actor
func (s *ActorSystem) StopActor(address message.Address) error {
	actor, err := s.manager.Get(address)
	if err != nil {
		return err
	}

	if err := actor.Stop(); err != nil {
		return err
	}

	return s.manager.Remove(address)
}

// StopAllActors 停止所有actor
func (s *ActorSystem) StopAllActors() error {
	return s.manager.StopAll()
}

// 全局默认actor注册表
var (
	defaultActorRegistry     *ActorRegistry
	defaultActorRegistryOnce sync.Once
)

// GetDefaultActorRegistry 获取默认actor注册表
func GetDefaultActorRegistry() *ActorRegistry {
	defaultActorRegistryOnce.Do(func() {
		defaultActorRegistry = NewActorRegistry()

		// 注册基础actor工厂
		defaultActorRegistry.Register(NewBaseActorFactory("base", func(address message.Address, parent Actor) (Actor, error) {
			return NewBaseActorWithParent(address, nil, parent)
		}))
	})
	return defaultActorRegistry
}

// 反射工厂：通过反射创建指定类型的actor
type ReflectActorFactory struct {
	actorType reflect.Type
}

// NewReflectActorFactory 创建新的反射actor工厂
func NewReflectActorFactory(actorType reflect.Type) *ReflectActorFactory {
	return &ReflectActorFactory{
		actorType: actorType,
	}
}

// Create 创建actor实例
func (f *ReflectActorFactory) Create(address message.Address) (Actor, error) {
	return f.create(address, nil)
}

// CreateWithParent 创建actor实例（指定父actor）
func (f *ReflectActorFactory) CreateWithParent(address message.Address, parent Actor) (Actor, error) {
	return f.create(address, parent)
}

func (f *ReflectActorFactory) create(address message.Address, parent Actor) (Actor, error) {
	// 检查类型是否实现了Actor接口
	actorInterface := reflect.TypeOf((*Actor)(nil)).Elem()
	if !f.actorType.Implements(actorInterface) {
		return nil, &ActorError{"type does not implement Actor interface"}
	}

	// 创建实例
	value := reflect.New(f.actorType)

	// 调用构造函数（如果存在）
	ctor := value.MethodByName("New")
	if ctor.IsValid() {
		// 尝试调用构造函数
		args := []reflect.Value{
			reflect.ValueOf(address),
			reflect.ValueOf(parent),
		}
		results := ctor.Call(args)

		if len(results) == 2 {
			// 检查错误
			if err, ok := results[1].Interface().(error); ok && err != nil {
				return nil, err
			}

			if actor, ok := results[0].Interface().(Actor); ok {
				return actor, nil
			}
		}
	}

	// 如果没有构造函数或构造函数失败，尝试直接转换
	if actor, ok := value.Interface().(Actor); ok {
		return actor, nil
	}

	return nil, &ActorError{"failed to create actor instance"}
}

// ActorType 返回actor类型
func (f *ReflectActorFactory) ActorType() string {
	return f.actorType.String()
}

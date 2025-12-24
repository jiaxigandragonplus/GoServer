# Actor 系统框架

基于Actor模型的并发编程框架，提供完整的Actor生命周期管理、消息传递、监控和错误恢复机制。

## 核心概念

### Actor
Actor是并发计算的基本单元，具有以下特性：
- 封装状态和行为
- 通过消息进行通信
- 独立的执行上下文
- 父子监督关系

### 消息
- 类型化的数据包
- 包含发送者、接收者、优先级等元数据
- 支持TTL和重试机制

### 邮箱
- 消息缓冲区
- 支持有界/无界队列
- 多种丢弃策略
- 死信队列支持

## 核心组件

### 1. Actor接口 (`actor.go`)
```go
type Actor interface {
    Address() message.Address
    Mailbox() mailbox.Mailbox
    Start(ctx context.Context) error
    Stop() error
    HandleMessage(ctx context.Context, envelope *message.Envelope) error
    IsRunning() bool
    
    // 生命周期钩子
    PreStart(ctx context.Context) error
    PostStart(ctx context.Context) error
    PreStop(ctx context.Context) error
    PostStop(ctx context.Context) error
    
    // 状态管理
    HealthCheck(ctx context.Context) (HealthStatus, error)
    GetState() State
    SetState(state State) error
}
```

### 2. BaseActor基类
提供Actor接口的基础实现，包含：
- 地址和邮箱管理
- 消息处理循环
- 状态跟踪
- 统计信息收集

### 3. ActorContext上下文 (`context.go`)
提供Actor的运行时环境：
- 父子Actor关系管理
- 监控器集成
- 监督策略配置
- 自定义数据存储

### 4. ActorFactory工厂 (`factory.go`)
- 类型化Actor创建
- 注册表管理
- 构建器模式支持
- 反射工厂

### 5. 监控系统 (`monitor.go`)
- 默认监控器实现
- 统计信息收集
- 失败记录
- 事件处理器

### 6. 错误恢复 (`recovery.go`)
- 错误处理器
- 重试机制
- 断路器模式
- 监督者策略

## 使用示例

### 创建自定义Actor
```go
type MyActor struct {
    *actor.BaseActor
    counter int
}

func NewMyActor(address message.Address) (*MyActor, error) {
    baseActor, err := actor.NewBaseActor(address, nil)
    if err != nil {
        return nil, err
    }
    return &MyActor{BaseActor: baseActor}, nil
}

func (a *MyActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
    msg := envelope.Message()
    // 处理消息逻辑
    return nil
}
```

### 使用Actor系统
```go
// 创建Actor系统
system := actor.NewActorSystem()

// 注册Actor工厂
factory := actor.NewBaseActorFactory("myactor", 
    func(address message.Address, parent actor.Actor) (actor.Actor, error) {
        return NewMyActor(address)
    })
system.Registry().Register(factory)

// 创建并启动Actor
myActor, err := system.CreateActor("myactor", address)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
if err := myActor.Start(ctx); err != nil {
    log.Fatal(err)
}

// 发送消息
msg := message.NewBaseMessage("test", "hello")
err = myActor.Send(ctx, address, msg)

// 停止Actor
myActor.Stop()
```

### 使用ActorBuilder
```go
actor, err := actor.NewActorBuilder("myactor").
    WithAddress(address).
    WithSupervisionStrategy(actor.SupervisionStrategyRestart).
    WithMonitor(monitor).
    Build()
```

## 生命周期管理

Actor支持完整的生命周期钩子：
1. `PreStart()` - 启动前准备
2. `PostStart()` - 启动后初始化
3. `PreStop()` - 停止前清理
4. `PostStop()` - 停止后收尾

## 状态管理

### Actor状态
- `StateCreated` - 已创建
- `StateStarting` - 启动中
- `StateRunning` - 运行中
- `StateStopping` - 停止中
- `StateStopped` - 已停止
- `StateFailed` - 失败

### 健康状态
- `HealthStatusHealthy` - 健康
- `HealthStatusDegraded` - 降级
- `HealthStatusUnhealthy` - 不健康
- `HealthStatusUnknown` - 未知

## 监督策略

### 策略类型
1. `SupervisionStrategyResume` - 恢复（继续处理下一条消息）
2. `SupervisionStrategyRestart` - 重启Actor
3. `SupervisionStrategyStop` - 停止Actor
4. `SupervisionStrategyEscalate` - 升级（让父Actor处理）

## 错误处理

### 重试机制
- 可配置的最大重试次数
- 指数退避延迟
- 错误类型识别

### 断路器模式
- 闭合/断开/半开状态
- 失败阈值检测
- 自动恢复机制

## 监控和统计

### 监控器功能
- Actor启动/停止事件
- 消息处理统计
- 失败记录
- 健康状态检查

### 状态管理器
- 实时状态跟踪
- 健康状态聚合
- 历史状态查询

## 配置选项

### 邮箱配置
```go
config := mailbox.Config{
    Capacity:       1000,
    DropPolicy:     mailbox.DropPolicyReject,
    PriorityEnabled: true,
    TTLEnabled:     true,
}
```

### Actor配置
- 监督策略
- 最大重启次数
- 监控器设置
- 上下文数据

## 最佳实践

1. **保持Actor无状态**：尽可能将状态外部化
2. **使用类型化消息**：明确消息契约
3. **合理设置邮箱容量**：避免内存泄漏
4. **实现健康检查**：便于系统监控
5. **使用监督策略**：提高系统韧性
6. **集成监控**：便于问题排查

## 扩展点

### 自定义监控器
实现`Monitor`接口来集成外部监控系统。

### 自定义错误处理器
实现`ErrorHandler`接口来定义特定的错误处理逻辑。

### 自定义邮箱
实现`Mailbox`接口来支持特殊的消息队列需求。

## 性能考虑

1. **邮箱容量**：根据消息吞吐量合理设置
2. **Actor数量**：避免创建过多Actor导致调度开销
3. **消息大小**：控制消息体积，避免大对象传输
4. **监控开销**：在生产环境中合理配置监控频率

## 相关模块

- `framework/actor/message` - 消息系统
- `framework/actor/mailbox` - 邮箱管理
- `framework/logger` - 日志系统（可选集成）

## 示例代码

参考 `framework/actor/example/` 目录：
- `example.go` - 基础使用示例
- `advanced_example.go` - 高级功能示例

## 许可证

MIT License
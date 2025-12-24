# Actor 容错机制

本模块为Actor系统提供了完整的错误处理和容错机制，包括监督策略、错误恢复、断路器模式和重试机制。

## 目录结构

```
framework/actor/fault_tolerance/
├── supervision.go      # 监督策略实现
├── recovery.go        # 错误恢复实现
├── circuit_breaker.go # 断路器模式实现
├── retry.go          # 重试机制实现
├── example/          # 示例代码
│   └── example.go    # 使用示例
└── README.md         # 本文档
```

## 功能概述

### 1. 监督策略 (Supervision)

监督策略定义了当子actor失败时，父actor应该采取的行动。

#### 支持的策略：
- **Resume**：继续处理下一条消息
- **Restart**：重启失败的actor
- **Stop**：停止失败的actor
- **Escalate**：将失败升级到父监督者

#### 监督者类型：
- **OneForOneSupervisor**：一对一监督（每个子actor独立监督）
- **OneForAllSupervisor**：一对所有监督（一个子actor失败影响所有子actor）
- **RestartingSupervisor**：重启监督者（专门用于重启策略）
- **StoppingSupervisor**：停止监督者（专门用于停止策略）

### 2. 错误恢复 (Recovery)

错误恢复机制提供了灵活的错误处理方式，支持不同类型的错误处理器。

#### 核心组件：
- **RecoveryManager**：恢复管理器，管理多个错误处理器
- **ErrorHandler**：错误处理器接口
- **DefaultErrorHandler**：默认错误处理器
- **LoggingErrorHandler**：日志错误处理器
- **CircuitBreakerErrorHandler**：断路器错误处理器
- **RetryErrorHandler**：重试错误处理器

### 3. 断路器模式 (Circuit Breaker)

断路器模式用于防止级联故障，当服务失败率达到阈值时自动断开。

#### 断路器状态：
- **Closed**：闭合状态（正常）
- **Open**：断开状态（停止服务）
- **HalfOpen**：半开状态（尝试恢复）

#### 断路器类型：
- **TimeWindowBreaker**：时间窗口断路器（基于时间窗口的失败计数）
- **CountBasedBreaker**：计数断路器（基于连续失败计数）
- **PercentageBreaker**：百分比断路器（基于失败百分比）

### 4. 重试机制 (Retry)

重试机制提供了多种重试策略，用于处理临时性故障。

#### 重试策略：
- **FixedRetryPolicy**：固定重试策略（固定延迟）
- **ExponentialBackoffRetryPolicy**：指数退避重试策略
- **FibonacciRetryPolicy**：斐波那契重试策略

#### 核心组件：
- **RetryExecutor**：重试执行器
- **ActorRetryHandler**：actor重试处理器
- **MessageRetryDecorator**：消息重试装饰器

## 快速开始

### 安装

```go
import "github.com/GooLuck/GoServer/framework/actor/fault_tolerance"
```

### 基本使用

#### 1. 创建监督者

```go
// 创建一对一监督者（重启策略）
supervisor := fault_tolerance.NewOneForOneSupervisor(
    fault_tolerance.SupervisionStrategyRestart,
)
supervisor.SetMaxRestarts(3, 1*time.Minute)

// 添加子actor
supervisor.AddChild(actor1)
supervisor.AddChild(actor2)

// 处理actor失败
err := supervisor.HandleFailure(failedActor, failureError)
```

#### 2. 使用断路器

```go
// 创建断路器
circuitBreaker := fault_tolerance.NewCircuitBreaker(3, 10*time.Second)

// 设置状态变更回调
circuitBreaker.SetOnStateChange(func(oldState, newState fault_tolerance.CircuitBreakerState) {
    fmt.Printf("断路器状态变更: %v -> %v\n", oldState, newState)
})

// 执行受保护的操作
err := circuitBreaker.Execute(func() error {
    return someOperation()
})
```

#### 3. 配置重试机制

```go
// 创建重试策略
retryPolicy := fault_tolerance.NewExponentialBackoffRetryPolicy(
    3,                    // 最大重试次数
    100*time.Millisecond, // 初始延迟
    5*time.Second,        // 最大延迟
)

// 创建重试执行器
retryExecutor := fault_tolerance.NewRetryExecutor(retryPolicy)

// 执行带重试的操作
ctx := context.Background()
err := retryExecutor.Execute(ctx, func() error {
    return someOperation()
})
```

#### 4. 错误恢复

```go
// 创建恢复管理器
recoveryManager := fault_tolerance.NewRecoveryManager()

// 注册自定义错误处理器
customHandler := fault_tolerance.NewDefaultErrorHandler()
recoveryManager.RegisterErrorHandler("CustomError", customHandler)

// 处理错误
err := recoveryManager.HandleError(actorCtx, envelope, someError)
```

## 高级配置

### 监督策略配置

```go
supervisor := fault_tolerance.NewOneForOneSupervisor(
    fault_tolerance.SupervisionStrategyRestart,
)

// 配置最大重启次数
supervisor.SetMaxRestarts(5, 30*time.Second)

// 设置监控器
supervisor.SetMonitor(actor.GetDefaultMonitor())
```

### 断路器配置

```go
circuitBreaker := fault_tolerance.NewCircuitBreaker(
    5,              // 失败阈值
    30*time.Second, // 时间窗口
)

// 配置半开状态超时
circuitBreaker.SetHalfOpenTimeout(10 * time.Second)

// 配置成功阈值
circuitBreaker.SetSuccessThreshold(2)
```

### 重试策略配置

```go
policy := fault_tolerance.NewExponentialBackoffRetryPolicy(
    5,                    // 最大重试次数
    200*time.Millisecond, // 初始延迟
    10*time.Second,       // 最大延迟
)

// 配置退避因子
policy.SetBackoffFactor(1.5)

// 配置抖动因子
policy.SetJitterFactor(0.2)

// 添加可重试错误类型
policy.AddRetryableError("NetworkError")
policy.AddRetryableError("TimeoutError")
```

## 集成示例

以下是一个完整的集成示例，展示了如何将各种容错机制组合使用：

```go
// 创建actor
worker, _ := NewExampleActor("worker")

// 创建断路器
circuitBreaker := fault_tolerance.NewCircuitBreaker(2, 5*time.Second)

// 创建重试策略
retryPolicy := fault_tolerance.NewFixedRetryPolicy(2, 1*time.Second)

// 创建消息重试装饰器
retryDecorator := fault_tolerance.NewMessageRetryDecorator(worker, retryPolicy)

// 创建监督者
supervisor := fault_tolerance.NewOneForOneSupervisor(
    fault_tolerance.SupervisionStrategyRestart,
)

// 添加actor到监督者
supervisor.AddChild(worker)

// 处理消息（带断路器和重试）
err := circuitBreaker.Execute(func() error {
    return retryDecorator.HandleMessage(ctx, envelope)
})

if err != nil {
    // 监督者处理失败
    supervisor.HandleFailure(worker, err)
}
```

## 最佳实践

### 1. 选择合适的监督策略
- 对于无状态服务：使用 **Restart** 策略
- 对于有状态服务：使用 **Resume** 或 **Stop** 策略
- 对于关键服务：使用 **Escalate** 策略

### 2. 合理配置断路器
- 根据服务特性设置合适的失败阈值
- 设置合理的半开状态超时时间
- 监控断路器状态变化

### 3. 优化重试策略
- 对于网络服务：使用指数退避策略
- 对于数据库操作：使用固定延迟策略
- 避免无限重试，设置合理的最大重试次数

### 4. 错误处理
- 区分可重试错误和不可重试错误
- 记录错误日志以便调试
- 提供有意义的错误信息

## API 参考

### 监督策略
- `NewOneForOneSupervisor(strategy SupervisionStrategy)`
- `NewOneForAllSupervisor(strategy SupervisionStrategy)`
- `NewRestartingSupervisor()`
- `NewStoppingSupervisor()`

### 断路器
- `NewCircuitBreaker(failureThreshold int, windowSize time.Duration)`
- `NewTimeWindowBreaker(failureThreshold int, windowSize time.Duration)`
- `NewCountBasedBreaker(maxConsecutiveFailures int)`
- `NewPercentageBreaker(maxFailurePercentage float64, windowSize time.Duration)`

### 重试机制
- `NewFixedRetryPolicy(maxAttempts int, delay time.Duration)`
- `NewExponentialBackoffRetryPolicy(maxAttempts int, initialDelay, maxDelay time.Duration)`
- `NewFibonacciRetryPolicy(maxAttempts int, initialDelay, maxDelay time.Duration)`
- `NewRetryExecutor(policy RetryPolicy)`

### 错误恢复
- `NewRecoveryManager()`
- `NewDefaultErrorHandler()`
- `NewLoggingErrorHandler()`
- `NewCircuitBreakerErrorHandler(circuitBreaker *CircuitBreaker)`
- `NewRetryErrorHandler(maxRetries int)`

## 运行示例

要运行示例代码，请执行：

```bash
cd framework/actor/fault_tolerance/example
go run example.go
```

## 贡献

欢迎提交问题和拉取请求来改进这个模块。

## 许可证

本项目使用MIT许可证。详情请参阅LICENSE文件。
package example

import (
	"context"
	"fmt"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/fault_tolerance"
	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// ExampleActor 示例actor
type ExampleActor struct {
	*actor.BaseActor
	name      string
	failCount int
}

// NewExampleActor 创建新的示例actor
func NewExampleActor(name string) (*ExampleActor, error) {
	addr, err := message.NewLocalAddress("local", "/example/"+name)
	if err != nil {
		return nil, err
	}

	baseActor, err := actor.NewBaseActor(addr, mailbox.GetDefaultManager())
	if err != nil {
		return nil, err
	}

	return &ExampleActor{
		BaseActor: baseActor,
		name:      name,
		failCount: 0,
	}, nil
}

// HandleMessage 处理消息
func (a *ExampleActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()
	fmt.Printf("Actor %s received message: type=%s\n", a.name, msg.Type())

	// 模拟失败：每3条消息失败一次
	a.failCount++
	if a.failCount%3 == 0 {
		return fmt.Errorf("simulated failure for actor %s", a.name)
	}

	// 模拟处理时间
	time.Sleep(50 * time.Millisecond)
	return nil
}

// ExampleSupervision 监督策略示例
func ExampleSupervision() {
	fmt.Println("=== 监督策略示例 ===")

	// 创建监督者
	supervisor := fault_tolerance.NewOneForOneSupervisor(fault_tolerance.SupervisionStrategyRestart)
	supervisor.SetMaxRestarts(3, 1*time.Minute)

	// 创建子actor
	actor1, _ := NewExampleActor("worker1")
	actor2, _ := NewExampleActor("worker2")

	// 添加子actor到监督者
	supervisor.AddChild(actor1)
	supervisor.AddChild(actor2)

	// 启动actor
	ctx := context.Background()
	actor1.Start(ctx)
	actor2.Start(ctx)

	// 模拟失败处理
	fmt.Println("模拟actor失败...")
	err := fmt.Errorf("test failure")
	supervisor.HandleFailure(actor1, err)

	// 获取重启信息
	if info, exists := supervisor.RestartInfo("actor://worker1/example"); exists {
		fmt.Printf("Actor worker1 重启次数: %d\n", info.Count)
	}

	// 停止actor
	actor1.Stop()
	actor2.Stop()

	fmt.Println()
}

// ExampleCircuitBreaker 断路器示例
func ExampleCircuitBreaker() {
	fmt.Println("=== 断路器示例 ===")

	// 创建断路器
	circuitBreaker := fault_tolerance.NewCircuitBreaker(3, 10*time.Second)

	// 设置状态变更回调
	circuitBreaker.SetOnStateChange(func(oldState, newState fault_tolerance.CircuitBreakerState) {
		states := []string{"Closed", "Open", "HalfOpen"}
		fmt.Printf("断路器状态变更: %s -> %s\n", states[oldState], states[newState])
	})

	// 模拟操作
	operation := func() error {
		// 模拟失败
		return fmt.Errorf("operation failed")
	}

	// 执行操作（受断路器保护）
	for i := 0; i < 5; i++ {
		err := circuitBreaker.Execute(operation)
		if err != nil {
			fmt.Printf("操作 %d 失败: %v\n", i+1, err)
		} else {
			fmt.Printf("操作 %d 成功\n", i+1)
		}
		time.Sleep(1 * time.Second)
	}

	// 重置断路器
	circuitBreaker.Reset()
	fmt.Println("断路器已重置")

	fmt.Println()
}

// ExampleRetry 重试机制示例
func ExampleRetry() {
	fmt.Println("=== 重试机制示例 ===")

	// 创建重试策略
	retryPolicy := fault_tolerance.NewExponentialBackoffRetryPolicy(
		3,                    // 最大重试次数
		100*time.Millisecond, // 初始延迟
		5*time.Second,        // 最大延迟
	)

	// 添加可重试错误类型
	retryPolicy.AddRetryableError("TemporaryError")

	// 创建重试执行器
	retryExecutor := fault_tolerance.NewRetryExecutor(retryPolicy)

	// 模拟操作（失败2次后成功）
	attempt := 0
	operation := func() error {
		attempt++
		fmt.Printf("尝试第 %d 次...\n", attempt)
		if attempt < 3 {
			return fmt.Errorf("temporary error")
		}
		fmt.Println("操作成功!")
		return nil
	}

	// 执行操作（带重试）
	ctx := context.Background()
	err := retryExecutor.Execute(ctx, operation)
	if err != nil {
		fmt.Printf("最终失败: %v\n", err)
	} else {
		fmt.Println("操作最终成功")
	}

	fmt.Println()
}

// ExampleRecovery 错误恢复示例
func ExampleRecovery() {
	fmt.Println("=== 错误恢复示例 ===")

	// 创建恢复管理器
	recoveryManager := fault_tolerance.NewRecoveryManager()

	// 创建自定义错误处理器（使用构造函数）
	customHandler := fault_tolerance.NewDefaultErrorHandler()

	// 注册错误处理器
	recoveryManager.RegisterErrorHandler("CustomError", customHandler)

	// 创建actor上下文（简化示例）
	actorCtx := &actor.ActorContext{}

	// 模拟消息
	testMsg := message.NewBaseMessage("test.message", nil)
	envelope := message.NewEnvelope(testMsg)

	// 处理错误
	testErr := fmt.Errorf("custom error")
	err := recoveryManager.HandleError(actorCtx, envelope, testErr)
	fmt.Printf("错误处理结果: %v\n", err)

	// 检查是否应该重试
	shouldRetry := recoveryManager.ShouldRetry(actorCtx, envelope, testErr)
	fmt.Printf("是否应该重试: %v\n", shouldRetry)

	// 获取重试延迟
	retryDelay := recoveryManager.GetRetryDelay(actorCtx, envelope, testErr)
	fmt.Printf("重试延迟: %v\n", retryDelay)

	fmt.Println()
}

// ExampleIntegration 集成示例
func ExampleIntegration() {
	fmt.Println("=== 集成示例 ===")

	// 创建actor
	worker, _ := NewExampleActor("integrated-worker")
	ctx := context.Background()
	worker.Start(ctx)

	// 创建断路器
	circuitBreaker := fault_tolerance.NewCircuitBreaker(2, 5*time.Second)

	// 创建重试策略
	retryPolicy := fault_tolerance.NewFixedRetryPolicy(2, 1*time.Second)

	// 创建消息重试装饰器
	retryDecorator := fault_tolerance.NewMessageRetryDecorator(worker, retryPolicy)

	// 创建监督者
	supervisor := fault_tolerance.NewOneForOneSupervisor(fault_tolerance.SupervisionStrategyRestart)

	// 添加actor到监督者
	supervisor.AddChild(worker)

	// 模拟消息处理
	testMsg := message.NewBaseMessage("test.integration", nil)
	envelope := message.NewEnvelope(testMsg)

	// 使用断路器保护的消息处理
	err := circuitBreaker.Execute(func() error {
		return retryDecorator.HandleMessage(ctx, envelope)
	})

	if err != nil {
		fmt.Printf("消息处理失败: %v\n", err)
		// 监督者处理失败
		supervisor.HandleFailure(worker, err)
	} else {
		fmt.Println("消息处理成功")
	}

	// 停止actor
	worker.Stop()

	fmt.Println()
}

// RunAllExamples 运行所有示例
func RunAllExamples() {
	fmt.Println("开始运行容错机制示例...")
	fmt.Println()

	ExampleSupervision()
	ExampleCircuitBreaker()
	ExampleRetry()
	ExampleRecovery()
	ExampleIntegration()

	fmt.Println("所有示例运行完成!")
}

// 主函数（用于测试）
func main() {
	RunAllExamples()
}

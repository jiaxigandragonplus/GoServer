package main

import (
	"context"
	"fmt"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/message"
)

// WorkerActor 工作actor，演示生命周期和错误处理
type WorkerActor struct {
	*actor.BaseActor
	workCount int
}

// NewWorkerActor 创建新的工作actor
func NewWorkerActor(address message.Address) (*WorkerActor, error) {
	baseActor, err := actor.NewBaseActor(address, nil)
	if err != nil {
		return nil, err
	}

	return &WorkerActor{
		BaseActor: baseActor,
		workCount: 0,
	}, nil
}

// PreStart 在actor启动前调用
func (a *WorkerActor) PreStart(ctx context.Context) error {
	fmt.Printf("WorkerActor %s: PreStart called\n", a.Address().String())
	return a.BaseActor.PreStart(ctx)
}

// PostStart 在actor启动后调用
func (a *WorkerActor) PostStart(ctx context.Context) error {
	fmt.Printf("WorkerActor %s: PostStart called\n", a.Address().String())
	return a.BaseActor.PostStart(ctx)
}

// PreStop 在actor停止前调用
func (a *WorkerActor) PreStop(ctx context.Context) error {
	fmt.Printf("WorkerActor %s: PreStop called, total work done: %d\n",
		a.Address().String(), a.workCount)
	return a.BaseActor.PreStop(ctx)
}

// PostStop 在actor停止后调用
func (a *WorkerActor) PostStop(ctx context.Context) error {
	fmt.Printf("WorkerActor %s: PostStop called\n", a.Address().String())
	return a.BaseActor.PostStop(ctx)
}

// HandleMessage 处理消息
func (a *WorkerActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()

	switch msg.Type() {
	case "work.do":
		a.workCount++
		fmt.Printf("WorkerActor %s: Doing work #%d\n", a.Address().String(), a.workCount)

		// 模拟工作
		time.Sleep(50 * time.Millisecond)

		// 回复结果
		replyMsg := message.NewBaseMessage("work.done", map[string]interface{}{
			"work_id":   a.workCount,
			"worker":    a.Address().String(),
			"timestamp": time.Now().Unix(),
		})
		return a.Reply(ctx, msg, replyMsg)

	case "work.status":
		replyMsg := message.NewBaseMessage("work.status.response", map[string]interface{}{
			"work_count": a.workCount,
			"is_running": a.IsRunning(),
			"state":      a.GetState(),
		})
		return a.Reply(ctx, msg, replyMsg)

	case "work.error":
		// 模拟错误
		return fmt.Errorf("simulated work error on work #%d", a.workCount+1)

	default:
		return fmt.Errorf("unknown message type: %s", msg.Type())
	}
}

// SupervisorActor 监督者actor，管理子actor
type SupervisorActor struct {
	*actor.BaseActor
}

// NewSupervisorActor 创建新的监督者actor
func NewSupervisorActor(address message.Address) (*SupervisorActor, error) {
	baseActor, err := actor.NewBaseActor(address, nil)
	if err != nil {
		return nil, err
	}

	return &SupervisorActor{
		BaseActor: baseActor,
	}, nil
}

// HandleMessage 处理消息
func (a *SupervisorActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()

	switch msg.Type() {
	case "supervisor.create_worker":
		// 创建子worker
		workerAddr, _ := message.NewLocalAddress("local", "/worker/"+fmt.Sprintf("%d", time.Now().UnixNano()))

		// 使用上下文创建子actor
		worker, err := NewWorkerActor(workerAddr)
		if err != nil {
			return err
		}

		// 启动子actor
		if err := a.Context().StartChild(ctx, worker); err != nil {
			return err
		}

		replyMsg := message.NewBaseMessage("supervisor.response", map[string]interface{}{
			"action":   "worker_created",
			"worker":   workerAddr.String(),
			"children": len(a.Context().Children()),
		})
		return a.Reply(ctx, msg, replyMsg)

	case "supervisor.list_workers":
		children := a.Context().Children()
		workers := make([]string, len(children))
		for i, child := range children {
			workers[i] = child.Address().String()
		}

		replyMsg := message.NewBaseMessage("supervisor.response", map[string]interface{}{
			"action":  "list_workers",
			"workers": workers,
			"count":   len(workers),
		})
		return a.Reply(ctx, msg, replyMsg)

	default:
		return fmt.Errorf("unknown message type: %s", msg.Type())
	}
}

func runAdvancedExamples() {
	fmt.Println("=== Actor系统高级功能示例 ===")

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 示例1: 生命周期钩子
	exampleLifecycleHooks(ctx)

	// 示例2: Actor上下文和子actor管理
	exampleActorContext(ctx)

	// 示例3: 工厂和注册表
	exampleFactoryRegistry(ctx)

	// 示例4: 监控和状态管理
	exampleMonitoring(ctx)

	// 示例5: 错误处理和恢复
	exampleErrorRecovery(ctx)

	fmt.Println("\n=== 所有高级示例执行完成 ===")
}

func exampleLifecycleHooks(ctx context.Context) {
	fmt.Println("\n--- 示例1: 生命周期钩子 ---")

	// 创建工作actor
	workerAddr, _ := message.NewLocalAddress("local", "/advanced/worker")
	worker, err := NewWorkerActor(workerAddr)
	if err != nil {
		fmt.Printf("创建工作actor失败: %v\n", err)
		return
	}

	// 启动actor（会触发PreStart和PostStart）
	fmt.Println("启动worker actor...")
	if err := worker.Start(ctx); err != nil {
		fmt.Printf("启动worker actor失败: %v\n", err)
		return
	}

	// 发送工作消息
	clientAddr, _ := message.NewLocalAddress("local", "/advanced/client")
	workMsg := message.NewBaseMessage("work.do", map[string]interface{}{
		"task": "test_task",
	})
	workMsg.SetSender(clientAddr)
	workMsg.SetReceiver(workerAddr)

	if err := worker.Send(ctx, workerAddr, workMsg); err != nil {
		fmt.Printf("发送工作消息失败: %v\n", err)
	} else {
		fmt.Println("已发送工作消息")
	}

	// 查询状态
	statusMsg := message.NewBaseMessage("work.status", nil)
	statusMsg.SetSender(clientAddr)
	statusMsg.SetReceiver(workerAddr)

	if err := worker.Send(ctx, workerAddr, statusMsg); err != nil {
		fmt.Printf("发送状态查询失败: %v\n", err)
	}

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 停止actor（会触发PreStop和PostStop）
	fmt.Println("停止worker actor...")
	if err := worker.Stop(); err != nil {
		fmt.Printf("停止worker actor失败: %v\n", err)
	} else {
		fmt.Println("Worker actor已停止")
	}
}

func exampleActorContext(ctx context.Context) {
	fmt.Println("\n--- 示例2: Actor上下文和子actor管理 ---")

	// 创建监督者actor
	supervisorAddr, _ := message.NewLocalAddress("local", "/advanced/supervisor")
	supervisor, err := NewSupervisorActor(supervisorAddr)
	if err != nil {
		fmt.Printf("创建监督者actor失败: %v\n", err)
		return
	}

	// 启动监督者
	if err := supervisor.Start(ctx); err != nil {
		fmt.Printf("启动监督者actor失败: %v\n", err)
		return
	}

	// 创建子worker
	clientAddr, _ := message.NewLocalAddress("local", "/advanced/client2")
	createMsg := message.NewBaseMessage("supervisor.create_worker", nil)
	createMsg.SetSender(clientAddr)
	createMsg.SetReceiver(supervisorAddr)

	for i := 0; i < 3; i++ {
		if err := supervisor.Send(ctx, supervisorAddr, createMsg); err != nil {
			fmt.Printf("发送创建worker消息失败: %v\n", err)
		} else {
			fmt.Printf("已发送创建worker消息 #%d\n", i+1)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 列出子worker
	listMsg := message.NewBaseMessage("supervisor.list_workers", nil)
	listMsg.SetSender(clientAddr)
	listMsg.SetReceiver(supervisorAddr)

	if err := supervisor.Send(ctx, supervisorAddr, listMsg); err != nil {
		fmt.Printf("发送列出worker消息失败: %v\n", err)
	}

	// 等待处理
	time.Sleep(300 * time.Millisecond)

	// 停止监督者（会自动停止所有子actor）
	if err := supervisor.Stop(); err != nil {
		fmt.Printf("停止监督者actor失败: %v\n", err)
	} else {
		fmt.Println("监督者actor已停止（包括所有子actor）")
	}
}

func exampleFactoryRegistry(ctx context.Context) {
	fmt.Println("\n--- 示例3: 工厂和注册表 ---")

	// 获取默认actor注册表
	registry := actor.GetDefaultActorRegistry()

	// 注册worker actor工厂
	workerFactory := actor.NewBaseActorFactory("worker", func(address message.Address, parent actor.Actor) (actor.Actor, error) {
		return NewWorkerActor(address)
	})

	if err := registry.Register(workerFactory); err != nil {
		fmt.Printf("注册worker工厂失败: %v\n", err)
	} else {
		fmt.Println("Worker actor工厂已注册")
	}

	// 使用工厂创建actor
	workerAddr, _ := message.NewLocalAddress("local", "/factory/worker1")
	worker, err := registry.Create("worker", workerAddr)
	if err != nil {
		fmt.Printf("使用工厂创建actor失败: %v\n", err)
	} else {
		fmt.Printf("使用工厂创建actor成功: %s\n", worker.Address().String())

		// 启动actor
		if err := worker.Start(ctx); err != nil {
			fmt.Printf("启动工厂创建的actor失败: %v\n", err)
		} else {
			fmt.Println("工厂创建的actor已启动")

			// 停止actor
			time.Sleep(100 * time.Millisecond)
			worker.Stop()
			fmt.Println("工厂创建的actor已停止")
		}
	}

	// 使用构建器创建actor
	fmt.Println("\n使用ActorBuilder创建actor...")
	workerAddr2, _ := message.NewLocalAddress("local", "/builder/worker1")
	builder := actor.NewActorBuilder("worker").
		WithAddress(workerAddr2).
		WithSupervisionStrategy(actor.SupervisionStrategyRestart)

	builtActor, err := builder.Build()
	if err != nil {
		fmt.Printf("使用构建器创建actor失败: %v\n", err)
	} else {
		fmt.Printf("使用构建器创建actor成功: %s\n", builtActor.Address().String())
		builtActor.Start(ctx)
		time.Sleep(50 * time.Millisecond)
		builtActor.Stop()
	}

	// 列出所有已注册的工厂类型
	factoryTypes := registry.FactoryTypes()
	fmt.Printf("\n已注册的工厂类型: %v\n", factoryTypes)
}

func exampleMonitoring(ctx context.Context) {
	fmt.Println("\n--- 示例4: 监控和状态管理 ---")

	// 获取默认监控器
	monitor := actor.GetDefaultMonitor()

	// 获取默认状态管理器
	stateManager := actor.GetDefaultStateManager()

	// 创建并启动多个actor
	actorList := make([]actor.Actor, 3)
	for i := 0; i < 3; i++ {
		addr, _ := message.NewLocalAddress("local", fmt.Sprintf("/monitor/actor%d", i))
		worker, err := NewWorkerActor(addr)
		if err != nil {
			fmt.Printf("创建监控actor %d失败: %v\n", i, err)
			continue
		}

		// 设置监控器
		if ctx := worker.Context(); ctx != nil {
			ctx.SetMonitor(monitor)
		}

		// 启动actor
		if err := worker.Start(ctx); err != nil {
			fmt.Printf("启动监控actor %d失败: %v\n", i, err)
			continue
		}

		actorList[i] = worker

		// 更新状态管理器
		stateManager.UpdateState(worker)
	}

	// 获取所有actor状态
	states := stateManager.GetAllStates()
	fmt.Printf("\n当前有 %d 个actor在状态管理中:\n", len(states))
	for _, state := range states {
		fmt.Printf("  - %s: 状态=%v, 健康=%v\n",
			state.Address, state.State, state.Health)
	}

	// 获取健康actor
	healthyActors := stateManager.GetStatesByHealth(actor.HealthStatusHealthy)
	fmt.Printf("\n健康actor数量: %d\n", len(healthyActors))

	// 停止所有actor
	for i, worker := range actorList {
		if worker != nil {
			worker.Stop()
			fmt.Printf("监控actor %d 已停止\n", i)

			// 更新状态
			stateManager.UpdateState(worker)
		}
	}

	// 再次检查状态
	states = stateManager.GetAllStates()
	fmt.Printf("\n停止后，状态管理中有 %d 个actor\n", len(states))
}

func exampleErrorRecovery(ctx context.Context) {
	fmt.Println("\n--- 示例5: 错误处理和恢复 ---")

	// 创建测试actor
	addr, _ := message.NewLocalAddress("local", "/recovery/test")
	actor, err := NewWorkerActor(addr)
	if err != nil {
		fmt.Printf("创建恢复测试actor失败: %v\n", err)
		return
	}

	// 启动actor
	if err := actor.Start(ctx); err != nil {
		fmt.Printf("启动恢复测试actor失败: %v\n", err)
		return
	}

	// 发送正常消息
	clientAddr, _ := message.NewLocalAddress("local", "/recovery/client")
	normalMsg := message.NewBaseMessage("work.do", map[string]interface{}{
		"task": "normal_work",
	})
	normalMsg.SetSender(clientAddr)
	normalMsg.SetReceiver(addr)

	if err := actor.Send(ctx, addr, normalMsg); err != nil {
		fmt.Printf("发送正常消息失败: %v\n", err)
	} else {
		fmt.Println("已发送正常工作消息")
	}

	// 发送错误消息
	errorMsg := message.NewBaseMessage("work.error", map[string]interface{}{
		"task": "error_work",
	})
	errorMsg.SetSender(clientAddr)
	errorMsg.SetReceiver(addr)

	if err := actor.Send(ctx, addr, errorMsg); err != nil {
		fmt.Printf("发送错误消息失败: %v\n", err)
	} else {
		fmt.Println("已发送错误工作消息（应触发错误处理）")
	}

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 检查actor状态
	health, _ := actor.HealthCheck(ctx)
	fmt.Printf("Actor健康状态: %v\n", health)
	fmt.Printf("Actor运行状态: %v\n", actor.IsRunning())

	// 停止actor
	if err := actor.Stop(); err != nil {
		fmt.Printf("停止恢复测试actor失败: %v\n", err)
	} else {
		fmt.Println("恢复测试actor已停止")
	}

	// 演示错误处理
	fmt.Println("\n演示错误处理模式...")
	fmt.Println("错误处理功能包括重试、断路器、监督策略等")
	fmt.Println("这些功能已在actor包的recovery.go中实现")

	// 演示重试逻辑
	fmt.Println("\n模拟消息重试逻辑:")
	for i := 1; i <= 3; i++ {
		fmt.Printf("  尝试 %d: ", i)
		if i < 3 {
			fmt.Println("失败，将重试...")
			time.Sleep(500 * time.Millisecond)
		} else {
			fmt.Println("成功!")
		}
	}
}

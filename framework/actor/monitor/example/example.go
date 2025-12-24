package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GooLuck/GoServer/framework/actor"
	"github.com/GooLuck/GoServer/framework/actor/mailbox"
	"github.com/GooLuck/GoServer/framework/actor/message"
	"github.com/GooLuck/GoServer/framework/actor/monitor"
)

// ExampleActor 示例actor
type ExampleActor struct {
	*actor.BaseActor
	name string
}

// NewExampleActor 创建新的示例actor
func NewExampleActor(name string) (*ExampleActor, error) {
	addr, err := message.NewLocalActorAddress(fmt.Sprintf("/example/%s", name))
	if err != nil {
		return nil, err
	}
	base, err := actor.NewBaseActor(addr, mailbox.GetDefaultManager())
	if err != nil {
		return nil, err
	}

	return &ExampleActor{
		BaseActor: base,
		name:      name,
	}, nil
}

// HandleMessage 处理消息
func (a *ExampleActor) HandleMessage(ctx context.Context, envelope *message.Envelope) error {
	msg := envelope.Message()
	fmt.Printf("Actor %s received message: type=%s\n", a.name, msg.Type())

	// 模拟处理时间
	time.Sleep(10 * time.Millisecond)

	return nil
}

// HealthCheck 健康检查
func (a *ExampleActor) HealthCheck(ctx context.Context) (actor.HealthStatus, error) {
	// 模拟随机健康状态
	// 在实际应用中，这里应该检查actor的实际状态
	return actor.HealthStatusHealthy, nil
}

func main() {
	fmt.Println("=== Actor监控和管理示例 ===")

	// 1. 创建性能指标收集器
	metricsCollector := monitor.NewMetricsCollector()
	fmt.Println("✓ 创建性能指标收集器")

	// 2. 创建健康检查管理器
	healthConfig := monitor.HealthCheckConfig{
		CheckInterval:         10 * time.Second,
		Timeout:               5 * time.Second,
		MaxFailures:           3,
		RecoveryCheckInterval: 5 * time.Second,
		EnableAutoRecovery:    true,
	}
	healthManager := monitor.NewHealthCheckManager(healthConfig)
	fmt.Println("✓ 创建健康检查管理器")

	// 3. 创建管理API
	apiConfig := monitor.DefaultAPIConfig()
	apiConfig.Port = 18080 // 使用不同的端口避免冲突
	managementAPI := monitor.NewManagementAPI(metricsCollector, healthManager, apiConfig)
	fmt.Println("✓ 创建管理API")

	// 4. 创建指标监控器
	metricsMonitor := monitor.NewMetricsMonitor(metricsCollector)
	fmt.Println("✓ 创建指标监控器")

	// 5. 创建示例actor
	actor1, err := NewExampleActor("actor1")
	if err != nil {
		log.Fatalf("创建actor1失败: %v", err)
	}
	fmt.Println("✓ 创建actor1")

	actor2, err := NewExampleActor("actor2")
	if err != nil {
		log.Fatalf("创建actor2失败: %v", err)
	}
	fmt.Println("✓ 创建actor2")

	// 6. 注册actor到管理器
	actorMgr := actor.GetDefaultActorManager()
	if err := actorMgr.Register(actor1); err != nil {
		log.Fatalf("注册actor1失败: %v", err)
	}
	if err := actorMgr.Register(actor2); err != nil {
		log.Fatalf("注册actor2失败: %v", err)
	}
	fmt.Println("✓ 注册actor到管理器")

	// 7. 注册actor到健康检查
	if err := healthManager.RegisterActor(actor1); err != nil {
		log.Fatalf("注册actor1到健康检查失败: %v", err)
	}
	if err := healthManager.RegisterActor(actor2); err != nil {
		log.Fatalf("注册actor2到健康检查失败: %v", err)
	}
	fmt.Println("✓ 注册actor到健康检查")

	// 8. 设置监控器到actor上下文
	actor1.Context().SetMonitor(metricsMonitor)
	actor2.Context().SetMonitor(metricsMonitor)
	fmt.Println("✓ 设置监控器到actor上下文")

	// 9. 启动健康检查管理器
	if err := healthManager.Start(); err != nil {
		log.Fatalf("启动健康检查管理器失败: %v", err)
	}
	fmt.Println("✓ 启动健康检查管理器")

	// 10. 启动管理API
	if err := managementAPI.Start(); err != nil {
		log.Fatalf("启动管理API失败: %v", err)
	}
	fmt.Println("✓ 启动管理API")

	// 11. 启动actor
	ctx := context.Background()
	if err := actor1.Start(ctx); err != nil {
		log.Fatalf("启动actor1失败: %v", err)
	}
	fmt.Println("✓ 启动actor1")

	if err := actor2.Start(ctx); err != nil {
		log.Fatalf("启动actor2失败: %v", err)
	}
	fmt.Println("✓ 启动actor2")

	// 12. 发送一些测试消息
	fmt.Println("\n发送测试消息...")
	for i := 0; i < 5; i++ {
		msg := message.NewBaseMessage(fmt.Sprintf("test-%d", i), fmt.Sprintf("Hello from test %d", i))
		if err := actor1.Send(ctx, actor2.Address(), msg); err != nil {
			fmt.Printf("发送消息失败: %v\n", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 13. 显示系统指标
	fmt.Println("\n=== 系统指标 ===")
	systemMetrics := metricsCollector.GetSystemMetrics()
	fmt.Printf("总actor数: %d\n", systemMetrics.TotalActors)
	fmt.Printf("活跃actor数: %d\n", systemMetrics.ActiveActors)
	fmt.Printf("总消息数: %d\n", systemMetrics.TotalMessages)
	fmt.Printf("平均消息延迟: %d ns\n", systemMetrics.AvgMessageLatency)

	// 14. 显示actor指标
	fmt.Println("\n=== Actor指标 ===")
	allActorMetrics := metricsCollector.GetAllActorMetrics()
	for _, actorMetrics := range allActorMetrics {
		fmt.Printf("Actor: %s\n", actorMetrics.ActorAddress)
		fmt.Printf("  类型: %s\n", actorMetrics.ActorType)
		fmt.Printf("  已处理消息数: %d\n", actorMetrics.MessagesProcessed)
		fmt.Printf("  平均处理时间: %d ns\n", actorMetrics.AvgProcessingTime)
		fmt.Printf("  健康状态: %v\n", actorMetrics.HealthStatus)
	}

	// 15. 显示健康检查结果
	fmt.Println("\n=== 健康检查结果 ===")
	healthResults := healthManager.GetAllHealthResults()
	for _, result := range healthResults {
		fmt.Printf("Actor: %s\n", result.ActorAddress)
		fmt.Printf("  健康状态: %v\n", result.HealthStatus)
		fmt.Printf("  最后检查时间: %v\n", result.LastCheckTime)
		fmt.Printf("  检查耗时: %v\n", result.CheckDuration)
		if result.Error != "" {
			fmt.Printf("  错误: %s\n", result.Error)
		}
	}

	// 16. 显示API信息
	fmt.Println("\n=== 管理API信息 ===")
	fmt.Printf("API地址: http://localhost:%d\n", apiConfig.Port)
	fmt.Println("可用端点:")
	fmt.Println("  GET  /health                    - 健康检查")
	fmt.Println("  GET  /ready                     - 就绪检查")
	fmt.Println("  GET  /api/v1/system            - 系统信息")
	fmt.Println("  GET  /api/v1/metrics           - 系统指标")
	fmt.Println("  GET  /api/v1/health            - 系统健康状态")
	fmt.Println("  GET  /api/v1/actors            - 列出所有actor")
	fmt.Println("  GET  /api/v1/actors/{address}  - 获取单个actor信息")
	fmt.Println("  POST /api/v1/actors/{address}/restart - 重启actor")
	fmt.Println("  POST /api/v1/actors/{address}/stop    - 停止actor")
	fmt.Println("  POST /api/v1/actors/{address}/start   - 启动actor")

	// 17. 等待一段时间
	fmt.Println("\n运行10秒...")
	time.Sleep(10 * time.Second)

	// 18. 停止服务
	fmt.Println("\n=== 停止服务 ===")

	// 停止actor
	if err := actor1.Stop(); err != nil {
		fmt.Printf("停止actor1失败: %v\n", err)
	} else {
		fmt.Println("✓ 停止actor1")
	}

	if err := actor2.Stop(); err != nil {
		fmt.Printf("停止actor2失败: %v\n", err)
	} else {
		fmt.Println("✓ 停止actor2")
	}

	// 停止健康检查管理器
	healthManager.Stop()
	fmt.Println("✓ 停止健康检查管理器")

	// 停止管理API
	if err := managementAPI.Stop(); err != nil {
		fmt.Printf("停止管理API失败: %v\n", err)
	} else {
		fmt.Println("✓ 停止管理API")
	}

	fmt.Println("\n=== 示例完成 ===")
	fmt.Println("可以通过以下命令测试API:")
	fmt.Printf("  curl http://localhost:%d/api/v1/system\n", apiConfig.Port)
	fmt.Printf("  curl http://localhost:%d/api/v1/actors\n", apiConfig.Port)
	fmt.Printf("  curl http://localhost:%d/api/v1/metrics\n", apiConfig.Port)
}

// LoggingMetricsHandler 日志指标处理器
type LoggingMetricsHandler struct{}

// OnMetricsUpdated 指标更新时调用
func (h *LoggingMetricsHandler) OnMetricsUpdated(collector *monitor.MetricsCollector) {
	metrics := collector.GetSystemMetrics()
	fmt.Printf("[METRICS] 系统指标更新: actors=%d, messages=%d, avg_latency=%dns\n",
		metrics.ActiveActors, metrics.TotalMessages, metrics.AvgMessageLatency)
}

// OnActorMetricsUpdated actor指标更新时调用
func (h *LoggingMetricsHandler) OnActorMetricsUpdated(address string, metrics monitor.ActorMetrics) {
	fmt.Printf("[METRICS] Actor指标更新: %s, processed=%d, avg_time=%dns\n",
		address, metrics.MessagesProcessed, metrics.AvgProcessingTime)
}

// OnMessageMetricsUpdated 消息指标更新时调用
func (h *LoggingMetricsHandler) OnMessageMetricsUpdated(msgType string, metrics monitor.MessageTypeMetrics) {
	fmt.Printf("[METRICS] 消息类型指标更新: %s, count=%d, avg_latency=%dns\n",
		msgType, metrics.Count, metrics.AvgLatency)
}

// LoggingHealthHandler 日志健康处理器
type LoggingHealthHandler struct{}

// OnHealthChanged 健康状态变更时调用
func (h *LoggingHealthHandler) OnHealthChanged(address string, oldStatus, newStatus actor.HealthStatus, result monitor.HealthCheckResult) {
	fmt.Printf("[HEALTH] 健康状态变更: %s, %v -> %v\n", address, oldStatus, newStatus)
}

// OnHealthCheckFailed 健康检查失败时调用
func (h *LoggingHealthHandler) OnHealthCheckFailed(address string, result monitor.HealthCheckResult) {
	fmt.Printf("[HEALTH] 健康检查失败: %s, error=%s\n", address, result.Error)
}

// OnHealthCheckRecovered 健康检查恢复时调用
func (h *LoggingHealthHandler) OnHealthCheckRecovered(address string, result monitor.HealthCheckResult) {
	fmt.Printf("[HEALTH] 健康检查恢复: %s\n", address)
}

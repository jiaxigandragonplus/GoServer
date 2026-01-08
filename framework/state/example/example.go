package example

import (
	"context"
	"fmt"
	"time"

	"github.com/GooLuck/GoServer/framework/state"
)

// CounterState 计数器状态
type CounterState struct {
	*state.BaseState
}

// NewCounterState 创建新的计数器状态
func NewCounterState(count int) *CounterState {
	return &CounterState{
		BaseState: state.NewBaseState("counter", map[string]interface{}{
			"count":        count,
			"last_updated": time.Now(),
		}),
	}
}

// GetCount 获取计数值
func (cs *CounterState) GetCount() int {
	data := cs.Data().(map[string]interface{})
	if count, ok := data["count"].(int); ok {
		return count
	}
	return 0
}

// Increment 增加计数
func (cs *CounterState) Increment() {
	data := cs.Data().(map[string]interface{})
	count := data["count"].(int)
	data["count"] = count + 1
	data["last_updated"] = time.Now()
	cs.SetData(data)
}

// CounterActor 计数器actor
type CounterActor struct {
	*state.StatefulBaseActor
}

// NewCounterActor 创建新的计数器actor
func NewCounterActor(actorID string, initialState *CounterState) *CounterActor {
	actor := &CounterActor{
		StatefulBaseActor: state.NewStatefulBaseActor(actorID, initialState, nil),
	}
	return actor
}

// Increment 增加计数
func (ca *CounterActor) Increment() error {
	currentState := ca.GetState().(*CounterState)
	newState := &CounterState{
		BaseState: currentState.Clone(),
	}
	newState.Increment()
	return ca.SetState(newState)
}

// GetCount 获取当前计数
func (ca *CounterActor) GetCount() int {
	currentState := ca.GetState().(*CounterState)
	return currentState.GetCount()
}

// RunExample 运行状态管理示例
func RunExample() {
	fmt.Println("=== Actor状态管理示例 ===")

	ctx := context.Background()

	// 1. 创建状态管理器
	fmt.Println("\n1. 创建状态管理器...")
	stateManager := state.NewMemoryStateManager()
	snapshotManager := state.NewMemorySnapshotManager()
	serializer := state.NewJSONStateSerializer()

	// 2. 创建恢复管理器
	fmt.Println("2. 创建恢复管理器...")
	recoveryManager := state.NewRecoveryManager(stateManager, snapshotManager, serializer)

	// 3. 创建计数器actor
	fmt.Println("3. 创建计数器actor...")
	initialState := NewCounterState(0)
	counterActor := NewCounterActor("counter-1", initialState)

	// 设置管理器
	counterActor.SetStateManager(stateManager)
	counterActor.SetSnapshotManager(snapshotManager)

	// 4. 测试状态持久化
	fmt.Println("\n4. 测试状态持久化...")

	// 增加计数
	for i := 0; i < 5; i++ {
		if err := counterActor.Increment(); err != nil {
			fmt.Printf("增加计数失败: %v\n", err)
		}
		fmt.Printf("  计数: %d\n", counterActor.GetCount())
	}

	// 持久化状态
	if err := counterActor.PersistState(ctx); err != nil {
		fmt.Printf("持久化状态失败: %v\n", err)
	} else {
		fmt.Println("  状态已持久化")
	}

	// 5. 测试快照机制
	fmt.Println("\n5. 测试快照机制...")

	// 创建快照
	snapshot, err := counterActor.TakeSnapshot(ctx)
	if err != nil {
		fmt.Printf("创建快照失败: %v\n", err)
	} else {
		fmt.Printf("  快照创建成功: ID=%s, 版本=%d\n",
			snapshot.ID(), snapshot.State().Version())
	}

	// 6. 测试状态恢复
	fmt.Println("\n6. 测试状态恢复...")

	// 创建新actor并恢复状态
	newCounterActor := NewCounterActor("counter-1", NewCounterState(0))
	newCounterActor.SetStateManager(stateManager)
	newCounterActor.SetSnapshotManager(snapshotManager)

	if err := newCounterActor.RestoreState(ctx); err != nil {
		fmt.Printf("恢复状态失败: %v\n", err)
	} else {
		fmt.Printf("  状态恢复成功: 计数=%d\n", newCounterActor.GetCount())
	}

	// 7. 测试从快照恢复
	fmt.Println("\n7. 测试从快照恢复...")

	// 修改状态
	for i := 0; i < 3; i++ {
		newCounterActor.Increment()
	}
	fmt.Printf("  修改后计数: %d\n", newCounterActor.GetCount())

	// 从快照恢复
	if err := newCounterActor.RestoreFromSnapshot(ctx, snapshot); err != nil {
		fmt.Printf("从快照恢复失败: %v\n", err)
	} else {
		fmt.Printf("  从快照恢复成功: 计数=%d\n", newCounterActor.GetCount())
	}

	// 8. 测试恢复管理器
	fmt.Println("\n8. 测试恢复管理器...")

	if err := recoveryManager.RecoverActor(ctx, "counter-1", newCounterActor); err != nil {
		fmt.Printf("恢复管理器恢复失败: %v\n", err)
	} else {
		fmt.Printf("  恢复管理器恢复成功: 计数=%d\n", newCounterActor.GetCount())
	}

	// 9. 测试分布式状态管理
	fmt.Println("\n9. 测试分布式状态管理...")

	// 创建多个副本
	replica1 := state.NewMemoryStateManager()
	replica2 := state.NewMemoryStateManager()

	distributedManager := state.NewDistributedStateManager(
		stateManager,
		[]state.StateManager{replica1, replica2},
		&state.StateConfig{
			ConsistencyLevel:  state.ConsistencyLevelStrong,
			ReplicationFactor: 2,
		},
	)

	// 使用分布式管理器保存状态
	distributedState := NewCounterState(100)
	if err := distributedManager.Save(context.Background(), "distributed-counter", distributedState); err != nil {
		fmt.Printf("分布式保存失败: %v\n", err)
	} else {
		fmt.Println("  分布式保存成功")
	}

	// 从分布式管理器加载状态
	loadedState, err := distributedManager.Load(context.Background(), "distributed-counter")
	if err != nil {
		fmt.Printf("分布式加载失败: %v\n", err)
	} else {
		counterState := &CounterState{BaseState: loadedState.(*state.BaseState)}
		fmt.Printf("  分布式加载成功: 计数=%d\n", counterState.GetCount())
	}

	// 10. 显示统计信息
	fmt.Println("\n10. 统计信息:")
	fmt.Printf("   - 状态变更次数: %d\n", counterActor.GetChangeCount())
	fmt.Printf("   - 快照数量: %d\n", snapshotManager.GetSnapshotCount("counter-1"))

	// 获取恢复日志
	recoveryLog := recoveryManager.GetRecoveryLog()
	fmt.Printf("   - 恢复操作次数: %d\n", len(recoveryLog))

	fmt.Println("\n=== 示例完成 ===")
}

// RunAdvancedExample 运行高级示例
func RunAdvancedExample() {
	fmt.Println("\n=== 高级状态管理示例 ===")

	// 创建配置
	config := &state.StateConfig{
		PersistenceEnabled: true,
		SnapshotInterval:   2 * time.Second,
		SnapshotThreshold:  3,
		ConsistencyLevel:   state.ConsistencyLevelEventual,
		ReplicationFactor:  2,
	}

	// 创建多个状态管理器
	managers := make([]state.StateManager, 3)
	for i := 0; i < 3; i++ {
		managers[i] = state.NewMemoryStateManager()
	}

	// 创建分布式管理器
	distributedManager := state.NewDistributedStateManager(
		managers[0],
		managers[1:],
		config,
	)

	// 创建状态同步服务
	distributedManagerImpl := distributedManager.(*state.DistributedStateManagerImpl)
	syncService := state.NewStateSyncService(
		distributedManagerImpl,
		5*time.Second,
	)

	// 启动同步服务
	if err := syncService.Start(); err != nil {
		fmt.Printf("启动同步服务失败: %v\n", err)
	}
	defer syncService.Stop()

	// 创建多个actor
	actors := make([]*CounterActor, 3)
	for i := 0; i < 3; i++ {
		actorID := fmt.Sprintf("advanced-counter-%d", i)
		initialState := NewCounterState(i * 10)
		actor := NewCounterActor(actorID, initialState)

		// 每个actor使用不同的状态管理器
		actor.SetStateManager(managers[i%len(managers)])
		actors[i] = actor
	}

	// 模拟并发状态更新
	fmt.Println("\n模拟并发状态更新...")
	for i := 0; i < 10; i++ {
		for _, actor := range actors {
			actor.Increment()
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 显示最终状态
	fmt.Println("\n最终状态:")
	for i, actor := range actors {
		actorID := fmt.Sprintf("advanced-counter-%d", i)
		fmt.Printf("  %s: 计数=%d, 变更次数=%d\n",
			actorID, actor.GetCount(), actor.GetChangeCount())
	}

	// 显示同步状态
	syncStatus := syncService.GetSyncStatus()
	fmt.Println("\n同步服务状态:")
	for key, value := range syncStatus {
		fmt.Printf("  %s: %v\n", key, value)
	}

	fmt.Println("\n=== 高级示例完成 ===")
}

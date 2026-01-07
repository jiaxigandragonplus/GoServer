package state

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BaseState 基础状态实现
type BaseState struct {
	stateType string
	version   int64
	timestamp time.Time
	data      interface{}
	mu        sync.RWMutex
}

// NewBaseState 创建新的基础状态
func NewBaseState(stateType string, data interface{}) *BaseState {
	return &BaseState{
		stateType: stateType,
		version:   1,
		timestamp: time.Now(),
		data:      data,
	}
}

// Type 返回状态类型
func (s *BaseState) Type() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stateType
}

// Version 返回版本号
func (s *BaseState) Version() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// Timestamp 返回时间戳
func (s *BaseState) Timestamp() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.timestamp
}

// Data 返回状态数据
func (s *BaseState) Data() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// SetData 设置状态数据
func (s *BaseState) SetData(data interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	s.version++
	s.timestamp = time.Now()
}

// IncrementVersion 增加版本号
func (s *BaseState) IncrementVersion() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.timestamp = time.Now()
}

// Clone 克隆状态
func (s *BaseState) Clone() *BaseState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &BaseState{
		stateType: s.stateType,
		version:   s.version,
		timestamp: s.timestamp,
		data:      s.data,
	}
}

// StatefulBaseActor 支持状态管理的基础actor
type StatefulBaseActor struct {
	actorID      string
	currentState State
	stateManager StateManager
	snapshotMgr  SnapshotManager
	config       *StateConfig
	mu           sync.RWMutex
	changeCount  int
	lastSnapshot time.Time
}

// NewStatefulBaseActor 创建新的支持状态管理的基础actor
func NewStatefulBaseActor(actorID string, initialState State, config *StateConfig) *StatefulBaseActor {
	if config == nil {
		config = DefaultConfig()
	}

	return &StatefulBaseActor{
		actorID:      actorID,
		currentState: initialState,
		config:       config,
		lastSnapshot: time.Now(),
	}
}

// GetState 获取当前状态
func (a *StatefulBaseActor) GetState() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentState
}

// SetState 设置状态
func (a *StatefulBaseActor) SetState(state State) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 保存旧状态用于可能的日志或事件
	_ = a.currentState // 标记为已使用
	a.currentState = state
	a.changeCount++

	// 检查是否需要创建快照
	if a.config.PersistenceEnabled && a.shouldTakeSnapshot() {
		go a.takeSnapshotAsync()
	}

	// 触发状态变更事件
	if a.stateManager != nil {
		// 这里可以触发事件，简化实现
	}

	return nil
}

// shouldTakeSnapshot 检查是否需要创建快照
func (a *StatefulBaseActor) shouldTakeSnapshot() bool {
	// 检查变更次数阈值
	if a.changeCount >= a.config.SnapshotThreshold {
		return true
	}

	// 检查时间间隔
	if time.Since(a.lastSnapshot) >= a.config.SnapshotInterval {
		return true
	}

	return false
}

// takeSnapshotAsync 异步创建快照
func (a *StatefulBaseActor) takeSnapshotAsync() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.snapshotMgr == nil {
		return
	}

	ctx := context.Background()
	metadata := map[string]string{
		"actor_id": a.actorID,
		"version":  fmt.Sprintf("%d", a.currentState.Version()),
	}

	_, err := a.snapshotMgr.Create(ctx, a.actorID, a.currentState, metadata)
	if err != nil {
		// 记录错误，但不影响主流程
		return
	}

	a.changeCount = 0
	a.lastSnapshot = time.Now()
}

// PersistState 持久化状态
func (a *StatefulBaseActor) PersistState(ctx context.Context) error {
	if a.stateManager == nil {
		return fmt.Errorf("state manager not set")
	}

	a.mu.RLock()
	state := a.currentState
	a.mu.RUnlock()

	return a.stateManager.Save(ctx, a.actorID, state)
}

// RestoreState 恢复状态
func (a *StatefulBaseActor) RestoreState(ctx context.Context) error {
	if a.stateManager == nil {
		return fmt.Errorf("state manager not set")
	}

	state, err := a.stateManager.Load(ctx, a.actorID)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.currentState = state
	a.mu.Unlock()

	return nil
}

// TakeSnapshot 创建快照
func (a *StatefulBaseActor) TakeSnapshot(ctx context.Context) (Snapshot, error) {
	if a.snapshotMgr == nil {
		return nil, fmt.Errorf("snapshot manager not set")
	}

	a.mu.RLock()
	state := a.currentState
	a.mu.RUnlock()

	metadata := map[string]string{
		"actor_id": a.actorID,
		"version":  fmt.Sprintf("%d", state.Version()),
		"time":     time.Now().Format(time.RFC3339),
	}

	return a.snapshotMgr.Create(ctx, a.actorID, state, metadata)
}

// RestoreFromSnapshot 从快照恢复
func (a *StatefulBaseActor) RestoreFromSnapshot(ctx context.Context, snapshot Snapshot) error {
	if a.snapshotMgr == nil {
		return fmt.Errorf("snapshot manager not set")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.currentState = snapshot.State()
	a.changeCount = 0
	a.lastSnapshot = time.Now()

	return nil
}

// SetStateManager 设置状态管理器
func (a *StatefulBaseActor) SetStateManager(manager StateManager) {
	a.stateManager = manager
}

// SetSnapshotManager 设置快照管理器
func (a *StatefulBaseActor) SetSnapshotManager(manager SnapshotManager) {
	a.snapshotMgr = manager
}

// GetChangeCount 获取变更次数
func (a *StatefulBaseActor) GetChangeCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.changeCount
}

// ResetChangeCount 重置变更次数
func (a *StatefulBaseActor) ResetChangeCount() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.changeCount = 0
}

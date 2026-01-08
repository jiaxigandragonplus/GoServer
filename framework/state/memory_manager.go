package state

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryStateManager 内存状态管理器
type MemoryStateManager struct {
	states map[string]State
	mu     sync.RWMutex
}

// NewMemoryStateManager 创建新的内存状态管理器
func NewMemoryStateManager() *MemoryStateManager {
	return &MemoryStateManager{
		states: make(map[string]State),
	}
}

// Save 保存状态
func (m *MemoryStateManager) Save(ctx context.Context, actorID string, state State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states[actorID] = state
	return nil
}

// Load 加载状态
func (m *MemoryStateManager) Load(ctx context.Context, actorID string) (State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[actorID]
	if !exists {
		return nil, fmt.Errorf("state not found for actor %s", actorID)
	}

	return state, nil
}

// Delete 删除状态
func (m *MemoryStateManager) Delete(ctx context.Context, actorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.states[actorID]; !exists {
		return fmt.Errorf("state not found for actor %s", actorID)
	}

	delete(m.states, actorID)
	return nil
}

// List 列出所有状态
func (m *MemoryStateManager) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.states))
	for key := range m.states {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys, nil
}

// Exists 检查状态是否存在
func (m *MemoryStateManager) Exists(ctx context.Context, actorID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.states[actorID]
	return exists, nil
}

// MemorySnapshot 内存快照实现
type MemorySnapshot struct {
	id        string
	actorID   string
	state     State
	timestamp time.Time
	metadata  map[string]string
}

// NewMemorySnapshot 创建新的内存快照
func NewMemorySnapshot(id, actorID string, state State, metadata map[string]string) *MemorySnapshot {
	return &MemorySnapshot{
		id:        id,
		actorID:   actorID,
		state:     state,
		timestamp: time.Now(),
		metadata:  metadata,
	}
}

// ID 返回快照ID
func (s *MemorySnapshot) ID() string {
	return s.id
}

// ActorID 返回actor ID
func (s *MemorySnapshot) ActorID() string {
	return s.actorID
}

// State 返回快照状态
func (s *MemorySnapshot) State() State {
	return s.state
}

// Timestamp 返回快照时间戳
func (s *MemorySnapshot) Timestamp() time.Time {
	return s.timestamp
}

// Metadata 返回快照元数据
func (s *MemorySnapshot) Metadata() map[string]string {
	return s.metadata
}

// MemorySnapshotManager 内存快照管理器
type MemorySnapshotManager struct {
	snapshots map[string][]*MemorySnapshot // actorID -> []snapshots
	mu        sync.RWMutex
}

// NewMemorySnapshotManager 创建新的内存快照管理器
func NewMemorySnapshotManager() *MemorySnapshotManager {
	return &MemorySnapshotManager{
		snapshots: make(map[string][]*MemorySnapshot),
	}
}

// Create 创建快照
func (m *MemorySnapshotManager) Create(ctx context.Context, actorID string, state State, metadata map[string]string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成快照ID
	snapshotID := fmt.Sprintf("%s-%d", actorID, time.Now().UnixNano())

	// 创建快照
	snapshot := NewMemorySnapshot(snapshotID, actorID, state, metadata)

	// 保存快照
	if _, exists := m.snapshots[actorID]; !exists {
		m.snapshots[actorID] = make([]*MemorySnapshot, 0)
	}
	m.snapshots[actorID] = append(m.snapshots[actorID], snapshot)

	return snapshot, nil
}

// Get 获取快照
func (m *MemorySnapshotManager) Get(ctx context.Context, snapshotID string) (Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 遍历所有快照查找
	for _, snapshots := range m.snapshots {
		for _, snapshot := range snapshots {
			if snapshot.ID() == snapshotID {
				return snapshot, nil
			}
		}
	}

	return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
}

// GetLatest 获取最新快照
func (m *MemorySnapshotManager) GetLatest(ctx context.Context, actorID string) (Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots, exists := m.snapshots[actorID]
	if !exists || len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found for actor %s", actorID)
	}

	// 返回最后一个快照（最新的）
	return snapshots[len(snapshots)-1], nil
}

// List 列出快照
func (m *MemorySnapshotManager) List(ctx context.Context, actorID string) ([]Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots, exists := m.snapshots[actorID]
	if !exists {
		return []Snapshot{}, nil
	}

	// 转换为接口切片
	result := make([]Snapshot, len(snapshots))
	for i, snapshot := range snapshots {
		result[i] = snapshot
	}

	return result, nil
}

// Delete 删除快照
func (m *MemorySnapshotManager) Delete(ctx context.Context, snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for actorID, snapshots := range m.snapshots {
		for i, snapshot := range snapshots {
			if snapshot.ID() == snapshotID {
				// 删除快照
				m.snapshots[actorID] = append(snapshots[:i], snapshots[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("snapshot not found: %s", snapshotID)
}

// Cleanup 清理旧快照
func (m *MemorySnapshotManager) Cleanup(ctx context.Context, actorID string, keepCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshots, exists := m.snapshots[actorID]
	if !exists {
		return nil
	}

	// 如果快照数量小于等于保留数量，不清理
	if len(snapshots) <= keepCount {
		return nil
	}

	// 保留最新的keepCount个快照
	m.snapshots[actorID] = snapshots[len(snapshots)-keepCount:]
	return nil
}

// GetSnapshotCount 获取快照数量
func (m *MemorySnapshotManager) GetSnapshotCount(actorID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots, exists := m.snapshots[actorID]
	if !exists {
		return 0
	}

	return len(snapshots)
}

// ClearAll 清除所有快照
func (m *MemorySnapshotManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.snapshots = make(map[string][]*MemorySnapshot)
}

package state

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RecoveryManager 状态恢复管理器
type RecoveryManager struct {
	stateManager    StateManager
	snapshotManager SnapshotManager
	serializer      StateSerializer
	recoveryLog     []RecoveryLogEntry
	mu              sync.RWMutex
}

// RecoveryLogEntry 恢复日志条目
type RecoveryLogEntry struct {
	Timestamp    time.Time
	ActorID      string
	Operation    string
	Success      bool
	Error        string
	SnapshotID   string
	StateType    string
	StateVersion int64
}

// NewRecoveryManager 创建新的恢复管理器
func NewRecoveryManager(stateManager StateManager, snapshotManager SnapshotManager, serializer StateSerializer) *RecoveryManager {
	return &RecoveryManager{
		stateManager:    stateManager,
		snapshotManager: snapshotManager,
		serializer:      serializer,
		recoveryLog:     make([]RecoveryLogEntry, 0),
	}
}

// RecoverActor 恢复actor状态
func (rm *RecoveryManager) RecoverActor(ctx context.Context, actorID string, actor StatefulActor) error {
	rm.mu.Lock()
	entry := RecoveryLogEntry{
		Timestamp: time.Now(),
		ActorID:   actorID,
		Operation: "recover",
	}
	rm.mu.Unlock()

	defer func() {
		rm.mu.Lock()
		entry.Success = true
		rm.recoveryLog = append(rm.recoveryLog, entry)
		rm.mu.Unlock()
	}()

	// 尝试从快照恢复
	if err := rm.recoverFromSnapshot(ctx, actorID, actor); err == nil {
		entry.Operation = "recover_from_snapshot"
		return nil
	}

	// 如果快照恢复失败，尝试从持久化状态恢复
	if err := rm.recoverFromPersistedState(ctx, actorID, actor); err == nil {
		entry.Operation = "recover_from_persisted"
		return nil
	}

	// 如果都失败，使用初始状态
	entry.Operation = "recover_initial"
	entry.Error = "both snapshot and persisted state recovery failed"
	return fmt.Errorf("failed to recover actor %s from any source", actorID)
}

// recoverFromSnapshot 从快照恢复
func (rm *RecoveryManager) recoverFromSnapshot(ctx context.Context, actorID string, actor StatefulActor) error {
	if rm.snapshotManager == nil {
		return fmt.Errorf("snapshot manager not available")
	}

	// 获取最新快照
	snapshot, err := rm.snapshotManager.GetLatest(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	// 从快照恢复
	if err := actor.RestoreFromSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	return nil
}

// recoverFromPersistedState 从持久化状态恢复
func (rm *RecoveryManager) recoverFromPersistedState(ctx context.Context, actorID string, actor StatefulActor) error {
	if rm.stateManager == nil {
		return fmt.Errorf("state manager not available")
	}

	// 从持久化状态恢复
	if err := actor.RestoreState(ctx); err != nil {
		return fmt.Errorf("failed to restore from persisted state: %w", err)
	}

	return nil
}

// CreateRecoveryPoint 创建恢复点
func (rm *RecoveryManager) CreateRecoveryPoint(ctx context.Context, actorID string, actor StatefulActor) (string, error) {
	// 创建快照作为恢复点
	snapshot, err := actor.TakeSnapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create recovery point: %w", err)
	}

	rm.mu.Lock()
	rm.recoveryLog = append(rm.recoveryLog, RecoveryLogEntry{
		Timestamp:    time.Now(),
		ActorID:      actorID,
		Operation:    "create_recovery_point",
		Success:      true,
		SnapshotID:   snapshot.ID(),
		StateType:    snapshot.State().Type(),
		StateVersion: snapshot.State().Version(),
	})
	rm.mu.Unlock()

	return snapshot.ID(), nil
}

// RollbackToRecoveryPoint 回滚到恢复点
func (rm *RecoveryManager) RollbackToRecoveryPoint(ctx context.Context, snapshotID string, actor StatefulActor) error {
	if rm.snapshotManager == nil {
		return fmt.Errorf("snapshot manager not available")
	}

	// 获取快照
	snapshot, err := rm.snapshotManager.Get(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	// 从快照恢复
	if err := actor.RestoreFromSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	rm.mu.Lock()
	rm.recoveryLog = append(rm.recoveryLog, RecoveryLogEntry{
		Timestamp:    time.Now(),
		ActorID:      snapshot.ActorID(),
		Operation:    "rollback_to_recovery_point",
		Success:      true,
		SnapshotID:   snapshotID,
		StateType:    snapshot.State().Type(),
		StateVersion: snapshot.State().Version(),
	})
	rm.mu.Unlock()

	return nil
}

// GetRecoveryLog 获取恢复日志
func (rm *RecoveryManager) GetRecoveryLog() []RecoveryLogEntry {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 返回日志副本
	log := make([]RecoveryLogEntry, len(rm.recoveryLog))
	copy(log, rm.recoveryLog)
	return log
}

// ClearRecoveryLog 清除恢复日志
func (rm *RecoveryManager) ClearRecoveryLog() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.recoveryLog = make([]RecoveryLogEntry, 0)
}

// JSONStateSerializer JSON状态序列化器
type JSONStateSerializer struct{}

// NewJSONStateSerializer 创建新的JSON状态序列化器
func NewJSONStateSerializer() *JSONStateSerializer {
	return &JSONStateSerializer{}
}

// Serialize 序列化状态
func (s *JSONStateSerializer) Serialize(state State) ([]byte, error) {
	data := map[string]interface{}{
		"type":      state.Type(),
		"version":   state.Version(),
		"timestamp": state.Timestamp(),
		"data":      state.Data(),
	}

	return json.Marshal(data)
}

// Deserialize 反序列化状态
func (s *JSONStateSerializer) Deserialize(data []byte) (State, error) {
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("failed to deserialize state: %w", err)
	}

	// 提取数据
	stateType, _ := decoded["type"].(string)
	version, _ := decoded["version"].(float64)
	timestampStr, _ := decoded["timestamp"].(string)
	stateData := decoded["data"]

	// 解析时间戳
	var timestamp time.Time
	if timestampStr != "" {
		var err error
		timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			timestamp = time.Now()
		}
	} else {
		timestamp = time.Now()
	}

	// 创建状态
	state := &BaseState{
		stateType: stateType,
		version:   int64(version),
		timestamp: timestamp,
		data:      stateData,
	}

	return state, nil
}

// StateRecoveryConfig 状态恢复配置
type StateRecoveryConfig struct {
	// AutoRecoveryEnabled 是否启用自动恢复
	AutoRecoveryEnabled bool
	// MaxRecoveryAttempts 最大恢复尝试次数
	MaxRecoveryAttempts int
	// RecoveryTimeout 恢复超时时间
	RecoveryTimeout time.Duration
	// SnapshotBeforeRecovery 恢复前是否创建快照
	SnapshotBeforeRecovery bool
	// LogRecoveryOperations 是否记录恢复操作
	LogRecoveryOperations bool
}

// DefaultRecoveryConfig 返回默认恢复配置
func DefaultRecoveryConfig() *StateRecoveryConfig {
	return &StateRecoveryConfig{
		AutoRecoveryEnabled:    true,
		MaxRecoveryAttempts:    3,
		RecoveryTimeout:        30 * time.Second,
		SnapshotBeforeRecovery: true,
		LogRecoveryOperations:  true,
	}
}

// StateRecoveryService 状态恢复服务
type StateRecoveryService struct {
	recoveryManager *RecoveryManager
	config          *StateRecoveryConfig
	mu              sync.RWMutex
}

// NewStateRecoveryService 创建新的状态恢复服务
func NewStateRecoveryService(recoveryManager *RecoveryManager, config *StateRecoveryConfig) *StateRecoveryService {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	return &StateRecoveryService{
		recoveryManager: recoveryManager,
		config:          config,
	}
}

// PerformRecovery 执行恢复
func (s *StateRecoveryService) PerformRecovery(ctx context.Context, actorID string, actor StatefulActor) error {
	if !s.config.AutoRecoveryEnabled {
		return fmt.Errorf("auto recovery is disabled")
	}

	// 设置超时上下文
	recoveryCtx, cancel := context.WithTimeout(ctx, s.config.RecoveryTimeout)
	defer cancel()

	// 如果需要，在恢复前创建快照
	if s.config.SnapshotBeforeRecovery {
		if _, err := s.recoveryManager.CreateRecoveryPoint(recoveryCtx, actorID, actor); err != nil && s.config.LogRecoveryOperations {
			// 记录错误但不中断恢复
		}
	}

	// 执行恢复
	var lastErr error
	for attempt := 1; attempt <= s.config.MaxRecoveryAttempts; attempt++ {
		if err := s.recoveryManager.RecoverActor(recoveryCtx, actorID, actor); err != nil {
			lastErr = err
			// 等待后重试
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		return nil
	}

	return fmt.Errorf("failed to recover actor %s after %d attempts: %w", actorID, s.config.MaxRecoveryAttempts, lastErr)
}

// GetRecoveryStatus 获取恢复状态
func (s *StateRecoveryService) GetRecoveryStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log := s.recoveryManager.GetRecoveryLog()
	successCount := 0
	failureCount := 0
	for _, entry := range log {
		if entry.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	return map[string]interface{}{
		"auto_recovery_enabled": s.config.AutoRecoveryEnabled,
		"max_recovery_attempts": s.config.MaxRecoveryAttempts,
		"recovery_timeout":      s.config.RecoveryTimeout.String(),
		"total_operations":      len(log),
		"successful_operations": successCount,
		"failed_operations":     failureCount,
		"last_operation_time":   nil,
	}
}

// EnableAutoRecovery 启用自动恢复
func (s *StateRecoveryService) EnableAutoRecovery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.AutoRecoveryEnabled = true
}

// DisableAutoRecovery 禁用自动恢复
func (s *StateRecoveryService) DisableAutoRecovery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.AutoRecoveryEnabled = false
}

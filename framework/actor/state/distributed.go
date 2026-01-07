package state

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DistributedStateManagerImpl 分布式状态管理器实现
type DistributedStateManagerImpl struct {
	localManager      StateManager
	replicas          []StateManager
	consistencyLevel  ConsistencyLevel
	replicationFactor int
	mu                sync.RWMutex
}

// NewDistributedStateManager 创建新的分布式状态管理器
func NewDistributedStateManager(localManager StateManager, replicas []StateManager, config *StateConfig) DistributedStateManager {
	if config == nil {
		config = DefaultConfig()
	}

	return &DistributedStateManagerImpl{
		localManager:      localManager,
		replicas:          replicas,
		consistencyLevel:  config.ConsistencyLevel,
		replicationFactor: config.ReplicationFactor,
	}
}

// Save 保存状态（分布式）
func (dm *DistributedStateManagerImpl) Save(ctx context.Context, actorID string, state State) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 保存到本地
	if err := dm.localManager.Save(ctx, actorID, state); err != nil {
		return fmt.Errorf("failed to save to local manager: %w", err)
	}

	// 根据一致性级别决定复制策略
	switch dm.consistencyLevel {
	case ConsistencyLevelStrong:
		// 强一致性：同步复制到所有副本
		return dm.replicateStrong(ctx, actorID, state)
	case ConsistencyLevelEventual:
		// 最终一致性：异步复制
		go dm.replicateEventual(ctx, actorID, state)
		return nil
	case ConsistencyLevelWeak:
		// 弱一致性：只保存到本地
		return nil
	default:
		return dm.replicateStrong(ctx, actorID, state)
	}
}

// replicateStrong 强一致性复制
func (dm *DistributedStateManagerImpl) replicateStrong(ctx context.Context, actorID string, state State) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(dm.replicas))

	// 复制到所有副本
	for i, replica := range dm.replicas {
		if i >= dm.replicationFactor {
			break
		}

		wg.Add(1)
		go func(r StateManager) {
			defer wg.Done()
			if err := r.Save(ctx, actorID, state); err != nil {
				errors <- fmt.Errorf("replica save failed: %w", err)
			}
		}(replica)
	}

	// 等待所有复制完成
	wg.Wait()
	close(errors)

	// 检查是否有错误
	var firstErr error
	for err := range errors {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// replicateEventual 最终一致性复制
func (dm *DistributedStateManagerImpl) replicateEventual(ctx context.Context, actorID string, state State) {
	// 使用后台上下文
	bgCtx := context.Background()

	for i, replica := range dm.replicas {
		if i >= dm.replicationFactor {
			break
		}

		go func(r StateManager) {
			// 重试逻辑
			for attempt := 1; attempt <= 3; attempt++ {
				if err := r.Save(bgCtx, actorID, state); err == nil {
					return
				}
				time.Sleep(time.Duration(attempt) * time.Second)
			}
		}(replica)
	}
}

// Load 加载状态（分布式）
func (dm *DistributedStateManagerImpl) Load(ctx context.Context, actorID string) (State, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// 首先尝试从本地加载
	state, err := dm.localManager.Load(ctx, actorID)
	if err == nil {
		return state, nil
	}

	// 如果本地没有，根据一致性级别决定
	switch dm.consistencyLevel {
	case ConsistencyLevelStrong, ConsistencyLevelEventual:
		// 从副本加载
		return dm.loadFromReplica(ctx, actorID)
	case ConsistencyLevelWeak:
		// 弱一致性：只从本地加载
		return nil, err
	default:
		return dm.loadFromReplica(ctx, actorID)
	}
}

// loadFromReplica 从副本加载
func (dm *DistributedStateManagerImpl) loadFromReplica(ctx context.Context, actorID string) (State, error) {
	var firstErr error

	// 尝试从每个副本加载
	for _, replica := range dm.replicas {
		state, err := replica.Load(ctx, actorID)
		if err == nil {
			// 加载成功，也保存到本地
			_ = dm.localManager.Save(ctx, actorID, state)
			return state, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	return nil, fmt.Errorf("failed to load from any replica: %w", firstErr)
}

// Delete 删除状态（分布式）
func (dm *DistributedStateManagerImpl) Delete(ctx context.Context, actorID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 从本地删除
	if err := dm.localManager.Delete(ctx, actorID); err != nil {
		return fmt.Errorf("failed to delete from local: %w", err)
	}

	// 从副本删除
	var wg sync.WaitGroup
	errors := make(chan error, len(dm.replicas))

	for _, replica := range dm.replicas {
		wg.Add(1)
		go func(r StateManager) {
			defer wg.Done()
			if err := r.Delete(ctx, actorID); err != nil {
				errors <- fmt.Errorf("replica delete failed: %w", err)
			}
		}(replica)
	}

	wg.Wait()
	close(errors)

	var firstErr error
	for err := range errors {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// List 列出所有状态
func (dm *DistributedStateManagerImpl) List(ctx context.Context) ([]string, error) {
	return dm.localManager.List(ctx)
}

// Exists 检查状态是否存在
func (dm *DistributedStateManagerImpl) Exists(ctx context.Context, actorID string) (bool, error) {
	// 首先检查本地
	exists, err := dm.localManager.Exists(ctx, actorID)
	if err == nil && exists {
		return true, nil
	}

	// 如果本地不存在，检查副本
	for _, replica := range dm.replicas {
		if exists, err := replica.Exists(ctx, actorID); err == nil && exists {
			return true, nil
		}
	}

	return false, nil
}

// Replicate 复制状态到其他节点
func (dm *DistributedStateManagerImpl) Replicate(ctx context.Context, actorID string, state State) error {
	return dm.replicateStrong(ctx, actorID, state)
}

// Sync 同步状态
func (dm *DistributedStateManagerImpl) Sync(ctx context.Context, actorID string) error {
	// 从副本加载最新状态
	state, err := dm.loadFromReplica(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// 保存到本地
	return dm.localManager.Save(ctx, actorID, state)
}

// GetConsistencyLevel 获取一致性级别
func (dm *DistributedStateManagerImpl) GetConsistencyLevel() ConsistencyLevel {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.consistencyLevel
}

// SetConsistencyLevel 设置一致性级别
func (dm *DistributedStateManagerImpl) SetConsistencyLevel(level ConsistencyLevel) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.consistencyLevel = level
}

// GetReplicationFactor 获取复制因子
func (dm *DistributedStateManagerImpl) GetReplicationFactor() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.replicationFactor
}

// SetReplicationFactor 设置复制因子
func (dm *DistributedStateManagerImpl) SetReplicationFactor(factor int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if factor < 1 {
		factor = 1
	}
	dm.replicationFactor = factor
}

// AddReplica 添加副本
func (dm *DistributedStateManagerImpl) AddReplica(replica StateManager) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.replicas = append(dm.replicas, replica)
}

// RemoveReplica 移除副本
func (dm *DistributedStateManagerImpl) RemoveReplica(index int) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if index < 0 || index >= len(dm.replicas) {
		return fmt.Errorf("invalid replica index: %d", index)
	}

	dm.replicas = append(dm.replicas[:index], dm.replicas[index+1:]...)
	return nil
}

// GetReplicaCount 获取副本数量
func (dm *DistributedStateManagerImpl) GetReplicaCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.replicas)
}

// StateSyncService 状态同步服务
type StateSyncService struct {
	distributedManager *DistributedStateManagerImpl
	syncInterval       time.Duration
	mu                 sync.RWMutex
	running            bool
	stopChan           chan struct{}
}

// NewStateSyncService 创建新的状态同步服务
func NewStateSyncService(manager *DistributedStateManagerImpl, syncInterval time.Duration) *StateSyncService {
	return &StateSyncService{
		distributedManager: manager,
		syncInterval:       syncInterval,
		stopChan:           make(chan struct{}),
	}
}

// Start 启动同步服务
func (s *StateSyncService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("sync service already running")
	}

	s.running = true
	go s.syncLoop()

	return nil
}

// Stop 停止同步服务
func (s *StateSyncService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("sync service not running")
	}

	close(s.stopChan)
	s.running = false
	return nil
}

// syncLoop 同步循环
func (s *StateSyncService) syncLoop() {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.performSync()
		}
	}
}

// performSync 执行同步
func (s *StateSyncService) performSync() {
	ctx := context.Background()

	// 获取所有本地状态
	keys, err := s.distributedManager.List(ctx)
	if err != nil {
		return
	}

	// 同步每个状态
	for _, key := range keys {
		_ = s.distributedManager.Sync(ctx, key)
	}
}

// ForceSync 强制同步指定状态
func (s *StateSyncService) ForceSync(actorID string) error {
	ctx := context.Background()
	return s.distributedManager.Sync(ctx, actorID)
}

// GetSyncStatus 获取同步状态
func (s *StateSyncService) GetSyncStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":            s.running,
		"sync_interval":      s.syncInterval.String(),
		"replica_count":      s.distributedManager.GetReplicaCount(),
		"consistency_level":  s.distributedManager.GetConsistencyLevel(),
		"replication_factor": s.distributedManager.GetReplicationFactor(),
	}
}

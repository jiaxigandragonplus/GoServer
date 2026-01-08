package state

import (
	"context"
	"time"
)

// State 表示actor的业务状态接口
type State interface {
	// Type 返回状态类型标识
	Type() string
	// Version 返回状态版本号
	Version() int64
	// Timestamp 返回状态时间戳
	Timestamp() time.Time
	// Data 返回状态数据
	Data() interface{}
	// SetData 设置状态数据
	SetData(data interface{})
	// IncrementVersion 增加版本号
	IncrementVersion()
}

// StatefulActor 支持状态管理的actor接口
type StatefulActor interface {
	// GetState 获取当前状态
	GetState() State
	// SetState 设置状态
	SetState(state State) error
	// PersistState 持久化状态
	PersistState(ctx context.Context) error
	// RestoreState 恢复状态
	RestoreState(ctx context.Context) error
	// TakeSnapshot 创建快照
	TakeSnapshot(ctx context.Context) (Snapshot, error)
	// RestoreFromSnapshot 从快照恢复
	RestoreFromSnapshot(ctx context.Context, snapshot Snapshot) error
}

// StateManager 状态管理器接口
type StateManager interface {
	// Save 保存状态
	Save(ctx context.Context, actorID string, state State) error
	// Load 加载状态
	Load(ctx context.Context, actorID string) (State, error)
	// Delete 删除状态
	Delete(ctx context.Context, actorID string) error
	// List 列出所有状态
	List(ctx context.Context) ([]string, error)
	// Exists 检查状态是否存在
	Exists(ctx context.Context, actorID string) (bool, error)
}

// Snapshot 快照接口
type Snapshot interface {
	// ID 返回快照ID
	ID() string
	// ActorID 返回actor ID
	ActorID() string
	// State 返回快照状态
	State() State
	// Timestamp 返回快照时间戳
	Timestamp() time.Time
	// Metadata 返回快照元数据
	Metadata() map[string]string
}

// SnapshotManager 快照管理器接口
type SnapshotManager interface {
	// Create 创建快照
	Create(ctx context.Context, actorID string, state State, metadata map[string]string) (Snapshot, error)
	// Get 获取快照
	Get(ctx context.Context, snapshotID string) (Snapshot, error)
	// GetLatest 获取最新快照
	GetLatest(ctx context.Context, actorID string) (Snapshot, error)
	// List 列出快照
	List(ctx context.Context, actorID string) ([]Snapshot, error)
	// Delete 删除快照
	Delete(ctx context.Context, snapshotID string) error
	// Cleanup 清理旧快照
	Cleanup(ctx context.Context, actorID string, keepCount int) error
}

// StateSerializer 状态序列化器接口
type StateSerializer interface {
	// Serialize 序列化状态
	Serialize(state State) ([]byte, error)
	// Deserialize 反序列化状态
	Deserialize(data []byte) (State, error)
}

// StateStorage 状态存储接口
type StateStorage interface {
	// Put 存储状态
	Put(ctx context.Context, key string, value []byte) error
	// Get 获取状态
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete 删除状态
	Delete(ctx context.Context, key string) error
	// List 列出所有键
	List(ctx context.Context, prefix string) ([]string, error)
}

// DistributedStateManager 分布式状态管理器接口
type DistributedStateManager interface {
	StateManager
	// Replicate 复制状态到其他节点
	Replicate(ctx context.Context, actorID string, state State) error
	// Sync 同步状态
	Sync(ctx context.Context, actorID string) error
	// GetConsistencyLevel 获取一致性级别
	GetConsistencyLevel() ConsistencyLevel
	// SetConsistencyLevel 设置一致性级别
	SetConsistencyLevel(level ConsistencyLevel)
}

// ConsistencyLevel 一致性级别
type ConsistencyLevel int

const (
	// ConsistencyLevelStrong 强一致性
	ConsistencyLevelStrong ConsistencyLevel = iota
	// ConsistencyLevelEventual 最终一致性
	ConsistencyLevelEventual
	// ConsistencyLevelWeak 弱一致性
	ConsistencyLevelWeak
)

// StateChangeEvent 状态变更事件
type StateChangeEvent struct {
	ActorID    string
	OldState   State
	NewState   State
	Timestamp  time.Time
	ChangeType StateChangeType
}

// StateChangeType 状态变更类型
type StateChangeType int

const (
	// StateChangeTypeCreate 创建
	StateChangeTypeCreate StateChangeType = iota
	// StateChangeTypeUpdate 更新
	StateChangeTypeUpdate
	// StateChangeTypeDelete 删除
	StateChangeTypeDelete
	// StateChangeTypeRestore 恢复
	StateChangeTypeRestore
)

// StateChangeHandler 状态变更处理器
type StateChangeHandler interface {
	// OnStateChanged 状态变更时调用
	OnStateChanged(event StateChangeEvent)
}

// StateConfig 状态配置
type StateConfig struct {
	// PersistenceEnabled 是否启用持久化
	PersistenceEnabled bool
	// SnapshotInterval 快照间隔
	SnapshotInterval time.Duration
	// SnapshotThreshold 快照阈值（状态变更次数）
	SnapshotThreshold int
	// StorageType 存储类型
	StorageType string
	// StorageConfig 存储配置
	StorageConfig map[string]interface{}
	// SerializerType 序列化器类型
	SerializerType string
	// ConsistencyLevel 一致性级别
	ConsistencyLevel ConsistencyLevel
	// ReplicationFactor 复制因子
	ReplicationFactor int
}

// DefaultConfig 返回默认配置
func DefaultConfig() *StateConfig {
	return &StateConfig{
		PersistenceEnabled: true,
		SnapshotInterval:   5 * time.Minute,
		SnapshotThreshold:  100,
		StorageType:        "memory",
		StorageConfig:      make(map[string]interface{}),
		SerializerType:     "json",
		ConsistencyLevel:   ConsistencyLevelStrong,
		ReplicationFactor:  3,
	}
}

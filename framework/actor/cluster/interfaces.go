package cluster

import (
	"context"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// Discovery 发现接口
type Discovery interface {
	// Start 启动发现模块
	Start(ctx context.Context) error
	// Stop 停止发现模块
	Stop() error
	// Discover 发现节点
	Discover(ctx context.Context) ([]*NodeInfo, error)
	// Register 注册自身节点
	Register(node *NodeInfo) error
	// Deregister 注销自身节点
	Deregister(nodeID string) error
}

// Transport 传输接口
type Transport interface {
	// Start 启动传输层
	Start(ctx context.Context) error
	// Stop 停止传输层
	Stop() error
	// Send 发送消息
	Send(ctx context.Context, address string, msg message.Message) error
	// Receive 接收消息
	Receive(ctx context.Context) (interface{}, error)
	// Broadcast 广播消息
	Broadcast(ctx context.Context, msg message.Message) error
	// Address 返回传输层地址
	Address() string
}

// Consensus 一致性接口
type Consensus interface {
	// Start 启动一致性模块
	Start(ctx context.Context) error
	// Stop 停止一致性模块
	Stop() error
	// Propose 提议值
	Propose(ctx context.Context, value []byte) error
	// Read 读取值
	Read(ctx context.Context) ([]byte, error)
	// Status 获取状态
	Status() ConsensusStatus
	// Leader 获取领导者
	Leader() string
	// AddNode 添加节点
	AddNode(nodeID string, address string) error
	// RemoveNode 移除节点
	RemoveNode(nodeID string) error
}

// ConsensusStatus 一致性状态
type ConsensusStatus int

const (
	// ConsensusStatusFollower 跟随者状态
	ConsensusStatusFollower ConsensusStatus = iota
	// ConsensusStatusCandidate 候选者状态
	ConsensusStatusCandidate
	// ConsensusStatusLeader 领导者状态
	ConsensusStatusLeader
	// ConsensusStatusStopped 已停止
	ConsensusStatusStopped
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	// HandleMessage 处理消息
	HandleMessage(ctx context.Context, msg interface{}) error
}

// ClusterMessage 集群消息接口
type ClusterMessage interface {
	// Type 返回消息类型
	Type() string
	// Sender 返回发送者
	Sender() string
	// Receiver 返回接收者
	Receiver() string
	// Data 返回数据
	Data() []byte
	// Timestamp 返回时间戳
	Timestamp() time.Time
}

// HeartbeatMessage 心跳消息
type HeartbeatMessage struct {
	NodeID  string
	MsgTime time.Time
	Status  ClusterStatus
	Role    NodeRole
}

func (m *HeartbeatMessage) Type() string {
	return "heartbeat"
}

func (m *HeartbeatMessage) Sender() string {
	return m.NodeID
}

func (m *HeartbeatMessage) Receiver() string {
	return ""
}

func (m *HeartbeatMessage) Data() []byte {
	return []byte{}
}

func (m *HeartbeatMessage) Timestamp() time.Time {
	return m.MsgTime
}

// JoinMessage 加入消息
type JoinMessage struct {
	Node    *NodeInfo
	MsgTime time.Time
}

func (m *JoinMessage) Type() string {
	return "join"
}

func (m *JoinMessage) Sender() string {
	return m.Node.ID
}

func (m *JoinMessage) Receiver() string {
	return ""
}

func (m *JoinMessage) Data() []byte {
	return []byte{}
}

func (m *JoinMessage) Timestamp() time.Time {
	return m.MsgTime
}

// LeaveMessage 离开消息
type LeaveMessage struct {
	NodeID  string
	MsgTime time.Time
}

func (m *LeaveMessage) Type() string {
	return "leave"
}

func (m *LeaveMessage) Sender() string {
	return m.NodeID
}

func (m *LeaveMessage) Receiver() string {
	return ""
}

func (m *LeaveMessage) Data() []byte {
	return []byte{}
}

func (m *LeaveMessage) Timestamp() time.Time {
	return m.MsgTime
}

// ActorMessage Actor消息
type ActorMessage struct {
	Message message.Message
	MsgTime time.Time
}

func (m *ActorMessage) Type() string {
	return "actor"
}

func (m *ActorMessage) Sender() string {
	if m.Message.Sender() != nil {
		return m.Message.Sender().String()
	}
	return ""
}

func (m *ActorMessage) Receiver() string {
	if m.Message.Receiver() != nil {
		return m.Message.Receiver().String()
	}
	return ""
}

func (m *ActorMessage) Data() []byte {
	// 在实际实现中，这里应该序列化消息
	return []byte{}
}

func (m *ActorMessage) Timestamp() time.Time {
	return m.MsgTime
}

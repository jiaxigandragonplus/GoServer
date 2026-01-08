package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RaftConsensus Raft一致性实现
type RaftConsensus struct {
	config      *ClusterConfig
	transport   Transport
	state       RaftState
	stateMu     sync.RWMutex
	nodes       map[string]*RaftNode
	nodesMu     sync.RWMutex
	selfID      string
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	log         []*LogEntry
	logMu       sync.RWMutex
	commitIndex int
	lastApplied int
	nextIndex   map[string]int
	matchIndex  map[string]int
	votes       map[string]bool
	votesMu     sync.Mutex
}

// RaftState Raft状态
type RaftState int

const (
	// RaftStateFollower 跟随者状态
	RaftStateFollower RaftState = iota
	// RaftStateCandidate 候选者状态
	RaftStateCandidate
	// RaftStateLeader 领导者状态
	RaftStateLeader
)

// RaftNode Raft节点
type RaftNode struct {
	ID      string
	Address string
	Role    NodeRole
}

// LogEntry 日志条目
type LogEntry struct {
	Term    int
	Index   int
	Command []byte
}

// RequestVoteArgs 请求投票参数
type RequestVoteArgs struct {
	Term         int
	CandidateID  string
	LastLogIndex int
	LastLogTerm  int
}

// RequestVoteReply 请求投票回复
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

// AppendEntriesArgs 追加条目参数
type AppendEntriesArgs struct {
	Term         int
	LeaderID     string
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []*LogEntry
	LeaderCommit int
}

// AppendEntriesReply 追加条目回复
type AppendEntriesReply struct {
	Term    int
	Success bool
}

// NewRaftConsensus 创建新的Raft共识
func NewRaftConsensus(config *ClusterConfig, transport Transport) (*RaftConsensus, error) {
	ctx, cancel := context.WithCancel(context.Background())

	rc := &RaftConsensus{
		config:      config,
		transport:   transport,
		state:       RaftStateFollower,
		nodes:       make(map[string]*RaftNode),
		selfID:      config.NodeID,
		ctx:         ctx,
		cancel:      cancel,
		log:         make([]*LogEntry, 0),
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make(map[string]int),
		matchIndex:  make(map[string]int),
		votes:       make(map[string]bool),
	}

	// 添加自身节点
	rc.nodes[config.NodeID] = &RaftNode{
		ID:      config.NodeID,
		Address: fmt.Sprintf("%s:%d", config.Host, config.Port),
		Role:    NodeRoleFollower,
	}

	return rc, nil
}

// Start 启动共识模块
func (rc *RaftConsensus) Start(ctx context.Context) error {
	rc.ctx, rc.cancel = context.WithCancel(ctx)

	// 启动选举超时定时器
	rc.wg.Add(1)
	go rc.electionTimeoutLoop()

	// 启动心跳定时器（如果是领导者）
	rc.wg.Add(1)
	go rc.heartbeatLoop()

	// 启动日志应用循环
	rc.wg.Add(1)
	go rc.applyLogLoop()

	return nil
}

// Stop 停止共识模块
func (rc *RaftConsensus) Stop() error {
	if rc.cancel != nil {
		rc.cancel()
	}

	rc.wg.Wait()
	return nil
}

// Propose 提议值
func (rc *RaftConsensus) Propose(ctx context.Context, value []byte) error {
	rc.stateMu.RLock()
	state := rc.state
	rc.stateMu.RUnlock()

	if state != RaftStateLeader {
		return fmt.Errorf("not leader, cannot propose")
	}

	// 创建日志条目
	entry := &LogEntry{
		Term:    rc.getCurrentTerm(),
		Index:   len(rc.log) + 1,
		Command: value,
	}

	// 添加到日志
	rc.logMu.Lock()
	rc.log = append(rc.log, entry)
	rc.logMu.Unlock()

	// 复制到其他节点
	return rc.replicateLog()
}

// Read 读取值
func (rc *RaftConsensus) Read(ctx context.Context) ([]byte, error) {
	// 读取已提交的日志
	rc.logMu.RLock()
	defer rc.logMu.RUnlock()

	if rc.commitIndex > 0 && rc.commitIndex <= len(rc.log) {
		return rc.log[rc.commitIndex-1].Command, nil
	}

	return nil, fmt.Errorf("no committed log entry")
}

// Status 获取状态
func (rc *RaftConsensus) Status() ConsensusStatus {
	rc.stateMu.RLock()
	defer rc.stateMu.RUnlock()

	switch rc.state {
	case RaftStateFollower:
		return ConsensusStatusFollower
	case RaftStateCandidate:
		return ConsensusStatusCandidate
	case RaftStateLeader:
		return ConsensusStatusLeader
	default:
		return ConsensusStatusStopped
	}
}

// Leader 获取领导者
func (rc *RaftConsensus) Leader() string {
	// 简化实现：返回自身ID（如果是领导者）
	rc.stateMu.RLock()
	defer rc.stateMu.RUnlock()

	if rc.state == RaftStateLeader {
		return rc.selfID
	}

	return ""
}

// AddNode 添加节点
func (rc *RaftConsensus) AddNode(nodeID string, address string) error {
	rc.nodesMu.Lock()
	defer rc.nodesMu.Unlock()

	if _, exists := rc.nodes[nodeID]; exists {
		return fmt.Errorf("node already exists: %s", nodeID)
	}

	rc.nodes[nodeID] = &RaftNode{
		ID:      nodeID,
		Address: address,
		Role:    NodeRoleFollower,
	}

	// 如果是领导者，初始化nextIndex和matchIndex
	rc.stateMu.RLock()
	if rc.state == RaftStateLeader {
		rc.nextIndex[nodeID] = len(rc.log) + 1
		rc.matchIndex[nodeID] = 0
	}
	rc.stateMu.RUnlock()

	return nil
}

// RemoveNode 移除节点
func (rc *RaftConsensus) RemoveNode(nodeID string) error {
	rc.nodesMu.Lock()
	defer rc.nodesMu.Unlock()

	if _, exists := rc.nodes[nodeID]; !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	delete(rc.nodes, nodeID)

	// 清理相关索引
	delete(rc.nextIndex, nodeID)
	delete(rc.matchIndex, nodeID)

	return nil
}

// electionTimeoutLoop 选举超时循环
func (rc *RaftConsensus) electionTimeoutLoop() {
	defer rc.wg.Done()

	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
			// 随机选举超时时间
			timeout := rc.config.ElectionTimeout + time.Duration(randInt(0, 500))*time.Millisecond
			timer := time.NewTimer(timeout)

			select {
			case <-rc.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				// 检查是否需要开始选举
				rc.startElection()
			}
		}
	}
}

// heartbeatLoop 心跳循环
func (rc *RaftConsensus) heartbeatLoop() {
	defer rc.wg.Done()

	ticker := time.NewTicker(rc.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-ticker.C:
			rc.sendHeartbeats()
		}
	}
}

// applyLogLoop 应用日志循环
func (rc *RaftConsensus) applyLogLoop() {
	defer rc.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-ticker.C:
			rc.applyCommittedLogs()
		}
	}
}

// startElection 开始选举
func (rc *RaftConsensus) startElection() {
	rc.stateMu.Lock()
	defer rc.stateMu.Unlock()

	// 只有跟随者可以开始选举
	if rc.state != RaftStateFollower {
		return
	}

	// 转换为候选者
	rc.state = RaftStateCandidate

	// 增加当前任期
	currentTerm := rc.getCurrentTerm()
	rc.setCurrentTerm(currentTerm + 1)

	// 投票给自己
	rc.votesMu.Lock()
	rc.votes[rc.selfID] = true
	rc.votesMu.Unlock()

	// 向其他节点请求投票
	rc.requestVotes()
}

// requestVotes 请求投票
func (rc *RaftConsensus) requestVotes() {
	rc.nodesMu.RLock()
	nodes := make([]*RaftNode, 0, len(rc.nodes))
	for _, node := range rc.nodes {
		if node.ID != rc.selfID {
			nodes = append(nodes, node)
		}
	}
	rc.nodesMu.RUnlock()

	currentTerm := rc.getCurrentTerm()
	lastLogIndex, lastLogTerm := rc.getLastLogInfo()

	args := &RequestVoteArgs{
		Term:         currentTerm,
		CandidateID:  rc.selfID,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	for _, node := range nodes {
		go rc.sendRequestVote(node, args)
	}
}

// sendRequestVote 发送请求投票
func (rc *RaftConsensus) sendRequestVote(node *RaftNode, args *RequestVoteArgs) {
	// 在实际实现中，这里应该通过RPC发送请求
	// 这里使用简化实现
	reply := &RequestVoteReply{
		Term:        args.Term,
		VoteGranted: true, // 简化：总是同意
	}

	rc.handleRequestVoteReply(node.ID, reply)
}

// handleRequestVoteReply 处理请求投票回复
func (rc *RaftConsensus) handleRequestVoteReply(nodeID string, reply *RequestVoteReply) {
	rc.stateMu.RLock()
	state := rc.state
	rc.stateMu.RUnlock()

	// 如果不是候选者，忽略回复
	if state != RaftStateCandidate {
		return
	}

	// 如果回复的任期更大，转换为跟随者
	if reply.Term > rc.getCurrentTerm() {
		rc.becomeFollower(reply.Term)
		return
	}

	// 如果获得投票
	if reply.VoteGranted {
		rc.votesMu.Lock()
		rc.votes[nodeID] = true
		voteCount := len(rc.votes)
		rc.votesMu.Unlock()

		// 检查是否获得大多数投票
		rc.nodesMu.RLock()
		totalNodes := len(rc.nodes)
		rc.nodesMu.RUnlock()

		if voteCount > totalNodes/2 {
			rc.becomeLeader()
		}
	}
}

// becomeFollower 转换为跟随者
func (rc *RaftConsensus) becomeFollower(term int) {
	rc.stateMu.Lock()
	rc.state = RaftStateFollower
	rc.setCurrentTerm(term)
	rc.stateMu.Unlock()

	// 清空投票
	rc.votesMu.Lock()
	rc.votes = make(map[string]bool)
	rc.votesMu.Unlock()
}

// becomeLeader 转换为领导者
func (rc *RaftConsensus) becomeLeader() {
	rc.stateMu.Lock()
	rc.state = RaftStateLeader
	rc.stateMu.Unlock()

	// 初始化nextIndex和matchIndex
	rc.nodesMu.RLock()
	for _, node := range rc.nodes {
		if node.ID != rc.selfID {
			rc.nextIndex[node.ID] = len(rc.log) + 1
			rc.matchIndex[node.ID] = 0
		}
	}
	rc.nodesMu.RUnlock()

	// 发送初始心跳
	rc.sendHeartbeats()
}

// sendHeartbeats 发送心跳
func (rc *RaftConsensus) sendHeartbeats() {
	rc.stateMu.RLock()
	state := rc.state
	rc.stateMu.RUnlock()

	// 只有领导者发送心跳
	if state != RaftStateLeader {
		return
	}

	rc.nodesMu.RLock()
	nodes := make([]*RaftNode, 0, len(rc.nodes))
	for _, node := range rc.nodes {
		if node.ID != rc.selfID {
			nodes = append(nodes, node)
		}
	}
	rc.nodesMu.RUnlock()

	currentTerm := rc.getCurrentTerm()
	leaderID := rc.selfID
	leaderCommit := rc.commitIndex

	for _, node := range nodes {
		go rc.sendAppendEntries(node, currentTerm, leaderID, leaderCommit)
	}
}

// sendAppendEntries 发送追加条目
func (rc *RaftConsensus) sendAppendEntries(node *RaftNode, term int, leaderID string, leaderCommit int) {
	// 获取nextIndex
	nextIdx := rc.nextIndex[node.ID]
	prevLogIndex := nextIdx - 1
	prevLogTerm := 0

	// 获取前一条日志的任期
	if prevLogIndex > 0 {
		rc.logMu.RLock()
		if prevLogIndex <= len(rc.log) {
			prevLogTerm = rc.log[prevLogIndex-1].Term
		}
		rc.logMu.RUnlock()
	}

	// 获取要发送的日志条目
	var entries []*LogEntry
	if nextIdx <= len(rc.log) {
		rc.logMu.RLock()
		entries = rc.log[nextIdx-1:]
		rc.logMu.RUnlock()
	}

	args := &AppendEntriesArgs{
		Term:         term,
		LeaderID:     leaderID,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: leaderCommit,
	}

	// 在实际实现中，这里应该通过RPC发送
	// 这里使用简化实现
	reply := &AppendEntriesReply{
		Term:    term,
		Success: true, // 简化：总是成功
	}

	rc.handleAppendEntriesReply(node.ID, args, reply)
}

// handleAppendEntriesReply 处理追加条目回复
func (rc *RaftConsensus) handleAppendEntriesReply(nodeID string, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rc.stateMu.RLock()
	state := rc.state
	rc.stateMu.RUnlock()

	// 如果不是领导者，忽略回复
	if state != RaftStateLeader {
		return
	}

	// 如果回复的任期更大，转换为跟随者
	if reply.Term > rc.getCurrentTerm() {
		rc.becomeFollower(reply.Term)
		return
	}

	if reply.Success {
		// 更新matchIndex和nextIndex
		if len(args.Entries) > 0 {
			lastEntryIndex := args.PrevLogIndex + len(args.Entries)
			rc.matchIndex[nodeID] = lastEntryIndex
			rc.nextIndex[nodeID] = lastEntryIndex + 1
		}

		// 更新提交索引
		rc.updateCommitIndex()
	} else {
		// 减少nextIndex并重试
		rc.nextIndex[nodeID] = max(1, rc.nextIndex[nodeID]-1)
	}
}

// updateCommitIndex 更新提交索引
func (rc *RaftConsensus) updateCommitIndex() {
	// 收集所有matchIndex
	matchIndices := make([]int, 0, len(rc.matchIndex)+1)
	matchIndices = append(matchIndices, len(rc.log)) // 领导者自身的日志长度

	rc.nodesMu.RLock()
	for _, node := range rc.nodes {
		if node.ID != rc.selfID {
			if idx, exists := rc.matchIndex[node.ID]; exists {
				matchIndices = append(matchIndices, idx)
			}
		}
	}
	rc.nodesMu.RUnlock()

	// 排序并找到中位数
	sortInts(matchIndices)
	N := matchIndices[len(matchIndices)/2]

	// 如果N大于当前提交索引，并且日志[N]的任期等于当前任期
	if N > rc.commitIndex {
		rc.logMu.RLock()
		if N <= len(rc.log) && rc.log[N-1].Term == rc.getCurrentTerm() {
			rc.commitIndex = N
		}
		rc.logMu.RUnlock()
	}
}

// applyCommittedLogs 应用已提交的日志
func (rc *RaftConsensus) applyCommittedLogs() {
	for rc.lastApplied < rc.commitIndex {
		rc.lastApplied++

		rc.logMu.RLock()
		if rc.lastApplied <= len(rc.log) {
			entry := rc.log[rc.lastApplied-1]
			// 在实际实现中，这里应该应用日志条目
			// 例如：执行状态机命令
			_ = entry.Command
		}
		rc.logMu.RUnlock()
	}
}

// replicateLog 复制日志到其他节点
func (rc *RaftConsensus) replicateLog() error {
	// 发送追加条目给所有跟随者
	rc.sendHeartbeats()
	return nil
}

// getCurrentTerm 获取当前任期
func (rc *RaftConsensus) getCurrentTerm() int {
	// 简化实现：使用配置中的值
	// 在实际实现中，应该持久化存储任期
	return 1
}

// setCurrentTerm 设置当前任期
func (rc *RaftConsensus) setCurrentTerm(term int) {
	// 简化实现
	// 在实际实现中，应该持久化存储任期
}

// getLastLogInfo 获取最后一条日志信息
func (rc *RaftConsensus) getLastLogInfo() (index int, term int) {
	rc.logMu.RLock()
	defer rc.logMu.RUnlock()

	if len(rc.log) == 0 {
		return 0, 0
	}

	lastLog := rc.log[len(rc.log)-1]
	return lastLog.Index, lastLog.Term
}

// GossipConsensus Gossip一致性实现（简化）
type GossipConsensus struct {
	config  *ClusterConfig
	state   map[string][]byte
	stateMu sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewGossipConsensus 创建新的Gossip共识
func NewGossipConsensus(config *ClusterConfig) (*GossipConsensus, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &GossipConsensus{
		config: config,
		state:  make(map[string][]byte),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start 启动Gossip共识
func (gc *GossipConsensus) Start(ctx context.Context) error {
	gc.ctx, gc.cancel = context.WithCancel(ctx)

	// 启动Gossip传播循环
	gc.wg.Add(1)
	go gc.gossipLoop()

	return nil
}

// Stop 停止Gossip共识
func (gc *GossipConsensus) Stop() error {
	if gc.cancel != nil {
		gc.cancel()
	}

	gc.wg.Wait()
	return nil
}

// Propose 提议值
func (gc *GossipConsensus) Propose(ctx context.Context, value []byte) error {
	// 设置状态
	key := "consensus-value"
	gc.stateMu.Lock()
	gc.state[key] = value
	gc.stateMu.Unlock()

	return nil
}

// Read 读取值
func (gc *GossipConsensus) Read(ctx context.Context) ([]byte, error) {
	gc.stateMu.RLock()
	defer gc.stateMu.RUnlock()

	if value, exists := gc.state["consensus-value"]; exists {
		return value, nil
	}

	return nil, fmt.Errorf("no value found")
}

// Status 获取状态
func (gc *GossipConsensus) Status() ConsensusStatus {
	return ConsensusStatusFollower
}

// Leader 获取领导者
func (gc *GossipConsensus) Leader() string {
	return ""
}

// AddNode 添加节点
func (gc *GossipConsensus) AddNode(nodeID string, address string) error {
	return nil
}

// RemoveNode 移除节点
func (gc *GossipConsensus) RemoveNode(nodeID string) error {
	return nil
}

// gossipLoop Gossip传播循环
func (gc *GossipConsensus) gossipLoop() {
	defer gc.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-gc.ctx.Done():
			return
		case <-ticker.C:
			// 在实际实现中，这里应该随机选择节点交换状态
			// 这里使用简化实现
		}
	}
}

// 辅助函数
func randInt(min, max int) int {
	return min + int(time.Now().UnixNano()%int64(max-min))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sortInts(arr []int) {
	for i := 0; i < len(arr); i++ {
		for j := i + 1; j < len(arr); j++ {
			if arr[i] > arr[j] {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}
}

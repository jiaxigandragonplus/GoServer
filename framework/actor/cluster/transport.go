package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// TCPTransport TCP传输实现
type TCPTransport struct {
	host       string
	port       int
	listener   net.Listener
	clients    map[string]net.Conn
	clientsMu  sync.RWMutex
	handlers   map[string]MessageHandler
	handlersMu sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	msgChan    chan interface{}
}

// NewTCPTransport 创建新的TCP传输
func NewTCPTransport(host string, port int) (*TCPTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	transport := &TCPTransport{
		host:     host,
		port:     port,
		clients:  make(map[string]net.Conn),
		handlers: make(map[string]MessageHandler),
		ctx:      ctx,
		cancel:   cancel,
		msgChan:  make(chan interface{}, 100),
	}

	return transport, nil
}

// Start 启动传输层
func (t *TCPTransport) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	// 启动TCP服务器
	addr := fmt.Sprintf("%s:%d", t.host, t.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	t.listener = listener

	// 启动接受连接循环
	t.wg.Add(1)
	go t.acceptLoop()

	// 启动消息处理循环
	t.wg.Add(1)
	go t.processMessages()

	return nil
}

// Stop 停止传输层
func (t *TCPTransport) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}

	// 关闭监听器
	if t.listener != nil {
		t.listener.Close()
	}

	// 关闭所有客户端连接
	t.clientsMu.Lock()
	for addr, conn := range t.clients {
		conn.Close()
		delete(t.clients, addr)
	}
	t.clientsMu.Unlock()

	// 等待所有goroutine结束
	t.wg.Wait()

	// 关闭消息通道
	close(t.msgChan)

	return nil
}

// Send 发送消息
func (t *TCPTransport) Send(ctx context.Context, address string, msg message.Message) error {
	// 连接到目标地址
	conn, err := t.getOrCreateConnection(address)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// 创建集群消息
	clusterMsg := &ActorMessage{
		Message: msg,
		MsgTime: time.Now(),
	}

	// 序列化消息
	data, err := t.serializeMessage(clusterMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// 发送消息
	_, err = conn.Write(data)
	if err != nil {
		// 连接可能已关闭，移除并重试
		t.removeConnection(address)
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// Receive 接收消息
func (t *TCPTransport) Receive(ctx context.Context) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-t.msgChan:
		return msg, nil
	}
}

// Broadcast 广播消息
func (t *TCPTransport) Broadcast(ctx context.Context, msg message.Message) error {
	t.clientsMu.RLock()
	clients := make([]net.Conn, 0, len(t.clients))
	for _, conn := range t.clients {
		clients = append(clients, conn)
	}
	t.clientsMu.RUnlock()

	var firstErr error
	for _, conn := range clients {
		// 创建集群消息
		clusterMsg := &ActorMessage{
			Message: msg,
			MsgTime: time.Now(),
		}

		// 序列化消息
		data, err := t.serializeMessage(clusterMsg)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// 发送消息
		if _, err := conn.Write(data); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// Address 返回传输层地址
func (t *TCPTransport) Address() string {
	return fmt.Sprintf("%s:%d", t.host, t.port)
}

// RegisterHandler 注册消息处理器
func (t *TCPTransport) RegisterHandler(msgType string, handler MessageHandler) {
	t.handlersMu.Lock()
	t.handlers[msgType] = handler
	t.handlersMu.Unlock()
}

// UnregisterHandler 注销消息处理器
func (t *TCPTransport) UnregisterHandler(msgType string) {
	t.handlersMu.Lock()
	delete(t.handlers, msgType)
	t.handlersMu.Unlock()
}

// acceptLoop 接受连接循环
func (t *TCPTransport) acceptLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			conn, err := t.listener.Accept()
			if err != nil {
				if t.ctx.Err() != nil {
					return
				}
				continue
			}

			// 处理新连接
			t.wg.Add(1)
			go t.handleConnection(conn)
		}
	}
}

// handleConnection 处理连接
func (t *TCPTransport) handleConnection(conn net.Conn) {
	defer t.wg.Done()
	defer conn.Close()

	addr := conn.RemoteAddr().String()

	// 添加到客户端列表
	t.clientsMu.Lock()
	t.clients[addr] = conn
	t.clientsMu.Unlock()

	// 连接关闭时移除
	defer func() {
		t.clientsMu.Lock()
		delete(t.clients, addr)
		t.clientsMu.Unlock()
	}()

	buffer := make([]byte, 4096)
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// 设置读取超时
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			n, err := conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// 超时，继续循环
					continue
				}
				// 连接关闭或错误
				return
			}

			if n == 0 {
				continue
			}

			// 处理接收到的数据
			data := buffer[:n]
			if err := t.processData(data); err != nil {
				// 记录错误但继续处理
				continue
			}
		}
	}
}

// processData 处理接收到的数据
func (t *TCPTransport) processData(data []byte) error {
	// 反序列化消息
	msg, err := t.deserializeMessage(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize message: %w", err)
	}

	// 发送到消息通道
	select {
	case t.msgChan <- msg:
		return nil
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
}

// processMessages 处理消息
func (t *TCPTransport) processMessages() {
	defer t.wg.Done()

	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-t.msgChan:
			t.handleMessage(msg)
		}
	}
}

// handleMessage 处理消息
func (t *TCPTransport) handleMessage(msg interface{}) {
	// 根据消息类型调用相应的处理器
	switch m := msg.(type) {
	case *HeartbeatMessage:
		t.handlersMu.RLock()
		handler, exists := t.handlers["heartbeat"]
		t.handlersMu.RUnlock()
		if exists {
			handler.HandleMessage(t.ctx, m)
		}
	case *JoinMessage:
		t.handlersMu.RLock()
		handler, exists := t.handlers["join"]
		t.handlersMu.RUnlock()
		if exists {
			handler.HandleMessage(t.ctx, m)
		}
	case *LeaveMessage:
		t.handlersMu.RLock()
		handler, exists := t.handlers["leave"]
		t.handlersMu.RUnlock()
		if exists {
			handler.HandleMessage(t.ctx, m)
		}
	case *ActorMessage:
		t.handlersMu.RLock()
		handler, exists := t.handlers["actor"]
		t.handlersMu.RUnlock()
		if exists {
			handler.HandleMessage(t.ctx, m)
		}
	}
}

// getOrCreateConnection 获取或创建连接
func (t *TCPTransport) getOrCreateConnection(address string) (net.Conn, error) {
	t.clientsMu.RLock()
	conn, exists := t.clients[address]
	t.clientsMu.RUnlock()

	if exists {
		return conn, nil
	}

	// 创建新连接
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// 添加到客户端列表
	t.clientsMu.Lock()
	t.clients[address] = conn
	t.clientsMu.Unlock()

	return conn, nil
}

// removeConnection 移除连接
func (t *TCPTransport) removeConnection(address string) {
	t.clientsMu.Lock()
	if conn, exists := t.clients[address]; exists {
		conn.Close()
		delete(t.clients, address)
	}
	t.clientsMu.Unlock()
}

// serializeMessage 序列化消息
func (t *TCPTransport) serializeMessage(msg interface{}) ([]byte, error) {
	// 使用JSON序列化
	return json.Marshal(msg)
}

// deserializeMessage 反序列化消息
func (t *TCPTransport) deserializeMessage(data []byte) (interface{}, error) {
	// 尝试解析为不同类型的消息
	var msgType struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &msgType); err != nil {
		return nil, err
	}

	switch msgType.Type {
	case "heartbeat":
		var msg HeartbeatMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "join":
		var msg JoinMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "leave":
		var msg LeaveMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "actor":
		var msg ActorMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType.Type)
	}
}

// SimpleTransport 简单传输实现（用于测试）
type SimpleTransport struct {
	address    string
	handlers   map[string]MessageHandler
	handlersMu sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	msgChan    chan interface{}
}

// NewSimpleTransport 创建新的简单传输
func NewSimpleTransport(address string) (*SimpleTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &SimpleTransport{
		address:  address,
		handlers: make(map[string]MessageHandler),
		ctx:      ctx,
		cancel:   cancel,
		msgChan:  make(chan interface{}, 100),
	}, nil
}

// Start 启动简单传输
func (st *SimpleTransport) Start(ctx context.Context) error {
	st.ctx, st.cancel = context.WithCancel(ctx)
	return nil
}

// Stop 停止简单传输
func (st *SimpleTransport) Stop() error {
	if st.cancel != nil {
		st.cancel()
	}
	close(st.msgChan)
	return nil
}

// Send 发送消息（模拟实现）
func (st *SimpleTransport) Send(ctx context.Context, address string, msg message.Message) error {
	// 模拟发送消息
	clusterMsg := &ActorMessage{
		Message: msg,
		MsgTime: time.Now(),
	}

	// 在实际实现中，这里应该通过网络发送
	// 这里只是将消息放入通道
	select {
	case st.msgChan <- clusterMsg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive 接收消息
func (st *SimpleTransport) Receive(ctx context.Context) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-st.msgChan:
		return msg, nil
	}
}

// Broadcast 广播消息（模拟实现）
func (st *SimpleTransport) Broadcast(ctx context.Context, msg message.Message) error {
	// 模拟广播
	clusterMsg := &ActorMessage{
		Message: msg,
		MsgTime: time.Now(),
	}

	select {
	case st.msgChan <- clusterMsg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Address 返回地址
func (st *SimpleTransport) Address() string {
	return st.address
}

// RegisterHandler 注册处理器
func (st *SimpleTransport) RegisterHandler(msgType string, handler MessageHandler) {
	st.handlersMu.Lock()
	st.handlers[msgType] = handler
	st.handlersMu.Unlock()
}

// UnregisterHandler 注销处理器
func (st *SimpleTransport) UnregisterHandler(msgType string) {
	st.handlersMu.Lock()
	delete(st.handlers, msgType)
	st.handlersMu.Unlock()
}

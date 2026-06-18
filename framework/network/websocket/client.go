package websocket

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

// WebSocketClient WebSocket 客户端
type WebSocketClient struct {
	addr       string
	eventMutex sync.Mutex
	// 回调函数保存在客户端上，以便在 Connect 之前就能设置
	onMessage func([]byte)
	onClose   func()
	onError   func(error)

	// stateMu 保护 conn 和 closed，避免 Close 与 receiveLoop/发送之间的数据竞争
	stateMu sync.Mutex
	conn    *WebSocketConn
	closed  bool
}

// NewWebSocketClient 创建新的 WebSocket 客户端
func NewWebSocketClient(addr string) *WebSocketClient {
	return &WebSocketClient{
		addr: addr,
	}
}

// Connect 连接到 WebSocket 服务器
func (wc *WebSocketClient) Connect() error {
	// 创建握手请求
	request := wc.createHandshakeRequest()

	// 使用 net.Dial 连接到服务器
	conn, err := net.Dial("tcp", wc.addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}

	// 发送握手请求
	// 限制日志长度
	logLen := 100
	if len(request) < logLen {
		logLen = len(request)
	}
	log.Printf("Sending handshake request (first %d chars): %s", logLen, request[:logLen])
	_, err = conn.Write([]byte(request))
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to send handshake request: %v", err)
	}

	// 读取握手响应
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to read handshake response: %v", err)
	}

	response := string(buf[:n])

	// 验证握手响应 - 暂时只检查 101 Switching Protocols
	// 完整的 Sec-WebSocket-Accept 验证可以在后续版本中添加
	if !strings.Contains(response, "101 Switching Protocols") {
		// 限制错误消息长度
		maxLen := 100
		if len(response) < maxLen {
			maxLen = len(response)
		}
		conn.Close()
		return fmt.Errorf("handshake failed: invalid response: %s", response[:maxLen])
	}

	// 创建 WebSocket 连接对象
	// 注意：回调保存在 WebSocketClient 上（见 onMessage/onClose/onError 字段），
	// 这样在 Connect 之前调用 SetOnXxx 也能生效。
	wc.stateMu.Lock()
	wc.conn = &WebSocketConn{
		conn:     conn,
		isServer: false,
	}
	wc.closed = false
	wc.stateMu.Unlock()

	// 启动接收循环
	go wc.receiveLoop()

	log.Printf("Connected to WebSocket server at %s", wc.addr)
	return nil
}

// createHandshakeRequest 创建 WebSocket 握手请求
func (wc *WebSocketClient) createHandshakeRequest() string {
	// 生成随机的 Sec-WebSocket-Key
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	secWebSocketKey := base64.StdEncoding.EncodeToString(keyBytes)

	return fmt.Sprintf(
		"GET / HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n"+
			"User-Agent: WebSocketClient/1.0\r\n"+
			"\r\n",
		wc.addr, secWebSocketKey,
	)
}

// SetOnMessage 设置消息回调
func (wc *WebSocketClient) SetOnMessage(callback func([]byte)) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()
	wc.onMessage = callback
}

// SetOnClose 设置关闭回调
func (wc *WebSocketClient) SetOnClose(callback func()) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()
	wc.onClose = callback
}

// SetOnError 设置错误回调
func (wc *WebSocketClient) SetOnError(callback func(error)) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()
	wc.onError = callback
}

// getOnMessage 等访问器在持有锁的情况下读取回调，避免与 SetOnXxx 的数据竞争
func (wc *WebSocketClient) getOnMessage() func([]byte) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()
	return wc.onMessage
}

func (wc *WebSocketClient) getOnClose() func() {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()
	return wc.onClose
}

func (wc *WebSocketClient) getOnError() func(error) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()
	return wc.onError
}

// SendText 发送文本消息
func (wc *WebSocketClient) SendText(text string) error {
	return wc.sendFrame(OpCodeText, []byte(text))
}

// SendBinary 发送二进制消息
func (wc *WebSocketClient) SendBinary(data []byte) error {
	return wc.sendFrame(OpCodeBinary, data)
}

// sendFrame 构建并发送一帧（客户端帧必须掩码）。写操作经 WebSocketConn.send 串行化。
func (wc *WebSocketClient) sendFrame(opcode byte, payload []byte) error {
	wc.stateMu.Lock()
	conn := wc.conn
	closed := wc.closed
	wc.stateMu.Unlock()

	if conn == nil || closed {
		return fmt.Errorf("not connected to server")
	}
	return conn.send(opcode, payload)
}

// Ping 发送 Ping 消息
func (wc *WebSocketClient) Ping(data []byte) error {
	return wc.sendFrame(OpCodePing, data)
}

// Close 关闭连接。可安全地被多次/并发调用，onClose 只会触发一次。
func (wc *WebSocketClient) Close() error {
	wc.stateMu.Lock()
	if wc.closed || wc.conn == nil {
		wc.stateMu.Unlock()
		return nil
	}
	wc.closed = true
	conn := wc.conn
	wc.stateMu.Unlock()

	// 发送关闭帧（best-effort）
	if err := conn.send(OpCodeClose, nil); err != nil {
		log.Printf("Error sending close frame: %v", err)
	}

	// 调用关闭回调（仅一次）
	if cb := wc.getOnClose(); cb != nil {
		cb()
	}

	// 关闭底层连接
	if c, ok := conn.conn.(net.Conn); ok {
		return c.Close()
	}
	return nil
}

// IsConnected 检查是否已连接
func (wc *WebSocketClient) IsConnected() bool {
	wc.stateMu.Lock()
	defer wc.stateMu.Unlock()
	return wc.conn != nil && !wc.closed
}

// receiveLoop 接收循环（在单独的 goroutine 中运行）
func (wc *WebSocketClient) receiveLoop() {
	wc.stateMu.Lock()
	conn := wc.conn
	wc.stateMu.Unlock()
	if conn == nil {
		return
	}

	netConn, ok := conn.conn.(net.Conn)
	if !ok {
		log.Printf("Error: connection is not net.Conn")
		wc.Close()
		return
	}

	// 使用固定大小的缓冲区读取数据
	// 注意：之前用 bytebuffer.Get() 返回的是长度为 0 的切片，
	// conn.Read 会立即返回 n=0，导致永远收不到任何数据。
	buf := make([]byte, 4096)

	for {
		n, err := netConn.Read(buf)
		if err != nil {
			// 主动关闭导致的读错误不应再上报 onError
			if !wc.isClosed() {
				if cb := wc.getOnError(); cb != nil {
					cb(err)
				}
			}
			wc.Close()
			return
		}

		// 解析帧
		frames, err := parseFrames(buf[:n], conn)
		if err != nil {
			if cb := wc.getOnError(); cb != nil {
				cb(err)
			}
			wc.Close()
			return
		}

		// 处理每个帧
		for _, frame := range frames {
			if err := wc.handleFrame(conn, frame); err != nil {
				if cb := wc.getOnError(); cb != nil {
					cb(err)
				}
				wc.Close()
				return
			}
		}
	}
}

// isClosed 返回连接是否已被主动关闭
func (wc *WebSocketClient) isClosed() bool {
	wc.stateMu.Lock()
	defer wc.stateMu.Unlock()
	return wc.closed
}

// handleFrame 处理一个完整的 WebSocket 帧
func (wc *WebSocketClient) handleFrame(wsConn *WebSocketConn, frame []byte) error {
	fin, opcode, payload, err := decodeFrame(frame)
	if err != nil {
		return err
	}

	// 分片重组
	done, op, msg, err := wsConn.reassemble(fin, opcode, payload)
	if err != nil {
		return err
	}
	if !done {
		return nil
	}

	switch op {
	case OpCodeText, OpCodeBinary:
		if cb := wc.getOnMessage(); cb != nil {
			cb(msg)
		}
	case OpCodePing:
		// 自动回复 Pong
		return wc.sendFrame(OpCodePong, msg)
	case OpCodePong:
		log.Println("Received pong")
	case OpCodeClose:
		log.Println("Received close frame")
		// Close 内部会触发 onClose 回调（仅一次）
		wc.Close()
	default:
		log.Printf("Unknown opcode: %d", op)
	}

	return nil
}

package websocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
)

const (
	// WebSocket 操作码
	OpCodeContinuation = 0x0
	OpCodeText         = 0x1
	OpCodeBinary       = 0x2
	OpCodeClose        = 0x8
	OpCodePing         = 0x9
	OpCodePong         = 0xA

	// 最大帧长度
	MaxFrameSize = 1024 * 1024 // 1MB
)

// WebSocketConn 表示一个 WebSocket 连接
type WebSocketConn struct {
	conn       any // 可以是 gnet.Conn 或 net.Conn
	isServer   bool
	handshaked bool // 是否已完成握手
	buffer     []byte
	bufferSize int
	mutex      sync.Mutex // 串行化对底层连接的写操作

	// 分片重组状态
	fragmenting    bool
	fragmentOpcode byte
	fragmentBuf    []byte
}

// reassemble 处理分片帧（FIN / Continuation），把分片消息重组为完整消息。
// 返回 done=true 时，finalOpcode/finalPayload 即为一条完整消息（或一个控制帧）。
// 控制帧 (Ping/Pong/Close) 不参与分片，直接原样返回。
func (wsConn *WebSocketConn) reassemble(fin bool, opcode byte, payload []byte) (done bool, finalOpcode byte, finalPayload []byte, err error) {
	switch opcode {
	case OpCodeContinuation:
		if !wsConn.fragmenting {
			return false, 0, nil, fmt.Errorf("unexpected continuation frame")
		}
		wsConn.fragmentBuf = append(wsConn.fragmentBuf, payload...)
		if len(wsConn.fragmentBuf) > MaxFrameSize {
			wsConn.resetFragment()
			return false, 0, nil, fmt.Errorf("fragmented message exceeds max size")
		}
		if !fin {
			return false, 0, nil, nil
		}
		finalOpcode = wsConn.fragmentOpcode
		finalPayload = wsConn.fragmentBuf
		wsConn.resetFragment()
		return true, finalOpcode, finalPayload, nil

	case OpCodeText, OpCodeBinary:
		if wsConn.fragmenting {
			return false, 0, nil, fmt.Errorf("expected continuation frame, got opcode %d", opcode)
		}
		if !fin {
			// 分片消息的第一帧，开始累积
			wsConn.fragmenting = true
			wsConn.fragmentOpcode = opcode
			wsConn.fragmentBuf = append([]byte(nil), payload...)
			return false, 0, nil, nil
		}
		return true, opcode, payload, nil

	default:
		// 控制帧：Ping/Pong/Close
		return true, opcode, payload, nil
	}
}

func (wsConn *WebSocketConn) resetFragment() {
	wsConn.fragmenting = false
	wsConn.fragmentOpcode = 0
	wsConn.fragmentBuf = nil
}

// send 线程安全地向底层连接写入一帧。服务端帧不掩码，客户端帧掩码。
func (wsConn *WebSocketConn) send(opcode byte, payload []byte) error {
	frame := buildFrame(opcode, payload, !wsConn.isServer)

	wsConn.mutex.Lock()
	defer wsConn.mutex.Unlock()

	switch c := wsConn.conn.(type) {
	case gnet.Conn:
		_, err := c.Write(frame)
		return err
	case net.Conn:
		_, err := c.Write(frame)
		return err
	default:
		return fmt.Errorf("unknown connection type: %T", wsConn.conn)
	}
}

// SendText 发送文本消息
func (wsConn *WebSocketConn) SendText(text []byte) error { return wsConn.send(OpCodeText, text) }

// SendBinary 发送二进制消息
func (wsConn *WebSocketConn) SendBinary(data []byte) error { return wsConn.send(OpCodeBinary, data) }

// WebSocketServer 实现 gNet.EventHandler
type WebSocketServer struct {
	addr           string
	messageHandler WebSocketHandlerFunc
}

// WebSocketHandlerFunc 定义 WebSocket 消息处理函数类型
type WebSocketHandlerFunc func(*WebSocketConn, []byte)

// NewWebSocketServer 创建新的 WebSocket 服务器
func NewWebSocketServer(addr string) *WebSocketServer {
	return &WebSocketServer{
		addr: addr,
	}
}

// OnMessage 注册消息处理函数。未注册时，服务端默认回显 (echo) 收到的消息。
func (ws *WebSocketServer) OnMessage(handler WebSocketHandlerFunc) {
	ws.messageHandler = handler
}

// OnBoot 实现 gNet.EventHandler 接口
func (ws *WebSocketServer) OnBoot(eng gnet.Engine) gnet.Action {
	log.Printf("WebSocket server is listening on %s", ws.addr)
	return gnet.None
}

// OnShutdown 实现 gNet.EventHandler 接口
func (ws *WebSocketServer) OnShutdown(eng gnet.Engine) {
	log.Println("WebSocket server is shutting down...")
}

// OnOpen 实现 gNet.EventHandler 接口
func (ws *WebSocketServer) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	log.Printf("New connection from %s", c.RemoteAddr().String())
	// 在连接建立时创建 WebSocketConn 上下文（握手前）
	// 这样在 OnTraffic 中即使第一次读取到 0 字节，上下文也不为 nil
	wsConn := &WebSocketConn{
		conn:     c,
		isServer: true,
	}
	c.SetContext(wsConn)
	log.Printf("Initial WebSocketConn context set for %s", c.RemoteAddr().String())
	return nil, gnet.None
}

// OnClose 实现 gNet.EventHandler 接口
func (ws *WebSocketServer) OnClose(c gnet.Conn, err error) gnet.Action {
	if err != nil {
		log.Printf("Connection %s closed with error: %v", c.RemoteAddr().String(), err)
	} else {
		log.Printf("Connection %s closed", c.RemoteAddr().String())
	}
	return gnet.None
}

// OnTraffic 实现 gNet.EventHandler 接口
func (ws *WebSocketServer) OnTraffic(c gnet.Conn) gnet.Action {
	log.Printf("OnTraffic called for %s", c.RemoteAddr().String())

	// 使用固定大小的缓冲区读取数据
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		log.Printf("Read error: %v", err)
		return gnet.Close
	}

	// 如果没有数据可读，等待更多数据
	if n == 0 {
		log.Printf("No data available yet for %s, waiting...", c.RemoteAddr().String())
		return gnet.None
	}

	data := buf[:n]

	// 调试：打印接收到的数据的前几个字节
	// 简单的 min 函数实现
	printLen := 10
	if len(data) < printLen {
		printLen = len(data)
	}
	log.Printf("Received %d bytes from %s, first bytes: %v", n, c.RemoteAddr().String(), data[:printLen])
	// 同时打印为字符串，便于调试
	if printLen > 0 {
		log.Printf("Data as string (first %d chars): %s", printLen, string(data[:printLen]))
	}

	// 获取 WebSocket 连接对象
	wsConn, ok := c.Context().(*WebSocketConn)
	if !ok {
		log.Printf("Connection context is not WebSocketConn for %s, context type: %T", c.RemoteAddr().String(), c.Context())
		return gnet.Close
	}

	// 检查是否是 HTTP 请求（握手阶段）
	if bytes.HasPrefix(data, []byte("GET ")) {
		log.Printf("Handshake request detected from %s", c.RemoteAddr().String())
		// 处理 WebSocket 握手
		response, err := ws.handleHandshake(data)
		if err != nil {
			log.Printf("Handshake error: %v", err)
			return gnet.Close
		}

		_, err = c.Write(response)
		if err != nil {
			log.Printf("Write handshake response error: %v", err)
			return gnet.Close
		}

		// 标记握手完成
		wsConn.handshaked = true
		log.Printf("WebSocket handshake completed for %s", c.RemoteAddr().String())
		return gnet.None
	} else {
		// 调试：检查数据中是否包含 GET
		if bytes.Contains(data, []byte("GET")) {
			printLen := 200
			if len(data) < printLen {
				printLen = len(data)
			}
			log.Printf("Data contains 'GET' but not at prefix. Full data (first %d chars): %s", printLen, string(data[:printLen]))
		}
	}

	// 解析 WebSocket 帧
	frames, err := ws.parseFrames(data, wsConn)
	if err != nil {
		log.Printf("Parse frames error: %v", err)
		return gnet.Close
	}

	// 处理每个帧
	for _, frame := range frames {
		action := ws.handleFrame(wsConn, frame)
		if action == gnet.Close {
			return gnet.Close
		}
	}

	return gnet.None
}

// OnTick 实现 gNet.EventHandler 接口
func (ws *WebSocketServer) OnTick() (time.Duration, gnet.Action) {
	// 每秒执行一次
	return time.Second, gnet.None
}

// handleHandshake 处理 WebSocket 握手
func (ws *WebSocketServer) handleHandshake(request []byte) ([]byte, error) {
	// 解析 HTTP 请求头
	lines := strings.Split(string(request), "\r\n")
	var upgradeHeader, connectionHeader, secWebSocketKey string

	for _, line := range lines {
		if strings.HasPrefix(line, "Upgrade:") {
			upgradeHeader = strings.TrimSpace(line[8:])
		} else if strings.HasPrefix(line, "Connection:") {
			connectionHeader = strings.TrimSpace(line[11:])
		} else if strings.HasPrefix(line, "Sec-WebSocket-Key:") {
			secWebSocketKey = strings.TrimSpace(line[18:])
		}
	}

	// 验证是否是有效的 WebSocket 握手请求
	if !strings.EqualFold(upgradeHeader, "websocket") ||
		!strings.Contains(strings.ToLower(connectionHeader), "upgrade") ||
		secWebSocketKey == "" {
		return nil, fmt.Errorf("invalid websocket handshake request")
	}

	// 生成握手响应
	acceptKey := generateAcceptKey(secWebSocketKey)
	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n"+
			"\r\n",
		acceptKey,
	)

	return []byte(response), nil
}

// generateAcceptKey 生成 WebSocket 握手响应的 Accept Key
func generateAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// parseFrames 解析 WebSocket 帧（委托给包级实现，便于服务端/客户端共用）
func (ws *WebSocketServer) parseFrames(data []byte, wsConn *WebSocketConn) ([][]byte, error) {
	return parseFrames(data, wsConn)
}

// handleFrame 处理一个完整的 WebSocket 帧
func (ws *WebSocketServer) handleFrame(wsConn *WebSocketConn, frame []byte) gnet.Action {
	fin, opcode, payload, err := decodeFrame(frame)
	if err != nil {
		log.Printf("Decode frame error: %v", err)
		return gnet.Close
	}

	// 分片重组
	done, op, msg, err := wsConn.reassemble(fin, opcode, payload)
	if err != nil {
		log.Printf("Reassemble error: %v", err)
		return gnet.Close
	}
	if !done {
		return gnet.None
	}

	switch op {
	case OpCodeText, OpCodeBinary:
		if ws.messageHandler != nil {
			ws.messageHandler(wsConn, msg)
		} else {
			// 默认行为：回显
			if err := wsConn.send(op, msg); err != nil {
				log.Printf("Echo error: %v", err)
				return gnet.Close
			}
		}
	case OpCodePing:
		if err := wsConn.send(OpCodePong, msg); err != nil {
			log.Printf("Send pong error: %v", err)
			return gnet.Close
		}
	case OpCodePong:
		log.Println("Received pong")
	case OpCodeClose:
		log.Println("Received close frame")
		return gnet.Close
	default:
		log.Printf("Unknown opcode: %d", op)
	}

	return gnet.None
}

// Start 启动 WebSocket 服务器
func (ws *WebSocketServer) Start() error {
	return gnet.Run(ws, "tcp://"+ws.addr, gnet.WithMulticore(true))
}

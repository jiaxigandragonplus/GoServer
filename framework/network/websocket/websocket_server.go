package websocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/pool/bytebuffer"
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
	c          gnet.Conn
	isServer   bool
	buffer     []byte
	bufferSize int
	mutex      sync.Mutex
}

// WebSocketServer 实现 gNet.EventHandler
type WebSocketServer struct {
	addr     string
	handlers map[string]WebSocketHandlerFunc
}

// WebSocketHandlerFunc 定义 WebSocket 处理函数类型
type WebSocketHandlerFunc func(*WebSocketConn, []byte)

// NewWebSocketServer 创建新的 WebSocket 服务器
func NewWebSocketServer(addr string) *WebSocketServer {
	return &WebSocketServer{
		addr:     addr,
		handlers: make(map[string]WebSocketHandlerFunc),
	}
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
	buf := bytebuffer.Get()
	defer bytebuffer.Put(buf)

	// 读取数据
	n, err := c.Read(buf.Bytes())
	if err != nil {
		log.Printf("Read error: %v", err)
		return gnet.Close
	}

	data := buf.Bytes()[:n]

	// 检查是否是 HTTP 请求（握手阶段）
	if bytes.HasPrefix(data, []byte("GET ")) {
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

		// 创建 WebSocket 连接对象并存储到连接上下文
		wsConn := &WebSocketConn{
			c:        c,
			isServer: true,
		}
		c.SetContext(wsConn)

		log.Printf("WebSocket handshake completed for %s", c.RemoteAddr().String())
		return gnet.None
	}

	// 获取 WebSocket 连接对象
	wsConn, ok := c.Context().(*WebSocketConn)
	if !ok {
		log.Printf("Connection context is not WebSocketConn")
		return gnet.Close
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

// parseFrames 解析 WebSocket 帧
func (ws *WebSocketServer) parseFrames(data []byte, wsConn *WebSocketConn) ([][]byte, error) {
	var frames [][]byte
	buf := data

	for len(buf) > 0 {
		if len(buf) < 2 {
			// 缓冲区不足，保存剩余数据
			wsConn.buffer = make([]byte, len(buf))
			copy(wsConn.buffer, buf)
			wsConn.bufferSize = len(buf)
			break
		}

		// 解析帧头
		fin := (buf[0] & 0x80) != 0
		opcode := buf[0] & 0x0F
		masked := (buf[1] & 0x80) != 0
		payloadLen := int(buf[1] & 0x7F)

		// 计算头部长度
		headerLen := 2
		if masked {
			headerLen += 4 // mask key 长度
		}

		// 处理扩展长度字段
		if payloadLen == 126 {
			if len(buf) < headerLen+2 {
				return nil, fmt.Errorf("insufficient data for extended length")
			}
			payloadLen = int(buf[2])<<8 | int(buf[3])
			headerLen += 2
		} else if payloadLen == 127 {
			if len(buf) < headerLen+8 {
				return nil, fmt.Errorf("insufficient data for extended length")
			}
			payloadLen = int(buf[2])<<56 | int(buf[3])<<48 | int(buf[4])<<40 | int(buf[5])<<32 |
				int(buf[6])<<24 | int(buf[7])<<16 | int(buf[8])<<8 | int(buf[9])
			headerLen += 8
		}

		totalFrameLen := headerLen + payloadLen

		if len(buf) < totalFrameLen {
			// 数据不完整，保存到缓冲区
			wsConn.buffer = make([]byte, len(buf))
			copy(wsConn.buffer, buf)
			wsConn.bufferSize = len(buf)
			break
		}

		// 提取帧数据
		frame := buf[:totalFrameLen]
		frames = append(frames, frame)

		// 更新缓冲区
		buf = buf[totalFrameLen:]
	}

	// 如果还有缓冲的数据，追加到现有缓冲区
	if wsConn.bufferSize > 0 && len(buf) > 0 {
		newBuf := make([]byte, wsConn.bufferSize+len(buf))
		copy(newBuf, wsConn.buffer)
		copy(newBuf[wsConn.bufferSize:], buf)
		wsConn.bufferSize = len(newBuf)
		wsConn.buffer = newBuf
	}

	return frames, nil
}

// handleFrame 处理 WebSocket 帧
func (ws *WebSocketServer) handleFrame(wsConn *WebSocketConn, frame []byte) gnet.Action {
	// 解析帧头
	opcode := frame[0] & 0x0F
	masked := (frame[1] & 0x80) != 0
	payloadLen := int(frame[1] & 0x7F)

	// 计算头部长度
	headerLen := 2
	maskKeyStart := 0
	if masked {
		maskKeyStart = headerLen
		headerLen += 4 // mask key 长度
	}

	// 处理扩展长度字段
	if payloadLen == 126 {
		payloadLen = int(frame[2])<<8 | int(frame[3])
		headerLen += 2
		maskKeyStart += 2
	} else if payloadLen == 127 {
		payloadLen = int(frame[2])<<56 | int(frame[3])<<48 | int(frame[4])<<40 | int(frame[5])<<32 |
			int(frame[6])<<24 | int(frame[7])<<16 | int(frame[8])<<8 | int(frame[9])
		headerLen += 8
		maskKeyStart += 8
	}

	// 提取有效载荷
	payload := frame[headerLen : headerLen+payloadLen]

	// 如果帧被掩码，则解码
	if masked {
		maskKey := frame[maskKeyStart : maskKeyStart+4]
		for i := 0; i < len(payload); i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	// 处理不同操作码
	switch opcode {
	case OpCodeText:
		log.Printf("Received text message: %s", string(payload))
		// 回显消息
		err := ws.sendText(wsConn.c, payload)
		if err != nil {
			log.Printf("Send text error: %v", err)
			return gnet.Close
		}
	case OpCodeBinary:
		log.Printf("Received binary message, length: %d", len(payload))
		// 回显二进制消息
		err := ws.sendBinary(wsConn.c, payload)
		if err != nil {
			log.Printf("Send binary error: %v", err)
			return gnet.Close
		}
	case OpCodePing:
		log.Println("Received ping, sending pong")
		err := ws.sendPong(wsConn.c, payload)
		if err != nil {
			log.Printf("Send pong error: %v", err)
			return gnet.Close
		}
	case OpCodePong:
		log.Println("Received pong")
	case OpCodeClose:
		log.Println("Received close frame")
		return gnet.Close
	default:
		log.Printf("Unknown opcode: %d", opcode)
	}

	return gnet.None
}

// sendFrame 发送 WebSocket 帧
func (ws *WebSocketServer) sendFrame(c gnet.Conn, opcode byte, payload []byte) error {
	// 构建帧头
	var header []byte
	payloadLen := len(payload)

	if payloadLen <= 125 {
		header = make([]byte, 2)
		header[0] = 0x80 | opcode // FIN=1, opcode
		header[1] = byte(payloadLen)
	} else if payloadLen <= 65535 {
		header = make([]byte, 4)
		header[0] = 0x80 | opcode // FIN=1, opcode
		header[1] = 126           // extended payload length
		header[2] = byte(payloadLen >> 8)
		header[3] = byte(payloadLen & 0xFF)
	} else {
		header = make([]byte, 10)
		header[0] = 0x80 | opcode // FIN=1, opcode
		header[1] = 127           // extended payload length
		header[2] = byte((payloadLen >> 56) & 0xFF)
		header[3] = byte((payloadLen >> 48) & 0xFF)
		header[4] = byte((payloadLen >> 40) & 0xFF)
		header[5] = byte((payloadLen >> 32) & 0xFF)
		header[6] = byte((payloadLen >> 24) & 0xFF)
		header[7] = byte((payloadLen >> 16) & 0xFF)
		header[8] = byte((payloadLen >> 8) & 0xFF)
		header[9] = byte(payloadLen & 0xFF)
	}

	// 合并头部和有效载荷
	frame := append(header, payload...)

	_, err := c.Write(frame)
	return err
}

// sendText 发送文本消息
func (ws *WebSocketServer) sendText(c gnet.Conn, text []byte) error {
	return ws.sendFrame(c, OpCodeText, text)
}

// sendBinary 发送二进制消息
func (ws *WebSocketServer) sendBinary(c gnet.Conn, data []byte) error {
	return ws.sendFrame(c, OpCodeBinary, data)
}

// sendPing 发送 Ping 消息
func (ws *WebSocketServer) sendPing(c gnet.Conn, data []byte) error {
	return ws.sendFrame(c, OpCodePing, data)
}

// sendPong 发送 Pong 消息
func (ws *WebSocketServer) sendPong(c gnet.Conn, data []byte) error {
	return ws.sendFrame(c, OpCodePong, data)
}

// Start 启动 WebSocket 服务器
func (ws *WebSocketServer) Start() error {
	return gnet.Run(ws, "tcp://"+ws.addr, gnet.WithMulticore(true))
}

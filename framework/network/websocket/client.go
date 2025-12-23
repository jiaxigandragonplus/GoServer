package websocket

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/panjf2000/gnet/v2/pkg/pool/bytebuffer"
)

// WebSocketClient WebSocket 客户端
type WebSocketClient struct {
	addr       string
	conn       *WebSocketConn
	eventMutex sync.Mutex
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
	wc.conn = &WebSocketConn{
		conn:      conn,
		isServer:  false,
		onMessage: func([]byte) {}, // 默认空实现
		onClose:   func() {},       // 默认空实现
		onError:   func(error) {},  // 默认空实现
	}

	// 启动接收循环
	go wc.ReceiveLoop()

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

	if wc.conn != nil {
		wc.conn.onMessage = callback
	}
}

// SetOnClose 设置关闭回调
func (wc *WebSocketClient) SetOnClose(callback func()) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()

	if wc.conn != nil {
		wc.conn.onClose = callback
	}
}

// SetOnError 设置错误回调
func (wc *WebSocketClient) SetOnError(callback func(error)) {
	wc.eventMutex.Lock()
	defer wc.eventMutex.Unlock()

	if wc.conn != nil {
		wc.conn.onError = callback
	}
}

// SendText 发送文本消息
func (wc *WebSocketClient) SendText(text string) error {
	if wc.conn == nil {
		return fmt.Errorf("not connected to server")
	}

	return wc.sendFrame(OpCodeText, []byte(text))
}

// SendBinary 发送二进制消息
func (wc *WebSocketClient) SendBinary(data []byte) error {
	if wc.conn == nil {
		return fmt.Errorf("not connected to server")
	}

	return wc.sendFrame(OpCodeBinary, data)
}

// sendFrame 发送 WebSocket 帧
func (wc *WebSocketClient) sendFrame(opcode byte, payload []byte) error {
	if wc.conn == nil {
		return fmt.Errorf("not connected to server")
	}

	// 构建帧头
	var header []byte
	payloadLen := len(payload)

	// 客户端到服务器的帧必须被掩码（设置掩码位为1）
	maskBit := byte(0x80)

	if payloadLen <= 125 {
		header = make([]byte, 6)  // 2字节头部 + 4字节掩码密钥
		header[0] = 0x80 | opcode // FIN=1, opcode
		header[1] = maskBit | byte(payloadLen)
	} else if payloadLen <= 65535 {
		header = make([]byte, 8)  // 4字节头部 + 4字节掩码密钥
		header[0] = 0x80 | opcode // FIN=1, opcode
		header[1] = maskBit | 126 // extended payload length
		header[2] = byte(payloadLen >> 8)
		header[3] = byte(payloadLen & 0xFF)
	} else {
		header = make([]byte, 14) // 10字节头部 + 4字节掩码密钥
		header[0] = 0x80 | opcode // FIN=1, opcode
		header[1] = maskBit | 127 // extended payload length
		header[2] = byte((payloadLen >> 56) & 0xFF)
		header[3] = byte((payloadLen >> 48) & 0xFF)
		header[4] = byte((payloadLen >> 40) & 0xFF)
		header[5] = byte((payloadLen >> 32) & 0xFF)
		header[6] = byte((payloadLen >> 24) & 0xFF)
		header[7] = byte((payloadLen >> 16) & 0xFF)
		header[8] = byte((payloadLen >> 8) & 0xFF)
		header[9] = byte(payloadLen & 0xFF)
	}

	// 生成随机掩码密钥（4字节）
	maskKey := make([]byte, 4)
	rand.Read(maskKey)

	// 将掩码密钥复制到头部
	maskKeyStart := len(header) - 4
	copy(header[maskKeyStart:], maskKey)

	// 对有效载荷进行掩码处理
	maskedPayload := make([]byte, len(payload))
	copy(maskedPayload, payload)
	for i := 0; i < len(maskedPayload); i++ {
		maskedPayload[i] ^= maskKey[i%4]
	}

	// 合并头部和掩码后的有效载荷
	frame := append(header, maskedPayload...)

	// 类型断言获取 net.Conn
	if conn, ok := wc.conn.conn.(net.Conn); ok {
		_, err := conn.Write(frame)
		return err
	}
	return fmt.Errorf("connection is not net.Conn")
}

// Ping 发送 Ping 消息
func (wc *WebSocketClient) Ping(data []byte) error {
	if wc.conn == nil {
		return fmt.Errorf("not connected to server")
	}

	return wc.sendFrame(OpCodePing, data)
}

// Close 关闭连接
func (wc *WebSocketClient) Close() error {
	if wc.conn == nil {
		return nil
	}

	// 发送关闭帧
	err := wc.sendFrame(OpCodeClose, []byte{})
	if err != nil {
		log.Printf("Error sending close frame: %v", err)
	}

	// 调用关闭回调
	if wc.conn.onClose != nil {
		wc.conn.onClose()
	}

	// 关闭底层连接
	if conn, ok := wc.conn.conn.(net.Conn); ok {
		err = conn.Close()
	} else {
		err = fmt.Errorf("connection is not net.Conn")
	}
	wc.conn = nil

	return err
}

// IsConnected 检查是否已连接
func (wc *WebSocketClient) IsConnected() bool {
	return wc.conn != nil
}

// receiveLoop 接收循环（在单独的 goroutine 中运行）
func (wc *WebSocketClient) ReceiveLoop() {
	if wc.conn == nil {
		return
	}

	buf := bytebuffer.Get()
	defer bytebuffer.Put(buf)

	for {
		// 读取数据
		var n int
		var err error
		if conn, ok := wc.conn.conn.(net.Conn); ok {
			n, err = conn.Read(buf.Bytes())
			if err != nil {
				if wc.conn.onError != nil {
					wc.conn.onError(err)
				}
				wc.Close()
				return
			}
		} else {
			log.Printf("Error: connection is not net.Conn")
			wc.Close()
			return
		}

		data := buf.Bytes()[:n]

		// 解析帧
		frames, err := wc.parseFrames(data, wc.conn)
		if err != nil {
			if wc.conn.onError != nil {
				wc.conn.onError(err)
			}
			wc.Close()
			return
		}

		// 处理每个帧
		for _, frame := range frames {
			err := wc.handleFrame(wc.conn, frame)
			if err != nil {
				if wc.conn.onError != nil {
					wc.conn.onError(err)
				}
				wc.Close()
				return
			}
		}
	}
}

// parseFrames 解析 WebSocket 帧
func (wc *WebSocketClient) parseFrames(data []byte, wsConn *WebSocketConn) ([][]byte, error) {
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
		_ = (buf[0] & 0x80) != 0 // fin (未使用)
		_ = buf[0] & 0x0F        // opcode (未使用)
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
func (wc *WebSocketClient) handleFrame(wsConn *WebSocketConn, frame []byte) error {
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

	// 如果帧被掩码，则解码（服务器到客户端的帧不应该被掩码）
	if masked {
		// 根据 WebSocket 规范，服务器到客户端的帧不应被掩码
		// 如果检测到掩码，可能是恶意行为
		log.Printf("Warning: Received masked frame from server, which is not allowed by WebSocket protocol")
		maskKey := frame[maskKeyStart : maskKeyStart+4]
		for i := 0; i < len(payload); i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	// 处理不同操作码
	switch opcode {
	case OpCodeText:
		log.Printf("Received text message: %s", string(payload))
		if wsConn.onMessage != nil {
			wsConn.onMessage(payload)
		}
	case OpCodeBinary:
		log.Printf("Received binary message, length: %d", len(payload))
		if wsConn.onMessage != nil {
			wsConn.onMessage(payload)
		}
	case OpCodePing:
		log.Println("Received ping, sending pong")
		// 自动回复 Pong
		err := wc.sendFrame(OpCodePong, payload)
		if err != nil {
			return err
		}
	case OpCodePong:
		log.Println("Received pong")
	case OpCodeClose:
		log.Println("Received close frame")
		if wsConn.onClose != nil {
			wsConn.onClose()
		}
		wc.Close()
		return nil
	default:
		log.Printf("Unknown opcode: %d", opcode)
	}

	return nil
}

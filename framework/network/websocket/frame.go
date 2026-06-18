package websocket

import (
	"crypto/rand"
	"fmt"
)

// decodeFrame 解析一个【完整】的 WebSocket 帧，返回 FIN 标志、操作码和（已解掩码、且为独立拷贝的）有效载荷。
//
// 与 parseFrames 不同，这里假定 frame 已经是一个完整的帧（由 parseFrames 切分得到）。
// 返回的 payload 是一份独立拷贝，调用方可安全持有/追加，不会被底层读缓冲区覆盖，
// 这对分片重组（reassemble 中的 append）尤其重要。
func decodeFrame(frame []byte) (fin bool, opcode byte, payload []byte, err error) {
	if len(frame) < 2 {
		return false, 0, nil, fmt.Errorf("frame too short: %d bytes", len(frame))
	}

	fin = frame[0]&0x80 != 0
	opcode = frame[0] & 0x0F
	masked := frame[1]&0x80 != 0
	payloadLen := int(frame[1] & 0x7F)
	offset := 2

	switch payloadLen {
	case 126:
		if len(frame) < 4 {
			return false, 0, nil, fmt.Errorf("frame too short for 16-bit length")
		}
		payloadLen = int(frame[2])<<8 | int(frame[3])
		offset = 4
	case 127:
		if len(frame) < 10 {
			return false, 0, nil, fmt.Errorf("frame too short for 64-bit length")
		}
		payloadLen = int(frame[2])<<56 | int(frame[3])<<48 | int(frame[4])<<40 | int(frame[5])<<32 |
			int(frame[6])<<24 | int(frame[7])<<16 | int(frame[8])<<8 | int(frame[9])
		offset = 10
	}

	// 防御：64 位扩展长度最高位被置位会得到负数；超大长度则可能触发巨量分配 (DoS)
	if payloadLen < 0 || payloadLen > MaxFrameSize {
		return false, 0, nil, fmt.Errorf("invalid payload length: %d", payloadLen)
	}

	var maskKey []byte
	if masked {
		if len(frame) < offset+4 {
			return false, 0, nil, fmt.Errorf("frame too short for mask key")
		}
		maskKey = frame[offset : offset+4]
		offset += 4
	}

	if len(frame) < offset+payloadLen {
		return false, 0, nil, fmt.Errorf("frame too short for payload: need %d, have %d", offset+payloadLen, len(frame))
	}

	// 拷贝一份，避免直接引用底层读缓冲区
	payload = make([]byte, payloadLen)
	copy(payload, frame[offset:offset+payloadLen])

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return fin, opcode, payload, nil
}

// buildFrame 构建一个 WebSocket 帧。masked 为 true 时（客户端→服务端）会生成随机掩码并对载荷掩码。
func buildFrame(opcode byte, payload []byte, masked bool) []byte {
	payloadLen := len(payload)

	var maskBit byte
	if masked {
		maskBit = 0x80
	}

	var header []byte
	switch {
	case payloadLen <= 125:
		header = make([]byte, 2)
		header[1] = maskBit | byte(payloadLen)
	case payloadLen <= 65535:
		header = make([]byte, 4)
		header[1] = maskBit | 126
		header[2] = byte(payloadLen >> 8)
		header[3] = byte(payloadLen & 0xFF)
	default:
		header = make([]byte, 10)
		header[1] = maskBit | 127
		header[2] = byte((payloadLen >> 56) & 0xFF)
		header[3] = byte((payloadLen >> 48) & 0xFF)
		header[4] = byte((payloadLen >> 40) & 0xFF)
		header[5] = byte((payloadLen >> 32) & 0xFF)
		header[6] = byte((payloadLen >> 24) & 0xFF)
		header[7] = byte((payloadLen >> 16) & 0xFF)
		header[8] = byte((payloadLen >> 8) & 0xFF)
		header[9] = byte(payloadLen & 0xFF)
	}
	header[0] = 0x80 | opcode // FIN=1, opcode

	if !masked {
		frame := make([]byte, len(header)+payloadLen)
		copy(frame, header)
		copy(frame[len(header):], payload)
		return frame
	}

	// 掩码帧：头部 + 4 字节掩码密钥 + 掩码后的载荷
	maskKey := make([]byte, 4)
	rand.Read(maskKey)

	frame := make([]byte, len(header)+4+payloadLen)
	copy(frame, header)
	copy(frame[len(header):], maskKey)
	base := len(header) + 4
	for i := 0; i < payloadLen; i++ {
		frame[base+i] = payload[i] ^ maskKey[i%4]
	}
	return frame
}

// parseFrames 从字节流中切分出完整的 WebSocket 帧。
//
// 关键点：
//  1. 每次解析前先把上一次读取剩余的不完整数据 (wsConn.buffer) 拼接到本次数据前面，
//     这样被 TCP 拆分到多次读取的帧才能正确重组（修复之前保存的 buffer 从未被取回导致的丢帧）。
//  2. 当扩展长度字段或整帧数据还不完整时，保存剩余数据并退出等待后续数据，而不是关闭连接。
//  3. 对 payload 长度做上限校验，避免恶意超大/负长度造成的无限缓冲或 panic。
func parseFrames(data []byte, wsConn *WebSocketConn) ([][]byte, error) {
	// 把上次剩余的不完整数据拼到本次数据前面
	if wsConn.bufferSize > 0 {
		combined := make([]byte, wsConn.bufferSize+len(data))
		copy(combined, wsConn.buffer[:wsConn.bufferSize])
		copy(combined[wsConn.bufferSize:], data)
		data = combined
		wsConn.buffer = nil
		wsConn.bufferSize = 0
	}

	var frames [][]byte
	buf := data

	for len(buf) > 0 {
		// 帧头至少需要 2 字节
		if len(buf) < 2 {
			break
		}

		masked := (buf[1] & 0x80) != 0
		payloadLen := int(buf[1] & 0x7F)

		// 计算头部长度（含掩码密钥）
		headerLen := 2
		if masked {
			headerLen += 4
		}

		// 处理扩展长度字段（数据不足时退出等待后续数据）
		if payloadLen == 126 {
			if len(buf) < 4 {
				break
			}
			payloadLen = int(buf[2])<<8 | int(buf[3])
			headerLen += 2
		} else if payloadLen == 127 {
			if len(buf) < 10 {
				break
			}
			payloadLen = int(buf[2])<<56 | int(buf[3])<<48 | int(buf[4])<<40 | int(buf[5])<<32 |
				int(buf[6])<<24 | int(buf[7])<<16 | int(buf[8])<<8 | int(buf[9])
			headerLen += 8
		}

		// 防御非法/超大长度
		if payloadLen < 0 || payloadLen > MaxFrameSize {
			return nil, fmt.Errorf("frame payload length %d out of range", payloadLen)
		}

		totalFrameLen := headerLen + payloadLen

		// 整帧尚未到齐，退出等待后续数据
		if len(buf) < totalFrameLen {
			break
		}

		frames = append(frames, buf[:totalFrameLen])
		buf = buf[totalFrameLen:]
	}

	// 保存剩余的不完整数据，等待下次读取时拼接
	if len(buf) > 0 {
		wsConn.buffer = make([]byte, len(buf))
		copy(wsConn.buffer, buf)
		wsConn.bufferSize = len(buf)
	}

	return frames, nil
}

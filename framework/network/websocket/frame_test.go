package websocket

import (
	"bytes"
	"testing"
)

// makeFrame 构建一个测试帧，可显式控制 FIN 位（buildFrame 默认 FIN=1）。
func makeFrame(fin bool, opcode byte, payload []byte, masked bool) []byte {
	f := buildFrame(opcode, payload, masked)
	if !fin {
		f[0] &^= 0x80 // 清除 FIN 位
	}
	return f
}

// ---------------------------------------------------------------------------
// buildFrame / decodeFrame 往返
// ---------------------------------------------------------------------------

func TestBuildDecodeRoundTrip(t *testing.T) {
	sizes := []int{0, 1, 5, 125, 126, 127, 200, 65535, 65536, 100000}
	for _, masked := range []bool{false, true} {
		for _, size := range sizes {
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i % 251)
			}

			frame := buildFrame(OpCodeBinary, payload, masked)
			fin, opcode, got, err := decodeFrame(frame)
			if err != nil {
				t.Fatalf("size=%d masked=%v: decode error: %v", size, masked, err)
			}
			if !fin {
				t.Errorf("size=%d masked=%v: expected FIN=true", size, masked)
			}
			if opcode != OpCodeBinary {
				t.Errorf("size=%d masked=%v: opcode=%d, want %d", size, masked, opcode, OpCodeBinary)
			}
			if !bytes.Equal(got, payload) {
				t.Errorf("size=%d masked=%v: payload mismatch", size, masked)
			}
		}
	}
}

func TestDecodeFrameUnmasksPayload(t *testing.T) {
	payload := []byte("hello websocket")
	frame := buildFrame(OpCodeText, payload, true) // 掩码帧

	// 掩码帧的载荷在线缆上不应等于明文
	if bytes.Contains(frame[6:], payload) {
		t.Fatalf("masked frame should not contain plaintext payload")
	}

	_, _, got, err := decodeFrame(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("unmasked payload = %q, want %q", got, payload)
	}
}

func TestDecodeFrameReturnsCopy(t *testing.T) {
	// decodeFrame 返回的 payload 必须是独立拷贝，修改它不应影响原帧
	payload := []byte("abc")
	frame := buildFrame(OpCodeText, payload, false)
	_, _, got, err := decodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	got[0] = 'X'
	// 原帧载荷起始于第 2 字节（无掩码、短载荷）
	if frame[2] == 'X' {
		t.Errorf("decodeFrame should return a copy, but modifying it mutated the source frame")
	}
}

func TestDecodeFrameErrors(t *testing.T) {
	cases := map[string][]byte{
		"too short (0)":         {},
		"too short (1)":         {0x81},
		"payload shorter than declared": {0x81, 0x05, 'a', 'b'}, // 声明 5 字节实际 2
		"16-bit header truncated":       {0x81, 126, 0x00},      // 缺一个长度字节
	}
	for name, frame := range cases {
		if _, _, _, err := decodeFrame(frame); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

// ---------------------------------------------------------------------------
// parseFrames —— 粘包 (multiple frames in one read)
// ---------------------------------------------------------------------------

func TestParseFramesSingle(t *testing.T) {
	conn := &WebSocketConn{}
	frame := buildFrame(OpCodeText, []byte("hi"), true)

	frames, err := parseFrames(frame, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	if conn.bufferSize != 0 {
		t.Errorf("leftover buffer should be empty, got %d", conn.bufferSize)
	}
}

func TestParseFramesSticky(t *testing.T) {
	conn := &WebSocketConn{}
	payloads := [][]byte{[]byte("one"), []byte("two"), []byte("three")}

	// 把三帧粘在一起一次性投喂
	var stream []byte
	for _, p := range payloads {
		stream = append(stream, buildFrame(OpCodeText, p, true)...)
	}

	frames, err := parseFrames(stream, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want 3", len(frames))
	}
	for i, f := range frames {
		_, _, got, err := decodeFrame(f)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, payloads[i]) {
			t.Errorf("frame %d = %q, want %q", i, got, payloads[i])
		}
	}
	if conn.bufferSize != 0 {
		t.Errorf("leftover should be empty, got %d", conn.bufferSize)
	}
}

// ---------------------------------------------------------------------------
// parseFrames —— 分包 (one frame split across multiple reads)
// ---------------------------------------------------------------------------

func TestParseFramesSplitByteByByte(t *testing.T) {
	conn := &WebSocketConn{}
	payload := []byte("a reasonably sized websocket message payload")
	frame := buildFrame(OpCodeText, payload, true)

	var collected [][]byte
	// 每次只投喂 1 个字节，模拟极端分包
	for i := 0; i < len(frame); i++ {
		frames, err := parseFrames(frame[i:i+1], conn)
		if err != nil {
			t.Fatalf("byte %d: %v", i, err)
		}
		collected = append(collected, frames...)
	}

	if len(collected) != 1 {
		t.Fatalf("got %d frames, want exactly 1 (only after last byte)", len(collected))
	}
	_, _, got, err := decodeFrame(collected[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %q, want %q", got, payload)
	}
	if conn.bufferSize != 0 {
		t.Errorf("leftover should be empty after full frame, got %d", conn.bufferSize)
	}
}

func TestParseFramesSplitInExtendedLength(t *testing.T) {
	conn := &WebSocketConn{}
	// 用 300 字节载荷触发 16 位扩展长度（126）
	payload := bytes.Repeat([]byte("x"), 300)
	frame := buildFrame(OpCodeBinary, payload, true)

	// 第一次只投喂 3 字节：足以读到 opcode+长度首字节，但扩展长度未完整
	frames, err := parseFrames(frame[:3], conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 0 {
		t.Fatalf("should not yield a frame yet, got %d", len(frames))
	}
	// 投喂剩余部分
	frames, err = parseFrames(frame[3:], conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	_, _, got, err := decodeFrame(frames[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch after reassembly")
	}
}

func TestParseFramesSplitThenSticky(t *testing.T) {
	// 综合场景：半个帧 + （另半个帧 + 一个完整帧）
	conn := &WebSocketConn{}
	f1 := buildFrame(OpCodeText, []byte("first-message"), true)
	f2 := buildFrame(OpCodeText, []byte("second"), true)

	split := len(f1) / 2

	frames, err := parseFrames(f1[:split], conn)
	if err != nil || len(frames) != 0 {
		t.Fatalf("partial frame should yield 0, got %d err=%v", len(frames), err)
	}

	// 后半个 f1 + 完整 f2 一起到达
	rest := append(append([]byte{}, f1[split:]...), f2...)
	frames, err = parseFrames(rest, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	wants := []string{"first-message", "second"}
	for i, f := range frames {
		_, _, got, _ := decodeFrame(f)
		if string(got) != wants[i] {
			t.Errorf("frame %d = %q, want %q", i, got, wants[i])
		}
	}
}

// ---------------------------------------------------------------------------
// parseFrames —— 防御性长度校验
// ---------------------------------------------------------------------------

func TestParseFramesOversizedLength(t *testing.T) {
	conn := &WebSocketConn{}
	// 127 扩展长度，声明约 4GB，远超 MaxFrameSize
	frame := []byte{0x82, 127, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF}
	if _, err := parseFrames(frame, conn); err == nil {
		t.Fatal("expected error for oversized payload length")
	}
}

func TestParseFramesNegativeLength(t *testing.T) {
	conn := &WebSocketConn{}
	// 127 扩展长度，最高位置 1 → int 解析为负数
	frame := []byte{0x82, 127, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	if _, err := parseFrames(frame, conn); err == nil {
		t.Fatal("expected error for negative payload length")
	}
}

func TestParseFramesEmpty(t *testing.T) {
	conn := &WebSocketConn{}
	frames, err := parseFrames(nil, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 0 {
		t.Errorf("got %d frames, want 0", len(frames))
	}
}

// ---------------------------------------------------------------------------
// reassemble —— 分片消息 (FIN / Continuation)
// ---------------------------------------------------------------------------

func TestReassembleNonFragmented(t *testing.T) {
	conn := &WebSocketConn{}
	done, op, msg, err := conn.reassemble(true, OpCodeText, []byte("complete"))
	if err != nil {
		t.Fatal(err)
	}
	if !done || op != OpCodeText || string(msg) != "complete" {
		t.Errorf("non-fragmented: done=%v op=%d msg=%q", done, op, msg)
	}
}

func TestReassembleFragmented(t *testing.T) {
	conn := &WebSocketConn{}

	// 第一帧：text, FIN=0
	done, _, _, err := conn.reassemble(false, OpCodeText, []byte("Hel"))
	if err != nil || done {
		t.Fatalf("first fragment: done=%v err=%v", done, err)
	}
	// 中间帧：continuation, FIN=0
	done, _, _, err = conn.reassemble(false, OpCodeContinuation, []byte("lo "))
	if err != nil || done {
		t.Fatalf("middle fragment: done=%v err=%v", done, err)
	}
	// 末帧：continuation, FIN=1
	done, op, msg, err := conn.reassemble(true, OpCodeContinuation, []byte("World"))
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("expected done=true on final fragment")
	}
	if op != OpCodeText {
		t.Errorf("reassembled opcode = %d, want %d (original text)", op, OpCodeText)
	}
	if string(msg) != "Hello World" {
		t.Errorf("reassembled msg = %q, want %q", msg, "Hello World")
	}
	if conn.fragmenting {
		t.Error("fragment state should be reset after completion")
	}
}

func TestReassembleUnexpectedContinuation(t *testing.T) {
	conn := &WebSocketConn{}
	// 没有开始分片就收到 continuation → 协议错误
	if _, _, _, err := conn.reassemble(true, OpCodeContinuation, []byte("x")); err == nil {
		t.Fatal("expected error for unexpected continuation frame")
	}
}

func TestReassembleNewDataWhileFragmenting(t *testing.T) {
	conn := &WebSocketConn{}
	// 开始一个分片消息
	if _, _, _, err := conn.reassemble(false, OpCodeText, []byte("part")); err != nil {
		t.Fatal(err)
	}
	// 分片未结束又来一个新的 data 帧（非 continuation）→ 协议错误
	if _, _, _, err := conn.reassemble(true, OpCodeText, []byte("new")); err == nil {
		t.Fatal("expected error for interleaved data frame during fragmentation")
	}
}

func TestReassembleControlPassthrough(t *testing.T) {
	conn := &WebSocketConn{}
	// 在分片过程中收到控制帧（ping），应原样直通且不破坏分片状态
	if _, _, _, err := conn.reassemble(false, OpCodeText, []byte("frag")); err != nil {
		t.Fatal(err)
	}
	done, op, msg, err := conn.reassemble(true, OpCodePing, []byte("ping-data"))
	if err != nil {
		t.Fatal(err)
	}
	if !done || op != OpCodePing || string(msg) != "ping-data" {
		t.Errorf("ping passthrough: done=%v op=%d msg=%q", done, op, msg)
	}
	if !conn.fragmenting {
		t.Error("fragment state must survive an interleaved control frame")
	}
	// 继续完成分片
	done, op, msg, err = conn.reassemble(true, OpCodeContinuation, []byte("-end"))
	if err != nil {
		t.Fatal(err)
	}
	if !done || op != OpCodeText || string(msg) != "frag-end" {
		t.Errorf("after control frame, reassembly = done=%v op=%d msg=%q", done, op, msg)
	}
}

// ---------------------------------------------------------------------------
// 端到端：消息 → 帧 → 任意分块的字节流 → 还原
// 同时覆盖粘包、分包、分片三种情况。
// ---------------------------------------------------------------------------

func TestStreamPipeline(t *testing.T) {
	conn := &WebSocketConn{}

	// 构造一段字节流：
	//  - 一条普通文本消息
	//  - 一条由 3 个分片组成的文本消息
	//  - 一条二进制消息
	var stream []byte
	stream = append(stream, buildFrame(OpCodeText, []byte("alpha"), true)...)

	// 分片消息 "beta-gamma-delta"
	stream = append(stream, makeFrame(false, OpCodeText, []byte("beta-"), true)...)
	stream = append(stream, makeFrame(false, OpCodeContinuation, []byte("gamma-"), true)...)
	stream = append(stream, makeFrame(true, OpCodeContinuation, []byte("delta"), true)...)

	stream = append(stream, buildFrame(OpCodeBinary, bytes.Repeat([]byte{0xAB}, 400), true)...)

	want := []string{"alpha", "beta-gamma-delta"}
	wantBinaryLen := 400

	var gotMsgs []string
	var gotBinaryLen int

	// 以参差不齐的块大小投喂，模拟真实 TCP 分包+粘包
	chunkSizes := []int{1, 2, 3, 5, 7, 11, 13, 50, 999}
	ci := 0
	for offset := 0; offset < len(stream); {
		size := chunkSizes[ci%len(chunkSizes)]
		ci++
		end := offset + size
		if end > len(stream) {
			end = len(stream)
		}

		frames, err := parseFrames(stream[offset:end], conn)
		if err != nil {
			t.Fatalf("parseFrames at offset %d: %v", offset, err)
		}
		for _, f := range frames {
			fin, opcode, payload, err := decodeFrame(f)
			if err != nil {
				t.Fatalf("decodeFrame: %v", err)
			}
			done, op, msg, err := conn.reassemble(fin, opcode, payload)
			if err != nil {
				t.Fatalf("reassemble: %v", err)
			}
			if !done {
				continue
			}
			switch op {
			case OpCodeText:
				gotMsgs = append(gotMsgs, string(msg))
			case OpCodeBinary:
				gotBinaryLen = len(msg)
			}
		}
		offset = end
	}

	if len(gotMsgs) != len(want) {
		t.Fatalf("got %d text messages %v, want %d %v", len(gotMsgs), gotMsgs, len(want), want)
	}
	for i := range want {
		if gotMsgs[i] != want[i] {
			t.Errorf("text msg %d = %q, want %q", i, gotMsgs[i], want[i])
		}
	}
	if gotBinaryLen != wantBinaryLen {
		t.Errorf("binary length = %d, want %d", gotBinaryLen, wantBinaryLen)
	}
}

package websocket

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/gnet/v2"
)

// fakeServer 是一个最小的原始 TCP WebSocket 服务端，仅用于测试客户端。
// 它完成握手，读取客户端的第一帧作为「开始」信号，然后向客户端推送若干帧。
type fakeServer struct {
	ln   net.Listener
	push func(c net.Conn) // 握手并收到客户端首帧后，由测试决定推送什么
}

func newFakeServer(t *testing.T, push func(c net.Conn)) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	fs := &fakeServer{ln: ln, push: push}
	go fs.serve(t)
	return fs
}

func (fs *fakeServer) addr() string { return fs.ln.Addr().String() }

func (fs *fakeServer) serve(t *testing.T) {
	conn, err := fs.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	// 读取握手请求直到空行
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if line == "\r\n" {
			break
		}
	}
	// 回复 101
	_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: test\r\n\r\n"))

	// 读取客户端发来的「开始」帧（已被 bufio 缓冲一部分，这里继续从 conn 读）
	// 客户端会在握手后调用 SendText 作为同步点，确保 101 响应已被完全消费。
	startBuf := make([]byte, 4096)
	wsc := &WebSocketConn{}
	// 把 bufio 中已缓冲的数据先取出
	if n := r.Buffered(); n > 0 {
		buffered := make([]byte, n)
		_, _ = r.Read(buffered)
		if frames, _ := parseFrames(buffered, wsc); len(frames) > 0 {
			// 已拿到开始帧
			fs.push(conn)
			return
		}
	}
	for {
		n, err := conn.Read(startBuf)
		if err != nil {
			return
		}
		frames, _ := parseFrames(startBuf[:n], wsc)
		if len(frames) > 0 {
			break
		}
	}
	fs.push(conn)
}

// TestClientReceivesMessages 验证修复 #1（接收循环能读到数据）和 #2（Connect 前设置的回调生效）。
func TestClientReceivesMessages(t *testing.T) {
	received := make(chan string, 8)

	fs := newFakeServer(t, func(c net.Conn) {
		// 服务端→客户端帧不掩码
		_, _ = c.Write(buildFrame(OpCodeText, []byte("server-hello"), false))
		// 一条分片消息
		_, _ = c.Write(makeFrame(false, OpCodeText, []byte("frag-"), false))
		_, _ = c.Write(makeFrame(true, OpCodeContinuation, []byte("end"), false))
		time.Sleep(100 * time.Millisecond)
	})

	client := NewWebSocketClient(fs.addr())

	// 关键：在 Connect 之前设置回调（修复 #2 之前这会被丢弃）
	client.SetOnMessage(func(msg []byte) {
		received <- string(msg)
	})

	if err := client.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	// 发送一帧作为同步点，确保服务端已消费完握手响应
	if err := client.SendText("start"); err != nil {
		t.Fatalf("send start: %v", err)
	}

	want := []string{"server-hello", "frag-end"}
	for _, w := range want {
		select {
		case got := <-received:
			if got != w {
				t.Errorf("received %q, want %q", got, w)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %q (回调未触发说明 #1/#2 未修好)", w)
		}
	}
}

// TestClientConcurrentClose 验证修复 #8：Close 可并发/重复调用且 onClose 只触发一次。
func TestClientConcurrentClose(t *testing.T) {
	fs := newFakeServer(t, func(c net.Conn) {
		time.Sleep(200 * time.Millisecond)
	})

	var closeCount int
	var mu sync.Mutex

	client := NewWebSocketClient(fs.addr())
	client.SetOnClose(func() {
		mu.Lock()
		closeCount++
		mu.Unlock()
	})

	if err := client.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.SendText("start"); err != nil {
		t.Fatalf("send start: %v", err)
	}

	// 并发调用 Close 多次
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Close()
		}()
	}
	wg.Wait()

	if client.IsConnected() {
		t.Error("client should report disconnected after Close")
	}

	mu.Lock()
	got := closeCount
	mu.Unlock()
	if got != 1 {
		t.Errorf("onClose called %d times, want exactly 1", got)
	}
}

// TestServerHandlerDispatch 验证修复 #4：服务端 OnMessage 注册的处理函数会被调用，
// 并能通过 WebSocketConn 回写。这里直接驱动 handleFrame，避免依赖 gnet 运行时。
func TestServerHandlerDispatch(t *testing.T) {
	srv := NewWebSocketServer("127.0.0.1:0")

	var got []byte
	srv.OnMessage(func(c *WebSocketConn, msg []byte) {
		got = append([]byte(nil), msg...)
	})

	// 用一对内存连接充当底层 conn，让 handler 内若回写也不至于 panic
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	go func() {
		// 排空，避免 send 阻塞（本用例 handler 不回写，仅防御）
		sink := make([]byte, 256)
		for {
			if _, err := c2.Read(sink); err != nil {
				return
			}
		}
	}()

	wsConn := &WebSocketConn{conn: c1, isServer: true}

	// 客户端发来的帧是掩码的
	frame := buildFrame(OpCodeText, []byte("dispatch-me"), true)
	if action := srv.handleFrame(wsConn, frame); action != gnet.None {
		t.Fatalf("handleFrame returned non-None action: %v", action)
	}

	if string(got) != "dispatch-me" {
		t.Errorf("handler got %q, want %q", got, "dispatch-me")
	}
}

// TestHandshakeRequestFormat 简单校验客户端握手请求格式（防回归）。
func TestHandshakeRequestFormat(t *testing.T) {
	wc := NewWebSocketClient("example.com:9999")
	req := wc.createHandshakeRequest()
	for _, must := range []string{
		"GET / HTTP/1.1\r\n",
		"Upgrade: websocket\r\n",
		"Connection: Upgrade\r\n",
		"Sec-WebSocket-Key: ",
		"Sec-WebSocket-Version: 13\r\n",
	} {
		if !strings.Contains(req, must) {
			t.Errorf("handshake request missing %q", must)
		}
	}
}

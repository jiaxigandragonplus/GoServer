package websocket

import (
	"log"
	"testing"
	"time"
)

func TestWebSocketClient(t *testing.T) {
	// 创建 WebSocket 客户端
	client := NewWebSocketClient("localhost:8080")

	// 设置消息回调
	client.SetOnMessage(func(message []byte) {
		log.Printf("Received message: %s", string(message))
	})

	// 设置关闭回调
	client.SetOnClose(func() {
		log.Println("Connection closed")
	})

	// 设置错误回调
	client.SetOnError(func(err error) {
		log.Printf("Error: %v", err)
	})

	// 连接到服务器
	err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// 启动接收循环（在单独的 goroutine 中）
	go client.receiveLoop()

	// 发送一些测试消息
	time.Sleep(1 * time.Second)

	err = client.SendText("Hello, WebSocket Server!")
	if err != nil {
		log.Printf("Failed to send text: %v", err)
	}

	time.Sleep(1 * time.Second)

	err = client.SendText("This is a test message from Go WebSocket Client.")
	if err != nil {
		log.Printf("Failed to send text: %v", err)
	}

	// 发送二进制消息
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	err = client.SendBinary(binaryData)
	if err != nil {
		log.Printf("Failed to send binary: %v", err)
	}

	// 发送 Ping
	err = client.Ping([]byte("test-ping"))
	if err != nil {
		log.Printf("Failed to send ping: %v", err)
	}

	// 让程序运行一段时间以接收消息
	time.Sleep(10 * time.Second)

	// 关闭连接
	err = client.Close()
	if err != nil {
		log.Printf("Error closing connection: %v", err)
	}

	log.Println("Client exited")
}

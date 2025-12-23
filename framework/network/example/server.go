package main

import (
	"log"

	"github.com/GooLuck/GoServer/framework/network/websocket"
)

func main() {
	// 创建 WebSocket 服务器
	server := websocket.NewWebSocketServer("localhost:8080")

	// 启动服务器
	log.Println("Starting WebSocket server on localhost:8080...")
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

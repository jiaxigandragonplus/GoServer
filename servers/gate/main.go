package main

import (
	"time"

	"github.com/GooLuck/GoServer/framework/logger"
)

func main() {
	initLogger()
}

func initLogger() {
	// 初始化日志
	logger.InitDefaultLogger("gate", logger.DebugLevel)
	logger.Debug("this is debug log", "timestamp", time.Now().Unix())
	logger.Info("this is info log")
}

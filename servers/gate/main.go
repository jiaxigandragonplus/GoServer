package main

import (
	"time"

	"github.com/GooLuck/GoServer/framework/logger"
)

func main() {
	initLogger()

	logger.Info("gate start", "timestamp", time.Now().Unix())
}

func initLogger() {
	// 初始化日志
	logger.InitDefaultLogger("gate", logger.DebugLevel)
	logger.Debug("this is debug log")
}

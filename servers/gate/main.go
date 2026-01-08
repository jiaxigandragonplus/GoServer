package main

import "github.com/GooLuck/GoServer/framework/logger"

func main() {
	initLogger()
}

func initLogger() {
	// 初始化日志
	logger.SetLevel(logger.DebugLevel)
	logger.Debug("this is debug log")
	logger.Info("this is info log")
}

package main

import (
	"time"

	"github.com/GooLuck/GoServer/framework/config"
	"github.com/GooLuck/GoServer/framework/logger"
)

func main() {
	initLogger()
	initConfig()

	logger.Info("gate start", "timestamp", time.Now().Unix())
}

func initLogger() {
	// 初始化日志
	logger.InitDefaultLogger("gate", logger.DebugLevel)
	logger.Debug("this is debug log")
}

type ClusterConfig struct {
	ClusterName string `json:"clusterName"`
}

func initConfig() {
	v, err := config.GetConfig("gate")
	if err != nil {
		panic(err)
	}

	clusterConfig := new(ClusterConfig)
	err = v.Unmarshal(clusterConfig)
	if err != nil {
		panic(err)
	}

	logger.Info("init config", "cluster info", clusterConfig)
}

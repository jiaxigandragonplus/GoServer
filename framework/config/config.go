package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var configDir string

func init() {
	configPath := os.Getenv("CONF_PATH")
	if configPath == "" {
		configPath = "./config"
	}
	// 转换为绝对路径
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		absPath = configPath
	}
	configDir = absPath
}
func GetConfig(uri string) (*viper.Viper, error) {
	// 创建新的viper实例
	v := viper.New()

	// 确定配置文件路径
	var configFile string
	if filepath.Ext(uri) == "" {
		// 如果没有扩展名，添加.json
		configFile = uri + ".json"
	} else {
		configFile = uri
	}

	// 构建完整路径
	fullPath := filepath.Join(configDir, configFile)

	// 设置viper配置
	v.SetConfigFile(fullPath)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v, nil
}

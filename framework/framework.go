package framework

import (
	"github.com/GooLuck/GoServer/framework/logger"
	"go.uber.org/zap"
)

// Config 框架配置
type Config struct {
	// Logger 日志配置
	Logger *logger.Config
	// 其他框架配置可以在这里添加
}

// Framework 框架实例
type Framework struct {
	config *Config
	logger logger.Logger
}

var (
	// defaultFramework 默认框架实例
	defaultFramework *Framework
)

// init 初始化默认框架实例
func init() {
	// 使用默认配置初始化
	cfg := &Config{
		Logger: &logger.Config{
			Level:       logger.InfoLevel,
			Format:      "console",
			Output:      "stdout",
			Development: true,
			Caller:      true,
			Stacktrace:  false,
		},
	}

	var err error
	defaultFramework, err = NewFramework(cfg)
	if err != nil {
		// 如果初始化失败，使用fallback
		fallbackLogger, _ := logger.NewLogger(nil)
		defaultFramework = &Framework{
			config: cfg,
			logger: fallbackLogger,
		}
	}
}

// NewFramework 创建新的框架实例
func NewFramework(cfg *Config) (*Framework, error) {
	if cfg == nil {
		cfg = &Config{
			Logger: &logger.Config{
				Level:       logger.InfoLevel,
				Format:      "console",
				Output:      "stdout",
				Development: true,
				Caller:      true,
			},
		}
	}

	// 创建日志实例
	loggerInstance, err := logger.NewLogger(cfg.Logger)
	if err != nil {
		return nil, err
	}

	return &Framework{
		config: cfg,
		logger: loggerInstance,
	}, nil
}

// GetDefaultFramework 获取默认框架实例
func GetDefaultFramework() *Framework {
	return defaultFramework
}

// SetDefaultFramework 设置默认框架实例
func SetDefaultFramework(fw *Framework) {
	defaultFramework = fw
}

// Logger 获取框架的日志实例
func (fw *Framework) Logger() logger.Logger {
	return fw.logger
}

// SetLogger 设置框架的日志实例
func (fw *Framework) SetLogger(l logger.Logger) {
	fw.logger = l
}

// 便捷函数 - 框架级别日志

// Debug 使用框架默认日志记录器记录调试级别日志
func Debug(msg string, fields ...zap.Field) {
	defaultFramework.logger.Debug(msg, fields...)
}

// Info 使用框架默认日志记录器记录信息级别日志
func Info(msg string, fields ...zap.Field) {
	defaultFramework.logger.Info(msg, fields...)
}

// Warn 使用框架默认日志记录器记录警告级别日志
func Warn(msg string, fields ...zap.Field) {
	defaultFramework.logger.Warn(msg, fields...)
}

// Error 使用框架默认日志记录器记录错误级别日志
func Error(msg string, fields ...zap.Field) {
	defaultFramework.logger.Error(msg, fields...)
}

// Fatal 使用框架默认日志记录器记录致命错误级别日志
func Fatal(msg string, fields ...zap.Field) {
	defaultFramework.logger.Fatal(msg, fields...)
}

// With 使用框架默认日志记录器添加字段
func With(fields ...zap.Field) logger.Logger {
	return defaultFramework.logger.With(fields...)
}

// Sync 刷新框架默认日志记录器缓冲区
func Sync() error {
	return defaultFramework.logger.Sync()
}

// GetLogger 获取框架默认日志记录器
func GetLogger() logger.Logger {
	return defaultFramework.logger
}

// SetLogger 设置框架默认日志记录器
func SetLogger(l logger.Logger) {
	defaultFramework.SetLogger(l)
}

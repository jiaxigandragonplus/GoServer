package main

import (
	"time"

	"github.com/GooLuck/GoServer/framework/logger"
	"go.uber.org/zap"
)

func main() {
	// 示例1: 使用默认日志记录器
	exampleDefaultLogger()

	// 示例2: 创建自定义配置的日志记录器
	exampleCustomLogger()

	// 示例3: 使用结构化日志
	exampleStructuredLogging()

	// 示例4: 文件日志记录
	exampleFileLogging()

	// 示例5: 性能对比
	examplePerformance()
}

// exampleDefaultLogger 示例：使用默认日志记录器
func exampleDefaultLogger() {
	logger.Info("使用默认日志记录器")
	logger.Debug("这是一条调试信息")
	logger.Warn("这是一条警告信息")
	logger.Error("这是一条错误信息")

	// 添加字段
	logger.Info("用户登录",
		logger.String("username", "john_doe"),
		logger.Int("user_id", 12345),
		logger.Bool("success", true),
	)
}

// exampleCustomLogger 示例：创建自定义配置的日志记录器
func exampleCustomLogger() {
	// 创建开发环境配置
	devConfig := &logger.Config{
		Level:       logger.DebugLevel,
		Format:      "console",
		Output:      "stdout",
		Development: true,
		Caller:      true,
		Stacktrace:  true,
	}

	devLogger, err := logger.NewLogger(devConfig)
	if err != nil {
		panic(err)
	}

	devLogger.Info("开发环境日志记录器已创建")
	devLogger.Debug("详细的调试信息")
	devLogger.Error("模拟错误", zap.String("error", "文件未找到"))

	// 创建生产环境配置
	prodConfig := &logger.Config{
		Level:       logger.InfoLevel,
		Format:      "json",
		Output:      "stdout",
		Development: false,
		Caller:      true,
		Stacktrace:  false,
	}

	prodLogger, err := logger.NewLogger(prodConfig)
	if err != nil {
		panic(err)
	}

	prodLogger.Info("生产环境日志记录器已创建",
		zap.String("service", "api-server"),
		zap.Int("port", 8080),
	)
}

// exampleStructuredLogging 示例：结构化日志记录
func exampleStructuredLogging() {
	// 创建带字段的日志记录器
	requestLogger := logger.With(
		logger.String("request_id", "req-12345"),
		logger.String("method", "GET"),
		logger.String("path", "/api/users"),
	)

	requestLogger.Info("处理请求开始")
	requestLogger.Debug("请求参数", logger.Any("query", map[string]interface{}{
		"page":     1,
		"per_page": 20,
		"filter":   "active",
	}))

	// 模拟处理过程
	time.Sleep(10 * time.Millisecond)

	requestLogger.Info("处理请求完成",
		logger.Duration("duration", 10*time.Millisecond),
		logger.Int("status_code", 200),
		logger.Int("response_size", 2048),
	)

	// 错误处理示例
	err := simulateError()
	if err != nil {
		requestLogger.Error("处理请求失败",
			logger.ErrorField(err),
			logger.String("stage", "database_query"),
			logger.Any("context", map[string]interface{}{
				"user_id": 123,
				"action":  "update_profile",
			}),
		)
	}
}

// exampleFileLogging 示例：文件日志记录
func exampleFileLogging() {
	// 创建文件日志记录器配置
	fileConfig := &logger.Config{
		Level:       logger.InfoLevel,
		Format:      "json",
		Output:      "file",
		FilePath:    "./logs/app.log",
		MaxSize:     100,  // 100MB
		MaxBackups:  10,   // 保留10个备份文件
		MaxAge:      30,   // 保留30天
		Compress:    true, // 压缩备份文件
		Development: false,
		Caller:      true,
		Stacktrace:  true,
	}

	fileLogger, err := logger.NewLogger(fileConfig)
	if err != nil {
		logger.Error("创建文件日志记录器失败", logger.ErrorField(err))
		return
	}

	// 设置默认日志记录器
	logger.SetDefaultLogger(fileLogger)

	// 现在所有日志都会写入文件
	logger.Info("文件日志记录已启用",
		logger.String("file_path", "./logs/app.log"),
		logger.Time("start_time", time.Now()),
	)

	// 模拟一些日志记录
	for i := 1; i <= 5; i++ {
		logger.Info("处理任务",
			logger.Int("task_id", i),
			logger.String("status", "processing"),
			logger.Float64("progress", float64(i)/5.0*100),
		)
		time.Sleep(100 * time.Millisecond)
	}

	logger.Info("所有任务完成",
		logger.Int("total_tasks", 5),
		logger.Duration("total_duration", 500*time.Millisecond),
	)

	// 刷新缓冲区
	if err := logger.Sync(); err != nil {
		logger.Error("刷新日志缓冲区失败", logger.ErrorField(err))
	}
}

// examplePerformance 示例：性能对比
func examplePerformance() {
	logger.Info("开始性能测试")

	start := time.Now()

	// 测试结构化日志性能
	for i := 0; i < 1000; i++ {
		logger.Debug("性能测试日志",
			logger.Int("iteration", i),
			logger.String("test", "structured_logging"),
			logger.Duration("elapsed", time.Since(start)),
		)
	}

	elapsed := time.Since(start)
	logger.Info("性能测试完成",
		logger.Int("iterations", 1000),
		logger.Duration("total_time", elapsed),
		logger.Float64("avg_time_per_log", float64(elapsed.Nanoseconds())/1000/1e6), // 毫秒
	)
}

// simulateError 模拟错误
func simulateError() error {
	return &CustomError{
		Message: "数据库连接失败",
		Code:    "DB_CONN_001",
	}
}

// CustomError 自定义错误类型
type CustomError struct {
	Message string
	Code    string
}

func (e *CustomError) Error() string {
	return e.Message + " (" + e.Code + ")"
}

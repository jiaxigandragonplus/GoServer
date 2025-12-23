package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Level 定义日志级别
type Level int8

const (
	// DebugLevel 调试级别，最详细的日志
	DebugLevel Level = iota - 1
	// InfoLevel 信息级别，默认级别
	InfoLevel
	// WarnLevel 警告级别
	WarnLevel
	// ErrorLevel 错误级别
	ErrorLevel
	// FatalLevel 致命错误级别，记录后程序会退出
	FatalLevel
)

// Config 日志配置
type Config struct {
	// Level 日志级别
	Level Level
	// Format 日志格式：json 或 console
	Format string
	// Output 输出目标：stdout, stderr, file
	Output string
	// FilePath 日志文件路径（当Output为file时有效）
	FilePath string
	// MaxSize 日志文件最大大小（MB）
	MaxSize int
	// MaxBackups 最大备份文件数
	MaxBackups int
	// MaxAge 最大保存天数
	MaxAge int
	// Compress 是否压缩备份文件
	Compress bool
	// Development 是否为开发模式
	Development bool
	// Caller 是否记录调用者信息
	Caller bool
	// Stacktrace 是否记录堆栈跟踪
	Stacktrace bool
}

// Logger 日志接口
type Logger interface {
	// Debug 记录调试级别日志
	Debug(msg string, fields ...zap.Field)
	// Info 记录信息级别日志
	Info(msg string, fields ...zap.Field)
	// Warn 记录警告级别日志
	Warn(msg string, fields ...zap.Field)
	// Error 记录错误级别日志
	Error(msg string, fields ...zap.Field)
	// Fatal 记录致命错误级别日志
	Fatal(msg string, fields ...zap.Field)
	// With 添加字段到日志记录器
	With(fields ...zap.Field) Logger
	// Sync 刷新缓冲区
	Sync() error
}

// zapLogger 基于zap的日志实现
type zapLogger struct {
	zap *zap.Logger
}

var (
	// defaultLogger 默认日志实例
	defaultLogger Logger
	once          sync.Once
)

// init 初始化默认日志配置
func init() {
	// 使用默认配置初始化
	cfg := &Config{
		Level:       InfoLevel,
		Format:      "console",
		Output:      "stdout",
		Development: true,
		Caller:      true,
		Stacktrace:  false,
	}

	// 为默认日志记录器创建自定义配置，跳过2层调用
	// 因为调用链是：用户代码 -> logger.Info() -> defaultLogger.Info() -> l.zap.Info()
	var err error
	defaultLogger, err = newLoggerWithSkip(cfg, 2)
	if err != nil {
		// 如果初始化失败，使用fallback
		fallbackLogger, _ := zap.NewDevelopment()
		defaultLogger = &zapLogger{zap: fallbackLogger}
	}
}

// newLoggerWithSkip 创建新的日志实例，指定跳过层数
func newLoggerWithSkip(cfg *Config, skip int) (Logger, error) {
	if cfg == nil {
		cfg = &Config{
			Level:       InfoLevel,
			Format:      "console",
			Output:      "stdout",
			Development: true,
			Caller:      true,
		}
	}

	// 设置编码器配置
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	if !cfg.Development {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.TimeKey = "time"
		encoderConfig.LevelKey = "level"
		encoderConfig.NameKey = "logger"
		encoderConfig.CallerKey = "caller"
		encoderConfig.MessageKey = "msg"
		encoderConfig.StacktraceKey = "stacktrace"
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	// 设置编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 设置日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case DebugLevel:
		level = zapcore.DebugLevel
	case InfoLevel:
		level = zapcore.InfoLevel
	case WarnLevel:
		level = zapcore.WarnLevel
	case ErrorLevel:
		level = zapcore.ErrorLevel
	case FatalLevel:
		level = zapcore.FatalLevel
	}

	// 设置输出
	var cores []zapcore.Core

	// 控制台输出
	if cfg.Output == "stdout" || cfg.Output == "stderr" || cfg.Output == "" {
		writer := zapcore.Lock(os.Stdout)
		if cfg.Output == "stderr" {
			writer = zapcore.Lock(os.Stderr)
		}
		core := zapcore.NewCore(encoder, writer, level)
		cores = append(cores, core)
	}

	// 文件输出
	if cfg.Output == "file" && cfg.FilePath != "" {
		// 确保目录存在
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log directory failed: %w", err)
		}

		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,    // MB
			MaxBackups: cfg.MaxBackups, // 备份文件数
			MaxAge:     cfg.MaxAge,     // 天数
			Compress:   cfg.Compress,   // 是否压缩
		}

		writer := zapcore.AddSync(lumberjackLogger)
		fileCore := zapcore.NewCore(encoder, writer, level)
		cores = append(cores, fileCore)
	}

	// 创建核心
	var core zapcore.Core
	if len(cores) == 0 {
		// 默认输出到stdout
		writer := zapcore.Lock(os.Stdout)
		core = zapcore.NewCore(encoder, writer, level)
	} else if len(cores) == 1 {
		core = cores[0]
	} else {
		core = zapcore.NewTee(cores...)
	}

	// 创建选项
	options := []zap.Option{}
	if cfg.Caller {
		// 跳过指定层数的调用，显示实际调用日志的代码位置
		options = append(options, zap.AddCaller(), zap.AddCallerSkip(skip))
	}
	if cfg.Stacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}
	if cfg.Development {
		options = append(options, zap.Development())
	}

	// 创建zap日志器
	zapLoggerInstance := zap.New(core, options...)

	return &zapLogger{zap: zapLoggerInstance}, nil
}

// NewLogger 创建新的日志实例
func NewLogger(cfg *Config) (Logger, error) {
	// 对于直接通过NewLogger创建的日志记录器，跳过1层调用
	// 因为调用链是：用户代码 -> customLogger.Info() -> l.zap.Info()
	return newLoggerWithSkip(cfg, 1)
}

// Debug 记录调试级别日志
func (l *zapLogger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}

// Info 记录信息级别日志
func (l *zapLogger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Warn 记录警告级别日志
func (l *zapLogger) Warn(msg string, fields ...zap.Field) {
	l.zap.Warn(msg, fields...)
}

// Error 记录错误级别日志
func (l *zapLogger) Error(msg string, fields ...zap.Field) {
	l.zap.Error(msg, fields...)
}

// Fatal 记录致命错误级别日志
func (l *zapLogger) Fatal(msg string, fields ...zap.Field) {
	l.zap.Fatal(msg, fields...)
}

// With 添加字段到日志记录器
func (l *zapLogger) With(fields ...zap.Field) Logger {
	return &zapLogger{zap: l.zap.With(fields...)}
}

// Sync 刷新缓冲区
func (l *zapLogger) Sync() error {
	return l.zap.Sync()
}

// GetDefaultLogger 获取默认日志实例
func GetDefaultLogger() Logger {
	return defaultLogger
}

// SetDefaultLogger 设置默认日志实例
func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

// 便捷函数

// Debug 使用默认日志记录器记录调试级别日志
func Debug(msg string, fields ...zap.Field) {
	defaultLogger.Debug(msg, fields...)
}

// Info 使用默认日志记录器记录信息级别日志
func Info(msg string, fields ...zap.Field) {
	defaultLogger.Info(msg, fields...)
}

// Warn 使用默认日志记录器记录警告级别日志
func Warn(msg string, fields ...zap.Field) {
	defaultLogger.Warn(msg, fields...)
}

// Error 使用默认日志记录器记录错误级别日志
func Error(msg string, fields ...zap.Field) {
	defaultLogger.Error(msg, fields...)
}

// Fatal 使用默认日志记录器记录致命错误级别日志
func Fatal(msg string, fields ...zap.Field) {
	defaultLogger.Fatal(msg, fields...)
}

// With 使用默认日志记录器添加字段
func With(fields ...zap.Field) Logger {
	return defaultLogger.With(fields...)
}

// Sync 刷新默认日志记录器缓冲区
func Sync() error {
	return defaultLogger.Sync()
}

// 辅助函数

// String 创建字符串字段
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int 创建整数字段
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Int64 创建64位整数字段
func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

// Float64 创建浮点数字段
func Float64(key string, value float64) zap.Field {
	return zap.Float64(key, value)
}

// Bool 创建布尔字段
func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

// Time 创建时间字段
func Time(key string, value time.Time) zap.Field {
	return zap.Time(key, value)
}

// Duration 创建时长字段
func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

// ErrorField 创建错误字段
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

// Any 创建任意类型字段
func Any(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

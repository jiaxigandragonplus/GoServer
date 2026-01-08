package logger

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/logger/transports"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// TransportConfig 单个传输配置
type TransportConfig struct {
	// Name 传输名称，用于从注册表获取传输工厂
	Name string
	// Enabled 是否启用该传输
	Enabled bool
	// Config 传输特定配置
	Config interface{}
	// Transport 直接提供的传输实例（如果提供，则忽略Name和Config）
	Transport transports.Transport
}

// Config 日志配置
type Config struct {
	// Level 日志级别
	Level Level
	// Format 日志格式：json 或 console
	Format string
	// Output 输出目标：stdout, stderr, file, transport (向后兼容)
	// 当设置了Transports时，此字段将被忽略
	Output string
	// FilePath 日志文件路径（当Output为file时有效，向后兼容）
	FilePath string
	// MaxSize 日志文件最大大小（MB，向后兼容）
	MaxSize int
	// MaxBackups 最大备份文件数（向后兼容）
	MaxBackups int
	// MaxAge 最大保存天数（向后兼容）
	MaxAge int
	// Compress 是否压缩备份文件（向后兼容）
	Compress bool
	// Development 是否为开发模式
	Development bool
	// Caller 是否记录调用者信息
	Caller bool
	// Stacktrace 是否记录堆栈跟踪
	Stacktrace bool
	// Transports 传输配置列表，支持多个输出目标
	Transports []*TransportConfig
	// Transport 自定义传输，当Output为"transport"时使用（向后兼容）
	// 如果为nil且Output为"transport"，则使用默认传输注册表
	Transport transports.Transport
	// TransportName 传输名称，用于从注册表获取传输工厂（向后兼容）
	// 当Transport为nil且Output为"transport"时使用
	TransportName string
	// TransportConfig 传输配置，用于创建传输（向后兼容）
	// 具体类型取决于传输类型
	TransportConfig interface{}
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
	// SetLevel 设置日志级别
	SetLevel(level Level) error
}

// zapLogger 基于zap的日志实现
type zapLogger struct {
	logType string // 日志名字
	zap     *zap.Logger
	config  *Config
	skip    int // 调用者跳过的层数
}

var (
	// defaultLogger 默认日志实例
	defaultLogger Logger
	loggers       map[string]Logger
	once          sync.Once
)

// init 初始化默认日志配置
func init() {
	defaultLogger, _ = createLogger("default", InfoLevel)
}

func InitDefaultLogger(logType string, logLevel Level) {
	defaultLogger, _ = createLogger(logType, logLevel)
}

func GetLogger(logType string) Logger {
	if loggers == nil {
		loggers = make(map[string]Logger)
	}
	if loggers[logType] == nil {
		// 创建新logger
		logLevel := DebugLevel
		loggers[logType], _ = createLogger(logType, logLevel)
	}
	return loggers[logType]
}

func createLogger(loggerType string, logLevel Level) (Logger, error) {
	cfg := &Config{
		Level:       logLevel,
		Format:      "console",
		Output:      "stdout",
		Development: true,
		Caller:      true,
		Stacktrace:  false,
		Transports:  make([]*TransportConfig, 0),
	}

	// 写文件
	fileTransportCfg := &TransportConfig{
		Name:    "file",
		Enabled: true,
		Config:  transports.NewFileConfig(loggerType),
	}
	cfg.Transports = append(cfg.Transports, fileTransportCfg)

	// 为默认日志记录器创建自定义配置，跳过2层调用
	// 因为调用链是：用户代码 -> logger.Info() -> defaultLogger.Info() -> l.zap.Info()
	var err error
	newLogger, err := newLoggerWithSkip(cfg, 2)
	if err != nil {
		// 如果初始化失败，使用fallback
		fallbackLogger, _ := zap.NewDevelopment()
		defaultLogger = &zapLogger{
			zap: fallbackLogger,
			config: &Config{
				Level:       InfoLevel,
				Format:      "console",
				Output:      "stdout",
				Development: true,
				Caller:      true,
				Stacktrace:  false,
			},
			skip: 2,
		}
	}

	return newLogger, nil
}

// applyEnvConfig 根据环境变量更新配置
func applyEnvConfig(cfg *Config) {
	// 环境变量格式：LOG_TRANSPORTS=stdout,file,kafka
	// 或者针对单个transport：LOG_TRANSPORT_STDOUT=true, LOG_TRANSPORT_FILE=false
	if envTransports := os.Getenv("LOG_TRANSPORTS"); envTransports != "" {
		// 如果设置了LOG_TRANSPORTS，则根据它更新所有transport的Enabled状态
		enabledTransports := make(map[string]bool)
		transportsList := strings.Split(envTransports, ",")
		for _, t := range transportsList {
			enabledTransports[strings.TrimSpace(t)] = true
		}

		// 更新Transports配置
		for i := range cfg.Transports {
			cfg.Transports[i].Enabled = enabledTransports[cfg.Transports[i].Name]
		}
	}

	// 检查单个transport的环境变量
	for i := range cfg.Transports {
		envVar := fmt.Sprintf("LOG_TRANSPORT_%s", strings.ToUpper(cfg.Transports[i].Name))
		if envValue := os.Getenv(envVar); envValue != "" {
			enabled := strings.ToLower(envValue) == "true" || envValue == "1"
			cfg.Transports[i].Enabled = enabled
		}
	}
}

// createTransportFromConfig 根据TransportConfig创建传输实例
func createTransportFromConfig(tc *TransportConfig) (transports.Transport, error) {
	// 如果直接提供了Transport，使用它
	if tc.Transport != nil {
		return tc.Transport, nil
	}

	// 从注册表获取传输工厂
	factory, ok := transports.Get(tc.Name)
	if !ok {
		return nil, fmt.Errorf("transport factory not found: %s", tc.Name)
	}

	// 如果工厂是FileTransportFactory，并且提供了FileConfig，则设置配置
	if _, ok := factory.(*transports.FileTransportFactory); ok {
		if fileConfig, ok := tc.Config.(*transports.FileConfig); ok {
			// 创建新的工厂实例以使用提供的配置
			factory = transports.NewFileTransportFactory(fileConfig)
		}
	}

	// 创建传输实例
	return factory.Create()
}

func Level2ZapLevel(level Level) zapcore.Level {
	switch level {
	case DebugLevel:
		return zap.DebugLevel
	case InfoLevel:
		return zap.InfoLevel
	case WarnLevel:
		return zap.WarnLevel
	case ErrorLevel:
		return zap.ErrorLevel
	case FatalLevel:
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

func createTransports(cfg *Config, logLevel Level) ([]zapcore.Core, error) {
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

	// 设置输出
	var cores []zapcore.Core

	// 设置日志级别
	level := Level2ZapLevel(cfg.Level)

	// 默认输出到stdout
	writer := zapcore.Lock(os.Stdout)
	cores = append(cores, zapcore.NewCore(encoder, writer, level))

	// 优先使用Transports配置（新方式）
	if len(cfg.Transports) > 0 {
		for _, tc := range cfg.Transports {
			if !tc.Enabled {
				continue
			}

			transport, err := createTransportFromConfig(tc)
			if err != nil {
				return nil, fmt.Errorf("create transport %s failed: %w", tc.Name, err)
			}

			// 创建core
			transportCore := zapcore.NewCore(encoder, transport, level)
			cores = append(cores, transportCore)
		}
	}

	return cores, nil
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

	// 应用环境变量配置
	applyEnvConfig(cfg)

	cores, err := createTransports(cfg, cfg.Level)
	if err != nil {
		return nil, err
	}

	if len(cores) == 0 {
		panic("empty log core")
	}

	// 创建核心
	var core zapcore.Core
	if len(cores) == 1 {
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

	// 创建配置副本
	configCopy := *cfg
	return &zapLogger{
		zap:    zapLoggerInstance,
		config: &configCopy,
		skip:   skip,
	}, nil
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
	return &zapLogger{
		zap:    l.zap.With(fields...),
		config: l.config,
		skip:   l.skip,
	}
}

// Sync 刷新缓冲区
func (l *zapLogger) Sync() error {
	return l.zap.Sync()
}

// SetLevel 设置日志级别
func (l *zapLogger) SetLevel(level Level) error {
	// 更新配置
	l.config.Level = level

	// 重新创建logger
	newLogger, err := newLoggerWithSkip(l.config, l.skip)
	if err != nil {
		return err
	}

	// 类型断言获取zapLogger
	if newZapLogger, ok := newLogger.(*zapLogger); ok {
		// 关闭旧的logger
		_ = l.zap.Sync()
		// 替换为新的
		l.zap = newZapLogger.zap
	}

	return nil
}

// GetDefaultLogger 获取默认日志实例
func GetDefaultLogger() Logger {
	return defaultLogger
}

// SetDefaultLogger 设置默认日志实例
func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

// SetLevel 设置默认日志记录器的日志级别
func SetLevel(level Level) error {
	return defaultLogger.SetLevel(level)
}

// GetLevel 获取默认日志记录器的当前日志级别
func GetLevel() Level {
	// 尝试通过类型断言获取配置
	if zapLogger, ok := defaultLogger.(*zapLogger); ok {
		return zapLogger.config.Level
	}
	// 如果无法获取，返回默认级别
	return InfoLevel
}

// 便捷函数
func Debug(msg string, args ...interface{}) {
	defaultLogger.Debug(msg, Fields(args...)...)
}

// Infof 使用键值对记录信息级别日志
func Info(msg string, args ...interface{}) {
	defaultLogger.Info(msg, Fields(args...)...)
}

// Warnf 使用键值对记录警告级别日志
func Warn(msg string, args ...interface{}) {
	defaultLogger.Warn(msg, Fields(args...)...)
}

// Errorf 使用键值对记录错误级别日志
func Error(msg string, args ...interface{}) {
	defaultLogger.Error(msg, Fields(args...)...)
}

// Fatalf 使用键值对记录致命错误级别日志
func Fatal(msg string, args ...interface{}) {
	defaultLogger.Fatal(msg, Fields(args...)...)
}

// Withf 使用键值对创建带字段的日志记录器
func With(args ...interface{}) Logger {
	return defaultLogger.With(Fields(args...)...)
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
func Any(key string, value any) zap.Field {
	return zap.Any(key, value)
}

// Fields 将键值对参数转换为zap.Field切片
// 参数必须是偶数个，格式为：key1, value1, key2, value2, ...
// key必须是string类型，value可以是任意类型
func Fields(args ...interface{}) []zap.Field {
	if len(args) == 0 {
		return nil
	}
	if len(args)%2 != 0 {
		// 参数不是偶数个，记录错误并返回空切片
		Error("Fields参数必须是偶数个键值对", zap.Int("args_count", len(args)))
		return nil
	}

	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			// key不是string类型，记录错误并跳过
			Error("Fields参数中key必须是string类型",
				zap.Int("position", i),
				zap.Any("key", args[i]),
				zap.String("type", reflect.TypeOf(args[i]).String()))
			continue
		}
		value := args[i+1]
		fields = append(fields, convertToField(key, value))
	}
	return fields
}

// convertToField 将值转换为适当的zap.Field
func convertToField(key string, value interface{}) zap.Field {
	if value == nil {
		return zap.Any(key, nil)
	}

	// 使用反射检查类型，选择最合适的zap函数
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return zap.String(key, v.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return zap.Int64(key, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return zap.Uint64(key, v.Uint())
	case reflect.Float32, reflect.Float64:
		return zap.Float64(key, v.Float())
	case reflect.Bool:
		return zap.Bool(key, v.Bool())
	case reflect.Struct:
		// 检查是否是time.Time类型
		if t, ok := value.(time.Time); ok {
			return zap.Time(key, t)
		}
		// 检查是否是time.Duration类型
		if d, ok := value.(time.Duration); ok {
			return zap.Duration(key, d)
		}
		// 检查是否是error类型
		if err, ok := value.(error); ok {
			return zap.Error(err)
		}
		// 其他结构体使用Any
		return zap.Any(key, value)
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Array:
		// 复杂类型使用Any
		return zap.Any(key, value)
	default:
		// 其他类型使用Any
		return zap.Any(key, value)
	}
}

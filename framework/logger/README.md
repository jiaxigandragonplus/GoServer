# 日志模块

基于 [Uber Zap](https://github.com/uber-go/zap) 的高性能结构化日志模块，为 GoServer 项目提供灵活的日志记录功能。

## 特性

- 🚀 **高性能**：基于 Uber Zap，零分配内存的日志记录
- 📊 **结构化日志**：支持丰富的字段类型和结构化输出
- 🎯 **多级别**：Debug、Info、Warn、Error、Fatal 五个日志级别
- 🔧 **动态级别调整**：支持运行时动态修改日志级别
- 📝 **多格式**：支持 JSON 和控制台两种输出格式
- 🗂️ **多输出**：支持标准输出、标准错误、文件输出
- 🔄 **文件轮转**：集成 Lumberjack 支持日志文件轮转
- 🔧 **可配置**：丰富的配置选项，适应不同环境需求
- 🏗️ **接口化设计**：易于扩展和替换实现

## 安装

该模块已集成到 GoServer 项目中，无需额外安装。

## 快速开始

### 使用默认日志记录器

```go
import "github.com/GooLuck/GoServer/framework/logger"

func main() {
    // 直接使用包级函数
    logger.Info("应用程序启动")
    logger.Debug("调试信息")
    logger.Warn("警告信息")
    logger.Error("错误信息")
    
    // 添加字段
    logger.Info("用户登录成功",
        logger.String("username", "john_doe"),
        logger.Int("user_id", 12345),
        logger.Bool("success", true),
    )
}
```

### 创建自定义日志记录器

```go
import "github.com/GooLuck/GoServer/framework/logger"

func main() {
    // 开发环境配置
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
    
    // 生产环境配置
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
        logger.String("service", "api-server"),
        logger.Int("port", 8080),
    )
}
```

## 配置选项

### Config 结构体

```go
type Config struct {
    // Level 日志级别: DebugLevel, InfoLevel, WarnLevel, ErrorLevel, FatalLevel
    Level Level
    
    // Format 日志格式: "json" 或 "console"
    Format string
    
    // Output 输出目标: "stdout", "stderr", "file"
    Output string
    
    // FilePath 日志文件路径（当Output为file时有效）
    FilePath string
    
    // MaxSize 日志文件最大大小（MB），默认100
    MaxSize int
    
    // MaxBackups 最大备份文件数，默认10
    MaxBackups int
    
    // MaxAge 最大保存天数，默认30
    MaxAge int
    
    // Compress 是否压缩备份文件，默认true
    Compress bool
    
    // Development 是否为开发模式，默认true
    Development bool
    
    // Caller 是否记录调用者信息，默认true
    Caller bool
    
    // Stacktrace 是否记录堆栈跟踪，默认false
    Stacktrace bool
}
```

### 默认配置

```go
&Config{
    Level:       InfoLevel,
    Format:      "console",
    Output:      "stdout",
    Development: true,
    Caller:      true,
    Stacktrace:  false,
}
```

## API 参考

### 包级函数

| 函数 | 描述 |
|------|------|
| `Debug(msg string, fields ...zap.Field)` | 记录调试级别日志 |
| `Info(msg string, fields ...zap.Field)` | 记录信息级别日志 |
| `Warn(msg string, fields ...zap.Field)` | 记录警告级别日志 |
| `Error(msg string, fields ...zap.Field)` | 记录错误级别日志 |
| `Fatal(msg string, fields ...zap.Field)` | 记录致命错误级别日志 |
| `With(fields ...zap.Field) Logger` | 创建带字段的日志记录器 |
| `Sync() error` | 刷新日志缓冲区 |
| `GetDefaultLogger() Logger` | 获取默认日志记录器 |
| `SetDefaultLogger(logger Logger)` | 设置默认日志记录器 |
| `SetLevel(level Level) error` | 设置默认日志记录器的日志级别 |
| `GetLevel() Level` | 获取默认日志记录器的当前日志级别 |

### 字段创建函数

| 函数 | 描述 |
|------|------|
| `String(key, value string) zap.Field` | 创建字符串字段 |
| `Int(key string, value int) zap.Field` | 创建整数字段 |
| `Int64(key string, value int64) zap.Field` | 创建64位整数字段 |
| `Float64(key string, value float64) zap.Field` | 创建浮点数字段 |
| `Bool(key string, value bool) zap.Field` | 创建布尔字段 |
| `Time(key string, value time.Time) zap.Field` | 创建时间字段 |
| `Duration(key string, value time.Duration) zap.Field` | 创建时长字段 |
| `ErrorField(err error) zap.Field` | 创建错误字段 |
| `Any(key string, value interface{}) zap.Field` | 创建任意类型字段 |

### Logger 接口

```go
type Logger interface {
    Debug(msg string, fields ...zap.Field)
    Info(msg string, fields ...zap.Field)
    Warn(msg string, fields ...zap.Field)
    Error(msg string, fields ...zap.Field)
    Fatal(msg string, fields ...zap.Field)
    With(fields ...zap.Field) Logger
    Sync() error
    SetLevel(level Level) error  // 设置日志级别
}
```

## 使用示例

### 基本使用

```go
// 简单日志
logger.Info("服务启动完成")

// 带字段的日志
logger.Info("用户操作",
    logger.String("action", "login"),
    logger.Int("user_id", 123),
    logger.String("ip", "192.168.1.100"),
)

// 错误处理
if err := someOperation(); err != nil {
    logger.Error("操作失败",
        logger.ErrorField(err),
        logger.String("operation", "database_query"),
    )
}
```

### 请求上下文日志

```go
func handleRequest(req *http.Request) {
    // 为请求创建带上下文的日志记录器
    requestLogger := logger.With(
        logger.String("request_id", generateRequestID()),
        logger.String("method", req.Method),
        logger.String("path", req.URL.Path),
        logger.String("client_ip", req.RemoteAddr),
    )
    
    requestLogger.Info("开始处理请求")
    
    // 处理请求...
    
    requestLogger.Info("请求处理完成",
        logger.Duration("duration", time.Since(start)),
        logger.Int("status_code", 200),
    )
}
```

### 文件日志记录

```go
func setupFileLogging() {
    config := &logger.Config{
        Level:       logger.InfoLevel,
        Format:      "json",
        Output:      "file",
        FilePath:    "./logs/app.log",
        MaxSize:     100,    // 100MB
        MaxBackups:  10,     // 保留10个备份
        MaxAge:      30,     // 保留30天
        Compress:    true,   // 压缩备份
        Development: false,
        Caller:      true,
    }
    
    fileLogger, err := logger.NewLogger(config)
    if err != nil {
        panic(err)
    }
    
    // 设置为默认日志记录器
    logger.SetDefaultLogger(fileLogger)
    
    logger.Info("文件日志记录已配置",
        logger.String("file", "./logs/app.log"),
    )
}
```

### 动态日志级别调整

```go
func dynamicLogLevelExample() {
    fmt.Println("当前日志级别:", logger.GetLevel())
    
    // 初始设置为Info级别，Debug日志不会显示
    logger.Info("Info日志: 这条会显示")
    logger.Debug("Debug日志: 这条不会显示")
    
    // 动态设置为Debug级别
    if err := logger.SetLevel(logger.DebugLevel); err != nil {
        logger.Error("设置日志级别失败", logger.ErrorField(err))
        return
    }
    
    fmt.Println("已设置为Debug级别")
    logger.Debug("Debug日志: 现在这条会显示")
    logger.Info("Info日志: 这条也会显示")
    
    // 在生产环境中，可以根据需要动态调整日志级别
    // 例如：在收到特定信号时开启Debug日志
    go func() {
        // 模拟收到调试信号
        time.Sleep(2 * time.Second)
        
        logger.Info("收到调试信号，开启Debug日志")
        logger.SetLevel(logger.DebugLevel)
        
        // 执行一些调试操作
        logger.Debug("调试信息", logger.String("component", "database"))
        logger.Debug("详细状态", logger.Any("state", map[string]interface{}{
            "connections": 10,
            "queries":     150,
        }))
        
        // 一段时间后恢复为Info级别
        time.Sleep(5 * time.Second)
        logger.Info("调试结束，恢复Info级别")
        logger.SetLevel(logger.InfoLevel)
    }()
}

// 自定义logger的级别调整
func customLoggerLevelExample() {
    cfg := &logger.Config{
        Level:       logger.WarnLevel,
        Format:      "console",
        Output:      "stdout",
        Development: true,
    }
    
    customLogger, err := logger.NewLogger(cfg)
    if err != nil {
        panic(err)
    }
    
    customLogger.Info("Info日志: 这条不会显示（级别为Warn）")
    customLogger.Warn("Warn日志: 这条会显示")
    
    // 动态调整自定义logger的级别
    customLogger.SetLevel(logger.InfoLevel)
    customLogger.Info("Info日志: 现在这条会显示")
}
```

## 最佳实践

### 1. 环境配置

- **开发环境**：使用 `console` 格式，开启 `Debug` 级别，便于调试
- **测试环境**：使用 `json` 格式，`Info` 级别，便于日志分析
- **生产环境**：使用 `json` 格式，`Warn` 或 `Error` 级别，文件输出

### 2. 结构化日志

尽量使用结构化日志而不是字符串拼接：

```go
// 推荐：结构化日志
logger.Info("用户登录",
    logger.String("username", username),
    logger.Int("user_id", userID),
    logger.Bool("success", success),
)

// 不推荐：字符串拼接
logger.Info(fmt.Sprintf("用户登录: username=%s, user_id=%d, success=%v", 
    username, userID, success))
```

### 3. 错误处理

为错误添加上下文信息：

```go
if err := db.Query(); err != nil {
    logger.Error("数据库查询失败",
        logger.ErrorField(err),
        logger.String("query", sql),
        logger.Any("params", params),
        logger.String("database", dbName),
    )
}
```

### 4. 性能考虑

- 在性能敏感的场景，避免在日志调用中执行复杂计算
- 使用 `Debug` 级别日志时，可以通过条件判断避免不必要的字段计算
- 定期调用 `Sync()` 确保日志写入（特别是在程序退出时）

## 运行示例

查看完整示例：

```bash
cd framework/logger/example
go run example.go
```

## 依赖

- `go.uber.org/zap` - 高性能日志库
- `gopkg.in/natefinch/lumberjack.v2` - 日志文件轮转

## 许可证

本项目使用 MIT 许可证。
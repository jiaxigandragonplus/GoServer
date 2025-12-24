# 序列化模块 (Serialization Module)

一个通用、可扩展的序列化框架，支持多种序列化格式和自定义编码器。

## 功能特性

- **多格式支持**: JSON, 二进制, Protobuf, MessagePack, YAML, XML
- **自定义编码器**: 支持注册和使用自定义序列化器
- **类型注册**: 支持自定义类型的序列化/反序列化
- **压缩加密**: 内置压缩(Gzip, Zlib, Snappy)和加密(AES, RSA)支持
- **流式处理**: 支持流式编码/解码
- **自动检测**: 根据内容类型自动检测序列化格式
- **线程安全**: 所有注册表和序列化器都是线程安全的

## 核心接口

### Serializer 接口
```go
type Serializer interface {
    Serialize(v interface{}) ([]byte, error)
    Deserialize(data []byte, v interface{}) error
    ContentType() string
    Format() Format
}
```

### CustomEncoder 接口
```go
type CustomEncoder interface {
    Encode(v interface{}) ([]byte, error)
    Decode(data []byte, v interface{}) error
    Format() Format
}
```

## 支持的格式

| 格式 | 常量 | 内容类型 | 说明 |
|------|------|----------|------|
| JSON | `FormatJSON` | `application/json` | 默认支持，带缩进选项 |
| 二进制 | `FormatBinary` | `application/octet-stream` | 使用Gob编码，支持压缩加密 |
| Protobuf | `FormatProtobuf` | `application/protobuf` | 需要实现proto.Message接口 |
| MessagePack | `FormatMsgPack` | `application/x-msgpack` | 需要外部库支持 |
| YAML | `FormatYAML` | `application/x-yaml` | 需要外部库支持 |
| XML | `FormatXML` | `application/xml` | 需要外部库支持 |

## 快速开始

### 基本使用

```go
import "github.com/GooLuck/GoServer/framework/serialization"

// 1. 使用默认注册表序列化
data, err := serialization.Serialize(obj, serialization.FormatJSON)

// 2. 使用默认注册表反序列化
err = serialization.Deserialize(data, serialization.FormatJSON, &obj)

// 3. 使用编解码器
codec := serialization.NewCodec(nil)
encoded, _ := codec.Encode(obj, serialization.FormatJSON)
decoded := make(map[string]interface{})
codec.Decode(encoded, serialization.FormatJSON, &decoded)
```

### JSON序列化示例

```go
// 创建JSON序列化器
jsonSerializer := serialization.NewJSONSerializer(
    serialization.WithIndent("  "),      // 缩进
    serialization.WithTagName("json"),   // 结构体标签
)

// 序列化
data, err := jsonSerializer.Serialize(map[string]interface{}{
    "name": "test",
    "value": 42,
})

// 反序列化
var result map[string]interface{}
err = jsonSerializer.Deserialize(data, &result)
```

### 二进制序列化（带压缩）

```go
// 创建带压缩的二进制序列化器
binarySerializer := serialization.NewBinarySerializer(
    serialization.WithCompression(serialization.CompressionGzip),
    serialization.WithEncryption(serialization.EncryptionAES),
)

// 序列化
compressedData, err := binarySerializer.Serialize(data)

// 反序列化（需要相同的压缩/加密选项）
var decodedData interface{}
err = binarySerializer.Deserialize(compressedData, &decodedData)
```

### 自定义类型注册

```go
// 创建类型注册表
typeRegistry := serialization.NewTypeRegistry()

// 注册类型
typeRegistry.RegisterInstance("Person", Person{})
typeRegistry.RegisterInstance("Product", Product{})

// 获取类型
typ, err := typeRegistry.Get("Person")

// 获取类型名称
name, err := typeRegistry.GetName(reflect.TypeOf(Person{}))
```

### 自定义编码器

```go
// 创建自定义编码器
customEncoder := &MyCustomEncoder{}

// 创建自定义序列化器
customSerializer := serialization.NewCustomSerializer(customEncoder)

// 注册到注册表
registry := serialization.NewSerializerRegistry()
registry.Register(serialization.Format("myformat"), customSerializer)
```

### 注册表构建器

```go
// 使用构建器创建注册表
registry := serialization.NewRegistryBuilder().
    WithJSON(serialization.WithIndent("  ")).
    WithBinary(serialization.WithCompression(serialization.CompressionGzip)).
    WithProtobuf().
    WithCustom("myformat", myFactory).
    Build()

// 使用注册表
serializer, _ := registry.Get(serialization.FormatJSON)
data, _ := serializer.Serialize(obj)
```

## 高级功能

### 流式处理

```go
// JSON流编码器
streamEncoder := serialization.NewJSONStreamEncoder(writer)
streamEncoder.Write(obj1)
streamEncoder.Write(obj2)

// JSON流解码器
streamDecoder := serialization.NewJSONStreamDecoder(reader)
for streamDecoder.More() {
    var obj interface{}
    streamDecoder.Next(&obj)
    // 处理obj
}
```

### 二进制读写器

```go
// 二进制写入器
writer := serialization.NewBinaryWriter(serialization.BinaryOrder)
writer.WriteString("hello")
writer.WriteUint32(42)
writer.WriteBool(true)
data := writer.Bytes()

// 二进制读取器
reader := serialization.NewBinaryReader(data, serialization.BinaryOrder)
str, _ := reader.ReadString()
num, _ := reader.ReadUint32()
flag, _ := reader.ReadBool()
```

### 链式编码器

```go
// 创建链式编码器（编码->压缩->加密）
chainEncoder := serialization.NewChainEncoder(
    serialization.NewJSONSerializer(),
    serialization.NewCompressionTransformer(serialization.CompressionGzip),
    serialization.NewEncryptionTransformer(serialization.EncryptionAES),
)

data, err := chainEncoder.Encode(obj)
```

## 与现有Actor消息系统集成

序列化模块可以与现有的Actor消息系统无缝集成：

```go
import (
    "github.com/GooLuck/GoServer/framework/actor/message"
    "github.com/GooLuck/GoServer/framework/serialization"
)

// 获取消息系统的序列化注册表
messageRegistry := message.GetDefaultSerializerRegistry()

// 注册序列化模块的序列化器
jsonSerializer := serialization.NewJSONSerializer()
messageRegistry.Register(message.FormatJSON, jsonSerializer)

// 现在消息系统可以使用序列化模块的JSON序列化器
```

## 测试

运行测试：
```bash
cd framework/serialization
go test -v
```

测试覆盖率：
```bash
go test -cover
```

## 性能考虑

1. **JSON**: 适合人类可读和Web API，性能中等
2. **二进制**: 适合高性能场景，体积小，序列化/反序列化快
3. **Protobuf**: 适合网络传输和跨语言，性能优秀
4. **压缩**: 对于大文本数据建议启用压缩
5. **缓存**: 序列化器实例可以缓存和复用

## 扩展指南

### 添加新的序列化格式

1. 实现`Serializer`接口
2. 实现`CustomEncoder`接口（可选）
3. 注册到`SerializerRegistry`
4. 添加格式常量到`Format`类型

### 添加压缩/加密算法

1. 实现`Transformer`接口
2. 添加常量到`Compression`或`Encryption`类型
3. 在`compress`/`decompress`或`encrypt`/`decrypt`函数中添加实现

## 依赖

- 标准库: `encoding/json`, `encoding/gob`, `encoding/binary`
- 可选: `google.golang.org/protobuf` (Protobuf支持)
- 可选: `github.com/vmihailenco/msgpack` (MessagePack支持)
- 可选: `gopkg.in/yaml.v3` (YAML支持)

## 许可证

MIT
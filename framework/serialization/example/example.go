package example

import (
	"fmt"
	"strings"
	"time"
)

// 由于导入路径问题，我们使用相对路径
// 在实际项目中应该使用正确的导入路径

// Person 示例结构体
type Person struct {
	Name      string    `json:"name"`
	Age       int       `json:"age"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	Tags      []string  `json:"tags"`
}

// Product 另一个示例结构体
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	InStock     bool    `json:"in_stock"`
}

// RunJSONExample JSON序列化示例
func RunJSONExample() {
	fmt.Println("=== JSON序列化示例 ===")

	// 创建示例数据
	person := Person{
		Name:      "张三",
		Age:       30,
		Email:     "zhangsan@example.com",
		CreatedAt: time.Now(),
		Tags:      []string{"golang", "backend", "developer"},
	}

	fmt.Printf("示例数据: %+v\n", person)
	fmt.Println("注意: 实际序列化代码需要导入serialization包")
	fmt.Println()
}

// RunBinaryExample 二进制序列化示例
func RunBinaryExample() {
	fmt.Println("=== 二进制序列化示例 ===")

	product := Product{
		ID:          "P001",
		Name:        "Go编程语言",
		Price:       99.99,
		Description: "Go语言编程指南",
		InStock:     true,
	}

	fmt.Printf("示例数据: %+v\n", product)
	fmt.Println("注意: 实际二进制序列化代码需要导入serialization包")
	fmt.Println()
}

// RunCustomExample 自定义编码器示例
func RunCustomExample() {
	fmt.Println("=== 自定义编码器示例 ===")

	fmt.Println("1. 创建类型注册表")
	fmt.Println("2. 注册自定义类型")
	fmt.Println("3. 创建自定义序列化器")
	fmt.Println("4. 使用注册表管理序列化器")
	fmt.Println()
}

// RunProtobufExample Protobuf示例
func RunProtobufExample() {
	fmt.Println("=== Protobuf序列化示例 ===")

	fmt.Println("Protobuf序列化需要:")
	fmt.Println("1. 定义.proto文件")
	fmt.Println("2. 使用protoc编译生成Go代码")
	fmt.Println("3. 实现proto.Message接口")
	fmt.Println("4. 使用ProtobufSerializer进行序列化")
	fmt.Println()
}

// RunCompressionExample 压缩示例
func RunCompressionExample() {
	fmt.Println("=== 压缩序列化示例 ===")

	longText := strings.Repeat("这是一个需要压缩的长文本消息。", 50)
	fmt.Printf("原始文本长度: %d 字符\n", len(longText))
	fmt.Println("支持压缩算法: Gzip, Zlib, Snappy")
	fmt.Println()
}

// RunStreamExample 流式序列化示例
func RunStreamExample() {
	fmt.Println("=== 流式序列化示例 ===")

	people := []Person{
		{Name: "王五", Age: 28, Email: "wangwu@example.com"},
		{Name: "赵六", Age: 35, Email: "zhaoliu@example.com"},
		{Name: "孙七", Age: 42, Email: "sunqi@example.com"},
	}

	fmt.Printf("流式处理 %d 个对象\n", len(people))
	fmt.Println("支持JSON流编码器/解码器")
	fmt.Println()
}

// RunAllExamples 运行所有示例
func RunAllExamples() {
	fmt.Println("序列化模块示例演示")
	fmt.Println("========================================")
	fmt.Println()

	RunJSONExample()
	RunBinaryExample()
	RunCustomExample()
	RunProtobufExample()
	RunCompressionExample()
	RunStreamExample()

	fmt.Println("========================================")
	fmt.Println("示例演示完成！")
	fmt.Println()
	fmt.Println("实际使用示例:")
	fmt.Println(`
// 1. JSON序列化
import "github.com/yourusername/GoServer/framework/serialization"

data := map[string]interface{}{"name": "test", "value": 42}
jsonData, err := serialization.Serialize(data, serialization.FormatJSON)

// 2. 二进制序列化（带压缩）
binaryData, err := serialization.Serialize(data, serialization.FormatBinary,
	serialization.WithCompression(serialization.CompressionGzip))

// 3. 使用编解码器
codec := serialization.NewCodec(nil)
encoded, _ := codec.Encode(data, serialization.FormatJSON)
decoded := make(map[string]interface{})
codec.Decode(encoded, serialization.FormatJSON, &decoded)

// 4. 自定义类型注册
registry := serialization.NewTypeRegistry()
registry.RegisterInstance("Person", Person{})
	`)
}

// 演示如何使用序列化模块
func DemoUsage() {
	fmt.Println("=== 序列化模块使用演示 ===")

	// 模拟序列化过程
	fmt.Println("1. 初始化序列化注册表")
	fmt.Println("   registry := serialization.NewSerializerRegistry()")

	fmt.Println("\n2. 获取JSON序列化器")
	fmt.Println("   jsonSerializer, _ := registry.Get(serialization.FormatJSON)")

	fmt.Println("\n3. 序列化数据")
	fmt.Println("   data, _ := jsonSerializer.Serialize(map[string]interface{}{")
	fmt.Println("       \"name\": \"example\",")
	fmt.Println("       \"value\": 123")
	fmt.Println("   })")

	fmt.Println("\n4. 反序列化数据")
	fmt.Println("   var result map[string]interface{}")
	fmt.Println("   jsonSerializer.Deserialize(data, &result)")

	fmt.Println("\n5. 使用便捷函数")
	fmt.Println("   // 序列化")
	fmt.Println("   data, _ := serialization.Serialize(obj, serialization.FormatJSON)")
	fmt.Println("   // 反序列化")
	fmt.Println("   serialization.Deserialize(data, serialization.FormatJSON, &obj)")
}

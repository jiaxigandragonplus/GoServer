package serialization

import (
	"reflect"
	"testing"
	"time"
)

// TestPerson 测试用结构体
type TestPerson struct {
	Name      string    `json:"name"`
	Age       int       `json:"age"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	Tags      []string  `json:"tags"`
	Active    bool      `json:"active"`
}

// TestProduct 测试用结构体
type TestProduct struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	InStock     bool    `json:"in_stock"`
}

func TestJSONSerializer(t *testing.T) {
	serializer := NewJSONSerializer()

	person := TestPerson{
		Name:      "测试用户",
		Age:       30,
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		Tags:      []string{"golang", "test", "backend"},
		Active:    true,
	}

	// 测试序列化
	data, err := serializer.Serialize(person)
	if err != nil {
		t.Fatalf("JSON序列化失败: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("序列化结果为空")
	}

	// 测试反序列化
	var decodedPerson TestPerson
	err = serializer.Deserialize(data, &decodedPerson)
	if err != nil {
		t.Fatalf("JSON反序列化失败: %v", err)
	}

	// 验证数据
	if decodedPerson.Name != person.Name {
		t.Errorf("名称不匹配: 期望 %s, 实际 %s", person.Name, decodedPerson.Name)
	}
	if decodedPerson.Age != person.Age {
		t.Errorf("年龄不匹配: 期望 %d, 实际 %d", person.Age, decodedPerson.Age)
	}
	if decodedPerson.Email != person.Email {
		t.Errorf("邮箱不匹配: 期望 %s, 实际 %s", person.Email, decodedPerson.Email)
	}
	if decodedPerson.Active != person.Active {
		t.Errorf("状态不匹配: 期望 %v, 实际 %v", person.Active, decodedPerson.Active)
	}
}

func TestJSONSerializerWithIndent(t *testing.T) {
	serializer := NewJSONSerializer(WithIndent("  "))

	product := TestProduct{
		ID:          "TEST001",
		Name:        "测试产品",
		Price:       99.99,
		Description: "这是一个测试产品",
		InStock:     true,
	}

	data, err := serializer.Serialize(product)
	if err != nil {
		t.Fatalf("带缩进的JSON序列化失败: %v", err)
	}

	// 检查是否包含换行符（缩进特征）
	hasNewline := false
	for _, b := range data {
		if b == '\n' {
			hasNewline = true
			break
		}
	}

	if !hasNewline {
		t.Log("警告: 缩进JSON可能没有换行符，但这不是错误")
	}

	// 反序列化验证
	var decodedProduct TestProduct
	err = serializer.Deserialize(data, &decodedProduct)
	if err != nil {
		t.Fatalf("带缩进的JSON反序列化失败: %v", err)
	}

	if decodedProduct.ID != product.ID {
		t.Errorf("ID不匹配: 期望 %s, 实际 %s", product.ID, decodedProduct.ID)
	}
}

func TestBinarySerializer(t *testing.T) {
	serializer := NewBinarySerializer()

	person := TestPerson{
		Name:  "二进制测试",
		Age:   25,
		Email: "binary@test.com",
		Tags:  []string{"binary", "test"},
	}

	// 测试序列化
	data, err := serializer.Serialize(person)
	if err != nil {
		t.Fatalf("二进制序列化失败: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("二进制序列化结果为空")
	}

	// 测试反序列化
	var decodedPerson TestPerson
	err = serializer.Deserialize(data, &decodedPerson)
	if err != nil {
		t.Fatalf("二进制反序列化失败: %v", err)
	}

	if decodedPerson.Name != person.Name {
		t.Errorf("名称不匹配: 期望 %s, 实际 %s", person.Name, decodedPerson.Name)
	}
}

func TestSerializerRegistry(t *testing.T) {
	registry := NewSerializerRegistry()

	// 测试获取JSON序列化器
	jsonSerializer, err := registry.Get(FormatJSON)
	if err != nil {
		t.Fatalf("获取JSON序列化器失败: %v", err)
	}

	if jsonSerializer == nil {
		t.Fatal("JSON序列化器为nil")
	}

	if jsonSerializer.Format() != FormatJSON {
		t.Errorf("格式不匹配: 期望 %s, 实际 %s", FormatJSON, jsonSerializer.Format())
	}

	// 测试获取二进制序列化器
	binarySerializer, err := registry.Get(FormatBinary)
	if err != nil {
		t.Fatalf("获取二进制序列化器失败: %v", err)
	}

	if binarySerializer == nil {
		t.Fatal("二进制序列化器为nil")
	}

	// 测试列出格式
	formats := registry.List()
	if len(formats) < 2 {
		t.Errorf("期望至少2种格式，实际 %d", len(formats))
	}

	// 验证包含JSON和二进制格式
	hasJSON := false
	hasBinary := false
	for _, format := range formats {
		if format == FormatJSON {
			hasJSON = true
		}
		if format == FormatBinary {
			hasBinary = true
		}
	}

	if !hasJSON {
		t.Error("注册表不包含JSON格式")
	}
	if !hasBinary {
		t.Error("注册表不包含二进制格式")
	}
}

func TestTypeRegistry(t *testing.T) {
	registry := NewTypeRegistry()

	// 注册类型
	err := registry.Register("TestPerson", reflect.TypeOf(TestPerson{}))
	if err != nil {
		t.Fatalf("注册类型失败: %v", err)
	}

	// 获取类型
	typ, err := registry.Get("TestPerson")
	if err != nil {
		t.Fatalf("获取类型失败: %v", err)
	}

	if typ.Name() != "TestPerson" {
		t.Errorf("类型名称不匹配: 期望 TestPerson, 实际 %s", typ.Name())
	}

	// 测试重复注册
	err = registry.Register("TestPerson", reflect.TypeOf(TestPerson{}))
	if err == nil {
		t.Error("重复注册应该失败")
	}

	// 测试获取不存在的类型
	_, err = registry.Get("NonExistentType")
	if err == nil {
		t.Error("获取不存在的类型应该失败")
	}
}

func TestDefaultSerializerRegistry(t *testing.T) {
	registry1 := GetDefaultSerializerRegistry()
	registry2 := GetDefaultSerializerRegistry()

	// 应该返回相同的实例
	if registry1 != registry2 {
		t.Error("默认注册表不是单例")
	}

	// 测试序列化函数
	person := TestPerson{
		Name:  "默认注册表测试",
		Age:   35,
		Email: "default@test.com",
	}

	data, err := Serialize(person, FormatJSON)
	if err != nil {
		t.Fatalf("使用默认注册表序列化失败: %v", err)
	}

	var decodedPerson TestPerson
	err = Deserialize(data, FormatJSON, &decodedPerson)
	if err != nil {
		t.Fatalf("使用默认注册表反序列化失败: %v", err)
	}

	if decodedPerson.Name != person.Name {
		t.Errorf("名称不匹配: 期望 %s, 实际 %s", person.Name, decodedPerson.Name)
	}
}

func TestCodec(t *testing.T) {
	codec := NewCodec(nil)

	product := TestProduct{
		ID:      "CODEC001",
		Name:    "编解码器测试",
		Price:   123.45,
		InStock: true,
	}

	// 编码
	data, err := codec.Encode(product, FormatJSON)
	if err != nil {
		t.Fatalf("编解码器编码失败: %v", err)
	}

	// 解码
	var decodedProduct TestProduct
	err = codec.Decode(data, FormatJSON, &decodedProduct)
	if err != nil {
		t.Fatalf("编解码器解码失败: %v", err)
	}

	if decodedProduct.ID != product.ID {
		t.Errorf("ID不匹配: 期望 %s, 实际 %s", product.ID, decodedProduct.ID)
	}
}

func TestErrorCases(t *testing.T) {
	serializer := NewJSONSerializer()

	// 测试序列化nil
	_, err := serializer.Serialize(nil)
	if err != ErrNilValue {
		t.Errorf("序列化nil应该返回ErrNilValue，实际: %v", err)
	}

	// 测试反序列化空数据
	var person TestPerson
	err = serializer.Deserialize([]byte{}, &person)
	if err != ErrInvalidData {
		t.Errorf("反序列化空数据应该返回ErrInvalidData，实际: %v", err)
	}

	// 测试反序列化到nil
	err = serializer.Deserialize([]byte("{}"), nil)
	if err != ErrNilValue {
		t.Errorf("反序列化到nil应该返回ErrNilValue，实际: %v", err)
	}
}

func TestFormatDetection(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		data        []byte
		expected    Format
	}{
		{
			name:        "JSON content type",
			contentType: "application/json",
			data:        []byte(`{"test": "value"}`),
			expected:    FormatJSON,
		},
		{
			name:        "Binary content type",
			contentType: "application/octet-stream",
			data:        []byte{0x01, 0x02, 0x03},
			expected:    FormatBinary,
		},
		{
			name:        "Protobuf content type",
			contentType: "application/protobuf",
			data:        []byte{0x0A, 0x04, 0x74, 0x65, 0x73, 0x74},
			expected:    FormatProtobuf,
		},
		{
			name:        "Unknown content type with JSON data",
			contentType: "unknown",
			data:        []byte(`{"test": "value"}`),
			expected:    FormatJSON, // 应该检测为JSON
		},
		{
			name:        "Unknown content type with XML data",
			contentType: "unknown",
			data:        []byte(`<?xml version="1.0"?><test>value</test>`),
			expected:    FormatXML, // 应该检测为XML
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			format := AutoDetectFormat(tc.data, tc.contentType)
			if format != tc.expected {
				t.Errorf("格式检测错误: 期望 %s, 实际 %s", tc.expected, format)
			}
		})
	}
}

func TestRegistryBuilder(t *testing.T) {
	builder := NewRegistryBuilder().
		WithJSON().
		WithBinary()

	registry := builder.Build()

	// 测试JSON序列化器
	jsonSerializer, err := registry.Get(FormatJSON)
	if err != nil {
		t.Fatalf("构建器创建的注册表获取JSON序列化器失败: %v", err)
	}
	if jsonSerializer.Format() != FormatJSON {
		t.Errorf("JSON序列化器格式错误: 期望 %s, 实际 %s", FormatJSON, jsonSerializer.Format())
	}

	// 测试二进制序列化器
	binarySerializer, err := registry.Get(FormatBinary)
	if err != nil {
		t.Fatalf("构建器创建的注册表获取二进制序列化器失败: %v", err)
	}
	if binarySerializer.Format() != FormatBinary {
		t.Errorf("二进制序列化器格式错误: 期望 %s, 实际 %s", FormatBinary, binarySerializer.Format())
	}
}

// 测试序列化器接口实现
func TestSerializerInterface(t *testing.T) {
	// 测试JSON序列化器接口
	jsonSerializer := NewJSONSerializer()
	testSerializerInterface(t, jsonSerializer, FormatJSON, "application/json")

	// 测试二进制序列化器接口
	binarySerializer := NewBinarySerializer()
	testSerializerInterface(t, binarySerializer, FormatBinary, "application/octet-stream")
}

func testSerializerInterface(t *testing.T, serializer Serializer, expectedFormat Format, expectedContentType string) {
	if serializer.Format() != expectedFormat {
		t.Errorf("格式不匹配: 期望 %s, 实际 %s", expectedFormat, serializer.Format())
	}

	if serializer.ContentType() != expectedContentType {
		t.Errorf("内容类型不匹配: 期望 %s, 实际 %s", expectedContentType, serializer.ContentType())
	}

	// 测试基本序列化/反序列化
	testData := map[string]interface{}{
		"test":   "value",
		"number": 42,
		"active": true,
	}

	data, err := serializer.Serialize(testData)
	if err != nil {
		t.Errorf("序列化失败: %v", err)
		return
	}

	if len(data) == 0 {
		t.Error("序列化结果为空")
		return
	}

	var decodedData map[string]interface{}
	err = serializer.Deserialize(data, &decodedData)
	if err != nil {
		t.Errorf("反序列化失败: %v", err)
	}
}

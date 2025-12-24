package message

import (
	"context"
	"testing"
	"time"
)

func TestBaseMessage(t *testing.T) {
	// 创建基础消息
	msg := NewBaseMessage("test.type", "test data")

	// 测试基本属性
	if msg.Type() != "test.type" {
		t.Errorf("期望消息类型为 'test.type'，实际为 '%s'", msg.Type())
	}

	if msg.Data() != "test data" {
		t.Errorf("期望消息数据为 'test data'，实际为 '%v'", msg.Data())
	}

	if msg.ID() == "" {
		t.Error("消息ID不应为空")
	}

	if msg.Priority() != PriorityNormal {
		t.Errorf("期望消息优先级为 PriorityNormal，实际为 %v", msg.Priority())
	}

	// 测试设置属性
	msg.SetPriority(PriorityHigh)
	if msg.Priority() != PriorityHigh {
		t.Errorf("期望消息优先级为 PriorityHigh，实际为 %v", msg.Priority())
	}

	msg.SetTTL(30 * time.Second)
	if msg.TTL() != 30*time.Second {
		t.Errorf("期望消息TTL为 30s，实际为 %v", msg.TTL())
	}

	// 测试头部
	msg.SetHeader("key1", "value1")
	if msg.GetHeader("key1") != "value1" {
		t.Errorf("期望头部 key1 的值为 'value1'，实际为 '%s'", msg.GetHeader("key1"))
	}

	// 测试克隆
	cloned := msg.Clone()
	if cloned.Type() != msg.Type() {
		t.Errorf("克隆消息类型不匹配")
	}
	if cloned.ID() != msg.ID() {
		t.Errorf("克隆消息ID不匹配")
	}
}

func TestAddress(t *testing.T) {
	// 测试本地地址
	localAddr, err := NewLocalAddress("actor", "/test/path")
	if err != nil {
		t.Fatalf("创建本地地址失败: %v", err)
	}
	if !localAddr.IsLocal() {
		t.Error("本地地址应标识为本地")
	}
	if localAddr.IsRemote() {
		t.Error("本地地址不应标识为远程")
	}
	if localAddr.Protocol() != "actor" {
		t.Errorf("期望协议为 'actor'，实际为 '%s'", localAddr.Protocol())
	}
	if localAddr.Path() != "/test/path" {
		t.Errorf("期望路径为 '/test/path'，实际为 '%s'", localAddr.Path())
	}

	// 测试远程地址
	remoteAddr, err := NewRemoteAddress("actor", "example.com", "8080", "/remote/path")
	if err != nil {
		t.Fatalf("创建远程地址失败: %v", err)
	}
	if !remoteAddr.IsRemote() {
		t.Error("远程地址应标识为远程")
	}
	if remoteAddr.IsLocal() {
		t.Error("远程地址不应标识为本地")
	}
	if remoteAddr.Host() != "example.com" {
		t.Errorf("期望主机为 'example.com'，实际为 '%s'", remoteAddr.Host())
	}
	if remoteAddr.Port() != "8080" {
		t.Errorf("期望端口为 '8080'，实际为 '%s'", remoteAddr.Port())
	}

	// 测试地址相等性
	addr1 := MustNewLocalAddress("actor", "/same/path")
	addr2 := MustNewLocalAddress("actor", "/same/path")
	if !addr1.Equal(addr2) {
		t.Error("相同路径的地址应相等")
	}

	addr3 := MustNewLocalAddress("actor", "/different/path")
	if addr1.Equal(addr3) {
		t.Error("不同路径的地址不应相等")
	}

	// 测试不支持的协议
	_, err = NewLocalAddress("unsupported", "/test")
	if err == nil {
		t.Error("不支持的协议应返回错误")
	}

	// 测试协议验证
	if !IsProtocolSupported("actor") {
		t.Error("'actor' 协议应受支持")
	}
	if !IsProtocolSupported("tcp") {
		t.Error("'tcp' 协议应受支持")
	}
	if IsProtocolSupported("unknown") {
		t.Error("'unknown' 协议不应受支持")
	}
}

func TestEnvelope(t *testing.T) {
	// 创建消息和信封
	msg := NewBaseMessage("envelope.test", "test data")
	envelope := NewEnvelope(msg)

	// 测试信封基本属性
	if envelope.Message().Type() != "envelope.test" {
		t.Errorf("期望信封消息类型为 'envelope.test'，实际为 '%s'", envelope.Message().Type())
	}

	delivery := envelope.Delivery()
	if delivery.Attempt != 1 {
		t.Errorf("期望投递尝试次数为 1，实际为 %d", delivery.Attempt)
	}
	if delivery.MaxAttempts != 3 {
		t.Errorf("期望最大尝试次数为 3，实际为 %d", delivery.MaxAttempts)
	}

	// 测试重试逻辑
	if !envelope.ShouldRetry() {
		t.Error("第一次尝试后应允许重试")
	}

	envelope.IncrementAttempt()
	if envelope.Delivery().Attempt != 2 {
		t.Errorf("期望投递尝试次数为 2，实际为 %d", envelope.Delivery().Attempt)
	}

	// 测试死信标记
	envelope.MarkAsDeadLetter(nil)
	if !envelope.Delivery().DeadLetter {
		t.Error("信封应标记为死信")
	}
}

func TestRouter(t *testing.T) {
	// 创建路由器
	router := NewBaseRouter()

	// 创建目标地址
	target1 := MustNewLocalAddress("actor", "/target1")
	target2 := MustNewLocalAddress("actor", "/target2")

	// 添加路由规则
	rule := RouteRule{
		ID:       "test-rule",
		Pattern:  "test.*",
		Targets:  []Address{target1, target2},
		Strategy: StrategyBroadcast,
		Priority: 1,
	}

	if err := router.AddRoute(rule); err != nil {
		t.Fatalf("添加路由规则失败: %v", err)
	}

	// 测试路由匹配
	msg := NewBaseMessage("test.message", "路由测试")
	ctx := context.Background()
	targets, err := router.Route(ctx, msg)
	if err != nil {
		t.Fatalf("路由消息失败: %v", err)
	}

	if len(targets) != 2 {
		t.Errorf("期望路由到 2 个目标，实际为 %d", len(targets))
	}

	// 测试不匹配的消息
	nonMatchMsg := NewBaseMessage("other.message", "不匹配的消息")
	targets, err = router.Route(ctx, nonMatchMsg)
	if err == nil {
		t.Error("不匹配的消息应返回错误")
	}

	// 测试移除规则
	if err := router.RemoveRoute("test-rule"); err != nil {
		t.Fatalf("移除路由规则失败: %v", err)
	}

	rules := router.GetRoutes()
	if len(rules) != 0 {
		t.Errorf("期望路由规则数量为 0，实际为 %d", len(rules))
	}
}

func TestSerialization(t *testing.T) {
	// 创建消息
	msg := NewBaseMessage("serialization.test", map[string]interface{}{
		"field":  "value",
		"number": 42,
	})

	// 设置地址
	sender := MustNewLocalAddress("actor", "/sender")
	receiver := MustNewLocalAddress("actor", "/receiver")
	msg.SetSender(sender)
	msg.SetReceiver(receiver)

	// 获取JSON序列化器
	registry := GetDefaultSerializerRegistry()
	serializer, err := registry.Get(FormatJSON)
	if err != nil {
		t.Fatalf("获取序列化器失败: %v", err)
	}

	// 序列化
	data, err := serializer.Serialize(msg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("序列化数据不应为空")
	}

	// 反序列化
	deserialized, err := serializer.Deserialize(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证反序列化结果
	if deserialized.Type() != msg.Type() {
		t.Errorf("期望消息类型为 '%s'，实际为 '%s'", msg.Type(), deserialized.Type())
	}

	if deserialized.ID() != msg.ID() {
		t.Errorf("期望消息ID为 '%s'，实际为 '%s'", msg.ID(), deserialized.ID())
	}

	// 使用编解码器测试
	codec := NewMessageCodec(nil)
	encoded, err := codec.Encode(msg, FormatJSON)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	decoded, err := codec.Decode(encoded, FormatJSON)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	if decoded.Type() != msg.Type() {
		t.Errorf("编解码器测试失败: 期望消息类型为 '%s'，实际为 '%s'", msg.Type(), decoded.Type())
	}
}

func TestRouterStrategies(t *testing.T) {
	// 创建目标地址
	targets := []Address{
		MustNewLocalAddress("actor", "/target1"),
		MustNewLocalAddress("actor", "/target2"),
		MustNewLocalAddress("actor", "/target3"),
	}

	// 测试广播策略
	router := NewBaseRouter()
	rule := RouteRule{
		ID:       "broadcast-test",
		Pattern:  "broadcast.*",
		Targets:  targets,
		Strategy: StrategyBroadcast,
		Priority: 1,
	}

	router.AddRoute(rule)
	msg := NewBaseMessage("broadcast.test", "广播测试")
	ctx := context.Background()

	resultTargets, err := router.Route(ctx, msg)
	if err != nil {
		t.Fatalf("广播策略路由失败: %v", err)
	}

	if len(resultTargets) != len(targets) {
		t.Errorf("广播策略应返回所有目标，期望 %d 个，实际 %d 个", len(targets), len(resultTargets))
	}

	// 测试轮询策略
	router2 := NewBaseRouter()
	rule2 := RouteRule{
		ID:       "roundrobin-test",
		Pattern:  "roundrobin.*",
		Targets:  targets,
		Strategy: StrategyRoundRobin,
		Priority: 1,
	}

	router2.AddRoute(rule2)

	// 多次路由应轮流选择目标
	selectedTargets := make(map[string]bool)
	for i := 0; i < 5; i++ {
		testMsg := NewBaseMessage("roundrobin.test", i)
		result, _ := router2.Route(ctx, testMsg)
		if len(result) > 0 {
			selectedTargets[result[0].String()] = true
		}
	}

	// 至少应选择不同的目标
	if len(selectedTargets) < 2 {
		t.Errorf("轮询策略应选择多个目标，实际选择了 %d 个", len(selectedTargets))
	}
}

func TestAddressPool(t *testing.T) {
	// 清空地址池
	ClearAddressPool()

	// 获取或创建地址
	addr1, err := GetOrCreateAddress("actor://localhost/user/alice")
	if err != nil {
		t.Fatalf("创建地址失败: %v", err)
	}

	// 再次获取相同地址应返回同一个实例
	addr2, err := GetOrCreateAddress("actor://localhost/user/alice")
	if err != nil {
		t.Fatalf("获取地址失败: %v", err)
	}

	if addr1 != addr2 {
		t.Error("相同地址字符串应返回同一个地址实例")
	}

	// 移除地址
	RemoveAddress("actor://localhost/user/alice")

	// 再次获取应创建新实例
	addr3, err := GetOrCreateAddress("actor://localhost/user/alice")
	if err != nil {
		t.Fatalf("重新创建地址失败: %v", err)
	}

	if addr1 == addr3 {
		t.Error("移除后重新获取应返回新实例")
	}
}

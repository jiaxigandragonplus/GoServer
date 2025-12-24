package message

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// Protocol 支持的协议类型
type Protocol string

const (
	// ProtocolLocal 本地协议（默认，进程内通信）
	ProtocolLocal Protocol = "local"
	// ProtocolSystem 系统协议（框架内部使用）
	ProtocolSystem Protocol = "system"
	// ProtocolTCP TCP协议（远程通信）
	ProtocolTCP Protocol = "tcp"
	// ProtocolWebSocket WebSocket协议
	ProtocolWebSocket Protocol = "ws"
	// ProtocolGRPC gRPC协议
	ProtocolGRPC Protocol = "grpc"
)

// 本地协议（进程内通信）
var localProtocols = map[Protocol]bool{
	ProtocolLocal:  true,
	ProtocolSystem: true,
}

// 网络协议（跨进程/机器通信）
var networkProtocols = map[Protocol]bool{
	ProtocolTCP:       true,
	ProtocolWebSocket: true,
	ProtocolGRPC:      true,
}

// 所有支持的协议
var supportedProtocols = map[Protocol]bool{
	ProtocolLocal:     true,
	ProtocolSystem:    true,
	ProtocolTCP:       true,
	ProtocolWebSocket: true,
	ProtocolGRPC:      true,
}

// IsProtocolSupported 检查协议是否受支持
func IsProtocolSupported(protocol string) bool {
	_, supported := supportedProtocols[Protocol(protocol)]
	return supported
}

// IsLocalProtocol 检查是否是本地协议
func IsLocalProtocol(protocol string) bool {
	_, isLocal := localProtocols[Protocol(protocol)]
	return isLocal
}

// IsNetworkProtocol 检查是否是网络协议
func IsNetworkProtocol(protocol string) bool {
	_, isNetwork := networkProtocols[Protocol(protocol)]
	return isNetwork
}

// ValidateProtocol 验证协议
func ValidateProtocol(protocol string) error {
	if protocol == "" {
		return fmt.Errorf("protocol cannot be empty")
	}

	if !IsProtocolSupported(protocol) {
		return fmt.Errorf("unsupported protocol: %s. Supported protocols: %v",
			protocol, getSupportedProtocolsList())
	}

	return nil
}

// getSupportedProtocolsList 获取支持的协议列表
func getSupportedProtocolsList() []string {
	protocols := make([]string, 0, len(supportedProtocols))
	for protocol := range supportedProtocols {
		protocols = append(protocols, string(protocol))
	}
	return protocols
}

// getLocalProtocolsList 获取本地协议列表
func getLocalProtocolsList() []string {
	protocols := make([]string, 0, len(localProtocols))
	for protocol := range localProtocols {
		protocols = append(protocols, string(protocol))
	}
	return protocols
}

// getNetworkProtocolsList 获取网络协议列表
func getNetworkProtocolsList() []string {
	protocols := make([]string, 0, len(networkProtocols))
	for protocol := range networkProtocols {
		protocols = append(protocols, string(protocol))
	}
	return protocols
}

// ActorAddress Actor地址实现
type ActorAddress struct {
	protocol string
	host     string
	port     string
	path     string
	address  string // 完整地址字符串
}

// NewLocalAddress 创建本地地址
func NewLocalAddress(protocol, path string) (*ActorAddress, error) {
	// 验证协议
	if err := ValidateProtocol(protocol); err != nil {
		return nil, err
	}

	// 确保是本地协议
	if !IsLocalProtocol(protocol) {
		return nil, fmt.Errorf("protocol %s is not a local protocol. Local protocols: %v",
			protocol, getLocalProtocolsList())
	}

	addr := &ActorAddress{
		protocol: protocol,
		host:     "localhost",
		port:     "",
		path:     path,
	}
	addr.address = addr.buildAddressString()
	return addr, nil
}

// NewRemoteAddress 创建远程地址
func NewRemoteAddress(protocol, host, port, path string) (*ActorAddress, error) {
	// 验证协议
	if err := ValidateProtocol(protocol); err != nil {
		return nil, err
	}

	// 验证主机
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty for remote address")
	}

	addr := &ActorAddress{
		protocol: protocol,
		host:     host,
		port:     port,
		path:     path,
	}
	addr.address = addr.buildAddressString()
	return addr, nil
}

// NewAddress 创建地址（自动判断本地/远程）
func NewAddress(protocol, host, port, path string) (*ActorAddress, error) {
	if IsLocalProtocol(protocol) {
		// 本地协议，忽略host和port
		return NewLocalAddress(protocol, path)
	} else if IsNetworkProtocol(protocol) {
		// 网络协议
		return NewRemoteAddress(protocol, host, port, path)
	} else {
		return nil, fmt.Errorf("unknown protocol type: %s", protocol)
	}
}

// NewLocalActorAddress 创建本地actor地址（简化接口）
func NewLocalActorAddress(path string) (*ActorAddress, error) {
	return NewLocalAddress(string(ProtocolLocal), path)
}

// NewRemoteActorAddress 创建远程actor地址（简化接口）
func NewRemoteActorAddress(protocol, host, port, path string) (*ActorAddress, error) {
	if !IsNetworkProtocol(protocol) {
		return nil, fmt.Errorf("protocol %s is not a network protocol", protocol)
	}
	return NewRemoteAddress(protocol, host, port, path)
}

// MustNewLocalActorAddress 创建本地actor地址（失败时panic）
func MustNewLocalActorAddress(path string) *ActorAddress {
	addr, err := NewLocalActorAddress(path)
	if err != nil {
		panic(err)
	}
	return addr
}

// MustNewRemoteActorAddress 创建远程actor地址（失败时panic）
func MustNewRemoteActorAddress(protocol, host, port, path string) *ActorAddress {
	addr, err := NewRemoteActorAddress(protocol, host, port, path)
	if err != nil {
		panic(err)
	}
	return addr
}

// 示例地址格式
// 本地: local:///user/actorA
// 远程: tcp://192.168.1.100:8080/user/actorC
// 系统: system:///deadLetters

// NewLocalAddressWithValidation 创建本地地址（带验证，返回错误）
func NewLocalAddressWithValidation(protocol, path string) (*ActorAddress, error) {
	return NewLocalAddress(protocol, path)
}

// NewRemoteAddressWithValidation 创建远程地址（带验证，返回错误）
func NewRemoteAddressWithValidation(protocol, host, port, path string) (*ActorAddress, error) {
	return NewRemoteAddress(protocol, host, port, path)
}

// MustNewLocalAddress 创建本地地址（如果失败则panic）
func MustNewLocalAddress(protocol, path string) *ActorAddress {
	addr, err := NewLocalAddress(protocol, path)
	if err != nil {
		panic(err)
	}
	return addr
}

// MustNewRemoteAddress 创建远程地址（如果失败则panic）
func MustNewRemoteAddress(protocol, host, port, path string) *ActorAddress {
	addr, err := NewRemoteAddress(protocol, host, port, path)
	if err != nil {
		panic(err)
	}
	return addr
}

// ParseAddress 解析地址字符串
func ParseAddress(addrStr string) (*ActorAddress, error) {
	// 解析URL格式的地址
	u, err := url.Parse(addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address: %w", err)
	}

	// 验证协议
	if err := ValidateProtocol(u.Scheme); err != nil {
		return nil, fmt.Errorf("invalid protocol in address: %w", err)
	}

	addr := &ActorAddress{
		protocol: u.Scheme,
		host:     u.Hostname(),
		port:     u.Port(),
		path:     u.Path,
		address:  addrStr,
	}

	// 对于本地协议，如果主机为空，设置为localhost
	if IsLocalProtocol(u.Scheme) && (addr.host == "" || addr.host == "localhost" || addr.host == "127.0.0.1" || addr.host == "::1") {
		addr.host = "localhost"
		// 重新构建地址字符串
		addr.address = addr.buildAddressString()
	}

	return addr, nil
}

func (a *ActorAddress) String() string {
	return a.address
}

func (a *ActorAddress) Protocol() string {
	return a.protocol
}

func (a *ActorAddress) Host() string {
	return a.host
}

func (a *ActorAddress) Port() string {
	return a.port
}

func (a *ActorAddress) Path() string {
	return a.path
}

func (a *ActorAddress) Equal(other Address) bool {
	if other == nil {
		return false
	}
	return a.String() == other.String()
}

func (a *ActorAddress) IsLocal() bool {
	return IsLocalProtocol(a.protocol)
}

func (a *ActorAddress) IsRemote() bool {
	return IsNetworkProtocol(a.protocol)
}

// buildAddressString 构建地址字符串
func (a *ActorAddress) buildAddressString() string {
	var builder strings.Builder

	builder.WriteString(a.protocol)
	builder.WriteString("://")
	builder.WriteString(a.host)

	if a.port != "" {
		builder.WriteString(":")
		builder.WriteString(a.port)
	}

	if a.path != "" {
		if !strings.HasPrefix(a.path, "/") {
			builder.WriteString("/")
		}
		builder.WriteString(a.path)
	}

	return builder.String()
}

// AddressPool 地址池，用于管理地址
type AddressPool struct {
	addresses map[string]Address
	mu        sync.RWMutex
}

var (
	globalAddressPool = &AddressPool{
		addresses: make(map[string]Address),
	}
)

// GetOrCreateAddress 获取或创建地址
func GetOrCreateAddress(addrStr string) (Address, error) {
	globalAddressPool.mu.RLock()
	if addr, exists := globalAddressPool.addresses[addrStr]; exists {
		globalAddressPool.mu.RUnlock()
		return addr, nil
	}
	globalAddressPool.mu.RUnlock()

	// 创建新地址
	addr, err := ParseAddress(addrStr)
	if err != nil {
		return nil, err
	}

	globalAddressPool.mu.Lock()
	globalAddressPool.addresses[addrStr] = addr
	globalAddressPool.mu.Unlock()

	return addr, nil
}

// RemoveAddress 移除地址
func RemoveAddress(addrStr string) {
	globalAddressPool.mu.Lock()
	delete(globalAddressPool.addresses, addrStr)
	globalAddressPool.mu.Unlock()
}

// ClearAddressPool 清空地址池
func ClearAddressPool() {
	globalAddressPool.mu.Lock()
	globalAddressPool.addresses = make(map[string]Address)
	globalAddressPool.mu.Unlock()
}

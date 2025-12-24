package message

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// AddressType 地址类型
type AddressType int

const (
	// AddressTypeLocal 本地地址
	AddressTypeLocal AddressType = iota
	// AddressTypeRemote 远程地址
	AddressTypeRemote
)

// Protocol 支持的协议类型
type Protocol string

const (
	// ProtocolActor 本地Actor协议
	ProtocolActor Protocol = "actor"
	// ProtocolTCP TCP协议
	ProtocolTCP Protocol = "tcp"
	// ProtocolWebSocket WebSocket协议
	ProtocolWebSocket Protocol = "ws"
	// ProtocolWebSocketSecure 安全的WebSocket协议
	ProtocolWebSocketSecure Protocol = "wss"
	// ProtocolHTTP HTTP协议
	ProtocolHTTP Protocol = "http"
	// ProtocolHTTPS HTTPS协议
	ProtocolHTTPS Protocol = "https"
	// ProtocolGRPC gRPC协议
	ProtocolGRPC Protocol = "grpc"
	// ProtocolInProc 进程内协议
	ProtocolInProc Protocol = "inproc"
	// ProtocolIPC 进程间通信协议
	ProtocolIPC Protocol = "ipc"
)

// 支持的协议列表
var supportedProtocols = map[Protocol]bool{
	ProtocolActor:           true,
	ProtocolTCP:             true,
	ProtocolWebSocket:       true,
	ProtocolWebSocketSecure: true,
	ProtocolHTTP:            true,
	ProtocolHTTPS:           true,
	ProtocolGRPC:            true,
	ProtocolInProc:          true,
	ProtocolIPC:             true,
}

// IsProtocolSupported 检查协议是否受支持
func IsProtocolSupported(protocol string) bool {
	_, supported := supportedProtocols[Protocol(protocol)]
	return supported
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

// ActorAddress Actor地址实现
type ActorAddress struct {
	protocol string
	host     string
	port     string
	path     string
	address  string // 完整地址字符串
	addrType AddressType
}

// NewLocalAddress 创建本地地址
func NewLocalAddress(protocol, path string) (*ActorAddress, error) {
	// 验证协议
	if err := ValidateProtocol(protocol); err != nil {
		return nil, err
	}

	addr := &ActorAddress{
		protocol: protocol,
		host:     "localhost",
		port:     "",
		path:     path,
		addrType: AddressTypeLocal,
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
		addrType: AddressTypeRemote,
	}
	addr.address = addr.buildAddressString()
	return addr, nil
}

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

	// 判断地址类型
	if addr.host == "localhost" || addr.host == "127.0.0.1" || addr.host == "::1" || addr.host == "" {
		addr.addrType = AddressTypeLocal
		// 对于本地地址，如果主机为空，设置为localhost
		if addr.host == "" {
			addr.host = "localhost"
			// 重新构建地址字符串
			addr.address = addr.buildAddressString()
		}
	} else {
		addr.addrType = AddressTypeRemote
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
	return a.addrType == AddressTypeLocal
}

func (a *ActorAddress) IsRemote() bool {
	return a.addrType == AddressTypeRemote
}

// Type 返回地址类型
func (a *ActorAddress) Type() AddressType {
	return a.addrType
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

package mailbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GooLuck/GoServer/framework/actor/message"
)

// MailboxManager 邮箱管理器接口
type MailboxManager interface {
	// CreateMailbox 为指定地址创建邮箱
	CreateMailbox(address message.Address, config Config) (Mailbox, error)
	// GetMailbox 获取指定地址的邮箱
	GetMailbox(address message.Address) (Mailbox, error)
	// GetOrCreateMailbox 获取或创建邮箱
	GetOrCreateMailbox(address message.Address, config Config) (Mailbox, error)
	// RemoveMailbox 移除指定地址的邮箱
	RemoveMailbox(address message.Address) error
	// HasMailbox 检查地址是否有邮箱
	HasMailbox(address message.Address) bool
	// ListMailboxes 列出所有邮箱地址
	ListMailboxes() []message.Address
	// Stats 返回管理器统计信息
	Stats() ManagerStats
	// Close 关闭所有邮箱
	Close() error
}

// ManagerStats 管理器统计信息
type ManagerStats struct {
	// TotalMailboxes 总邮箱数
	TotalMailboxes int
	// ActiveMailboxes 活跃邮箱数
	ActiveMailboxes int
	// TotalMessages 总消息数
	TotalMessages int
	// CreatedTime 创建时间
	CreatedTime time.Time
	// LastActivityTime 最后活动时间
	LastActivityTime time.Time
}

// DefaultMailboxManager 默认邮箱管理器实现
type DefaultMailboxManager struct {
	mailboxes map[string]Mailbox
	addresses map[string]message.Address // 地址字符串到地址对象的映射
	configs   map[string]Config
	stats     ManagerStats
	mu        sync.RWMutex
	closed    bool
}

// NewDefaultMailboxManager 创建新的默认邮箱管理器
func NewDefaultMailboxManager() *DefaultMailboxManager {
	return &DefaultMailboxManager{
		mailboxes: make(map[string]Mailbox),
		addresses: make(map[string]message.Address),
		configs:   make(map[string]Config),
		stats: ManagerStats{
			CreatedTime: time.Now(),
		},
	}
}

// CreateMailbox 创建邮箱
func (m *DefaultMailboxManager) CreateMailbox(address message.Address, config Config) (Mailbox, error) {
	if address == nil {
		return nil, errors.New("address cannot be nil")
	}

	addrStr := address.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, errors.New("mailbox manager is closed")
	}

	if _, exists := m.mailboxes[addrStr]; exists {
		return nil, fmt.Errorf("mailbox already exists for address: %s", addrStr)
	}

	mailbox := New(config)
	m.mailboxes[addrStr] = mailbox
	m.addresses[addrStr] = address
	m.configs[addrStr] = config

	m.stats.TotalMailboxes++
	m.stats.ActiveMailboxes++
	m.stats.LastActivityTime = time.Now()

	return mailbox, nil
}

// GetMailbox 获取邮箱
func (m *DefaultMailboxManager) GetMailbox(address message.Address) (Mailbox, error) {
	if address == nil {
		return nil, errors.New("address cannot be nil")
	}

	addrStr := address.String()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, errors.New("mailbox manager is closed")
	}

	mailbox, exists := m.mailboxes[addrStr]
	if !exists {
		return nil, fmt.Errorf("mailbox not found for address: %s", addrStr)
	}

	return mailbox, nil
}

// GetOrCreateMailbox 获取或创建邮箱
func (m *DefaultMailboxManager) GetOrCreateMailbox(address message.Address, config Config) (Mailbox, error) {
	if address == nil {
		return nil, errors.New("address cannot be nil")
	}

	addrStr := address.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, errors.New("mailbox manager is closed")
	}

	// 检查是否已存在
	if mailbox, exists := m.mailboxes[addrStr]; exists {
		return mailbox, nil
	}

	// 创建新邮箱
	mailbox := New(config)
	m.mailboxes[addrStr] = mailbox
	m.addresses[addrStr] = address
	m.configs[addrStr] = config

	m.stats.TotalMailboxes++
	m.stats.ActiveMailboxes++
	m.stats.LastActivityTime = time.Now()

	return mailbox, nil
}

// RemoveMailbox 移除邮箱
func (m *DefaultMailboxManager) RemoveMailbox(address message.Address) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	addrStr := address.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("mailbox manager is closed")
	}

	mailbox, exists := m.mailboxes[addrStr]
	if !exists {
		return fmt.Errorf("mailbox not found for address: %s", addrStr)
	}

	// 关闭邮箱
	if err := mailbox.Close(); err != nil {
		// 记录错误但继续移除
		fmt.Printf("warning: failed to close mailbox %s: %v\n", addrStr, err)
	}

	delete(m.mailboxes, addrStr)
	delete(m.addresses, addrStr)
	delete(m.configs, addrStr)

	m.stats.ActiveMailboxes--
	m.stats.LastActivityTime = time.Now()

	return nil
}

// HasMailbox 检查是否有邮箱
func (m *DefaultMailboxManager) HasMailbox(address message.Address) bool {
	if address == nil {
		return false
	}

	addrStr := address.String()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return false
	}

	_, exists := m.mailboxes[addrStr]
	return exists
}

// ListMailboxes 列出所有邮箱地址
func (m *DefaultMailboxManager) ListMailboxes() []message.Address {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return []message.Address{}
	}

	addresses := make([]message.Address, 0, len(m.addresses))
	for _, addr := range m.addresses {
		addresses = append(addresses, addr)
	}
	return addresses
}

// Stats 返回统计信息
func (m *DefaultMailboxManager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats

	// 计算总消息数
	totalMessages := 0
	for addrStr, mailbox := range m.mailboxes {
		totalMessages += mailbox.Size()
		_ = addrStr // 避免未使用变量警告
	}
	stats.TotalMessages = totalMessages

	return stats
}

// Close 关闭所有邮箱
func (m *DefaultMailboxManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	var lastErr error

	// 关闭所有邮箱
	for addrStr, mailbox := range m.mailboxes {
		if err := mailbox.Close(); err != nil {
			lastErr = err
			fmt.Printf("warning: failed to close mailbox %s: %v\n", addrStr, err)
		}
	}

	// 清空映射
	m.mailboxes = make(map[string]Mailbox)
	m.addresses = make(map[string]message.Address)
	m.configs = make(map[string]Config)
	m.stats.ActiveMailboxes = 0
	m.stats.LastActivityTime = time.Now()

	return lastErr
}

// DeliveryService 消息投递服务
type DeliveryService struct {
	manager MailboxManager
	router  message.Router
}

// NewDeliveryService 创建新的投递服务
func NewDeliveryService(manager MailboxManager, router message.Router) *DeliveryService {
	return &DeliveryService{
		manager: manager,
		router:  router,
	}
}

// Deliver 投递消息到指定地址
func (ds *DeliveryService) Deliver(ctx context.Context, msg message.Message) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}

	receiver := msg.Receiver()
	if receiver == nil {
		return errors.New("message receiver cannot be nil")
	}

	// 获取或创建邮箱
	mailbox, err := ds.manager.GetOrCreateMailbox(receiver, DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to get mailbox for %s: %w", receiver.String(), err)
	}

	// 创建信封
	envelope := message.NewEnvelope(msg)

	// 投递到邮箱
	return mailbox.Push(ctx, envelope)
}

// DeliverWithRouting 使用路由投递消息
func (ds *DeliveryService) DeliverWithRouting(ctx context.Context, msg message.Message) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}

	// 使用路由器获取目标地址
	targets, err := ds.router.Route(ctx, msg)
	if err != nil {
		return fmt.Errorf("routing failed: %w", err)
	}

	// 投递到所有目标
	var lastErr error
	for _, target := range targets {
		// 克隆消息并设置接收者
		clonedMsg := msg.Clone()
		clonedMsg.SetReceiver(target)

		if err := ds.Deliver(ctx, clonedMsg); err != nil {
			lastErr = err
			// 继续投递到其他目标
		}
	}

	return lastErr
}

// DefaultConfig 返回默认邮箱配置
func DefaultConfig() Config {
	return Config{
		Capacity:        1000, // 默认容量
		DropPolicy:      DropPolicyReject,
		PriorityEnabled: true,
		TTLEnabled:      true,
		DeadLetterQueue: &DeadLetterQueueConfig{
			Enabled:       true,
			Capacity:      100,
			RetentionTime: 24 * time.Hour,
		},
	}
}

// 全局默认邮箱管理器
var (
	defaultManager     MailboxManager
	defaultManagerOnce sync.Once
)

// GetDefaultManager 获取默认邮箱管理器
func GetDefaultManager() MailboxManager {
	defaultManagerOnce.Do(func() {
		defaultManager = NewDefaultMailboxManager()
	})
	return defaultManager
}

// SetDefaultManager 设置默认邮箱管理器
func SetDefaultManager(manager MailboxManager) {
	defaultManager = manager
}

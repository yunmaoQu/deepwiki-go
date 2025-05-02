package rag

import (
	"fmt"
	"sync"

	"github.com/deepwiki-go/internal/config"
)

// RAGManager 管理多个 RAG 提供者的实例
type RAGManager struct {
	registry *ProviderRegistry
	config   *config.Config
	mu       sync.RWMutex
}

// NewRAGManager 创建一个新的 RAG 管理器
func NewRAGManager(cfg *config.Config) *RAGManager {
	return &RAGManager{
		registry: NewProviderRegistry(),
		config:   cfg,
	}
}

// RegisterProvider 注册一个新的 RAG 提供者
func (m *RAGManager) RegisterProvider(provider RAGProvider) error {
	return m.registry.Register(provider)
}

// SetActiveProvider 设置活动的 RAG 提供者
func (m *RAGManager) SetActiveProvider(name string) error {
	return m.registry.SetActive(name)
}

// GetActiveProvider 获取当前活动的 RAG 提供者
func (m *RAGManager) GetActiveProvider() (RAGProvider, error) {
	return m.registry.GetActive()
}

// ListProviders 列出所有注册的提供者
func (m *RAGManager) ListProviders() []string {
	return m.registry.ListProviders()
}

// UnregisterProvider 注销一个 RAG 提供者
func (m *RAGManager) UnregisterProvider(name string) error {
	return m.registry.Unregister(name)
}

// Close 关闭所有提供者并清理资源
func (m *RAGManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for _, name := range m.ListProviders() {
		if err := m.UnregisterProvider(name); err != nil {
			errs = append(errs, fmt.Errorf("关闭提供者 %s 失败: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭 RAG 管理器时发生错误: %v", errs)
	}
	return nil
}

package rag

import (
	"fmt"
	"sync"

	"github.com/deepwiki-go/internal/models"
)

// RAGProvider 定义了 RAG 提供者需要实现的接口
type RAGProvider interface {
	// Name 返回提供者的唯一名称
	Name() string
	// Initialize 初始化提供者
	Initialize() error
	// PrepareRetriever 为仓库准备检索器
	PrepareRetriever(repoURLOrPath string, accessToken string) error
	// RetrieveDocuments 检索与查询相关的文档
	RetrieveDocuments(query string) ([]models.Document, error)
	// GenerateStreamingResponse 生成流式响应
	GenerateStreamingResponse(prompt string) (chan string, error)
	// IndexDocument 索引文档
	IndexDocument(doc *models.Document) error
	// GetDocument 获取文档
	GetDocument(id string) (*models.Document, error)
	// DeleteDocument 删除文档
	DeleteDocument(id string) error
	// Close 清理资源
	Close() error
}

// ProviderRegistry 管理 RAG 提供者的注册表
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]RAGProvider
	active    string
}

// NewProviderRegistry 创建一个新的提供者注册表
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]RAGProvider),
	}
}

// Register 注册一个新的 RAG 提供者
func (r *ProviderRegistry) Register(provider RAGProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("提供者 %s 已经注册", name)
	}

	if err := provider.Initialize(); err != nil {
		return fmt.Errorf("初始化提供者 %s 失败: %v", name, err)
	}

	r.providers[name] = provider
	// 如果这是第一个注册的提供者，将其设置为活动提供者
	if r.active == "" {
		r.active = name
	}
	return nil
}

// SetActive 设置活动的 RAG 提供者
func (r *ProviderRegistry) SetActive(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.providers[name]
	if !exists {
		return fmt.Errorf("提供者 %s 未注册", name)
	}

	r.active = name
	return nil
}

// GetActive 获取当前活动的 RAG 提供者
func (r *ProviderRegistry) GetActive() (RAGProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.active == "" {
		return nil, fmt.Errorf("没有活动的 RAG 提供者")
	}

	provider, exists := r.providers[r.active]
	if !exists {
		return nil, fmt.Errorf("活动提供者 %s 不存在", r.active)
	}

	return provider, nil
}

// ListProviders 列出所有注册的提供者
func (r *ProviderRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Unregister 注销一个 RAG 提供者
func (r *ProviderRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[name]
	if !exists {
		return fmt.Errorf("提供者 %s 未注册", name)
	}

	// 如果要注销的是当前活动的提供者，需要先切换到其他提供者
	if r.active == name {
		// 寻找另一个可用的提供者
		for otherName := range r.providers {
			if otherName != name {
				r.active = otherName
				break
			}
		}
		// 如果没有其他提供者，清空活动提供者
		if r.active == name {
			r.active = ""
		}
	}

	// 清理提供者资源
	if err := provider.Close(); err != nil {
		return fmt.Errorf("关闭提供者 %s 失败: %v", name, err)
	}

	delete(r.providers, name)
	return nil
}

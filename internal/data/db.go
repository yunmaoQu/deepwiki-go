package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/deepwiki-go/internal/models"
)

// DBManager 管理文档数据的持久化
type DBManager struct {
	mu        sync.RWMutex
	dbPath    string
	documents map[string]*models.Document
}

// NewDBManager 创建新的数据库管理器
func NewDBManager(dbPath string) (*DBManager, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %v", err)
	}

	manager := &DBManager{
		dbPath:    dbPath,
		documents: make(map[string]*models.Document),
	}

	// 加载现有数据
	if err := manager.load(); err != nil {
		return nil, err
	}

	return manager, nil
}

// LoadDocuments 加载所有文档
func (m *DBManager) LoadDocuments() map[string]*models.Document {
	m.mu.RLock()
	defer m.mu.RUnlock()

	docs := make(map[string]*models.Document, len(m.documents))
	for k, v := range m.documents {
		docs[k] = v
	}
	return docs
}

// SaveDocument 保存文档
func (m *DBManager) SaveDocument(doc *models.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.documents[doc.ID] = doc
	return m.save()
}

// DeleteDocument 删除文档
func (m *DBManager) DeleteDocument(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.documents, id)
	return m.save()
}

// load 从文件加载数据
func (m *DBManager) load() error {
	data, err := os.ReadFile(m.dbPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取数据库文件失败: %v", err)
	}

	return json.Unmarshal(data, &m.documents)
}

// save 保存数据到文件
func (m *DBManager) save() error {
	data, err := json.MarshalIndent(m.documents, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}

	return os.WriteFile(m.dbPath, data, 0644)
}

// internal/data/storage.go
package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/deepwiki-go/internal/models"
	"github.com/deepwiki-go/pkg/utils"
)

// VectorStore 提供向量存储和检索
type VectorStore struct {
	basePath  string
	documents []models.Document
}

// NewVectorStore 创建新的向量存储
func NewVectorStore() *VectorStore {
	basePath := filepath.Join(utils.GetDefaultRootPath(), "vectorstore")
	// 确保目录存在
	os.MkdirAll(basePath, 0755)

	return &VectorStore{
		basePath:  basePath,
		documents: []models.Document{},
	}
}

// SaveDocuments 保存文档到向量存储
func (v *VectorStore) SaveDocuments(docs []models.Document, repoID string) error {
	if len(docs) == 0 {
		return nil
	}

	// 创建仓库存储目录
	repoPath := filepath.Join(v.basePath, repoID)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %v", err)
	}

	// 保存文档
	docsFile := filepath.Join(repoPath, "documents.json")
	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化文档失败: %v", err)
	}

	if err := os.WriteFile(docsFile, data, 0644); err != nil {
		return fmt.Errorf("保存文档失败: %v", err)
	}

	// 更新内存中的文档
	v.documents = append(v.documents, docs...)

	return nil
}

// LoadDocuments 从存储加载文档
func (v *VectorStore) LoadDocuments(repoID string) ([]models.Document, error) {
	docsFile := filepath.Join(v.basePath, repoID, "documents.json")

	// 检查文件是否存在
	if _, err := os.Stat(docsFile); os.IsNotExist(err) {
		return nil, nil // 文件不存在，返回空列表
	}

	// 读取文件
	data, err := os.ReadFile(docsFile)
	if err != nil {
		return nil, fmt.Errorf("读取文档文件失败: %v", err)
	}

	// 解析文档
	var docs []models.Document
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil, fmt.Errorf("解析文档失败: %v", err)
	}

	// 更新内存中的文档
	v.documents = append(v.documents, docs...)

	return docs, nil
}

// SearchSimilar 使用向量相似度搜索相似文档
func (v *VectorStore) SearchSimilar(queryVector []float32, topK int) ([]models.Document, error) {
	if len(v.documents) == 0 {
		return nil, errors.New("没有可用文档")
	}

	if len(queryVector) == 0 {
		return nil, errors.New("查询向量不能为空")
	}

	// 计算所有文档与查询向量的相似度
	type docWithScore struct {
		doc   models.Document
		score float32
	}

	var scored []docWithScore

	for _, doc := range v.documents {
		if len(doc.Vector) == 0 {
			continue
		}

		// 计算余弦相似度
		score := cosineSimilarity(queryVector, doc.Vector)
		scored = append(scored, docWithScore{doc, score})
	}

	// 按相似度排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 限制结果数量
	resultCount := topK
	if resultCount > len(scored) {
		resultCount = len(scored)
	}

	// 提取排序后的文档
	result := make([]models.Document, resultCount)
	for i := 0; i < resultCount; i++ {
		result[i] = scored[i].doc
	}

	return result, nil
}

// DeleteDocuments 删除仓库的所有文档
func (v *VectorStore) DeleteDocuments(repoID string) error {
	repoPath := filepath.Join(v.basePath, repoID)

	// 检查目录是否存在
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil // 目录不存在，无需删除
	}

	// 删除目录及其内容
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("删除文档失败: %v", err)
	}

	// 更新内存中的文档
	filteredDocs := []models.Document{}
	for _, doc := range v.documents {
		// 假设元数据中有仓库ID
		if meta, ok := doc.MetaData["repo_id"].(string); ok && meta != repoID {
			filteredDocs = append(filteredDocs, doc)
		}
	}
	v.documents = filteredDocs

	return nil
}

// 余弦相似度计算
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(sqrt(float64(normA))) * float32(sqrt(float64(normB))))
}

// 平方根辅助函数（因为Go没有float32的sqrt）
func sqrt(x float64) float64 {
	return float64(math.Sqrt(x))
}

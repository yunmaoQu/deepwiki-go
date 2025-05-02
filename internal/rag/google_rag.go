package rag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/deepwiki-go/internal/config"
	"github.com/deepwiki-go/internal/data"
	"github.com/deepwiki-go/internal/models"

	"cloud.google.com/go/vertexai/genai"
)

// GoogleRAG 实现基于 Google Vertex AI 的检索增强生成
type GoogleRAG struct {
	Memory       *Memory
	Config       *config.Config
	DbManager    *data.DatabaseManager
	RepoURL      string
	Documents    []models.Document
	GoogleClient *genai.Client
}

// NewGoogleRAG 创建一个新的 Google RAG 实例
func NewGoogleRAG(cfg *config.Config) *GoogleRAG {
	dbManager, err := data.NewDatabaseManager(cfg)
	if err != nil {
		panic(fmt.Sprintf("初始化DatabaseManager失败: %v", err))
	}
	return &GoogleRAG{
		Memory:    NewMemory(),
		Config:    cfg,
		DbManager: dbManager,
	}
}

// Name 返回提供者的唯一名称
func (r *GoogleRAG) Name() string {
	return "google"
}

// Initialize 初始化提供者
func (r *GoogleRAG) Initialize() error {
	// 初始化 Google 生成式 AI 客户端
	if r.Config.Google.APIKey == "" || r.Config.Google.ProjectID == "" {
		return fmt.Errorf("缺少必要的 Google AI 配置")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, r.Config.Google.ProjectID, r.Config.Google.Location)
	if err != nil {
		return fmt.Errorf("初始化 Google AI 客户端失败: %v", err)
	}
	r.GoogleClient = client
	return nil
}

// PrepareRetriever 为仓库准备检索器
func (r *GoogleRAG) PrepareRetriever(repoURLOrPath string, accessToken string) error {
	r.RepoURL = repoURLOrPath
	if err := r.DbManager.PrepareDatabase(repoURLOrPath, accessToken); err != nil {
		return err
	}
	// 这里可以根据需要加载文档列表（如有必要）
	return nil
}

// IndexDocument 索引文档
func (r *GoogleRAG) IndexDocument(doc *models.Document) error {
	// 将文档添加到数据库
	if err := r.DbManager.AddDocument(doc); err != nil {
		return fmt.Errorf("添加文档到数据库失败: %v", err)
	}

	// 更新内存中的文档列表
	r.Documents = append(r.Documents, *doc)
	return nil
}

// GetDocument 获取文档
func (r *GoogleRAG) GetDocument(id string) (*models.Document, error) {
	// 从数据库中获取文档
	doc, err := r.DbManager.GetDocument(id)
	if err != nil {
		return nil, fmt.Errorf("从数据库获取文档失败: %v", err)
	}
	return doc, nil
}

// DeleteDocument 删除文档
func (r *GoogleRAG) DeleteDocument(id string) error {
	// 从数据库中删除文档
	if err := r.DbManager.DeleteDocument(id); err != nil {
		return fmt.Errorf("从数据库删除文档失败: %v", err)
	}

	// 更新内存中的文档列表
	for i, doc := range r.Documents {
		if doc.ID == id {
			// 从切片中删除该文档
			r.Documents = append(r.Documents[:i], r.Documents[i+1:]...)
			break
		}
	}
	return nil
}

// RetrieveDocuments 检索与查询相关的文档
func (r *GoogleRAG) RetrieveDocuments(query string) ([]models.Document, error) {
	if len(r.Documents) == 0 {
		return nil, errors.New("没有可用于检索的文档")
	}

	// 使用向量搜索检索相关文档
	relevantDocs, err := r.DbManager.SearchDocuments(query, r.Config.Retriever.TopK)
	if err != nil {
		return nil, err
	}

	// 使用上下文历史记录增强检索结果
	if relevantDocs, err = r.enhanceRetrievalWithMemory(query, relevantDocs); err != nil {
		log.Printf("增强检索结果时出错: %v", err)
		// 继续使用原始结果
	}

	return relevantDocs, nil
}

// enhanceRetrievalWithMemory 使用上下文历史记录增强检索结果
func (r *GoogleRAG) enhanceRetrievalWithMemory(query string, docs []models.Document) ([]models.Document, error) {
	// 从记忆中获取相关上下文
	context := r.Memory.GetRelevantContext(query)
	if context == "" {
		return docs, nil // 没有相关上下文，使用原始结果
	}

	// 使用上下文重新排序文档
	enhancedDocs := r.reRankDocumentsWithContext(docs, context)

	return enhancedDocs, nil
}

// reRankDocumentsWithContext 使用上下文重新排序文档
func (r *GoogleRAG) reRankDocumentsWithContext(docs []models.Document, context string) []models.Document {
	// 创建一个文档副本进行排序
	result := make([]models.Document, len(docs))
	copy(result, docs)

	// 使用上下文相关性对文档进行排序
	// 这里使用简单的启发式方法：检查文档内容是否包含上下文中的关键词
	contextKeywords := extractKeywords(context)

	// 计算每个文档的上下文相关性分数
	type scoredDoc struct {
		doc   models.Document
		score float64
	}

	scoredDocs := make([]scoredDoc, len(result))
	for i, doc := range result {
		scoredDocs[i] = scoredDoc{
			doc:   doc,
			score: calculateContextScore(doc, contextKeywords),
		}
	}

	// 根据分数排序
	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	// 转换回文档列表
	for i, sd := range scoredDocs {
		result[i] = sd.doc
	}

	return result
}

// calculateContextScore 计算文档与上下文的相关性分数
func calculateContextScore(doc models.Document, contextKeywords []string) float64 {
	score := 0.0

	// 简单的关键词匹配
	for _, keyword := range contextKeywords {
		if strings.Contains(strings.ToLower(doc.Text), strings.ToLower(keyword)) {
			score += 1.0
		}
		if strings.Contains(strings.ToLower(doc.Title), strings.ToLower(keyword)) {
			score += 2.0 // 标题匹配权重更高
		}
	}

	// 如果文档被标记为重要，增加其分数
	if doc.Importance == "high" {
		score *= 1.5
	}

	return score
}

// extractKeywords 从文本中提取关键词
func extractKeywords(text string) []string {
	// 移除常见停用词并分割文本
	stopWords := map[string]bool{
		"的": true, "了": true, "和": true, "是": true, "在": true,
		"这": true, "有": true, "我": true, "们": true, "为": true,
	}

	words := strings.Fields(text)
	var keywords []string

	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ",.!?;:\"'()[]{}"))
		if word != "" && !stopWords[word] && len(word) > 1 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// GenerateStreamingResponse 生成流式响应
func (r *GoogleRAG) GenerateStreamingResponse(prompt string) (chan string, error) {
	if r.GoogleClient == nil {
		return nil, errors.New("Google AI 客户端未初始化")
	}

	// 创建一个通道用于流式传输响应
	responseCh := make(chan string)

	// 在 goroutine 中处理生成
	go func() {
		defer close(responseCh)

		ctx := context.Background()

		// 设置生成参数
		temperature := float32(0.7)
		topP := float32(0.8)
		topK := int32(40)
		maxTokens := int32(2048)

		// 创建生成请求
		model := r.GoogleClient.GenerativeModel("gemini-2.5-pro")
		model.Temperature = &temperature
		model.TopP = &topP
		model.TopK = &topK
		model.MaxOutputTokens = &maxTokens

		iter := model.GenerateContentStream(ctx, genai.Text(prompt))

		// 流式传输响应
		for {
			resp, err := iter.Next()
			if err != nil {
				if err == io.EOF {
					return
				}
				if err == context.Canceled {
					return
				}
				responseCh <- fmt.Sprintf("\n错误: %v", err)
				return
			}

			// 发送响应块
			for _, part := range resp.Candidates[0].Content.Parts {
				if text, ok := part.(genai.Text); ok {
					responseCh <- string(text)
				}
			}
		}
	}()

	return responseCh, nil
}

// Close 清理资源
func (r *GoogleRAG) Close() error {
	if r.GoogleClient != nil {
		return r.GoogleClient.Close()
	}
	return nil
}

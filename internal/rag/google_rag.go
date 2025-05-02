package rag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

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
	return &GoogleRAG{
		Memory:    NewMemory(),
		Config:    cfg,
		DbManager: data.NewDatabaseManager(),
	}
}

// Name 返回提供者的唯一名称
func (r *GoogleRAG) Name() string {
	return "google"
}

// Initialize 初始化提供者
func (r *GoogleRAG) Initialize() error {
	// 初始化 Google 生成式 AI 客户端
	if r.Config.GoogleAPIKey == "" || r.Config.ProjectID == "" {
		return fmt.Errorf("缺少必要的 Google AI 配置")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, r.Config.ProjectID, r.Config.Location)
	if err != nil {
		return fmt.Errorf("初始化 Google AI 客户端失败: %v", err)
	}
	r.GoogleClient = client
	return nil
}

// PrepareRetriever 为仓库准备检索器
func (r *GoogleRAG) PrepareRetriever(repoURLOrPath string, accessToken string) error {
	r.RepoURL = repoURLOrPath
	var err error
	r.Documents, err = r.DbManager.PrepareDatabase(repoURLOrPath, accessToken)
	if err != nil {
		return err
	}

	log.Printf("已加载 %d 个文档用于检索", len(r.Documents))
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

	return relevantDocs, nil
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
		model := r.GoogleClient.GenerativeModel("gemini-1.0-pro")
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

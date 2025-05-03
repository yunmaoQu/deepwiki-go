package rag

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/deepwiki-go/internal/config"
	"github.com/deepwiki-go/internal/data"
	"github.com/deepwiki-go/internal/models"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIRAG 实现基于 OpenAI 的检索增强生成
type OpenAIRAG struct {
	Memory       *Memory
	Config       *config.Config
	DbManager    *data.DatabaseManager
	RepoURL      string
	OpenAIClient *openai.Client
}

// NewOpenAIRAG 创建一个新的 OpenAI RAG 实例
func NewOpenAIRAG(cfg *config.Config) (*OpenAIRAG, error) {
	dbManager, err := data.NewDatabaseManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create DatabaseManager: %w", err)
	}
	return &OpenAIRAG{
		Memory:    NewMemory(),
		Config:    cfg,
		DbManager: dbManager,
	}, nil
}

// Name 返回提供者的唯一名称
func (r *OpenAIRAG) Name() string {
	return "openai"
}

// Initialize 初始化提供者
func (r *OpenAIRAG) Initialize() error {
	if r.Config.OpenAIAPIKey == "" {
		return fmt.Errorf("缺少必要的 OpenAI API Key")
	}
	r.OpenAIClient = openai.NewClient(r.Config.OpenAIAPIKey)
	return nil
}

// PrepareRetriever 为仓库准备检索器
func (r *OpenAIRAG) PrepareRetriever(repoURLOrPath string, accessToken string) error {
	r.RepoURL = repoURLOrPath
	err := r.DbManager.PrepareDatabase(repoURLOrPath, accessToken)
	if err != nil {
		return fmt.Errorf("failed to prepare database: %w", err)
	}

	log.Println("Database prepared for retrieval")
	return nil
}

// RetrieveDocuments 检索与查询相关的文档
func (r *OpenAIRAG) RetrieveDocuments(query string) ([]models.Document, error) {
	// 使用向量搜索检索相关文档
	relevantDocs, err := r.DbManager.SearchDocuments(query, r.Config.Retriever.TopK)
	if err != nil {
		return nil, err
	}

	return relevantDocs, nil
}

// GenerateStreamingResponse 生成流式响应
func (r *OpenAIRAG) GenerateStreamingResponse(prompt string) (chan string, error) {
	if r.OpenAIClient == nil {
		return nil, errors.New("OpenAI 客户端未初始化")
	}
	responseCh := make(chan string)
	go func() {
		defer close(responseCh)
		req := openai.ChatCompletionRequest{
			Model: openai.O4Mini2020416,
			Messages: []openai.ChatCompletionMessage{{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			}},
			Stream: true,
		}
		stream, err := r.OpenAIClient.CreateChatCompletionStream(context.Background(), req)
		if err != nil {
			responseCh <- "请求发送失败: " + err.Error()
			return
		}
		defer stream.Close()
		for {
			resp, err := stream.Recv()
			if err != nil {
				break
			}
			if len(resp.Choices) > 0 {
				responseCh <- resp.Choices[0].Delta.Content
			}
		}
	}()
	return responseCh, nil
}

// Close 清理资源
func (r *OpenAIRAG) Close() error {
	r.OpenAIClient = nil
	return nil
}

// IndexDocument 索引文档
func (r *OpenAIRAG) IndexDocument(doc *models.Document) error {
	if r.DbManager == nil {
		return errors.New("数据库管理器未初始化")
	}
	return r.DbManager.AddDocument(doc)
}

// GetDocument 获取文档
func (r *OpenAIRAG) GetDocument(id string) (*models.Document, error) {
	if r.DbManager == nil {
		return nil, errors.New("数据库管理器未初始化")
	}
	return r.DbManager.GetDocument(id)
}

// DeleteDocument 删除文档
func (r *OpenAIRAG) DeleteDocument(id string) error {
	if r.DbManager == nil {
		return errors.New("数据库管理器未初始化")
	}
	return r.DbManager.DeleteDocument(id)
}

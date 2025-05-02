// internal/rag/rag.go
package rag

import (
        "context"
        "errors"
        "fmt"
        "log"
        
        "github.com/deepwiki-go/internal/config"
        "github.com/deepwiki-go/internal/data"
        "github.com/deepwiki-go/internal/models"
        
        "cloud.google.com/go/vertexai/genai"
)

// RAG 实现检索增强生成
type RAG struct {
        Memory       *Memory
        Config       *config.Config
        DbManager    *data.DatabaseManager
        RepoURL      string
        Documents    []models.Document
        GoogleClient *genai.Client
}

// NewRAG 创建一个新的 RAG 实例
func NewRAG(cfg *config.Config) *RAG {
        r := &RAG{
                Memory:    NewMemory(),
                Config:    cfg,
                DbManager: data.NewDatabaseManager(),
        }
        
        // 初始化 Google 生成式 AI 客户端
        if cfg.GoogleAPIKey != "" {
                ctx := context.Background()
                client, err := genai.NewClient(ctx, cfg.GoogleAPIKey)
                if err != nil {
                        log.Printf("警告: 无法初始化 Google AI 客户端: %v", err)
                } else {
                        r.GoogleClient = client
                }
        }
        
        return r
}

// PrepareRetriever 为仓库准备检索器
func (r *RAG) PrepareRetriever(repoURLOrPath string, accessToken string) error {
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
func (r *RAG) RetrieveDocuments(query string) ([]models.Document, error) {
        if len(r.Documents) == 0 {
                return nil, errors.New("没有可用于检索的文档")
        }
        
        // 使用向量搜索检索相关文档
        // 这里是简化实现，实际实现需要向量嵌入和相似度搜索
        relevantDocs, err := r.DbManager.SearchDocuments(query, r.Config.Retriever.TopK)
        if err != nil {
                return nil, err
        }
        
        return relevantDocs, nil
}

// GenerateStreamingResponse 生成流式响应
func (r *RAG) GenerateStreamingResponse(prompt string) (chan string, error) {
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
                genConfig := &genai.GenerationConfig{
                        Temperature:    0.7,
                        TopP:           0.8,
                        TopK:           40,
                        MaxOutputTokens: 2048,
                }
                
                // 创建生成请求
                resp, err := r.GoogleClient.GenerateContentStream(ctx, genConfig, prompt)
                if err != nil {
                        responseCh <- fmt.Sprintf("\n错误: %v", err)
                        return
                }
                
                // 流式传输响应
                for {
                        resp, err := resp.Next()
                        if err != nil {
                                if err == context.Canceled {
                                        return
                                }
                                responseCh <- fmt.Sprintf("\n错误: %v", err)
                                return
                        }
                        
                        if resp == nil || len(resp.Candidates) == 0 {
                                break
                        }
                        
                        // 发送响应块
                        for _, part := range resp.Candidates[0].Content.Parts {
                                if text, ok := part.(string); ok {
                                        responseCh <- text
                                }
                        }
                }
                
                // 将响应添加到对话历史
                // 注意: 在实际实现中，您需要收集完整的响应
        }()
        
        return responseCh, nil
}